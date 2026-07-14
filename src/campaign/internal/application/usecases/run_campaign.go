package usecases

import (
	"context"
	"fmt"
	"log"
	"time"

	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// RunCampaign execute une campagne planifiee.
//
// Flux :
//  1. Charger la campagne.
//  2. UpdateStatusAtomic(scheduled → running) — anti-double-run.
//     Si 0 ligne affectee, un autre worker a deja pris la campagne : on sort sans erreur.
//  3. Charger les destinataires pending.
//  4. Creer un CampaignRun.
//  5. Pour chaque destinataire pending :
//     a. Appeler MessagingClient.SendMessage (idempotent par destinataire).
//     b. Marquer comme sent + message_id, ou failed si erreur.
//     c. Incrementer les compteurs du run.
//     d. Rate limiting : si RateLimit > 0 (messages/min), dormir l'intervalle.
//  6. Marquer la campagne completed (ou failed si tous ont echoue).
//  7. Finaliser le run (finished_at).
//
// Rate limiting (token bucket simplifie) :
//   - RateLimit = 0 → illimite (aucune pause entre messages).
//   - RateLimit = N → N messages/minute → intervalle = 60s/N entre messages.
//   - Implemente par time.Sleep — adapte pour un service standalone Go stdlib.
//
// Anti-double-run :
//   - UpdateStatusAtomic utilise UPDATE ... WHERE id=... AND status='scheduled'.
//   - Si rowsAffected == 0, la campagne est deja traitee par un autre goroutine/pod.
//   - Le worker courant sort silencieusement (nil).
type RunCampaign struct {
	Campaigns  output.CampaignRepository
	Recipients output.RecipientRepository
	Runs       output.RunRepository
	Messaging  output.MessagingClient
}

// Execute execute la campagne identifiee par campaignID.
func (uc *RunCampaign) Execute(ctx context.Context, campaignID string) error {
	// 1. Charger la campagne.
	campaign, err := uc.Campaigns.Get(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("run_campaign: charger campagne %s: %w", campaignID, err)
	}

	// 2. Transition atomique scheduled → running (anti-double-run).
	// Si rowsAffected == 0, un autre worker a deja pris la campagne.
	rows, err := uc.Campaigns.UpdateStatusAtomic(ctx, campaignID, domain.StatusScheduled, domain.StatusRunning)
	if err != nil {
		return fmt.Errorf("run_campaign: UpdateStatusAtomic %s: %w", campaignID, err)
	}
	if rows == 0 {
		log.Printf("run_campaign: campagne %s deja prise par un autre worker", campaignID)
		return nil
	}
	campaign.Status = domain.StatusRunning

	// 3. Charger les destinataires pending.
	recipients, err := uc.Recipients.ListPendingByCampaign(ctx, campaignID)
	if err != nil {
		uc.tryFail(ctx, campaignID)
		return fmt.Errorf("run_campaign: charger destinataires %s: %w", campaignID, err)
	}

	// 4. Creer le run.
	run := domain.NewCampaignRun(campaignID, len(recipients))
	runID, err := uc.Runs.Create(ctx, run)
	if err != nil {
		uc.tryFail(ctx, campaignID)
		return fmt.Errorf("run_campaign: creer run %s: %w", campaignID, err)
	}

	// 5. Calculer l'intervalle de rate limiting (token bucket par run).
	var interval time.Duration
	if campaign.RateLimit > 0 {
		// RateLimit messages/min → intervalle entre messages.
		interval = time.Minute / time.Duration(campaign.RateLimit)
	}

	// 6. Envoyer les messages un par un.
	failedCount := 0
	for i, r := range recipients {
		// Rate limiting : attendre avant chaque message sauf le premier.
		if i > 0 && interval > 0 {
			select {
			case <-ctx.Done():
				log.Printf("run_campaign: contexte annule durant l'envoi (campagne %s)", campaignID)
				_ = uc.Runs.Finish(ctx, runID)
				return nil
			case <-time.After(interval):
			}
		}

		messageID, sendErr := uc.Messaging.SendMessage(
			ctx,
			campaign.WorkspaceID,
			r.Recipient,
			campaign.MessageBody,
			campaign.ChannelStrategy,
		)
		if sendErr != nil {
			log.Printf("run_campaign: erreur envoi destinataire %s (campagne %s): %v",
				r.Recipient, campaignID, sendErr)
			if markErr := uc.Recipients.MarkFailed(ctx, r.ID); markErr != nil {
				log.Printf("run_campaign: MarkFailed %d: %v", r.ID, markErr)
			}
			if incErr := uc.Runs.IncrementFailed(ctx, runID); incErr != nil {
				log.Printf("run_campaign: IncrementFailed run %d: %v", runID, incErr)
			}
			failedCount++
			continue
		}

		if markErr := uc.Recipients.MarkSent(ctx, r.ID, messageID); markErr != nil {
			log.Printf("run_campaign: MarkSent %d: %v", r.ID, markErr)
		}
		if incErr := uc.Runs.IncrementSent(ctx, runID); incErr != nil {
			log.Printf("run_campaign: IncrementSent run %d: %v", runID, incErr)
		}
	}

	// 7. Finaliser la campagne.
	if len(recipients) > 0 && failedCount == len(recipients) {
		// Tous les envois ont echoue.
		_, _ = uc.Campaigns.UpdateStatusAtomic(ctx, campaignID, domain.StatusRunning, domain.StatusFailed)
	} else {
		_, _ = uc.Campaigns.UpdateStatusAtomic(ctx, campaignID, domain.StatusRunning, domain.StatusCompleted)
	}

	// 8. Finaliser le run.
	if finishErr := uc.Runs.Finish(ctx, runID); finishErr != nil {
		log.Printf("run_campaign: Finish run %d: %v", runID, finishErr)
	}

	return nil
}

// tryFail tente de marquer la campagne comme echouee.
func (uc *RunCampaign) tryFail(ctx context.Context, campaignID string) {
	_, err := uc.Campaigns.UpdateStatusAtomic(ctx, campaignID, domain.StatusRunning, domain.StatusFailed)
	if err != nil {
		log.Printf("run_campaign: tryFail %s: %v", campaignID, err)
	}
}

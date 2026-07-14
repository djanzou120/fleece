package usecases

import (
	"context"
	"fmt"

	"fleece/src/campaign/internal/application/ports/input"
	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// ScheduleCampaign planifie une campagne (draft → scheduled).
//
// Gardes metier :
//   - La campagne doit exister.
//   - message_body doit etre non vide (ErrMissingMessageBody).
//   - scheduled_at doit etre non nil (commande ScheduleCampaignCommand.ScheduledAt).
//   - La campagne doit etre en etat draft (ErrInvalidTransition).
type ScheduleCampaign struct {
	Campaigns output.CampaignRepository
}

// Execute planifie la campagne.
func (uc *ScheduleCampaign) Execute(ctx context.Context, cmd input.ScheduleCampaignCommand) (*domain.Campaign, error) {
	if cmd.CampaignID == "" {
		return nil, fmt.Errorf("schedule_campaign: campaign_id requis")
	}
	if cmd.ScheduledAt.IsZero() {
		return nil, domain.ErrMissingSchedule
	}

	campaign, err := uc.Campaigns.Get(ctx, cmd.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("schedule_campaign: charger campagne: %w", err)
	}

	// Mettre a jour le message_body si fourni.
	if cmd.MessageBody != "" {
		campaign.MessageBody = cmd.MessageBody
	}

	// La methode domaine valide les gardes (message_body != "", status == draft).
	if err := campaign.Schedule(cmd.ScheduledAt); err != nil {
		return nil, err
	}

	if err := uc.Campaigns.Save(ctx, campaign); err != nil {
		return nil, fmt.Errorf("schedule_campaign: persister: %w", err)
	}

	return campaign, nil
}

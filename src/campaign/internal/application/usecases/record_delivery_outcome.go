package usecases

import (
	"context"
	"fmt"
	"log"

	"fleece/src/campaign/internal/application/ports/input"
	"fleece/src/campaign/internal/application/ports/output"
	"fleece/src/campaign/internal/domain"
)

// RecordDeliveryOutcome met a jour le statut d'un destinataire sur reception d'un DLR.
//
// Correlation DLR :
//   - Le DLR contient un message_id (retourne par MessagingClient.SendMessage).
//   - On cherche le destinataire dont le message_id correspond.
//   - Si message_id est vide ou NULL, l'outcome est ignore (log warning, pas d'erreur).
//   - Si message_id ne correspond a aucun destinataire, l'outcome est ignore sans erreur.
//     Ce comportement est documente : un DLR peut arriver apres une reprise ou hors contexte.
//
// Note T-020 : les evenements message.delivered/message.failed sont aussi consommes par
// le service analytics. Les deux consumers sont independants (isolation D1).
type RecordDeliveryOutcome struct {
	Recipients output.RecipientRepository
}

// Execute traite un DLR et met a jour le statut du destinataire.
func (uc *RecordDeliveryOutcome) Execute(ctx context.Context, cmd input.RecordDeliveryOutcomeCommand) error {
	// Gerer le cas message_id vide (NULL dans le DLR) sans planter.
	if cmd.MessageID == "" {
		log.Printf("record_delivery_outcome: message_id vide/NULL, DLR ignore")
		return nil
	}

	status := domain.RecipientDelivered
	if !cmd.Success {
		status = domain.RecipientFailed
	}

	rowsAffected, err := uc.Recipients.UpdateStatusByMessageID(ctx, cmd.MessageID, status)
	if err != nil {
		return fmt.Errorf("record_delivery_outcome: UpdateStatusByMessageID %s: %w", cmd.MessageID, err)
	}

	if rowsAffected == 0 {
		// message_id inconnu (DLR orphelin) : log sans erreur.
		log.Printf("record_delivery_outcome: message_id %s inconnu, DLR ignore", cmd.MessageID)
		return nil
	}

	// Incrementer les compteurs du run est delibrement omis ici :
	// le schema n'a pas de FK entre campaign_recipients et campaign_runs.
	// La correlation run-recipient se fait via campaign_id et les compteurs
	// sont incrementes dans RunCampaign lors de l'envoi (sent/failed).
	// Les compteurs delivered/failed du run ne sont pas mis a jour sur DLR
	// dans cette implementation V1 (simplification documente).
	// TODO(T-020): analytics consomme les memes evenements pour ses KPIs.

	return nil
}

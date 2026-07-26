package intelligenceprocessor

// on_message_failed.go — handler de l'evenement AMQP "message.failed"
// (routing key "message.failed", contrat D-M21 — voir consumer.go).
//
// Ce handler est le JUMEAU de src/core-processor/on_message_failed.go pour la
// MEME routing key, mais recu sur une QUEUE DISTINCTE (voir consumer.go) : la
// ou core-processor transitionne messaging.messages.status ET rembourse le
// wallet (Decision C, PM M-021 : ce worker ne touche JAMAIS wallet.wallets ni
// wallet.wallet_transactions — deux workers qui crediteraient doubleraient
// tout remboursement), ce worker enrichit le score contact (contact_intel),
// l'agregat analytics et la correlation campagne — toute la mecanique
// commune avec on_message_delivered.go (transaction unique, lecture
// autonome, idempotence) est factorisee dans delivery_outcome.go
// (processDeliveryOutcome).
import "context"

// handleMessageFailed traite l'evenement "message.failed".
func handleMessageFailed(ctx context.Context, s *Service, body []byte) error {
	evt, err := parseMessageEvent(body)
	if err != nil {
		return newPermanentError(err)
	}
	if err := validateMessageID(evt.MessageID); err != nil {
		return newPermanentError(err)
	}

	if err := processDeliveryOutcome(ctx, s, evt, false); err != nil {
		return err
	}

	s.Logger.Info("intelligence-processor: message.failed traite",
		"message_id", evt.MessageID, "external_id", evt.ExternalID, "provider_id", evt.ProviderID)
	return nil
}

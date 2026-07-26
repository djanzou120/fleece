package intelligenceprocessor

// on_message_delivered.go — handler de l'evenement AMQP "message.delivered"
// (routing key "message.delivered", contrat D-M21 — voir consumer.go).
//
// Ce handler est le JUMEAU de src/core-processor/on_message_delivered.go pour
// la MEME routing key, mais recu sur une QUEUE DISTINCTE (voir consumer.go) :
// la ou core-processor transitionne messaging.messages.status, ce worker
// enrichit le score contact (contact_intel), l'agregat analytics et la
// correlation campagne — toute la mecanique commune avec
// on_message_failed.go (transaction unique, lecture autonome, idempotence)
// est factorisee dans delivery_outcome.go (processDeliveryOutcome).
import "context"

// handleMessageDelivered traite l'evenement "message.delivered".
func handleMessageDelivered(ctx context.Context, s *Service, body []byte) error {
	evt, err := parseMessageEvent(body)
	if err != nil {
		return newPermanentError(err)
	}
	if err := validateMessageID(evt.MessageID); err != nil {
		return newPermanentError(err)
	}

	if err := processDeliveryOutcome(ctx, s, evt, true); err != nil {
		return err
	}

	s.Logger.Info("intelligence-processor: message.delivered traite",
		"message_id", evt.MessageID, "external_id", evt.ExternalID, "provider_id", evt.ProviderID)
	return nil
}

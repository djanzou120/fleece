package messaging

import (
	"context"
	"encoding/json"
	"log"

	"fleece/src/campaign/internal/application/ports/input"
)

// dlrEvent est la structure JSON des evenements DLR consommes depuis RabbitMQ.
// Produits par le Messaging Service sur message.delivered / message.failed.
//
// Note T-020 : le service analytics consomme les memes evenements de facon independante.
// Les deux consumers sont isoles et ne se coordonnent pas.
type dlrEvent struct {
	MessageID string `json:"message_id"` // peut etre absent/vide
	Phone     string `json:"phone"`
	Channel   string `json:"channel"`
	Success   bool   `json:"success"`
}

// recordDeliveryOutcomeUseCase est l'interface locale du use case RecordDeliveryOutcome.
// Definie localement pour eviter l'import circulaire et respecter la regle de dependance.
type recordDeliveryOutcomeUseCase interface {
	Execute(ctx context.Context, cmd input.RecordDeliveryOutcomeCommand) error
}

// DLRConsumer consomme les evenements DLR (message.delivered, message.failed)
// depuis RabbitMQ et appelle RecordDeliveryOutcome.
//
// Correlation DLR :
//   - Chaque evenement contient un message_id (identifiant du message dans le service messaging).
//   - Le use case RecordDeliveryOutcome recherche le destinataire par message_id.
//   - Si message_id est absent ou vide : l'outcome est ignore (log warning).
//   - Si message_id ne correspond a aucun destinataire : DLR orphelin (log, pas d'erreur).
type DLRConsumer struct {
	broker    Broker
	queueName string
	uc        recordDeliveryOutcomeUseCase
}

// NewDLRConsumer cree un DLRConsumer.
// queueName est la queue RabbitMQ a consommer (ex. "campaign.dlr").
func NewDLRConsumer(broker Broker, queueName string, uc recordDeliveryOutcomeUseCase) *DLRConsumer {
	return &DLRConsumer{
		broker:    broker,
		queueName: queueName,
		uc:        uc,
	}
}

// Start demarre la consommation et bloque jusqu'a l'annulation de ctx.
func (c *DLRConsumer) Start(ctx context.Context) error {
	return c.broker.Consume(ctx, c.queueName, func(body []byte) error {
		return c.handleMessage(ctx, body)
	})
}

// handleMessage deserialise et traite un evenement DLR.
func (c *DLRConsumer) handleMessage(ctx context.Context, body []byte) error {
	var event dlrEvent
	if err := json.Unmarshal(body, &event); err != nil {
		// Message malformed : on logue et on acquitte (pas de retry infini).
		log.Printf("[campaign:dlr_consumer] message malformed: %v — body: %s", err, string(body))
		return nil
	}

	// Gerer le cas message_id absent/vide sans planter.
	if event.MessageID == "" {
		log.Printf("[campaign:dlr_consumer] message_id absent dans le DLR phone=%q channel=%q, ignore", event.Phone, event.Channel)
		return nil
	}

	cmd := input.RecordDeliveryOutcomeCommand{
		MessageID: event.MessageID,
		Success:   event.Success,
	}

	if err := c.uc.Execute(ctx, cmd); err != nil {
		log.Printf("[campaign:dlr_consumer] RecordDeliveryOutcome messageID=%s: %v", event.MessageID, err)
		// Retourner l'erreur pour que le broker puisse rejeter/requeue le message.
		return err
	}

	return nil
}

package messaging

import (
	"context"
	"encoding/json"
	"log"

	"fleece/src/contact-intelligence/internal/domain"
)

// dlrEvent est la structure JSON des evenements DLR consommes depuis RabbitMQ.
// Produits par le Messaging Service sur message.delivered / message.failed.
type dlrEvent struct {
	Phone   string `json:"phone"`
	Channel string `json:"channel"`
	Success bool   `json:"success"`
}

// recordDeliveryOutcomeUseCase est l'interface locale du use case RecordDeliveryOutcome.
// Definie localement pour eviter l'import circulaire et respecter la regle de dependance.
type recordDeliveryOutcomeUseCase interface {
	Execute(ctx context.Context, phone string, ch domain.Channel, success bool) error
}

// DLRConsumer consomme les evenements DLR (message.delivered, message.failed)
// depuis RabbitMQ et appelle RecordDeliveryOutcome.
type DLRConsumer struct {
	broker    Broker
	queueName string
	uc        recordDeliveryOutcomeUseCase
}

// NewDLRConsumer cree un DLRConsumer.
// queueName est la queue RabbitMQ a consommer (ex. "contact_intel.dlr").
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
		log.Printf("[dlr_consumer] message malformed: %v — body: %s", err, string(body))
		return nil
	}

	ch := domain.Channel(event.Channel)
	if event.Phone == "" || !ch.IsValid() {
		log.Printf("[dlr_consumer] champs invalides: phone=%q channel=%q", event.Phone, event.Channel)
		return nil
	}

	if err := c.uc.Execute(ctx, event.Phone, ch, event.Success); err != nil {
		log.Printf("[dlr_consumer] RecordDeliveryOutcome phone=%s channel=%s: %v", event.Phone, event.Channel, err)
		// Retourner l'erreur pour que le broker puisse rejeter/requeue le message.
		return err
	}

	return nil
}

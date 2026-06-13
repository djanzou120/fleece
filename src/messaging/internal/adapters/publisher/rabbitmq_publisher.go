// Package publisher implémente le port output.EventPublisher via l'abstraction Broker.
// Couche 3 (Interface Adapters, driven).
//
// TODO(amqp): quand AMQPBroker sera disponible, injecter le broker AMQP réel
// à la place de NoopBroker dans le composition root (main.go). Ce fichier ne changera pas.
package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"fleece/src/messaging/internal/adapters/messaging"
	"fleece/src/messaging/internal/domain"
)

// RabbitMQPublisher implémente output.EventPublisher en sérialisant les événements
// en JSON et en les publiant via le Broker de couche 3.
type RabbitMQPublisher struct {
	broker messaging.Broker
}

// NewRabbitMQPublisher crée un RabbitMQPublisher avec le Broker fourni.
func NewRabbitMQPublisher(broker messaging.Broker) *RabbitMQPublisher {
	return &RabbitMQPublisher{broker: broker}
}

// eventPayload est la structure sérialisée publiée dans RabbitMQ.
type eventPayload struct {
	Event       string `json:"event"`
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"`
	Channel     string `json:"channel"`
	Recipient   string `json:"recipient"`
}

// Publish sérialise le message en JSON et le publie sur la routing key égale au
// nom de l'événement (ex: "message.created", "message.sent", …).
func (p *RabbitMQPublisher) Publish(ctx context.Context, event string, m *domain.Message) error {
	payload := eventPayload{
		Event:       event,
		MessageID:   m.ID,
		WorkspaceID: m.WorkspaceID,
		Status:      string(m.Status),
		Channel:     string(m.Channel),
		Recipient:   m.Recipient,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("publisher: marshal event %s: %w", event, err)
	}
	if err := p.broker.Publish(ctx, event, body); err != nil {
		return fmt.Errorf("publisher: publish event %s: %w", event, err)
	}
	return nil
}

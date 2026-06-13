// Package publisher implemente le port output.EventPublisher via l'abstraction Broker.
// Couche 3 (Interface Adapters, driven).
//
// TODO(amqp): quand AMQPBroker sera disponible, injecter le broker AMQP reel
// a la place de NoopBroker dans le composition root (main.go). Ce fichier ne changera pas.
package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"fleece/src/wallet/internal/adapters/messaging"
	"fleece/src/wallet/internal/domain"
)

// EventPublisher implemente output.EventPublisher en serialisant les evenements
// en JSON et en les publiant via le Broker de couche 3.
type EventPublisher struct {
	broker messaging.Broker
}

// NewEventPublisher cree un EventPublisher avec le Broker fourni.
func NewEventPublisher(broker messaging.Broker) *EventPublisher {
	return &EventPublisher{broker: broker}
}

// walletEventPayload est la structure serialisee publiee dans RabbitMQ.
type walletEventPayload struct {
	Event       string `json:"event"`
	WorkspaceID string `json:"workspace_id"`
	Kind        string `json:"kind"`
	Amount      int64  `json:"amount"`
	MessageID   string `json:"message_id,omitempty"`
}

// Publish serialise la transaction en JSON et la publie sur la routing key egale au
// nom de l'evenement (ex: "wallet.debited", "wallet.credited", "wallet.refunded").
func (p *EventPublisher) Publish(ctx context.Context, event string, t *domain.WalletTransaction) error {
	payload := walletEventPayload{
		Event:       event,
		WorkspaceID: t.WorkspaceID,
		Kind:        string(t.Kind),
		Amount:      t.Amount,
		MessageID:   t.MessageID,
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

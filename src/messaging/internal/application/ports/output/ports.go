package output

import (
	"context"

	"fleece/src/messaging/internal/domain"
)

// MessageRepository persiste les messages. Implémenté en couche 3 (Postgres,
// schéma "messaging").
type MessageRepository interface {
	Save(ctx context.Context, m *domain.Message) error
	Get(ctx context.Context, id string) (*domain.Message, error)
}

// RouteAttempt est une étape (canal, fournisseur) de la chaîne de fallback
// produite par le Routing Service.
type RouteAttempt struct {
	Channel  domain.Channel
	Provider string
}

// RoutingGateway interroge le Routing Service (client REST interne en couche 3).
type RoutingGateway interface {
	Decide(ctx context.Context, m *domain.Message) ([]RouteAttempt, error)
}

// WalletGateway interroge le Wallet Service.
type WalletGateway interface {
	HasBalance(ctx context.Context, workspaceID string) (bool, error)
	Debit(ctx context.Context, workspaceID, messageID string) error
	Refund(ctx context.Context, workspaceID, messageID string) error
}

// ProviderGateway délègue l'envoi au Provider Service.
type ProviderGateway interface {
	Send(ctx context.Context, m *domain.Message, attempt RouteAttempt) error
}

// EventPublisher publie les événements de domaine (RabbitMQ en couche 3).
type EventPublisher interface {
	Publish(ctx context.Context, event string, m *domain.Message) error
}

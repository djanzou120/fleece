// Package input — ports pilotants : interfaces exposees par le service webhook (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package input

import (
	"context"

	"fleece/src/webhook/internal/domain"
)

// DeliverEventUseCase livre un evenement a tous les endpoints actifs d'un workspace
// abonnes a cet evenement.
type DeliverEventUseCase interface {
	// Execute signe le payload et l'envoie a chaque endpoint abonne.
	// Pour chaque endpoint, une WebhookDelivery est creee et persistee.
	Execute(ctx context.Context, workspaceID, event string, payload []byte) error
}

// RetryDeliveryUseCase retente la livraison d'un evenement apres un echec.
type RetryDeliveryUseCase interface {
	// Execute retente la livraison identifiee par deliveryID.
	// Incremente attempts, recalcule nextRetryAt via backoff exponentiel.
	// Retourne domain.ErrMaxRetriesExceeded si le max est atteint.
	Execute(ctx context.Context, deliveryID int64) error
}

// RegisterEndpointUseCase enregistre un nouvel endpoint webhook pour un workspace.
type RegisterEndpointUseCase interface {
	// Execute valide et persiste l'endpoint.
	// Genere un secret aleatoire si aucun n'est fourni.
	// Retourne l'endpoint cree avec son secret.
	Execute(ctx context.Context, endpoint *domain.WebhookEndpoint) (*domain.WebhookEndpoint, error)
}

// Package output — ports pilotes : interfaces requises par les use cases (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package output

import (
	"context"
	"time"

	"fleece/src/webhook/internal/domain"
)

// EndpointRepository persiste les endpoints webhook.
// Implemente en couche 3 (Postgres, schema "webhook").
type EndpointRepository interface {
	// Save persiste (insert ou update) un endpoint.
	Save(ctx context.Context, e *domain.WebhookEndpoint) error

	// FindByID recupere un endpoint par son identifiant.
	// Retourne domain.ErrEndpointNotFound si l'endpoint est introuvable.
	FindByID(ctx context.Context, id string) (*domain.WebhookEndpoint, error)

	// FindByWorkspaceAndEvent retourne les endpoints actifs d'un workspace
	// abonnes a l'evenement donne.
	FindByWorkspaceAndEvent(ctx context.Context, workspaceID, event string) ([]*domain.WebhookEndpoint, error)

	// ListByWorkspace retourne tous les endpoints d'un workspace.
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.WebhookEndpoint, error)

	// Delete supprime un endpoint par son identifiant.
	Delete(ctx context.Context, id string) error
}

// DeliveryRepository persiste les livraisons de webhook.
// Implemente en couche 3 (Postgres, schema "webhook").
type DeliveryRepository interface {
	// Save persiste une livraison (insert ou update).
	Save(ctx context.Context, d *domain.WebhookDelivery) error

	// FindByID recupere une livraison par son identifiant.
	FindByID(ctx context.Context, id int64) (*domain.WebhookDelivery, error)

	// FindFailedByEndpoint retourne les livraisons en echec d'un endpoint,
	// ordonnees par date de creation (les plus anciennes en premier).
	FindFailedByEndpoint(ctx context.Context, endpointID string) ([]*domain.WebhookDelivery, error)

	// FindPendingRetries retourne les livraisons dont next_retry_at est echu (next_retry_at <= at)
	// et dont le statut justifie un retry (failed). Utilisee par le scheduler periodique.
	FindPendingRetries(ctx context.Context, at time.Time) ([]*domain.WebhookDelivery, error)
}

// HTTPDispatcher envoie le payload d'un evenement a l'URL cible via HTTP POST.
// Implemente en couche 3 (adapters/dispatcher).
type HTTPDispatcher interface {
	// Dispatch effectue un POST HTTP sur url avec le payload donne.
	// Le header X-Fleece-Signature est positionne a la valeur de signature.
	// Retourne le code HTTP de la reponse et une erreur eventuelle.
	Dispatch(ctx context.Context, url string, payload []byte, signature string) (statusCode int, err error)
}

package output

import (
	"context"
	"time"
)

// ProviderRepository persiste la table provider.provider_messages.
// Schema 0005 : colonnes internal_id (uuid), provider_id (text), external_id (text).
// Schema 0012 : colonnes status (varchar(50) NOT NULL DEFAULT 'pending'), updated_at (timestamptz).
// PK = (internal_id, provider_id).
type ProviderRepository interface {
	// Save insere ou met a jour une entree dans provider.provider_messages.
	// internalId est l'UUID du message cote Fleece (format string uuid, ex. "550e8400-…").
	// providerId est l'identifiant du fournisseur utilise.
	// externalId est l'identifiant attribue par le fournisseur.
	// Le status est initialise a "pending" et updated_at a NULL a la creation.
	Save(ctx context.Context, internalId, providerId, externalId string) error

	// FindByInternalID retrouve l'externalId associe a un internalId + providerId.
	// Retourne ("", nil) si l'entree n'existe pas.
	FindByInternalID(ctx context.Context, internalId, providerId string) (externalId string, err error)

	// UpdateStatus met a jour le statut de livraison et la date de mise a jour
	// d'un enregistrement identifie par son externalId (identifiant fournisseur).
	// status est la valeur string du statut (ex. "delivered", "failed", "rejected").
	// Retourne nil si aucune ligne n'est trouvee (idempotent).
	UpdateStatus(ctx context.Context, externalId string, status string, updatedAt time.Time) error
}

// EventPublisher publie les evenements de domaine provider (via Broker RabbitMQ en C3).
type EventPublisher interface {
	// Publish serialise et publie un evenement sur la routing key donnee.
	Publish(ctx context.Context, event string, payload any) error
}

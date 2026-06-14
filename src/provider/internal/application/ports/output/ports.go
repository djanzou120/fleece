package output

import "context"

// ProviderRepository persiste la table provider.provider_messages.
// Schema 0005 : colonnes exactes internal_id (uuid), provider_id (text), external_id (text).
// PK = (internal_id, provider_id).
//
// Note : les colonnes status, recipient, content n'existent pas dans le schema 0005.
// Seuls internal_id, provider_id et external_id sont persistes.
type ProviderRepository interface {
	// Save insere ou met a jour une entree dans provider.provider_messages.
	// internalId est l'UUID du message cote Fleece (format string uuid, ex. "550e8400-…").
	// providerId est l'identifiant du fournisseur utilise.
	// externalId est l'identifiant attribue par le fournisseur.
	Save(ctx context.Context, internalId, providerId, externalId string) error

	// FindByInternalID retrouve l'externalId associe a un internalId + providerId.
	// Retourne ("", nil) si l'entree n'existe pas.
	FindByInternalID(ctx context.Context, internalId, providerId string) (externalId string, err error)
}

// EventPublisher publie les evenements de domaine provider (via Broker RabbitMQ en C3).
type EventPublisher interface {
	// Publish serialise et publie un evenement sur la routing key donnee.
	Publish(ctx context.Context, event string, payload any) error
}

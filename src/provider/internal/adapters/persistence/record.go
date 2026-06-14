// Package persistence implemente le port output.ProviderRepository via *sql.DB.
// Couche 3 (Interface Adapters, driven).
//
// Schema 0005 - table provider.provider_messages :
//
//	internal_id uuid NOT NULL
//	provider_id text NOT NULL
//	external_id text NOT NULL
//	PRIMARY KEY (internal_id, provider_id)
//
// Note : les champs status, recipient, content ne figurent PAS dans le schema 0005.
// Seuls internal_id, provider_id et external_id sont persistes/lus.
package persistence

// providerMessageRecord est le modele de la table provider.provider_messages.
// Il ne mappe que les colonnes existantes dans le schema 0005.
type providerMessageRecord struct {
	InternalID string // uuid stocke comme text (Postgres uuid)
	ProviderID string
	ExternalID string
}

// Package persistence implemente le port output.ProviderRepository via *sql.DB.
// Couche 3 (Interface Adapters, driven).
//
// Schema actuel - table provider.provider_messages :
//
//	internal_id uuid        NOT NULL
//	provider_id text        NOT NULL
//	external_id text        NOT NULL
//	status      varchar(50) NOT NULL DEFAULT 'pending'  (ajoute en 0012)
//	updated_at  timestamptz                              (ajoute en 0012)
//	PRIMARY KEY (internal_id, provider_id)
package persistence

import "time"

// providerMessageRecord est le modele de la table provider.provider_messages.
// Il mappe les colonnes du schema 0005 (internal_id, provider_id, external_id)
// et celles ajoutees en 0012 (status, updated_at).
type providerMessageRecord struct {
	InternalID string     // uuid stocke comme text (Postgres uuid)
	ProviderID string
	ExternalID string
	Status     string     // statut de livraison : "pending", "delivered", "failed", "rejected"
	UpdatedAt  *time.Time // nullable : non renseignee avant le premier DLR
}

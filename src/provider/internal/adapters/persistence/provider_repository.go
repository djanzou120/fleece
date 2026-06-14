package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ProviderRepository implemente output.ProviderRepository via *sql.DB.
// Il n'accede qu'au schema "provider" (table provider.provider_messages).
type ProviderRepository struct {
	db *sql.DB
}

// NewProviderRepository cree un ProviderRepository avec la connexion fournie.
func NewProviderRepository(db *sql.DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

// Save insere ou met a jour un enregistrement dans provider.provider_messages.
// Seules les colonnes du schema 0005 sont ecrites : internal_id, provider_id, external_id.
// ON CONFLICT effectue un UPDATE de external_id.
func (r *ProviderRepository) Save(ctx context.Context, internalId, providerId, externalId string) error {
	if r.db == nil {
		return fmt.Errorf("persistence: Save: base de donnees non initialisee")
	}
	const query = `
		INSERT INTO provider.provider_messages (internal_id, provider_id, external_id)
		VALUES ($1::uuid, $2, $3)
		ON CONFLICT (internal_id, provider_id) DO UPDATE
		  SET external_id = EXCLUDED.external_id
	`
	_, err := r.db.ExecContext(ctx, query, internalId, providerId, externalId)
	if err != nil {
		return fmt.Errorf("persistence: Save provider_message (internal=%s provider=%s): %w",
			internalId, providerId, err)
	}
	return nil
}

// FindByInternalID retrouve l'externalId associe a un internalId + providerId.
// Retourne ("", nil) si l'entree n'existe pas (sql.ErrNoRows).
func (r *ProviderRepository) FindByInternalID(ctx context.Context, internalId, providerId string) (string, error) {
	if r.db == nil {
		return "", fmt.Errorf("persistence: FindByInternalID: base de donnees non initialisee")
	}
	const query = `
		SELECT external_id
		  FROM provider.provider_messages
		 WHERE internal_id = $1::uuid
		   AND provider_id = $2
	`
	var rec providerMessageRecord
	err := r.db.QueryRowContext(ctx, query, internalId, providerId).Scan(&rec.ExternalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("persistence: FindByInternalID (internal=%s provider=%s): %w",
			internalId, providerId, err)
	}
	return rec.ExternalID, nil
}

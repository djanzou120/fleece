package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fleece/src/provider/internal/application/ports/output"
)

// Assertion de conformite a la compilation : ProviderRepository doit implémenter output.ProviderRepository.
// Couche 3 → couche 2 : sens autorise (vers l'interieur).
var _ output.ProviderRepository = (*ProviderRepository)(nil)

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
// Colonnes ecrites : internal_id, provider_id, external_id, status (initialise a "pending"),
// updated_at (NULL a la creation, renseignee lors du premier DLR via UpdateStatus).
// ON CONFLICT effectue un UPDATE de external_id (status et updated_at sont preserves).
func (r *ProviderRepository) Save(ctx context.Context, internalId, providerId, externalId string) error {
	if r.db == nil {
		return fmt.Errorf("persistence: Save: base de donnees non initialisee")
	}
	const query = `
		INSERT INTO provider.provider_messages (internal_id, provider_id, external_id, status, updated_at)
		VALUES ($1::uuid, $2, $3, 'pending', NULL)
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

// UpdateStatus met a jour le status et le updated_at d'un enregistrement
// identifie par son external_id (identifiant fournisseur, connu du DLR).
// Retourne nil sans erreur si aucune ligne n'est trouvee (idempotent).
func (r *ProviderRepository) UpdateStatus(ctx context.Context, externalId string, status string, updatedAt time.Time) error {
	if r.db == nil {
		return nil
	}
	const query = `
		UPDATE provider.provider_messages
		   SET status     = $2,
		       updated_at = $3
		 WHERE external_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, externalId, status, updatedAt)
	if err != nil {
		return fmt.Errorf("persistence: UpdateStatus (external=%s status=%s): %w",
			externalId, status, err)
	}
	return nil
}

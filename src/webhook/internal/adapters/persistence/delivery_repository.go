package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"fleece/src/webhook/internal/application/ports/output"
	"fleece/src/webhook/internal/domain"
)

// Assertion de conformite : DeliveryRepository doit implementer output.DeliveryRepository.
// Le compilateur echoue ici si une methode est manquante ou mal typee.
var _ output.DeliveryRepository = (*DeliveryRepository)(nil)

// DeliveryRepository implemente output.DeliveryRepository via *sql.DB.
// Il n'accede qu'au schema "webhook" (tables prefixees webhook.).
//
// Ecarts de schema documentes dans record.go :
//   - "payload" non persiste (absent de 0006)
//   - "next_retry_at" non persiste (absent de 0006)
type DeliveryRepository struct {
	db *sql.DB
}

// NewDeliveryRepository cree un DeliveryRepository avec la connexion fournie.
func NewDeliveryRepository(db *sql.DB) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

// Save persiste une livraison dans webhook.webhook_deliveries.
// Si ID est 0 (nouvelle livraison), un INSERT est effectue avec RETURNING id.
// Sinon, un UPDATE est effectue sur les colonnes persistees (status, attempts).
func (r *DeliveryRepository) Save(ctx context.Context, d *domain.WebhookDelivery) error {
	if d.ID == 0 {
		return r.insert(ctx, d)
	}
	return r.update(ctx, d)
}

// insert insere une nouvelle livraison et renseigne d.ID avec la valeur generee.
func (r *DeliveryRepository) insert(ctx context.Context, d *domain.WebhookDelivery) error {
	rec := fromDelivery(d)
	const query = `
		INSERT INTO webhook.webhook_deliveries (endpoint_id, event, status, attempts)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	// TODO(schema): payload et next_retry_at non persistes (absent de 0006).
	row := r.db.QueryRowContext(ctx, query,
		rec.EndpointID,
		rec.Event,
		rec.Status,
		rec.Attempts,
	)
	if err := row.Scan(&d.ID); err != nil {
		return fmt.Errorf("persistence: insert delivery endpoint=%s event=%s: %w", d.EndpointID, d.Event, err)
	}
	return nil
}

// update met a jour le statut et le nombre de tentatives d'une livraison existante.
func (r *DeliveryRepository) update(ctx context.Context, d *domain.WebhookDelivery) error {
	rec := fromDelivery(d)
	const query = `
		UPDATE webhook.webhook_deliveries
		   SET status   = $1,
		       attempts = $2
		 WHERE id = $3
	`
	// TODO(schema): next_retry_at non persiste (absent de 0006).
	_, err := r.db.ExecContext(ctx, query, rec.Status, rec.Attempts, rec.ID)
	if err != nil {
		return fmt.Errorf("persistence: update delivery %d: %w", d.ID, err)
	}
	return nil
}

// FindByID recupere une livraison par son identifiant.
func (r *DeliveryRepository) FindByID(ctx context.Context, id int64) (*domain.WebhookDelivery, error) {
	const query = `
		SELECT id, endpoint_id, event, status, attempts, created_at
		  FROM webhook.webhook_deliveries
		 WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	var rec deliveryRecord
	err := row.Scan(
		&rec.ID,
		&rec.EndpointID,
		&rec.Event,
		&rec.Status,
		&rec.Attempts,
		&rec.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("persistence: FindByID delivery %d: introuvable", id)
		}
		return nil, fmt.Errorf("persistence: FindByID delivery %d: %w", id, err)
	}
	return rec.toEntity(), nil
}

// FindFailedByEndpoint retourne les livraisons en echec d'un endpoint.
func (r *DeliveryRepository) FindFailedByEndpoint(ctx context.Context, endpointID string) ([]*domain.WebhookDelivery, error) {
	const query = `
		SELECT id, endpoint_id, event, status, attempts, created_at
		  FROM webhook.webhook_deliveries
		 WHERE endpoint_id = $1
		   AND status = 'failed'
		 ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, endpointID)
	if err != nil {
		return nil, fmt.Errorf("persistence: FindFailedByEndpoint endpoint=%s: %w", endpointID, err)
	}
	defer rows.Close()

	var deliveries []*domain.WebhookDelivery
	for rows.Next() {
		var rec deliveryRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.EndpointID,
			&rec.Event,
			&rec.Status,
			&rec.Attempts,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("persistence: FindFailedByEndpoint scan: %w", err)
		}
		deliveries = append(deliveries, rec.toEntity())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: FindFailedByEndpoint rows: %w", err)
	}
	return deliveries, nil
}

package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"fleece/src/webhook/internal/application/ports/output"
	"fleece/src/webhook/internal/domain"
)

// Assertion de conformite : DeliveryRepository doit implementer output.DeliveryRepository.
// Le compilateur echoue ici si une methode est manquante ou mal typee.
var _ output.DeliveryRepository = (*DeliveryRepository)(nil)

// DeliveryRepository implemente output.DeliveryRepository via *sql.DB.
// Il n'accede qu'au schema "webhook" (tables prefixees webhook.).
type DeliveryRepository struct {
	db *sql.DB
}

// NewDeliveryRepository cree un DeliveryRepository avec la connexion fournie.
func NewDeliveryRepository(db *sql.DB) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

// Save persiste une livraison dans webhook.webhook_deliveries.
// Si ID est 0 (nouvelle livraison), un INSERT est effectue avec RETURNING id.
// Sinon, un UPDATE est effectue sur les colonnes persistees (status, attempts, payload, next_retry_at).
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
		INSERT INTO webhook.webhook_deliveries (endpoint_id, event, status, attempts, payload, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	row := r.db.QueryRowContext(ctx, query,
		rec.EndpointID,
		rec.Event,
		rec.Status,
		rec.Attempts,
		rec.Payload,
		rec.NextRetryAt,
	)
	if err := row.Scan(&d.ID); err != nil {
		return fmt.Errorf("persistence: insert delivery endpoint=%s event=%s: %w", d.EndpointID, d.Event, err)
	}
	return nil
}

// update met a jour le statut, le nombre de tentatives, le payload et next_retry_at d'une livraison existante.
func (r *DeliveryRepository) update(ctx context.Context, d *domain.WebhookDelivery) error {
	rec := fromDelivery(d)
	const query = `
		UPDATE webhook.webhook_deliveries
		   SET status        = $1,
		       attempts      = $2,
		       payload       = $3,
		       next_retry_at = $4
		 WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query, rec.Status, rec.Attempts, rec.Payload, rec.NextRetryAt, rec.ID)
	if err != nil {
		return fmt.Errorf("persistence: update delivery %d: %w", d.ID, err)
	}
	return nil
}

// FindByID recupere une livraison par son identifiant.
func (r *DeliveryRepository) FindByID(ctx context.Context, id int64) (*domain.WebhookDelivery, error) {
	const query = `
		SELECT id, endpoint_id, event, status, attempts, payload, next_retry_at, created_at
		  FROM webhook.webhook_deliveries
		 WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	rec, err := scanDelivery(row)
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
		SELECT id, endpoint_id, event, status, attempts, payload, next_retry_at, created_at
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

	return scanDeliveries(rows)
}

// FindPendingRetries retourne les livraisons dont next_retry_at est echu et dont
// le statut indique qu'un retry est possible (failed, pas delivered ni exhausted).
// Appelee par le scheduler de retry toutes les 60 s.
func (r *DeliveryRepository) FindPendingRetries(ctx context.Context, at time.Time) ([]*domain.WebhookDelivery, error) {
	if r.db == nil {
		return nil, nil
	}
	const query = `
		SELECT id, endpoint_id, event, status, attempts, payload, next_retry_at, created_at
		  FROM webhook.webhook_deliveries
		 WHERE next_retry_at IS NOT NULL
		   AND next_retry_at <= $1
		   AND status = 'failed'
		 ORDER BY next_retry_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, at)
	if err != nil {
		return nil, fmt.Errorf("persistence: FindPendingRetries at=%s: %w", at.Format(time.RFC3339), err)
	}
	defer rows.Close()

	return scanDeliveries(rows)
}

// --- helpers ---

// scanDelivery scanne une ligne de webhook_deliveries.
func scanDelivery(row rowScanner) (deliveryRecord, error) {
	var rec deliveryRecord
	err := row.Scan(
		&rec.ID,
		&rec.EndpointID,
		&rec.Event,
		&rec.Status,
		&rec.Attempts,
		&rec.Payload,
		&rec.NextRetryAt,
		&rec.CreatedAt,
	)
	if err != nil {
		return deliveryRecord{}, err
	}
	return rec, nil
}

// scanDeliveries boucle sur des *sql.Rows et retourne les entites.
func scanDeliveries(rows *sql.Rows) ([]*domain.WebhookDelivery, error) {
	var deliveries []*domain.WebhookDelivery
	for rows.Next() {
		rec, err := scanDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("persistence: scan delivery: %w", err)
		}
		deliveries = append(deliveries, rec.toEntity())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: rows error: %w", err)
	}
	return deliveries, nil
}

// Package persistence implemente les repositories webhook via *sql.DB.
// Couche 3 (Interface Adapters, driven).
//
// Toutes les colonnes de la migration 0010_webhook_schema.sql sont desormais persistees :
//   - webhook_endpoints : "active" (BOOLEAN NOT NULL DEFAULT true)
//   - webhook_deliveries : "payload" (BYTEA), "next_retry_at" (TIMESTAMPTZ nullable)
package persistence

import (
	"time"

	"fleece/src/webhook/internal/domain"
)

// endpointRecord est le modele de la table webhook.webhook_endpoints.
type endpointRecord struct {
	ID          string
	WorkspaceID string
	URL         string
	Secret      string
	Events      []string // text[] Postgres
	Active      bool
	CreatedAt   time.Time
}

// toEntity convertit un endpointRecord en entite domaine.
func (r endpointRecord) toEntity() *domain.WebhookEndpoint {
	return &domain.WebhookEndpoint{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		URL:         r.URL,
		Secret:      r.Secret,
		Events:      r.Events,
		Active:      r.Active,
	}
}

// fromEndpoint convertit une entite domaine en endpointRecord.
func fromEndpoint(e *domain.WebhookEndpoint) endpointRecord {
	return endpointRecord{
		ID:          e.ID,
		WorkspaceID: e.WorkspaceID,
		URL:         e.URL,
		Secret:      e.Secret,
		Events:      e.Events,
		Active:      e.Active,
	}
}

// deliveryRecord est le modele de la table webhook.webhook_deliveries.
type deliveryRecord struct {
	ID          int64
	EndpointID  string
	Event       string
	Status      string
	Attempts    int
	Payload     []byte
	NextRetryAt *time.Time // nullable TIMESTAMPTZ
	CreatedAt   time.Time
}

// toEntity convertit un deliveryRecord en entite domaine.
func (r deliveryRecord) toEntity() *domain.WebhookDelivery {
	d := &domain.WebhookDelivery{
		ID:         r.ID,
		EndpointID: r.EndpointID,
		Event:      r.Event,
		Status:     domain.DeliveryStatus(r.Status),
		Attempts:   r.Attempts,
		Payload:    r.Payload,
	}
	if r.NextRetryAt != nil {
		d.NextRetryAt = *r.NextRetryAt
	}
	return d
}

// fromDelivery convertit une entite domaine en deliveryRecord.
func fromDelivery(d *domain.WebhookDelivery) deliveryRecord {
	rec := deliveryRecord{
		ID:         d.ID,
		EndpointID: d.EndpointID,
		Event:      d.Event,
		Status:     string(d.Status),
		Attempts:   d.Attempts,
		Payload:    d.Payload,
	}
	if !d.NextRetryAt.IsZero() {
		t := d.NextRetryAt
		rec.NextRetryAt = &t
	}
	return rec
}

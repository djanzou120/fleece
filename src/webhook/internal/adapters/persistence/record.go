// Package persistence implemente les repositories webhook via *sql.DB.
// Couche 3 (Interface Adapters, driven).
//
// Ecarts de schema (migration 0006_webhook.sql) documentes :
//   - webhook_endpoints : pas de colonne "active" → par defaut true a la lecture.
//   - webhook_deliveries : pas de colonne "payload" ni "next_retry_at" → non persistes.
package persistence

import (
	"time"

	"fleece/src/webhook/internal/domain"
)

// endpointRecord est le modele de la table webhook.webhook_endpoints.
// Seules les colonnes presentes dans 0006_webhook.sql sont listees ici.
type endpointRecord struct {
	ID          string
	WorkspaceID string
	URL         string
	Secret      string
	Events      []string // text[] Postgres
	CreatedAt   time.Time
	// TODO(schema): colonne "active" absente de 0006 — non persistee.
	// La valeur true est injectee par defaut a la lecture.
}

// toEntity convertit un endpointRecord en entite domaine.
// Active est force a true car la colonne n'existe pas en base (0006_webhook.sql).
func (r endpointRecord) toEntity() *domain.WebhookEndpoint {
	return &domain.WebhookEndpoint{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		URL:         r.URL,
		Secret:      r.Secret,
		Events:      r.Events,
		// TODO(schema): colonne "active" absente de 0006 — active=true par defaut.
		Active: true,
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
		// TODO(schema): colonne "active" absente de 0006 — non persistee.
	}
}

// deliveryRecord est le modele de la table webhook.webhook_deliveries.
// Seules les colonnes presentes dans 0006_webhook.sql sont listees ici.
type deliveryRecord struct {
	ID         int64
	EndpointID string
	Event      string
	Status     string
	Attempts   int
	CreatedAt  time.Time
	// TODO(schema): colonnes "payload" et "next_retry_at" absentes de 0006 — non persistees.
	// Le payload reste en memoire ; le scheduling de retry est gere en applicatif.
}

// toEntity convertit un deliveryRecord en entite domaine.
// Payload et NextRetryAt sont absents du schema et valent leur zero value.
func (r deliveryRecord) toEntity() *domain.WebhookDelivery {
	return &domain.WebhookDelivery{
		ID:         r.ID,
		EndpointID: r.EndpointID,
		Event:      r.Event,
		Status:     domain.DeliveryStatus(r.Status),
		Attempts:   r.Attempts,
		// TODO(schema): payload absent de 0006 — non restaure depuis la base.
		// TODO(schema): next_retry_at absent de 0006 — non restaure depuis la base.
	}
}

// fromDelivery convertit une entite domaine en deliveryRecord.
func fromDelivery(d *domain.WebhookDelivery) deliveryRecord {
	return deliveryRecord{
		ID:         d.ID,
		EndpointID: d.EndpointID,
		Event:      d.Event,
		Status:     string(d.Status),
		Attempts:   d.Attempts,
		// TODO(schema): payload non persiste (absent de 0006).
		// TODO(schema): next_retry_at non persiste (absent de 0006).
	}
}

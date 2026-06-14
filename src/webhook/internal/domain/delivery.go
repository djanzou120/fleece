package domain

import "time"

// WebhookDelivery represente une tentative de livraison d'un evenement vers un endpoint.
type WebhookDelivery struct {
	// ID est l'identifiant auto-incremente de la livraison (bigserial).
	ID int64

	// EndpointID est l'UUID de l'endpoint cible.
	EndpointID string

	// Event est le type d'evenement livre (ex: "message.delivered").
	Event string

	// Payload est le corps JSON de l'evenement.
	// Persiste via la colonne "payload" (BYTEA) ajoutee en 0010_webhook_schema.sql,
	// pour permettre un redispatch fiable lors des retries.
	Payload []byte

	// Status est l'etat courant de la livraison.
	Status DeliveryStatus

	// Attempts est le nombre de tentatives effectuees.
	Attempts int

	// NextRetryAt indique quand la prochaine tentative doit etre effectuee.
	// Persiste via la colonne "next_retry_at" (TIMESTAMPTZ) ajoutee en 0010_webhook_schema.sql,
	// pour un scheduling de retry durable (lu par le scheduler via FindPendingRetries).
	NextRetryAt time.Time
}

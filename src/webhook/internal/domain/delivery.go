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
	// NOTE(schema): colonne "payload" absente de 0006 — non persistee.
	// Le payload reste en memoire le temps de la livraison.
	Payload []byte

	// Status est l'etat courant de la livraison.
	Status DeliveryStatus

	// Attempts est le nombre de tentatives effectuees.
	Attempts int

	// NextRetryAt indique quand la prochaine tentative doit etre effectuee.
	// NOTE(schema): colonne "next_retry_at" absente de 0006 — non persistee.
	// Le scheduling de retry est assure par la logique applicative.
	NextRetryAt time.Time
}

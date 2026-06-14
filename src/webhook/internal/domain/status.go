// Package domain — entites, value objects, erreurs metier du service webhook (couche 1).
//
// Clean Architecture : aucun import externe ; zéro framework.
// Voir .ia/ARCHITECTURE.md.
package domain

// DeliveryStatus represente l'etat d'une livraison de webhook.
type DeliveryStatus string

const (
	// StatusPending indique que la livraison n'a pas encore ete tentee.
	StatusPending DeliveryStatus = "pending"

	// StatusDelivered indique que la livraison a reussi (reponse 2xx recue).
	StatusDelivered DeliveryStatus = "delivered"

	// StatusFailed indique que la derniere tentative a echoue (reponse non-2xx ou erreur reseau).
	StatusFailed DeliveryStatus = "failed"

	// StatusExhausted indique que le nombre maximum de tentatives a ete atteint.
	StatusExhausted DeliveryStatus = "exhausted"
)

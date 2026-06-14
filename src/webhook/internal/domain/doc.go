// Package domain contient les entites, value objects et regles metier pures
// du service webhook (couche 1).
//
// Ce package n'importe aucun package externe ni de framework.
// Il definit : WebhookEndpoint, WebhookDelivery, DeliveryStatus,
// la logique de signature HMAC-SHA256 et les erreurs metier.
//
// Voir .ia/ARCHITECTURE.md pour la regle de dependance Clean Architecture.
package domain

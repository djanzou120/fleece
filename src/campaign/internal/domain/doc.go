// Package domain contient les entites, value objects, machines a etats et erreurs
// metier du service Campaign. Il est pur : zero import externe, zero I/O.
//
// Entites :
//   - Campaign : campagne marketing (machine a etats draft→scheduled→running→completed/failed/paused/cancelled).
//   - CampaignRecipient : destinataire d'une campagne (pending/sent/delivered/failed).
//   - CampaignRun : execution d'une campagne (compteurs total/sent/delivered/failed).
//
// Value objects :
//   - Money : montant en centimes + devise (coherent avec le service wallet).
//   - CampaignStatus, RecipientStatus : enumerations de statuts.
//
// Regles metier cles :
//   - draft→scheduled exige message_body != "" ET scheduled_at != nil.
//   - Toutes les autres transitions non declarees retournent ErrInvalidTransition.
package domain

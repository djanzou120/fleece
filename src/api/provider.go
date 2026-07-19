package api

import "context"

// ProviderResult est le resultat d'un envoi via un provider.
type ProviderResult struct {
	ExternalID string
	Status     string // "sent", "failed", "rejected"
	Cost       int64  // en centimes
}

// Provider est le port d'envoi de messages via un fournisseur externe.
// Chaque implementation concrete (SMS, WhatsApp, Telegram…) vit dans
// src/api/providers/ et est couplee a cette interface uniquement.
type Provider interface {
	Send(ctx context.Context, to, body string) (ProviderResult, error)
	EstimateCost(to, body string) int64
	GetDeliveryStatus(ctx context.Context, externalID string) (string, error)
}

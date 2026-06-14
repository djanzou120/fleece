// Package output — ports pilotes : interfaces requises par les use cases (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package output

import (
	"context"

	"fleece/src/provider/internal/domain"
)

// Provider est le port central definissant le contrat d'un fournisseur de messagerie.
// Chaque fournisseur concret (WhatsApp Meta, SMS Twilio, …) implemente cette interface
// en couche 3 (adapters/providers). L'ajout d'un fournisseur = ajouter un fichier en C3
// sans modifier aucune couche interne.
type Provider interface {
	// Send envoie un message sortant via le fournisseur.
	// Retourne l'identifiant externe attribue par le fournisseur (non vide en cas de succes).
	// Retourne domain.ErrProviderUnavailable si le fournisseur est injoignable.
	// Retourne domain.ErrSendFailed pour toute autre erreur d'envoi.
	Send(ctx context.Context, msg domain.OutboundMessage) (externalId string, err error)

	// EstimateCost retourne le cout estimatif d'un message pour le canal et le pays donnes.
	// Retourne domain.ErrProviderUnavailable si le fournisseur ne peut pas repondre.
	EstimateCost(ctx context.Context, channel, country string) (domain.Money, error)

	// GetDeliveryStatus interroge le fournisseur pour obtenir le statut de livraison
	// d'un message identifie par son externalId (identifiant fournisseur).
	// Retourne domain.ErrProviderUnavailable si le fournisseur est injoignable.
	GetDeliveryStatus(ctx context.Context, externalId string) (domain.DeliveryStatus, error)
}

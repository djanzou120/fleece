// Package input — ports pilotants : interfaces exposees par le service (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package input

import (
	"context"

	"fleece/src/provider/internal/domain"
)

// DispatchUseCase orchestre l'envoi d'un message sortant via un fournisseur.
type DispatchUseCase interface {
	// Execute selectionne le fournisseur, envoie le message, persiste le mapping
	// internalId↔externalId et publie un evenement.
	//
	// Retourne l'externalId attribue par le fournisseur.
	// Retourne domain.ErrProviderNotFound si aucun fournisseur ne correspond a msg.ProviderId.
	// Retourne domain.ErrSendFailed (wrappant la cause) si l'envoi echoue.
	Execute(ctx context.Context, msg domain.OutboundMessage) (externalId string, err error)
}

// HandleDLRUseCase traite un delivery receipt (DLR) entrant depuis un fournisseur.
type HandleDLRUseCase interface {
	// Execute met a jour le statut de livraison d'un message identifie par son externalId,
	// publie un evenement "provider.delivered" ou "provider.failed".
	//
	// TODO(persistence): le schema 0005 ne contient pas de colonne status dans
	// provider_messages. Le statut est donc uniquement mis a jour en memoire et
	// publie via evenement. Une colonne status devra etre ajoutee en migration future.
	Execute(ctx context.Context, externalId string, status domain.DeliveryStatus) error
}

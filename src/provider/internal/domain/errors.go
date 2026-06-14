// Package domain — entites, value objects, erreurs metier (couche 1).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package domain

import "errors"

// Erreurs metier du domaine provider.
var (
	// ErrProviderUnavailable est retournee quand un fournisseur est desactive
	// ou temporairement injoignable.
	ErrProviderUnavailable = errors.New("provider_unavailable")

	// ErrSendFailed est retournee quand l'envoi d'un message via un fournisseur a echoue.
	ErrSendFailed = errors.New("send_failed")

	// ErrProviderNotFound est retournee quand aucun fournisseur n'est enregistre
	// pour l'identifiant ou le canal demande.
	ErrProviderNotFound = errors.New("provider_not_found")
)

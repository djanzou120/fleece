package providers

import "errors"

// Erreurs internes du package providers.
// Ces sentinelles sont equivalentes aux erreurs du domaine provider mais sont
// locales au package afin d'eviter toute dependance vers src/provider.
var (
	// errSendFailed est retournee quand l'envoi d'un message via un fournisseur a echoue.
	errSendFailed = errors.New("send_failed")

	// errProviderUnavailable est retournee quand un fournisseur est desactive
	// ou temporairement injoignable.
	errProviderUnavailable = errors.New("provider_unavailable")
)

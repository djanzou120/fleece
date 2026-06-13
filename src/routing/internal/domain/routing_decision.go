package domain

// ProviderRef est une reference legere vers un provider de repli.
type ProviderRef struct {
	// ProviderID est l'identifiant du provider.
	ProviderID string
}

// RoutingDecision est le resultat de la selection d'un provider par SelectProvider.
// Elle contient le provider choisi ainsi que la chaine de repli ordonnee.
type RoutingDecision struct {
	// ProviderID est l'identifiant du provider selectionne.
	ProviderID string
	// Channel est le canal de communication.
	Channel Channel
	// EstimatedCost est le cout estime de l'envoi (unitaire ou total selon l'appelant).
	EstimatedCost Money
	// Strategy est la strategie appliquee pour cette decision.
	Strategy RoutingStrategy
	// FallbackChain contient les providers de repli dans l'ordre de preference,
	// excluant le provider principal.
	FallbackChain []ProviderRef
}

package domain

// ProviderPricing represente le tarif d'un provider pour un canal et un pays donnés.
// Aligne sur la table routing.provider_pricing (schema 0004).
//
// Note devise : la table ne stocke pas la devise ; elle est injectee par l'adapter
// de persistence via la constante DefaultCurrency de la config (ex. "XAF").
type ProviderPricing struct {
	// ID est l'identifiant technique (bigserial) de la ligne.
	ID int64
	// ProviderID est l'identifiant du provider (ex. "orange_cm", "twilio").
	ProviderID string
	// Channel est le canal de communication.
	Channel Channel
	// Country est le code pays (ISO 3166-1 alpha-2, ex. "CM", "SN").
	Country string
	// Cost est le cout unitaire d'envoi d'un message (en centimes, devise injectee par l'adapter).
	Cost Money
}

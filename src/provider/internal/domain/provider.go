package domain

// Provider represente un fournisseur de messagerie enregistre dans le systeme.
// Seuls Id, Channel et Active sont persistes (schema 0005 : table provider.providers).
type Provider struct {
	// Id est l'identifiant unique du fournisseur (PK dans provider.providers).
	Id string

	// Name est le nom d'affichage du fournisseur.
	// non persiste (absent du schema 0005)
	Name string

	// Channel est le canal de communication supporte (ex. "whatsapp", "sms").
	// Persiste dans la colonne provider.providers.channel.
	Channel string

	// Country est le code ISO 3166-1 alpha-2 du pays cible (ex. "CM", "FR").
	// non persiste (absent du schema 0005)
	Country string

	// Active indique si le fournisseur est actif.
	// Mappe sur la colonne provider.providers.enabled.
	Active bool
}

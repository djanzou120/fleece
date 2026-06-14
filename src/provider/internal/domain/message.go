package domain

// OutboundMessage represente un message sortant a envoyer via un fournisseur.
type OutboundMessage struct {
	// Id est l'identifiant interne du message (UUID, genere par le service appelant).
	Id string

	// ProviderId est l'identifiant du fournisseur cible.
	ProviderId string

	// Channel est le canal de communication (ex. "whatsapp", "sms").
	Channel string

	// Recipient est le destinataire (numero de telephone ou identifiant canal).
	Recipient string

	// Content est le contenu textuel du message.
	Content string

	// Metadata contient des metadonnees optionnelles (ex. template_id, language).
	Metadata map[string]string
}

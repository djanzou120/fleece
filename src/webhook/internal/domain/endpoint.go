package domain

// WebhookEndpoint represente un endpoint enregistre par un workspace pour recevoir
// des evenements de la plateforme Fleece.
type WebhookEndpoint struct {
	// ID est l'identifiant unique de l'endpoint (UUID).
	ID string

	// WorkspaceID identifie le workspace proprietaire de cet endpoint.
	WorkspaceID string

	// URL est l'adresse a laquelle les evenements sont envoyes via HTTP POST.
	URL string

	// Secret est la cle HMAC-SHA256 utilisee pour signer le payload.
	Secret string

	// Events liste les types d'evenements auxquels cet endpoint est abonne.
	// Exemples : "message.delivered", "wallet.debited".
	Events []string

	// Active indique si l'endpoint est actif.
	// NOTE(schema): colonne "active" absente de 0006 — non persistee.
	// A la lecture depuis Postgres, l'endpoint est considere actif par defaut (true).
	Active bool
}

// Subscribes retourne true si l'endpoint est abonne a l'evenement donne
// et qu'il est actif.
func (e *WebhookEndpoint) Subscribes(event string) bool {
	if !e.Active {
		return false
	}
	for _, ev := range e.Events {
		if ev == event {
			return true
		}
	}
	return false
}

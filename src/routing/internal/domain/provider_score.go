package domain

// ProviderScore represente le score de delivrabilite d'un provider sur un canal.
// Aligne sur la table routing.provider_scores (schema 0004).
//
// Note : la table ne stocke pas de champ country ni avg_latency_ms ; l'entite
// reste simple et aligne sur le schema reel.
type ProviderScore struct {
	// ProviderID est l'identifiant du provider.
	ProviderID string
	// Channel est le canal de communication.
	Channel Channel
	// Score est le taux de delivrabilite de 0 a 100.
	// Un provider sans score connu est traite comme ayant un score de 0.
	Score int
}

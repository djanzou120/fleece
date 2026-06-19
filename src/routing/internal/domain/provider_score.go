package domain

// ProviderScore represente le score de delivrabilite d'un provider sur un canal.
// Aligne sur la table routing.provider_scores (schema 0004 + migration 0013).
type ProviderScore struct {
	// ProviderID est l'identifiant du provider.
	ProviderID string
	// Channel est le canal de communication.
	Channel Channel
	// Score est le taux de delivrabilite de 0 a 100.
	// Un provider sans score connu est traite comme ayant un score de 0.
	Score int
	// AvgLatencyMs est la latence moyenne observee en millisecondes pour ce provider/canal.
	// 0 signifie aucune donnee de latence disponible.
	AvgLatencyMs int
	// Country est le code pays ISO 3166-1 alpha-2 optionnel.
	// Permet d'affiner les scores par pays si necessaire.
	Country string
}

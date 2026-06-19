package domain

// RoutingConfig est la configuration personnalisee d'une regle de routage Custom.
// Elle est stockee en JSONB dans la table routing.routing_rules (colonne config).
// Le champ "preferred_order" liste les provider IDs dans l'ordre de preference
// explicite du workspace. Les providers absents de cette liste sont tries en fin
// de chaine par score decroissant (HighestDelivery), puis par identifiant.
//
// Exemple :
//
//	{"preferred_order": ["twilio", "orange", "mtn"]}
type RoutingConfig map[string]any

// RoutingRule represente la regle de routage configuree pour un workspace.
// Aligne sur la table routing.routing_rules (schema 0004 + migration 0013).
type RoutingRule struct {
	// WorkspaceID est l'identifiant du workspace (UUID).
	WorkspaceID string
	// Strategy est la strategie de routage choisie par le workspace.
	Strategy RoutingStrategy
	// Channel est le canal de communication concerne par cette regle.
	// Chaine vide signifie que la regle s'applique a tous les canaux.
	Channel string
	// Config est la configuration personnalisee de la strategie Custom.
	// Nil si la strategie n'est pas Custom ou si aucune config n'est definie.
	Config RoutingConfig
}

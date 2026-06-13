package domain

// RoutingRule represente la regle de routage configuree pour un workspace.
// Aligne sur la table routing.routing_rules (schema 0004).
//
// Note : la table ne stocke ni channel ni config JSON ; l'entite
// reste simple et aligne sur le schema reel.
type RoutingRule struct {
	// WorkspaceID est l'identifiant du workspace (UUID).
	WorkspaceID string
	// Strategy est la strategie de routage choisie par le workspace.
	Strategy RoutingStrategy
}

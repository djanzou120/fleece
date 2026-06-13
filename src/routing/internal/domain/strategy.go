package domain

import "fmt"

// RoutingStrategy represente la strategie de selection du provider.
// Les valeurs correspondent aux chaînes stockees en base de donnees (snake_case).
type RoutingStrategy string

const (
	// StrategyLowestCost selectionne le provider au cout le plus bas.
	StrategyLowestCost RoutingStrategy = "lowest_cost"
	// StrategyHighestDelivery selectionne le provider avec le meilleur taux de delivrabilite.
	StrategyHighestDelivery RoutingStrategy = "highest_delivery"
	// StrategyFastest selectionne le provider le plus rapide.
	// NOTE : le schema 0004 ne stocke pas de metrique de latence ; cette strategie
	// retombe sur le comportement de HighestDelivery (tri par score decroissant).
	// TODO(latence): implementer la strategie reelle une fois la colonne avg_latency_ms ajoutee.
	StrategyFastest RoutingStrategy = "fastest"
	// StrategyCustom est une strategie personnalisee par workspace.
	// NOTE : le schema 0004 ne stocke pas de configuration JSON ; cette strategie
	// retombe actuellement sur HighestDelivery.
	// TODO(custom): implementer la strategie reelle une fois la colonne config ajoutee.
	StrategyCustom RoutingStrategy = "custom"
)

// ParseStrategy valide et retourne la RoutingStrategy correspondant a s.
// Retourne ErrInvalidStrategy si s n'est pas une strategie reconnue.
func ParseStrategy(s string) (RoutingStrategy, error) {
	switch RoutingStrategy(s) {
	case StrategyLowestCost, StrategyHighestDelivery, StrategyFastest, StrategyCustom:
		return RoutingStrategy(s), nil
	default:
		return "", fmt.Errorf("routing: strategie inconnue %q: %w", s, ErrInvalidStrategy)
	}
}

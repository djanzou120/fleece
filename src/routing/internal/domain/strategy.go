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
	// StrategyFastest selectionne le provider avec la plus faible latence moyenne (avg_latency_ms ASC).
	// En cas d'egalite de latence (ou si toutes les latences sont a 0 — absence de donnee),
	// le tie-break est applique par score decroissant puis par cout croissant puis par identifiant.
	StrategyFastest RoutingStrategy = "fastest"
	// StrategyCustom est une strategie personnalisee par workspace.
	// Elle utilise la liste "preferred_order" de RoutingRule.Config pour classer
	// les providers dans un ordre explicite. Les providers absents de cette liste
	// sont apposes en fin de chaine, tries par score decroissant.
	// Si Config est nil ou ne contient pas "preferred_order", retombe sur HighestDelivery.
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

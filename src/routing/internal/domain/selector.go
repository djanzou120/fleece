package domain

import (
	"fmt"
	"sort"
)

// SelectProvider selectionne le meilleur provider parmi les candidats selon la strategie donnee.
//
// Parametres :
//   - candidates : liste des tarifs disponibles pour le canal et le pays demandes.
//   - scores     : liste des scores de delivrabilite pour le canal. Un provider absent
//     de cette liste est traite comme ayant un score de 0.
//   - strategy   : strategie de selection a appliquer.
//
// Retourne ErrNoProviderAvailable si candidates est vide.
// Retourne ErrInvalidStrategy si la strategie n'est pas reconnue.
//
// La RoutingDecision retournee contient :
//   - le provider selectionne (1er dans l'ordre de tri),
//   - FallbackChain = les providers suivants dans l'ordre de tri (repli ordonne).
func SelectProvider(candidates []ProviderPricing, scores []ProviderScore, strategy RoutingStrategy) (RoutingDecision, error) {
	if len(candidates) == 0 {
		return RoutingDecision{}, fmt.Errorf("selector: %w", ErrNoProviderAvailable)
	}

	// Construire un index score par providerId pour un acces O(1).
	scoreMap := make(map[string]int, len(scores))
	for _, s := range scores {
		scoreMap[s.ProviderID] = s.Score
	}

	// Copier les candidats pour ne pas modifier le slice original.
	sorted := make([]ProviderPricing, len(candidates))
	copy(sorted, candidates)

	switch strategy {
	case StrategyLowestCost:
		sortLowestCost(sorted)

	case StrategyHighestDelivery:
		sortHighestDelivery(sorted, scoreMap)

	case StrategyFastest:
		// NOTE : le schema routing.provider_scores (0004) ne stocke pas de metrique
		// de latence (pas de colonne avg_latency_ms). En l'absence de cette donnee,
		// la strategie Fastest retombe sur le tri par score decroissant (highest delivery).
		// TODO(latence): implementer le tri reel par latence une fois la colonne ajoutee.
		sortHighestDelivery(sorted, scoreMap)

	case StrategyCustom:
		// NOTE : le schema routing.routing_rules (0004) ne stocke pas de configuration
		// JSON par workspace. En l'absence de cette donnee, Custom retombe sur HighestDelivery.
		// TODO(custom): implementer la logique personnalisee une fois la colonne config ajoutee.
		sortHighestDelivery(sorted, scoreMap)

	default:
		return RoutingDecision{}, fmt.Errorf("selector: strategie inconnue %q: %w", strategy, ErrInvalidStrategy)
	}

	best := sorted[0]

	// Construire la chaine de repli (tous sauf le premier).
	fallback := make([]ProviderRef, 0, len(sorted)-1)
	for _, p := range sorted[1:] {
		fallback = append(fallback, ProviderRef{ProviderID: p.ProviderID})
	}

	return RoutingDecision{
		ProviderID:    best.ProviderID,
		Channel:       best.Channel,
		EstimatedCost: best.Cost,
		Strategy:      strategy,
		FallbackChain: fallback,
	}, nil
}

// sortLowestCost trie par cout croissant, tie-break par providerId (ordre lexicographique).
func sortLowestCost(candidates []ProviderPricing) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Cost.Amount != candidates[j].Cost.Amount {
			return candidates[i].Cost.Amount < candidates[j].Cost.Amount
		}
		return candidates[i].ProviderID < candidates[j].ProviderID
	})
}

// sortHighestDelivery trie par score decroissant, tie-break par cout croissant puis par providerId.
// Un provider absent du scoreMap est considere avec un score de 0.
func sortHighestDelivery(candidates []ProviderPricing, scoreMap map[string]int) {
	sort.SliceStable(candidates, func(i, j int) bool {
		si := scoreMap[candidates[i].ProviderID]
		sj := scoreMap[candidates[j].ProviderID]
		if si != sj {
			return si > sj // decroissant
		}
		// Tie-break 1 : cout croissant.
		if candidates[i].Cost.Amount != candidates[j].Cost.Amount {
			return candidates[i].Cost.Amount < candidates[j].Cost.Amount
		}
		// Tie-break 2 : lexicographique pour determinisme.
		return candidates[i].ProviderID < candidates[j].ProviderID
	})
}

package domain

import (
	"fmt"
	"sort"
)

// SelectProvider selectionne le meilleur provider parmi les candidats selon la regle donnee.
//
// Parametres :
//   - candidates : liste des tarifs disponibles pour le canal et le pays demandes.
//   - scores     : liste des scores de delivrabilite pour le canal. Un provider absent
//     de cette liste est traite comme ayant un score de 0 et une latence de 0.
//   - rule       : regle de routage du workspace (strategy + config optionnelle).
//
// Retourne ErrNoProviderAvailable si candidates est vide.
// Retourne ErrInvalidStrategy si la strategie n'est pas reconnue.
//
// La RoutingDecision retournee contient :
//   - le provider selectionne (1er dans l'ordre de tri),
//   - FallbackChain = les providers suivants dans l'ordre de tri (repli ordonne).
func SelectProvider(candidates []ProviderPricing, scores []ProviderScore, rule RoutingRule) (RoutingDecision, error) {
	if len(candidates) == 0 {
		return RoutingDecision{}, fmt.Errorf("selector: %w", ErrNoProviderAvailable)
	}

	// Construire un index score par providerId pour un acces O(1).
	scoreMap := make(map[string]int, len(scores))
	latencyMap := make(map[string]int, len(scores))
	for _, s := range scores {
		scoreMap[s.ProviderID] = s.Score
		latencyMap[s.ProviderID] = s.AvgLatencyMs
	}

	// Copier les candidats pour ne pas modifier le slice original.
	sorted := make([]ProviderPricing, len(candidates))
	copy(sorted, candidates)

	switch rule.Strategy {
	case StrategyLowestCost:
		sortLowestCost(sorted)

	case StrategyHighestDelivery:
		sortHighestDelivery(sorted, scoreMap)

	case StrategyFastest:
		sortFastest(sorted, latencyMap, scoreMap)

	case StrategyCustom:
		sortCustom(sorted, rule.Config, scoreMap)

	default:
		return RoutingDecision{}, fmt.Errorf("selector: strategie inconnue %q: %w", rule.Strategy, ErrInvalidStrategy)
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
		Strategy:      rule.Strategy,
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

// sortFastest trie par latence moyenne croissante (avg_latency_ms ASC).
//
// Semantique du tie-break :
//  1. Latence croissante (valeur la plus basse = provider le plus rapide).
//  2. En cas d'egalite de latence (y compris latence = 0, ce qui signifie
//     absence de donnee), le tie-break est applique par score decroissant
//     pour privilegier le provider le plus fiable parmi les equally-fast.
//  3. Deuxieme tie-break : cout croissant.
//  4. Troisieme tie-break : ordre lexicographique de l'identifiant pour garantir
//     le determinisme.
//
// Consequence : si toutes les latences sont a 0 (aucune donnee), le comportement
// est identique a HighestDelivery, ce qui constitue un repli safe et documente.
func sortFastest(candidates []ProviderPricing, latencyMap, scoreMap map[string]int) {
	sort.SliceStable(candidates, func(i, j int) bool {
		li := latencyMap[candidates[i].ProviderID]
		lj := latencyMap[candidates[j].ProviderID]
		if li != lj {
			return li < lj // croissant : latence plus basse = plus rapide
		}
		// Tie-break 1 : score decroissant.
		si := scoreMap[candidates[i].ProviderID]
		sj := scoreMap[candidates[j].ProviderID]
		if si != sj {
			return si > sj
		}
		// Tie-break 2 : cout croissant.
		if candidates[i].Cost.Amount != candidates[j].Cost.Amount {
			return candidates[i].Cost.Amount < candidates[j].Cost.Amount
		}
		// Tie-break 3 : lexicographique.
		return candidates[i].ProviderID < candidates[j].ProviderID
	})
}

// sortCustom applique un ordre de preference explicite issu de RoutingConfig.
//
// Semantique de Config["preferred_order"] :
//   - Si Config est nil ou ne contient pas la cle "preferred_order", retombe sur HighestDelivery.
//   - "preferred_order" doit etre une []any dont chaque element est un string (provider ID).
//   - Les providers presents dans preferred_order sont classes en tete dans l'ordre donne.
//   - Les providers absents de preferred_order sont apposes en fin de chaine, tries par
//     score decroissant (HighestDelivery) pour garantir un repli coherent.
//   - Un element non-string dans preferred_order est ignore.
func sortCustom(candidates []ProviderPricing, config RoutingConfig, scoreMap map[string]int) {
	// Extraire l'ordre preferentiel depuis la config.
	preferredOrder := extractPreferredOrder(config)
	if len(preferredOrder) == 0 {
		// Pas de preferred_order : retombe sur HighestDelivery.
		sortHighestDelivery(candidates, scoreMap)
		return
	}

	// Construire un index de rang (position dans preferredOrder), O(1) par lookup.
	rankMap := make(map[string]int, len(preferredOrder))
	for i, id := range preferredOrder {
		rankMap[id] = i
	}

	// Les providers hors preferred_order recoivent un rang tres grand
	// pour etre places apres tous les providers de la liste.
	const notPreferred = 1<<31 - 1 // math.MaxInt32

	sort.SliceStable(candidates, func(i, j int) bool {
		ri, inI := rankMap[candidates[i].ProviderID]
		rj, inJ := rankMap[candidates[j].ProviderID]

		// Cas 1 : les deux sont dans preferred_order → comparer par rang.
		if inI && inJ {
			return ri < rj
		}
		// Cas 2 : seulement i est dans preferred_order → i gagne.
		if inI {
			return true
		}
		// Cas 3 : seulement j est dans preferred_order → j gagne.
		if inJ {
			return false
		}
		// Cas 4 : aucun dans preferred_order → tri par score decroissant (HighestDelivery).
		_ = notPreferred
		si := scoreMap[candidates[i].ProviderID]
		sj := scoreMap[candidates[j].ProviderID]
		if si != sj {
			return si > sj
		}
		if candidates[i].Cost.Amount != candidates[j].Cost.Amount {
			return candidates[i].Cost.Amount < candidates[j].Cost.Amount
		}
		return candidates[i].ProviderID < candidates[j].ProviderID
	})
}

// extractPreferredOrder extrait la liste des provider IDs depuis RoutingConfig.
// Retourne nil si Config est nil, si la cle "preferred_order" est absente,
// ou si la valeur n'est pas une liste de strings.
func extractPreferredOrder(config RoutingConfig) []string {
	if config == nil {
		return nil
	}
	raw, ok := config["preferred_order"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if ok && s != "" {
			result = append(result, s)
		}
	}
	return result
}

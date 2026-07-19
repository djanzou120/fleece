package api

import (
	"errors"
	"sort"
)

// ErrNoProvider est retournee quand la liste de providers est vide.
var ErrNoProvider = errors.New("aucun provider disponible")

// ErrInvalidStrategy est retournee quand la strategie de routage est inconnue.
var ErrInvalidStrategy = errors.New("strategie de routage inconnue")

// ProviderScore represente le score d'un provider pour une decision de routage.
// Le champ Cost exprime le cout unitaire en centimes (peut etre 0 si inconnu).
// Il est utilise par la strategie "lowest_cost" pour trier par cout croissant.
type ProviderScore struct {
	ProviderID string
	Channel    string
	Score      int   // 0–100 ; score de delivrabilite historique
	Cost       int64 // en centimes ; 0 = inconnu
}

// SelectProvider choisit le provider optimal selon la strategie donnee.
//
// Strategies reconnues :
//
//   - "highest_delivery" : provider avec le Score le plus eleve.
//     Tie-break par ordre lexicographique croissant de ProviderID (deterministe).
//
//   - "lowest_cost" : provider avec le cout (Cost) le plus bas.
//     Tie-break par ProviderID lexicographique (deterministe).
//     NOTE : le champ Cost doit etre renseigne par l'appelant (ex. lecture de
//     routing.provider_pricing en Phase 2). Si tous les couts sont 0 (inconnus),
//     le tri est entierement determine par le tie-break ProviderID — ce qui reste
//     deterministe et testable. Le filtrage des providers sans tarification sera
//     gere au niveau de l'endpoint HTTP Phase 2 (M-013).
//
//   - "round_robin" : sans etat partage, un vrai round-robin est impossible dans
//     une fonction pure. La fonction retourne le premier provider dans l'ordre
//     lexicographique de ProviderID (tri stable, deterministe et testable).
//     Le round-robin stateful reel sera gere a l'endpoint Phase 2 via un compteur
//     atomique ou une table de curseur en base.
//
// Retourne ErrNoProvider si providers est vide, ErrInvalidStrategy si la strategie
// est inconnue.
func SelectProvider(providers []ProviderScore, strategy string) (string, error) {
	if len(providers) == 0 {
		return "", ErrNoProvider
	}

	switch strategy {
	case "highest_delivery":
		return selectHighestDelivery(providers), nil

	case "lowest_cost":
		return selectLowestCost(providers), nil

	case "round_robin":
		// Sélection deterministe : premier provider dans l'ordre lexicographique.
		// Le round-robin stateful reel sera implemente a l'endpoint Phase 2.
		return selectLexFirst(providers), nil

	default:
		return "", ErrInvalidStrategy
	}
}

// selectHighestDelivery retourne le ProviderID ayant le Score le plus eleve.
// Tie-break : ordre lexicographique croissant de ProviderID.
func selectHighestDelivery(providers []ProviderScore) string {
	sorted := make([]ProviderScore, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score > sorted[j].Score // score decroissant
		}
		return sorted[i].ProviderID < sorted[j].ProviderID // tie-break lexicographique
	})
	return sorted[0].ProviderID
}

// selectLowestCost retourne le ProviderID ayant le Cost le plus bas.
// Tie-break : ordre lexicographique croissant de ProviderID.
func selectLowestCost(providers []ProviderScore) string {
	sorted := make([]ProviderScore, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Cost != sorted[j].Cost {
			return sorted[i].Cost < sorted[j].Cost // cout croissant
		}
		return sorted[i].ProviderID < sorted[j].ProviderID // tie-break lexicographique
	})
	return sorted[0].ProviderID
}

// selectLexFirst retourne le ProviderID qui vient en premier dans l'ordre
// lexicographique. Utilise pour round_robin (deterministe, sans etat).
func selectLexFirst(providers []ProviderScore) string {
	best := providers[0].ProviderID
	for _, p := range providers[1:] {
		if p.ProviderID < best {
			best = p.ProviderID
		}
	}
	return best
}

// CombineScore fusionne score provider et score contact (50/50).
//
// Formule : clamp((clamp(providerScore,0,100) + clamp(contactScore,0,100) + 1) / 2, 0, 100)
//
// Le +1 avant la division entiere assure un arrondi round-half-up.
// Les entrees hors [0,100] sont bornees avant calcul.
//
// Identique a l'original src/routing/internal/domain/contact_score.go :CombineScore.
//
// Exemple :
//
//	providerScore=90, contactScore=40 → (90+40+1)/2 = 65
//	providerScore=70, contactScore=40 → (70+40+1)/2 = 55
func CombineScore(providerScore, contactScore int) int {
	ps := clamp(providerScore, 0, 100)
	cs := clamp(contactScore, 0, 100)
	combined := (ps + cs + 1) / 2
	return clamp(combined, 0, 100)
}

// clamp borne v dans [min, max].
// Defini une seule fois dans ce fichier ; reutilise par scorer.go.
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

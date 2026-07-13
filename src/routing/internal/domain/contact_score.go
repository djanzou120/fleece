package domain

// CombineScore calcule un score combine a partir du score provider et du score contact.
//
// Semantique retenue (option b — fonction pure du domaine, use case orchestre) :
//
//   - Le score provider (providerScore) mesure la fiabilite historique du provider sur
//     le canal (base de donnees routing.provider_scores).
//   - Le score contact (contactScore) mesure la joignabilite du destinataire sur ce canal
//     (service contact-intelligence, Laplace α=1 : 0=jamais joignable, 100=toujours
//     joignable, 50=inconnu ou neutre).
//
// Formule : combined = round(0.5 * providerScore + 0.5 * contactScore), borne [0, 100].
//
// Sémantique canal vs provider :
//
//	Le contactScore s'applique au CANAL, pas au provider individuel.
//	Tous les providers du meme canal recoivent le meme bonus/malus contact.
//	Le tri final reste donc discrimine par le providerScore (le meilleur provider
//	sur un canal peu joignable conserve un avantage relatif sur les autres providers
//	du meme canal), mais le canal globalement peu joignable est penalise par rapport
//	a un canal ou le contact est facilement joignable.
//
// Exemple :
//
//	providers SMS : p1 score=90, p2 score=70
//	contactScore SMS = 40 (peu joignable sur SMS)
//	→ p1 combined = round(0.5*90 + 0.5*40) = 65
//	→ p2 combined = round(0.5*70 + 0.5*40) = 55
//	Le tri final : p1 > p2 (order preserve), mais les 2 penalises vs canal WhatsApp score=80.
//
// Les scores hors [0,100] sont bornes en entree pour garantir la propriete du resultat.
func CombineScore(providerScore, contactScore int) int {
	// Borner les entrees pour garantir que le resultat est dans [0,100].
	ps := clamp(providerScore, 0, 100)
	cs := clamp(contactScore, 0, 100)

	// Moyenne ponderee : 50% provider + 50% contact.
	// Utilise l'arithmetique entiere avec arrondi correct (round-half-up).
	combined := (ps + cs + 1) / 2 // +1 avant /2 = arrondi vers le haut (round half up)
	return clamp(combined, 0, 100)
}

// clamp borne v dans [min, max].
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

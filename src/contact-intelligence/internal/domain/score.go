package domain

// DeliveryScore est un value object representant un score de delivrabilite
// borne entre 0 et 100 (inclus). L'invariant est verifie a la construction.
//
// Formule de scoring (lissage de Laplace) :
//
//	score = (successCount + 1) / (successCount + failureCount + 2) * 100
//
// Le lissage de Laplace (additive smoothing, alpha=1) evite les scores extremes
// (0 ou 100) sur les petits echantillons :
//   - 0 succes, 0 echec  → score ≈ 50 (prior neutre)
//   - 1 succes, 0 echec  → score ≈ 67 (confiance moderee)
//   - 10 succes, 0 echec → score ≈ 92 (haute confiance)
//   - 0 succes, 10 ech.  → score ≈  8 (mauvais canal, mais pas 0)
//
// Proprietes :
//   - Deterministique : meme entrees → meme sortie.
//   - Bornee : 0 < score < 100 (jamais 0 ni 100 sur donnees reelles).
//   - Monotone : plus de succes → score plus eleve.
type DeliveryScore struct {
	value int
}

// NewDeliveryScore cree un DeliveryScore depuis une valeur entiere [0, 100].
// Si la valeur est hors bornes elle est clampee.
func NewDeliveryScore(v int) DeliveryScore {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return DeliveryScore{value: v}
}

// Value retourne la valeur entiere du score.
func (d DeliveryScore) Value() int { return d.value }

// ComputeScore calcule le score de delivrabilite par lissage de Laplace.
// successCount et failureCount doivent etre >= 0.
// Retourne un DeliveryScore dans [0, 100].
func ComputeScore(successCount, failureCount int) DeliveryScore {
	if successCount < 0 {
		successCount = 0
	}
	if failureCount < 0 {
		failureCount = 0
	}
	// Formule : (s+1) / (s+f+2) * 100, arrondissement entier.
	numerator := (successCount + 1) * 100
	denominator := successCount + failureCount + 2
	score := numerator / denominator
	return NewDeliveryScore(score)
}

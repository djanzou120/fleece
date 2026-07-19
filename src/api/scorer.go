package api

// Laplace calcule un score de joignabilite par lissage de Laplace (alpha=1).
//
// Formule : floor((success+1)/(success+failure+2)*100), borne [0,100].
//
// Le lissage de Laplace (additive smoothing, alpha=1) evite les scores extremes
// sur les petits echantillons :
//   - success=0, failure=0  → (0+1)*100/(0+0+2) = 50  (prior neutre — naturel, pas de cas special)
//   - success=1, failure=0  → (1+1)*100/(1+0+2) = 66  (confiance moderee)
//   - success=10, failure=0 → (10+1)*100/(10+0+2) = 91 (haute confiance)
//   - success=0, failure=10 → (0+1)*100/(0+10+2) = 8  (mauvais canal)
//
// Les entrees negatives sont bornees a 0. Le resultat est garanti dans [0,100].
//
// Identique a ComputeScore dans src/contact-intelligence/internal/domain/score.go.
// Reutilise clamp defini dans routing.go.
func Laplace(success, failure int) int {
	if success < 0 {
		success = 0
	}
	if failure < 0 {
		failure = 0
	}
	numerator := (success + 1) * 100
	denominator := success + failure + 2
	score := numerator / denominator
	return clamp(score, 0, 100)
}

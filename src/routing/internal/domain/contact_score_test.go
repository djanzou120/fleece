package domain

import "testing"

// TestCombineScore_EqualWeight verifie la moyenne ponderee 50/50 de base.
func TestCombineScore_EqualWeight(t *testing.T) {
	cases := []struct {
		name          string
		providerScore int
		contactScore  int
		want          int
	}{
		{"deux scores egaux 80", 80, 80, 80},
		{"deux scores egaux 50", 50, 50, 50},
		{"provider haut contact bas", 90, 40, 65},          // (90+40+1)/2 = 65
		{"provider bas contact haut", 40, 90, 65},          // (40+90+1)/2 = 65
		{"contact neutre (50)", 90, 50, 70},                // (90+50+1)/2 = 70
		{"contact inconnu 50 (prior Laplace)", 70, 50, 60}, // (70+50+1)/2 = 60
		{"contact parfait 100", 80, 100, 90},               // (80+100+1)/2 = 90
		{"contact zero", 80, 0, 40},                        // (80+0+1)/2 = 40
		{"deux zeros", 0, 0, 0},
		{"deux cents", 100, 100, 100},
		{"arrondi impair", 70, 71, 71}, // (70+71+1)/2 = 71 (round half-up)
		{"arrondi pair", 70, 70, 70},   // (70+70+1)/2 = 70
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CombineScore(tc.providerScore, tc.contactScore)
			if got != tc.want {
				t.Errorf("CombineScore(%d, %d) = %d, want %d", tc.providerScore, tc.contactScore, got, tc.want)
			}
		})
	}
}

// TestCombineScore_Bounds verifie que les scores hors [0,100] sont bornes.
func TestCombineScore_Bounds(t *testing.T) {
	cases := []struct {
		name          string
		providerScore int
		contactScore  int
		wantMin       int
		wantMax       int
	}{
		{"entrees valides", 80, 60, 0, 100},
		{"provider negatif", -10, 50, 0, 100},
		{"provider > 100", 150, 50, 0, 100},
		{"contact negatif", 50, -5, 0, 100},
		{"contact > 100", 50, 200, 0, 100},
		{"tous deux hors bornes", -100, 200, 0, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CombineScore(tc.providerScore, tc.contactScore)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("CombineScore(%d, %d) = %d hors [%d, %d]", tc.providerScore, tc.contactScore, got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// TestCombineScore_Monotone verifie que le score augmente quand le contact est plus joignable.
func TestCombineScore_Monotone(t *testing.T) {
	// Avec le meme provider score, un contact plus joignable → combined plus eleve.
	providerScore := 70
	prev := CombineScore(providerScore, 0)
	for cs := 1; cs <= 100; cs++ {
		curr := CombineScore(providerScore, cs)
		if curr < prev {
			t.Errorf("monotonie brisee : CombineScore(%d, %d)=%d < CombineScore(%d, %d)=%d", providerScore, cs, curr, providerScore, cs-1, prev)
		}
		prev = curr
	}
}

// TestCombineScore_SemantiqueCanal verifie que deux providers du meme canal recoivent
// le meme bonus/malus contact, et que le tri relatif entre eux est preserve.
func TestCombineScore_SemantiqueCanal(t *testing.T) {
	// Deux providers sur le meme canal, contact peu joignable (score=30).
	p1Score := 90 // meilleur provider
	p2Score := 70 // provider moins bon
	contactScore := 30

	p1Combined := CombineScore(p1Score, contactScore) // (90+30+1)/2 = 60
	p2Combined := CombineScore(p2Score, contactScore) // (70+30+1)/2 = 50

	// L'ordre relatif des providers doit etre preserve apres combinaison.
	if p1Combined <= p2Combined {
		t.Errorf("ordre relatif brise : p1Combined=%d <= p2Combined=%d (attendu p1>p2)", p1Combined, p2Combined)
	}

	// Les deux sont penalises (combines < scores originaux).
	if p1Combined >= p1Score {
		t.Errorf("p1 aurait du etre penalise : combined=%d >= original=%d", p1Combined, p1Score)
	}
	if p2Combined >= p2Score {
		t.Errorf("p2 aurait du etre penalise : combined=%d >= original=%d", p2Combined, p2Score)
	}
}

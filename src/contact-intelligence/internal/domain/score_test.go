package domain

import "testing"

// TestComputeScore_PriorNeutre verifie que sans donnees, le score est proche de 50.
func TestComputeScore_PriorNeutre(t *testing.T) {
	s := ComputeScore(0, 0)
	// (0+1)/(0+0+2)*100 = 50
	if s.Value() != 50 {
		t.Errorf("prior neutre: attendu 50, obtenu %d", s.Value())
	}
}

// TestComputeScore_BorneMax verifie que le score ne depasse pas 100.
func TestComputeScore_BorneMax(t *testing.T) {
	// Tres grand nombre de succes.
	s := ComputeScore(1000, 0)
	if s.Value() > 100 {
		t.Errorf("borne max: score %d > 100", s.Value())
	}
}

// TestComputeScore_BorneMin verifie que le score ne tombe pas en dessous de 0.
func TestComputeScore_BorneMin(t *testing.T) {
	// Tres grand nombre d'echecs.
	s := ComputeScore(0, 1000)
	if s.Value() < 0 {
		t.Errorf("borne min: score %d < 0", s.Value())
	}
}

// TestComputeScore_Determinisme verifie que la meme entree produit toujours la meme sortie.
func TestComputeScore_Determinisme(t *testing.T) {
	for i := 0; i < 100; i++ {
		a := ComputeScore(10, 5)
		b := ComputeScore(10, 5)
		if a.Value() != b.Value() {
			t.Fatalf("non deterministe: %d != %d", a.Value(), b.Value())
		}
	}
}

// TestComputeScore_Monotonie verifie que plus de succes = score plus eleve.
func TestComputeScore_Monotonie(t *testing.T) {
	low := ComputeScore(1, 10).Value()
	mid := ComputeScore(5, 10).Value()
	high := ComputeScore(10, 5).Value()
	if !(low < mid && mid < high) {
		t.Errorf("non monotone: low=%d mid=%d high=%d", low, mid, high)
	}
}

// TestComputeScore_PetitEchantillon verifie le lissage sur les petits echantillons.
func TestComputeScore_PetitEchantillon(t *testing.T) {
	// 1 succes, 0 echec → ne doit pas etre 100
	s1 := ComputeScore(1, 0)
	if s1.Value() >= 100 {
		t.Errorf("1 succes ne doit pas donner 100, obtenu %d", s1.Value())
	}
	// 0 succes, 1 echec → ne doit pas etre 0
	s2 := ComputeScore(0, 1)
	if s2.Value() <= 0 {
		t.Errorf("1 echec ne doit pas donner 0, obtenu %d", s2.Value())
	}
}

// TestNewDeliveryScore_Clamping verifie le clampage des valeurs hors bornes.
func TestNewDeliveryScore_Clamping(t *testing.T) {
	cases := []struct {
		input    int
		expected int
	}{
		{-10, 0},
		{0, 0},
		{50, 50},
		{100, 100},
		{150, 100},
	}
	for _, tc := range cases {
		got := NewDeliveryScore(tc.input).Value()
		if got != tc.expected {
			t.Errorf("NewDeliveryScore(%d): attendu %d, obtenu %d", tc.input, tc.expected, got)
		}
	}
}

// TestComputeScore_Formule verifie quelques valeurs connues de la formule de Laplace.
func TestComputeScore_Formule(t *testing.T) {
	cases := []struct {
		s, f     int
		expected int
	}{
		{0, 0, 50},   // (1)/(2)*100 = 50
		{1, 0, 66},   // (2)/(3)*100 = 66
		{0, 1, 33},   // (1)/(3)*100 = 33
		{9, 1, 83},   // (10)/(12)*100 = 83
		{99, 1, 98},  // (100)/(102)*100 = 98
	}
	for _, tc := range cases {
		got := ComputeScore(tc.s, tc.f).Value()
		if got != tc.expected {
			t.Errorf("ComputeScore(%d,%d): attendu %d, obtenu %d", tc.s, tc.f, tc.expected, got)
		}
	}
}

package api

import (
	"errors"
	"testing"
)

func TestSelectProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []ProviderScore
		strategy  string
		wantID    string
		wantErr   error
	}{
		// --- highest_delivery ---
		{
			name: "highest_delivery choisit le score max",
			providers: []ProviderScore{
				{ProviderID: "twilio", Score: 70},
				{ProviderID: "vonage", Score: 90},
				{ProviderID: "sinch", Score: 80},
			},
			strategy: "highest_delivery",
			wantID:   "vonage",
		},
		{
			name: "highest_delivery tie-break lexicographique",
			providers: []ProviderScore{
				{ProviderID: "provider-b", Score: 85},
				{ProviderID: "provider-a", Score: 85},
				{ProviderID: "provider-c", Score: 85},
			},
			strategy: "highest_delivery",
			wantID:   "provider-a", // premier lexicographiquement
		},
		{
			name: "highest_delivery un seul provider",
			providers: []ProviderScore{
				{ProviderID: "only", Score: 50},
			},
			strategy: "highest_delivery",
			wantID:   "only",
		},
		// --- lowest_cost ---
		{
			name: "lowest_cost choisit le cout le plus bas",
			providers: []ProviderScore{
				{ProviderID: "expensive", Cost: 500},
				{ProviderID: "cheap", Cost: 100},
				{ProviderID: "mid", Cost: 300},
			},
			strategy: "lowest_cost",
			wantID:   "cheap",
		},
		{
			name: "lowest_cost tie-break lexicographique sur cout egal",
			providers: []ProviderScore{
				{ProviderID: "provider-z", Cost: 200},
				{ProviderID: "provider-a", Cost: 200},
			},
			strategy: "lowest_cost",
			wantID:   "provider-a",
		},
		{
			name: "lowest_cost couts tous nuls — tie-break lexicographique",
			providers: []ProviderScore{
				{ProviderID: "c-provider", Cost: 0},
				{ProviderID: "a-provider", Cost: 0},
				{ProviderID: "b-provider", Cost: 0},
			},
			strategy: "lowest_cost",
			wantID:   "a-provider",
		},
		// --- round_robin ---
		{
			name: "round_robin retourne le premier provider lexicographique",
			providers: []ProviderScore{
				{ProviderID: "zeta"},
				{ProviderID: "alpha"},
				{ProviderID: "gamma"},
			},
			strategy: "round_robin",
			wantID:   "alpha",
		},
		{
			name: "round_robin un seul provider",
			providers: []ProviderScore{
				{ProviderID: "solo"},
			},
			strategy: "round_robin",
			wantID:   "solo",
		},
		// --- erreurs ---
		{
			name:      "liste vide retourne ErrNoProvider",
			providers: []ProviderScore{},
			strategy:  "highest_delivery",
			wantErr:   ErrNoProvider,
		},
		{
			name: "strategie inconnue retourne ErrInvalidStrategy",
			providers: []ProviderScore{
				{ProviderID: "p1", Score: 50},
			},
			strategy: "unknown_strategy",
			wantErr:  ErrInvalidStrategy,
		},
		{
			name:      "nil providers retourne ErrNoProvider",
			providers: nil,
			strategy:  "highest_delivery",
			wantErr:   ErrNoProvider,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := SelectProvider(tc.providers, tc.strategy)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("SelectProvider() erreur = %v, voulu %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SelectProvider() erreur inattendue : %v", err)
			}
			if got != tc.wantID {
				t.Errorf("SelectProvider() = %q, voulu %q", got, tc.wantID)
			}
		})
	}
}

func TestCombineScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		providerScore int
		contactScore  int
		want          int
	}{
		// Cas de l'original contact_score.go
		{name: "ps=90 cs=40 → 65", providerScore: 90, contactScore: 40, want: 65},
		{name: "ps=70 cs=40 → 55", providerScore: 70, contactScore: 40, want: 55},
		// Bornes
		{name: "ps=100 cs=100 → 100", providerScore: 100, contactScore: 100, want: 100},
		{name: "ps=0 cs=0 → 0", providerScore: 0, contactScore: 0, want: 0},
		{name: "ps=50 cs=50 → 50", providerScore: 50, contactScore: 50, want: 50},
		// Arrondi round-half-up : (50+51+1)/2 = 51
		{name: "ps=50 cs=51 → 51", providerScore: 50, contactScore: 51, want: 51},
		// Entrees hors [0,100] clampees
		{name: "ps=-10 cs=50 → clamp ps=0 → 25", providerScore: -10, contactScore: 50, want: 25},
		{name: "ps=150 cs=50 → clamp ps=100 → 75", providerScore: 150, contactScore: 50, want: 75},
		{name: "ps=0 cs=-50 → clamp cs=0 → 0", providerScore: 0, contactScore: -50, want: 0},
		{name: "ps=100 cs=200 → clamp cs=100 → 100", providerScore: 100, contactScore: 200, want: 100},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CombineScore(tc.providerScore, tc.contactScore)
			if got != tc.want {
				t.Errorf("CombineScore(%d, %d) = %d, voulu %d",
					tc.providerScore, tc.contactScore, got, tc.want)
			}
		})
	}
}

package domain

import (
	"errors"
	"testing"
)

// helpers de construction
func mkPricing(providerID, channel, country string, cost int64, currency string) ProviderPricing {
	return ProviderPricing{
		ProviderID: providerID,
		Channel:    Channel(channel),
		Country:    country,
		Cost:       Money{Amount: cost, Currency: currency},
	}
}

func mkScore(providerID, channel string, score int) ProviderScore {
	return ProviderScore{
		ProviderID: providerID,
		Channel:    Channel(channel),
		Score:      score,
	}
}

// TestSelectProvider_NoCandidates verifie que ErrNoProviderAvailable est retourne.
func TestSelectProvider_NoCandidates(t *testing.T) {
	_, err := SelectProvider(nil, nil, StrategyHighestDelivery)
	if err == nil {
		t.Fatal("attendu ErrNoProviderAvailable")
	}
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Errorf("attendu ErrNoProviderAvailable, obtenu %v", err)
	}
}

func TestSelectProvider_EmptyCandidates(t *testing.T) {
	_, err := SelectProvider([]ProviderPricing{}, nil, StrategyLowestCost)
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Errorf("attendu ErrNoProviderAvailable, obtenu %v", err)
	}
}

// TestSelectProvider_InvalidStrategy verifie que ErrInvalidStrategy est retourne.
func TestSelectProvider_InvalidStrategy(t *testing.T) {
	candidates := []ProviderPricing{mkPricing("p1", "sms", "CM", 100, "XAF")}
	_, err := SelectProvider(candidates, nil, "unknown_strategy")
	if err == nil {
		t.Fatal("attendu ErrInvalidStrategy")
	}
	if !errors.Is(err, ErrInvalidStrategy) {
		t.Errorf("attendu ErrInvalidStrategy, obtenu %v", err)
	}
}

// TestSelectProvider_LowestCost verifie le tri par cout croissant.
func TestSelectProvider_LowestCost(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_expensive", "sms", "CM", 500, "XAF"),
		mkPricing("p_cheap", "sms", "CM", 100, "XAF"),
		mkPricing("p_medium", "sms", "CM", 300, "XAF"),
	}
	decision, err := SelectProvider(candidates, nil, StrategyLowestCost)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	if decision.ProviderID != "p_cheap" {
		t.Errorf("attendu p_cheap, obtenu %s", decision.ProviderID)
	}
	if decision.EstimatedCost.Amount != 100 {
		t.Errorf("cout attendu 100, obtenu %d", decision.EstimatedCost.Amount)
	}
	if decision.Strategy != StrategyLowestCost {
		t.Errorf("strategie attendue %s, obtenu %s", StrategyLowestCost, decision.Strategy)
	}
}

// TestSelectProvider_LowestCost_FallbackChain verifie l'ordre de la FallbackChain.
func TestSelectProvider_LowestCost_FallbackChain(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p3", "sms", "CM", 500, "XAF"),
		mkPricing("p1", "sms", "CM", 100, "XAF"),
		mkPricing("p2", "sms", "CM", 300, "XAF"),
	}
	decision, err := SelectProvider(candidates, nil, StrategyLowestCost)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	if decision.ProviderID != "p1" {
		t.Errorf("attendu p1, obtenu %s", decision.ProviderID)
	}
	if len(decision.FallbackChain) != 2 {
		t.Fatalf("attendu 2 fallbacks, obtenu %d", len(decision.FallbackChain))
	}
	if decision.FallbackChain[0].ProviderID != "p2" {
		t.Errorf("fallback[0] attendu p2, obtenu %s", decision.FallbackChain[0].ProviderID)
	}
	if decision.FallbackChain[1].ProviderID != "p3" {
		t.Errorf("fallback[1] attendu p3, obtenu %s", decision.FallbackChain[1].ProviderID)
	}
}

// TestSelectProvider_LowestCost_TieBreak verifie le tie-break par providerId.
func TestSelectProvider_LowestCost_TieBreak(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("provider_b", "sms", "CM", 100, "XAF"),
		mkPricing("provider_a", "sms", "CM", 100, "XAF"),
	}
	decision, err := SelectProvider(candidates, nil, StrategyLowestCost)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Tie-break lexicographique : provider_a < provider_b
	if decision.ProviderID != "provider_a" {
		t.Errorf("tie-break: attendu provider_a, obtenu %s", decision.ProviderID)
	}
}

// TestSelectProvider_HighestDelivery verifie le tri par score decroissant.
func TestSelectProvider_HighestDelivery(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_low", "sms", "CM", 100, "XAF"),
		mkPricing("p_high", "sms", "CM", 200, "XAF"),
		mkPricing("p_mid", "sms", "CM", 150, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_low", "sms", 40),
		mkScore("p_high", "sms", 95),
		mkScore("p_mid", "sms", 70),
	}
	decision, err := SelectProvider(candidates, scores, StrategyHighestDelivery)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	if decision.ProviderID != "p_high" {
		t.Errorf("attendu p_high, obtenu %s", decision.ProviderID)
	}
	if decision.Strategy != StrategyHighestDelivery {
		t.Errorf("strategie attendue %s, obtenu %s", StrategyHighestDelivery, decision.Strategy)
	}
}

// TestSelectProvider_HighestDelivery_FallbackChainOrder verifie l'ordre de la FallbackChain.
func TestSelectProvider_HighestDelivery_FallbackChainOrder(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_low", "sms", "CM", 100, "XAF"),
		mkPricing("p_high", "sms", "CM", 200, "XAF"),
		mkPricing("p_mid", "sms", "CM", 150, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_low", "sms", 40),
		mkScore("p_high", "sms", 95),
		mkScore("p_mid", "sms", 70),
	}
	decision, err := SelectProvider(candidates, scores, StrategyHighestDelivery)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	if len(decision.FallbackChain) != 2 {
		t.Fatalf("attendu 2 fallbacks, obtenu %d", len(decision.FallbackChain))
	}
	// Ordre decroissant de score : p_mid (70), puis p_low (40)
	if decision.FallbackChain[0].ProviderID != "p_mid" {
		t.Errorf("fallback[0] attendu p_mid, obtenu %s", decision.FallbackChain[0].ProviderID)
	}
	if decision.FallbackChain[1].ProviderID != "p_low" {
		t.Errorf("fallback[1] attendu p_low, obtenu %s", decision.FallbackChain[1].ProviderID)
	}
}

// TestSelectProvider_HighestDelivery_MissingScore verifie qu'un provider sans score = 0.
func TestSelectProvider_HighestDelivery_MissingScore(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_scored", "sms", "CM", 100, "XAF"),
		mkPricing("p_unscored", "sms", "CM", 50, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_scored", "sms", 60),
		// p_unscored absent : score implicite = 0
	}
	decision, err := SelectProvider(candidates, scores, StrategyHighestDelivery)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// p_scored (score=60) doit gagner sur p_unscored (score=0)
	if decision.ProviderID != "p_scored" {
		t.Errorf("attendu p_scored, obtenu %s", decision.ProviderID)
	}
}

// TestSelectProvider_Fastest_FallbackToHighestDelivery verifie que Fastest se comporte
// comme HighestDelivery (fallback documente, schema sans latence).
func TestSelectProvider_Fastest_FallbackToHighestDelivery(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_fast_score", "sms", "CM", 200, "XAF"),
		mkPricing("p_slow_score", "sms", "CM", 100, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_fast_score", "sms", 90),
		mkScore("p_slow_score", "sms", 30),
	}
	decision, err := SelectProvider(candidates, scores, StrategyFastest)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Sans latence : retombe sur highest_delivery → p_fast_score (score=90)
	if decision.ProviderID != "p_fast_score" {
		t.Errorf("attendu p_fast_score (fallback highest_delivery), obtenu %s", decision.ProviderID)
	}
	if decision.Strategy != StrategyFastest {
		t.Errorf("la strategie retournee doit etre Fastest, obtenu %s", decision.Strategy)
	}
}

// TestSelectProvider_SingleProvider verifie le cas d'un provider unique.
func TestSelectProvider_SingleProvider(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("only_one", "whatsapp", "SN", 250, "XAF"),
	}
	decision, err := SelectProvider(candidates, nil, StrategyLowestCost)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	if decision.ProviderID != "only_one" {
		t.Errorf("attendu only_one, obtenu %s", decision.ProviderID)
	}
	if len(decision.FallbackChain) != 0 {
		t.Errorf("FallbackChain doit etre vide pour un seul provider, obtenu %d", len(decision.FallbackChain))
	}
}

// TestSelectProvider_Custom_FallbackToHighestDelivery verifie que Custom retombe sur HighestDelivery.
func TestSelectProvider_Custom_FallbackToHighestDelivery(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_best", "sms", "CM", 300, "XAF"),
		mkPricing("p_cheap", "sms", "CM", 100, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_best", "sms", 88),
		mkScore("p_cheap", "sms", 20),
	}
	decision, err := SelectProvider(candidates, scores, StrategyCustom)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Sans config JSON : retombe sur highest_delivery → p_best (score=88)
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best (custom fallback), obtenu %s", decision.ProviderID)
	}
}

// TestSelectProvider_HighestDelivery_TieBreakByCost verifie le tie-break par cout
// quand deux providers ont le meme score.
func TestSelectProvider_HighestDelivery_TieBreakByCost(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_expensive", "sms", "CM", 500, "XAF"),
		mkPricing("p_cheap", "sms", "CM", 100, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_expensive", "sms", 80),
		mkScore("p_cheap", "sms", 80),
	}
	decision, err := SelectProvider(candidates, scores, StrategyHighestDelivery)
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Scores egaux → tie-break par cout croissant → p_cheap
	if decision.ProviderID != "p_cheap" {
		t.Errorf("tie-break cout: attendu p_cheap, obtenu %s", decision.ProviderID)
	}
}

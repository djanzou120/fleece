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

func mkScoreWithLatency(providerID, channel string, score, latencyMs int) ProviderScore {
	return ProviderScore{
		ProviderID:   providerID,
		Channel:      Channel(channel),
		Score:        score,
		AvgLatencyMs: latencyMs,
	}
}

// rule est un helper qui construit une RoutingRule minimale pour les tests.
func rule(strategy RoutingStrategy) RoutingRule {
	return RoutingRule{Strategy: strategy}
}

// ruleWithConfig construit une RoutingRule Custom avec une Config.
func ruleWithConfig(config RoutingConfig) RoutingRule {
	return RoutingRule{Strategy: StrategyCustom, Config: config}
}

// TestSelectProvider_NoCandidates verifie que ErrNoProviderAvailable est retourne.
func TestSelectProvider_NoCandidates(t *testing.T) {
	_, err := SelectProvider(nil, nil, rule(StrategyHighestDelivery))
	if err == nil {
		t.Fatal("attendu ErrNoProviderAvailable")
	}
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Errorf("attendu ErrNoProviderAvailable, obtenu %v", err)
	}
}

func TestSelectProvider_EmptyCandidates(t *testing.T) {
	_, err := SelectProvider([]ProviderPricing{}, nil, rule(StrategyLowestCost))
	if !errors.Is(err, ErrNoProviderAvailable) {
		t.Errorf("attendu ErrNoProviderAvailable, obtenu %v", err)
	}
}

// TestSelectProvider_InvalidStrategy verifie que ErrInvalidStrategy est retourne.
func TestSelectProvider_InvalidStrategy(t *testing.T) {
	candidates := []ProviderPricing{mkPricing("p1", "sms", "CM", 100, "XAF")}
	_, err := SelectProvider(candidates, nil, rule("unknown_strategy"))
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
	decision, err := SelectProvider(candidates, nil, rule(StrategyLowestCost))
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
	decision, err := SelectProvider(candidates, nil, rule(StrategyLowestCost))
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
	decision, err := SelectProvider(candidates, nil, rule(StrategyLowestCost))
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
	decision, err := SelectProvider(candidates, scores, rule(StrategyHighestDelivery))
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
	decision, err := SelectProvider(candidates, scores, rule(StrategyHighestDelivery))
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
	decision, err := SelectProvider(candidates, scores, rule(StrategyHighestDelivery))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// p_scored (score=60) doit gagner sur p_unscored (score=0)
	if decision.ProviderID != "p_scored" {
		t.Errorf("attendu p_scored, obtenu %s", decision.ProviderID)
	}
}

// TestSelectProvider_Fastest_ByLatency verifie le tri par latence croissante.
func TestSelectProvider_Fastest_ByLatency(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_slow", "sms", "CM", 100, "XAF"),
		mkPricing("p_fast", "sms", "CM", 200, "XAF"),
		mkPricing("p_mid", "sms", "CM", 150, "XAF"),
	}
	scores := []ProviderScore{
		mkScoreWithLatency("p_slow", "sms", 90, 500),
		mkScoreWithLatency("p_fast", "sms", 60, 80),
		mkScoreWithLatency("p_mid", "sms", 75, 200),
	}
	decision, err := SelectProvider(candidates, scores, rule(StrategyFastest))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// p_fast a la latence la plus faible (80 ms) meme avec un score inferieur
	if decision.ProviderID != "p_fast" {
		t.Errorf("attendu p_fast (latence=80ms), obtenu %s", decision.ProviderID)
	}
	if decision.Strategy != StrategyFastest {
		t.Errorf("strategie attendue %s, obtenu %s", StrategyFastest, decision.Strategy)
	}
}

// TestSelectProvider_Fastest_TieBreakByScore verifie le tie-break par score
// quand deux providers ont la meme latence.
func TestSelectProvider_Fastest_TieBreakByScore(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_a", "sms", "CM", 100, "XAF"),
		mkPricing("p_b", "sms", "CM", 100, "XAF"),
	}
	scores := []ProviderScore{
		mkScoreWithLatency("p_a", "sms", 50, 150),
		mkScoreWithLatency("p_b", "sms", 90, 150), // meme latence, meilleur score
	}
	decision, err := SelectProvider(candidates, scores, rule(StrategyFastest))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Latences egales → tie-break score decroissant → p_b (score=90)
	if decision.ProviderID != "p_b" {
		t.Errorf("tie-break score: attendu p_b, obtenu %s", decision.ProviderID)
	}
}

// TestSelectProvider_Fastest_NoLatencyData verifie que l'absence de donnee de latence
// (tous a 0) produit un comportement identique a HighestDelivery.
func TestSelectProvider_Fastest_NoLatencyData(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_fast_score", "sms", "CM", 200, "XAF"),
		mkPricing("p_slow_score", "sms", "CM", 100, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_fast_score", "sms", 90), // AvgLatencyMs=0 (zero value)
		mkScore("p_slow_score", "sms", 30),
	}
	decision, err := SelectProvider(candidates, scores, rule(StrategyFastest))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Toutes latences a 0 → tie-break score decroissant → p_fast_score (score=90)
	if decision.ProviderID != "p_fast_score" {
		t.Errorf("attendu p_fast_score (tie-break score, latence=0), obtenu %s", decision.ProviderID)
	}
	if decision.Strategy != StrategyFastest {
		t.Errorf("la strategie retournee doit etre Fastest, obtenu %s", decision.Strategy)
	}
}

// TestSelectProvider_Fastest_FallbackChainOrder verifie l'ordre de la chaine de repli.
func TestSelectProvider_Fastest_FallbackChainOrder(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_slow", "sms", "CM", 100, "XAF"),
		mkPricing("p_fast", "sms", "CM", 200, "XAF"),
		mkPricing("p_mid", "sms", "CM", 150, "XAF"),
	}
	scores := []ProviderScore{
		mkScoreWithLatency("p_slow", "sms", 90, 500),
		mkScoreWithLatency("p_fast", "sms", 60, 80),
		mkScoreWithLatency("p_mid", "sms", 75, 200),
	}
	decision, err := SelectProvider(candidates, scores, rule(StrategyFastest))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	if len(decision.FallbackChain) != 2 {
		t.Fatalf("attendu 2 fallbacks, obtenu %d", len(decision.FallbackChain))
	}
	// Ordre latence croissante : p_fast(80) → p_mid(200) → p_slow(500)
	if decision.FallbackChain[0].ProviderID != "p_mid" {
		t.Errorf("fallback[0] attendu p_mid, obtenu %s", decision.FallbackChain[0].ProviderID)
	}
	if decision.FallbackChain[1].ProviderID != "p_slow" {
		t.Errorf("fallback[1] attendu p_slow, obtenu %s", decision.FallbackChain[1].ProviderID)
	}
}

// TestSelectProvider_SingleProvider verifie le cas d'un provider unique.
func TestSelectProvider_SingleProvider(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("only_one", "whatsapp", "SN", 250, "XAF"),
	}
	decision, err := SelectProvider(candidates, nil, rule(StrategyLowestCost))
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

// TestSelectProvider_Custom_WithPreferredOrder verifie l'ordre explicite via Config.
func TestSelectProvider_Custom_WithPreferredOrder(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("mtn", "sms", "CM", 100, "XAF"),
		mkPricing("orange", "sms", "CM", 150, "XAF"),
		mkPricing("twilio", "sms", "CM", 300, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("mtn", "sms", 80),
		mkScore("orange", "sms", 60),
		mkScore("twilio", "sms", 95),
	}
	config := RoutingConfig{
		"preferred_order": []any{"orange", "twilio", "mtn"},
	}
	decision, err := SelectProvider(candidates, scores, ruleWithConfig(config))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// preferred_order explicite : orange d'abord
	if decision.ProviderID != "orange" {
		t.Errorf("attendu orange (premier dans preferred_order), obtenu %s", decision.ProviderID)
	}
	if len(decision.FallbackChain) != 2 {
		t.Fatalf("attendu 2 fallbacks, obtenu %d", len(decision.FallbackChain))
	}
	if decision.FallbackChain[0].ProviderID != "twilio" {
		t.Errorf("fallback[0] attendu twilio, obtenu %s", decision.FallbackChain[0].ProviderID)
	}
	if decision.FallbackChain[1].ProviderID != "mtn" {
		t.Errorf("fallback[1] attendu mtn, obtenu %s", decision.FallbackChain[1].ProviderID)
	}
	if decision.Strategy != StrategyCustom {
		t.Errorf("strategie attendue custom, obtenu %s", decision.Strategy)
	}
}

// TestSelectProvider_Custom_PartialPreferredOrder verifie que les providers hors liste
// sont apposes en fin de chaine par score decroissant.
func TestSelectProvider_Custom_PartialPreferredOrder(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_a", "sms", "CM", 100, "XAF"), // hors liste, score 80
		mkPricing("p_b", "sms", "CM", 150, "XAF"), // dans liste
		mkPricing("p_c", "sms", "CM", 200, "XAF"), // hors liste, score 50
	}
	scores := []ProviderScore{
		mkScore("p_a", "sms", 80),
		mkScore("p_b", "sms", 40),
		mkScore("p_c", "sms", 50),
	}
	config := RoutingConfig{
		"preferred_order": []any{"p_b"},
	}
	decision, err := SelectProvider(candidates, scores, ruleWithConfig(config))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// p_b est dans la liste → en tete. p_a (score=80) et p_c (score=50) hors liste → HighestDelivery
	if decision.ProviderID != "p_b" {
		t.Errorf("attendu p_b, obtenu %s", decision.ProviderID)
	}
	if len(decision.FallbackChain) != 2 {
		t.Fatalf("attendu 2 fallbacks, obtenu %d", len(decision.FallbackChain))
	}
	// Hors liste : p_a (score=80) avant p_c (score=50)
	if decision.FallbackChain[0].ProviderID != "p_a" {
		t.Errorf("fallback[0] attendu p_a (score=80), obtenu %s", decision.FallbackChain[0].ProviderID)
	}
	if decision.FallbackChain[1].ProviderID != "p_c" {
		t.Errorf("fallback[1] attendu p_c (score=50), obtenu %s", decision.FallbackChain[1].ProviderID)
	}
}

// TestSelectProvider_Custom_FallbackToHighestDelivery verifie le retour sur HighestDelivery
// quand Config est nil.
func TestSelectProvider_Custom_FallbackToHighestDelivery(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_best", "sms", "CM", 300, "XAF"),
		mkPricing("p_cheap", "sms", "CM", 100, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_best", "sms", 88),
		mkScore("p_cheap", "sms", 20),
	}
	// Config nil : retombe sur HighestDelivery
	decision, err := SelectProvider(candidates, scores, rule(StrategyCustom))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Sans config JSON : retombe sur highest_delivery → p_best (score=88)
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best (custom fallback HighestDelivery), obtenu %s", decision.ProviderID)
	}
}

// TestSelectProvider_Custom_EmptyPreferredOrder verifie le fallback sur HighestDelivery
// quand preferred_order est vide.
func TestSelectProvider_Custom_EmptyPreferredOrder(t *testing.T) {
	candidates := []ProviderPricing{
		mkPricing("p_best", "sms", "CM", 300, "XAF"),
		mkPricing("p_cheap", "sms", "CM", 100, "XAF"),
	}
	scores := []ProviderScore{
		mkScore("p_best", "sms", 88),
		mkScore("p_cheap", "sms", 20),
	}
	config := RoutingConfig{
		"preferred_order": []any{}, // vide
	}
	decision, err := SelectProvider(candidates, scores, ruleWithConfig(config))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// preferred_order vide → retombe sur HighestDelivery → p_best
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best (empty preferred_order → HighestDelivery), obtenu %s", decision.ProviderID)
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
	decision, err := SelectProvider(candidates, scores, rule(StrategyHighestDelivery))
	if err != nil {
		t.Fatalf("SelectProvider inattendu: %v", err)
	}
	// Scores egaux → tie-break par cout croissant → p_cheap
	if decision.ProviderID != "p_cheap" {
		t.Errorf("tie-break cout: attendu p_cheap, obtenu %s", decision.ProviderID)
	}
}

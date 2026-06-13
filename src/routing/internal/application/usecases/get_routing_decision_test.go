package usecases

import (
	"context"
	"errors"
	"testing"

	"fleece/src/routing/internal/domain"
)

// --- Mocks des ports output ---

type mockPricingRepo struct {
	pricing []domain.ProviderPricing
	err     error
}

func (m *mockPricingRepo) ListByChannelCountry(_ context.Context, _, _ string) ([]domain.ProviderPricing, error) {
	return m.pricing, m.err
}

type mockScoreRepo struct {
	scores   []domain.ProviderScore
	listErr  error
	upsertErr error
}

func (m *mockScoreRepo) ListByChannel(_ context.Context, _ string) ([]domain.ProviderScore, error) {
	return m.scores, m.listErr
}

func (m *mockScoreRepo) Upsert(_ context.Context, _ domain.ProviderScore) error {
	return m.upsertErr
}

type mockRuleRepo struct {
	rule domain.RoutingRule
	err  error
}

func (m *mockRuleRepo) GetByWorkspace(_ context.Context, _ string) (domain.RoutingRule, error) {
	return m.rule, m.err
}

// helpers
func pricing(providerID string, cost int64) domain.ProviderPricing {
	return domain.ProviderPricing{
		ProviderID: providerID,
		Channel:    domain.ChannelSMS,
		Country:    "CM",
		Cost:       domain.Money{Amount: cost, Currency: "XAF"},
	}
}

func score(providerID string, s int) domain.ProviderScore {
	return domain.ProviderScore{
		ProviderID: providerID,
		Channel:    domain.ChannelSMS,
		Score:      s,
	}
}

// TestGetRoutingDecision_RuleFound_AppliesStrategy verifie que la strategie de la regle est appliquee.
func TestGetRoutingDecision_RuleFound_AppliesStrategy(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_cheap", 100),
			pricing("p_best", 300),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_cheap", 40),
			score("p_best", 95),
		},
	}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyHighestDelivery,
		},
	}

	uc := GetRoutingDecision{
		Pricing: pricingRepo,
		Scores:  scoreRepo,
		Rules:   ruleRepo,
	}

	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", 1)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}
	// HighestDelivery : p_best (score=95) doit etre selectionne
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best, obtenu %s", decision.ProviderID)
	}
	if decision.Strategy != domain.StrategyHighestDelivery {
		t.Errorf("strategie attendue highest_delivery, obtenu %s", decision.Strategy)
	}
}

// TestGetRoutingDecision_RuleNotFound_DefaultHighestDelivery verifie le fallback sur HighestDelivery.
func TestGetRoutingDecision_RuleNotFound_DefaultHighestDelivery(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_cheap", 100),
			pricing("p_best", 300),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_cheap", 40),
			score("p_best", 90),
		},
	}
	ruleRepo := &mockRuleRepo{
		err: domain.ErrRuleNotFound,
	}

	uc := GetRoutingDecision{
		Pricing: pricingRepo,
		Scores:  scoreRepo,
		Rules:   ruleRepo,
	}

	decision, err := uc.Execute(context.Background(), "ws-unknown", "sms", "CM", 1)
	if err != nil {
		t.Fatalf("Execute inattendu (ErrRuleNotFound doit etre absorbe): %v", err)
	}
	// Pas de regle → defaut HighestDelivery → p_best (score=90)
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best (defaut HighestDelivery), obtenu %s", decision.ProviderID)
	}
}

// TestGetRoutingDecision_EmptyPricing_PropagatesErrNoProviderAvailable verifie la propagation.
func TestGetRoutingDecision_EmptyPricing_PropagatesErrNoProviderAvailable(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{}, // aucun provider
	}
	scoreRepo := &mockScoreRepo{}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyLowestCost,
		},
	}

	uc := GetRoutingDecision{
		Pricing: pricingRepo,
		Scores:  scoreRepo,
		Rules:   ruleRepo,
	}

	_, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", 1)
	if err == nil {
		t.Fatal("attendu une erreur")
	}
	if !errors.Is(err, domain.ErrNoProviderAvailable) {
		t.Errorf("attendu ErrNoProviderAvailable, obtenu %v", err)
	}
}

// TestGetRoutingDecision_RuleFound_LowestCost verifie la strategie LowestCost via la regle.
func TestGetRoutingDecision_RuleFound_LowestCost(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_cheap", 100),
			pricing("p_best_score", 500),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_cheap", 30),
			score("p_best_score", 99),
		},
	}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-2",
			Strategy:    domain.StrategyLowestCost,
		},
	}

	uc := GetRoutingDecision{
		Pricing: pricingRepo,
		Scores:  scoreRepo,
		Rules:   ruleRepo,
	}

	decision, err := uc.Execute(context.Background(), "ws-2", "sms", "CM", 10)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}
	// LowestCost : p_cheap (cost=100) doit etre selectionne malgre son score inferieur
	if decision.ProviderID != "p_cheap" {
		t.Errorf("attendu p_cheap, obtenu %s", decision.ProviderID)
	}
}

// TestGetRoutingDecision_PricingRepoError_Propagates verifie la propagation des erreurs repo.
func TestGetRoutingDecision_PricingRepoError_Propagates(t *testing.T) {
	repoErr := errors.New("db connection lost")
	pricingRepo := &mockPricingRepo{err: repoErr}
	scoreRepo := &mockScoreRepo{}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{WorkspaceID: "ws-1", Strategy: domain.StrategyHighestDelivery},
	}

	uc := GetRoutingDecision{
		Pricing: pricingRepo,
		Scores:  scoreRepo,
		Rules:   ruleRepo,
	}

	_, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", 1)
	if err == nil {
		t.Fatal("attendu une erreur de repo")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("attendu db connection lost wrappee, obtenu %v", err)
	}
}

package usecases

import (
	"context"
	"errors"
	"testing"

	"fleece/src/routing/internal/domain"
)

// mockContactScore est un mock du port output.ContactScoreClient.
type mockContactScore struct {
	score int
	found bool
	err   error
	// callCount permet de verifier que GetChannelScore n'est pas appele inutilement.
	callCount int
}

func (m *mockContactScore) GetChannelScore(_ context.Context, _, _ string) (int, bool, error) {
	m.callCount++
	return m.score, m.found, m.err
}

// --- Mocks des ports output ---

type mockPricingRepo struct {
	pricing []domain.ProviderPricing
	err     error
}

func (m *mockPricingRepo) ListByChannelCountry(_ context.Context, _, _ string) ([]domain.ProviderPricing, error) {
	return m.pricing, m.err
}

type mockScoreRepo struct {
	scores    []domain.ProviderScore
	listErr   error
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

	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "", 1)
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

	decision, err := uc.Execute(context.Background(), "ws-unknown", "sms", "CM", "", 1)
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

	_, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "", 1)
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

	decision, err := uc.Execute(context.Background(), "ws-2", "sms", "CM", "", 10)
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

	_, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "", 1)
	if err == nil {
		t.Fatal("attendu une erreur de repo")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("attendu db connection lost wrappee, obtenu %v", err)
	}
}

// --- Tests T-016 : enrichissement contact score ---

// TestGetRoutingDecision_ContactScore_CombinesWithProviderScore verifie que le score
// contact est combine avec le score provider pour la strategie highest_delivery.
// Semantique : combined = round(0.5*providerScore + 0.5*contactScore).
func TestGetRoutingDecision_ContactScore_CombinesWithProviderScore(t *testing.T) {
	// p_high_provider : score provider=90, p_low_provider : score provider=70.
	// contactScore=40 (peu joignable sur SMS).
	// p_high_provider combined = (90+40+1)/2 = 65
	// p_low_provider combined  = (70+40+1)/2 = 55
	// L'ordre doit etre preserve : p_high_provider toujours selectionne.
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_high_provider", 200),
			pricing("p_low_provider", 100),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_high_provider", 90),
			score("p_low_provider", 70),
		},
	}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyHighestDelivery,
		},
	}
	contactClient := &mockContactScore{score: 40, found: true}

	uc := GetRoutingDecision{
		Pricing:      pricingRepo,
		Scores:       scoreRepo,
		Rules:        ruleRepo,
		ContactScore: contactClient,
	}

	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "+237612345678", 1)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}
	if decision.ProviderID != "p_high_provider" {
		t.Errorf("attendu p_high_provider (combined=65 > 55), obtenu %s", decision.ProviderID)
	}
	if contactClient.callCount != 1 {
		t.Errorf("GetChannelScore appele %d fois, attendu 1", contactClient.callCount)
	}
}

// TestGetRoutingDecision_ContactScore_FallbackOnError verifie le fallback gracieux :
// si GetChannelScore retourne une erreur, le routage continue avec provider_scores seuls.
func TestGetRoutingDecision_ContactScore_FallbackOnError(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_best", 200),
			pricing("p_second", 100),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_best", 95),
			score("p_second", 60),
		},
	}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyHighestDelivery,
		},
	}
	// Mock qui retourne une erreur (service indisponible).
	contactClient := &mockContactScore{err: errors.New("contact-intelligence: connection refused")}

	uc := GetRoutingDecision{
		Pricing:      pricingRepo,
		Scores:       scoreRepo,
		Rules:        ruleRepo,
		ContactScore: contactClient,
	}

	// L'Execute NE DOIT PAS echouer meme si le service contact-intelligence est down.
	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "+237612345678", 1)
	if err != nil {
		t.Fatalf("Execute ne doit pas echouer si contact-intelligence est indisponible: %v", err)
	}
	// Comportement degrade : provider_scores seuls → p_best (score=95) selectionne.
	if decision.ProviderID != "p_best" {
		t.Errorf("degrade vers provider_scores : attendu p_best, obtenu %s", decision.ProviderID)
	}
}

// TestGetRoutingDecision_ContactScore_UnknownContact verifie que found=false (score=50)
// est traite comme un prior neutre et ne bloque pas le routage.
func TestGetRoutingDecision_ContactScore_UnknownContact(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_best", 200),
			pricing("p_second", 100),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_best", 90),
			score("p_second", 50),
		},
	}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyHighestDelivery,
		},
	}
	// Contact inconnu → score=50, found=false (prior neutre Laplace).
	contactClient := &mockContactScore{score: 50, found: false}

	uc := GetRoutingDecision{
		Pricing:      pricingRepo,
		Scores:       scoreRepo,
		Rules:        ruleRepo,
		ContactScore: contactClient,
	}

	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "+237699000000", 1)
	if err != nil {
		t.Fatalf("Execute inattendu pour contact inconnu: %v", err)
	}
	// p_best combined = (90+50+1)/2 = 70 > p_second combined = (50+50+1)/2 = 50.
	// p_best doit rester selectionne.
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best avec contact inconnu (prior 50), obtenu %s", decision.ProviderID)
	}
}

// TestGetRoutingDecision_ContactScore_NilClient_NoCall verifie que si ContactScore est nil
// (non cable), aucun appel n'est effectue et le routage fonctionne normalement.
func TestGetRoutingDecision_ContactScore_NilClient_NoCall(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_best", 200),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_best", 90),
		},
	}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyHighestDelivery,
		},
	}

	// ContactScore = nil : pas de client contact-intelligence cable.
	uc := GetRoutingDecision{
		Pricing:      pricingRepo,
		Scores:       scoreRepo,
		Rules:        ruleRepo,
		ContactScore: nil, // non cable
	}

	// Avec recipient fourni, mais ContactScore nil → pas d'appel, pas d'erreur.
	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "+237612345678", 1)
	if err != nil {
		t.Fatalf("Execute inattendu avec ContactScore nil: %v", err)
	}
	// Comportement inchange (V1 pre-T-016) : provider_scores seuls.
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best, obtenu %s", decision.ProviderID)
	}
}

// TestGetRoutingDecision_ContactScore_EmptyRecipient_NoCall verifie que si recipient est vide,
// le client contact-intelligence n'est pas appele meme si ContactScore est cable.
func TestGetRoutingDecision_ContactScore_EmptyRecipient_NoCall(t *testing.T) {
	pricingRepo := &mockPricingRepo{
		pricing: []domain.ProviderPricing{
			pricing("p_best", 200),
		},
	}
	scoreRepo := &mockScoreRepo{
		scores: []domain.ProviderScore{
			score("p_best", 90),
		},
	}
	ruleRepo := &mockRuleRepo{
		rule: domain.RoutingRule{
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyHighestDelivery,
		},
	}
	contactClient := &mockContactScore{score: 70, found: true}

	uc := GetRoutingDecision{
		Pricing:      pricingRepo,
		Scores:       scoreRepo,
		Rules:        ruleRepo,
		ContactScore: contactClient,
	}

	// recipient vide → pas d'appel contact.
	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "", 1)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}
	if decision.ProviderID != "p_best" {
		t.Errorf("attendu p_best, obtenu %s", decision.ProviderID)
	}
	if contactClient.callCount != 0 {
		t.Errorf("GetChannelScore ne doit pas etre appele si recipient est vide, appele %d fois", contactClient.callCount)
	}
}

// TestGetRoutingDecision_ContactScore_NotCalledForLowestCost verifie que le client
// contact-intelligence n'est pas appele pour les strategies autres que highest_delivery.
func TestGetRoutingDecision_ContactScore_NotCalledForLowestCost(t *testing.T) {
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
			WorkspaceID: "ws-1",
			Strategy:    domain.StrategyLowestCost, // pas highest_delivery !
		},
	}
	contactClient := &mockContactScore{score: 80, found: true}

	uc := GetRoutingDecision{
		Pricing:      pricingRepo,
		Scores:       scoreRepo,
		Rules:        ruleRepo,
		ContactScore: contactClient,
	}

	decision, err := uc.Execute(context.Background(), "ws-1", "sms", "CM", "+237612345678", 1)
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}
	// LowestCost : p_cheap selectionne (cout=100).
	if decision.ProviderID != "p_cheap" {
		t.Errorf("attendu p_cheap (LowestCost), obtenu %s", decision.ProviderID)
	}
	// GetChannelScore ne doit PAS etre appele pour une strategie autre que highest_delivery.
	if contactClient.callCount != 0 {
		t.Errorf("GetChannelScore ne doit pas etre appele pour LowestCost, appele %d fois", contactClient.callCount)
	}
}

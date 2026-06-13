package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"fleece/src/routing/internal/domain"
)

// --- Mocks des use cases ---

type mockGetDecision struct {
	decision domain.RoutingDecision
	err      error
}

func (m *mockGetDecision) Execute(_ context.Context, _, _, _ string, _ int) (domain.RoutingDecision, error) {
	return m.decision, m.err
}

type mockUpdateScore struct {
	err error
}

func (m *mockUpdateScore) Execute(_ context.Context, _, _ string, _ int) error {
	return m.err
}

// --- helpers ---

func newHandler(getErr error, getDecision domain.RoutingDecision, scoreErr error) *RoutingHandler {
	return NewRoutingHandler(&mockGetDecision{decision: getDecision, err: getErr}, &mockUpdateScore{err: scoreErr})
}

func postJSON(t *testing.T, h http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

// --- Tests POST /route ---

// TestRoute_HappyPath verifie que 200 est renvoye avec la decision correcte.
func TestRoute_HappyPath(t *testing.T) {
	decision := domain.RoutingDecision{
		ProviderID:    "p_best",
		Channel:       domain.ChannelSMS,
		EstimatedCost: domain.Money{Amount: 150, Currency: "XAF"},
		Strategy:      domain.StrategyHighestDelivery,
		FallbackChain: []domain.ProviderRef{{ProviderID: "p_fallback"}},
	}
	h := newHandler(nil, decision, nil)

	rr := postJSON(t, h.Route, "/route", RouteRequest{
		WorkspaceID:    "ws-1",
		Channel:        "sms",
		Country:        "CM",
		RecipientCount: 1,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("attendu 200, obtenu %d — body: %s", rr.Code, rr.Body.String())
	}
	var resp RouteResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ProviderID != "p_best" {
		t.Errorf("attendu p_best, obtenu %s", resp.ProviderID)
	}
	if resp.EstimatedCost != 150 {
		t.Errorf("cout attendu 150, obtenu %d", resp.EstimatedCost)
	}
	if len(resp.FallbackChain) != 1 || resp.FallbackChain[0] != "p_fallback" {
		t.Errorf("FallbackChain inattendue: %v", resp.FallbackChain)
	}
}

// TestRoute_MissingWorkspaceID verifie que 400 est renvoye si workspace_id est absent.
func TestRoute_MissingWorkspaceID(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.Route, "/route", RouteRequest{Channel: "sms", Country: "CM", RecipientCount: 1})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestRoute_MissingChannel verifie que 400 est renvoye si channel est absent.
func TestRoute_MissingChannel(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.Route, "/route", RouteRequest{WorkspaceID: "ws-1", Country: "CM", RecipientCount: 1})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestRoute_MissingCountry verifie que 400 est renvoye si country est absent.
func TestRoute_MissingCountry(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.Route, "/route", RouteRequest{WorkspaceID: "ws-1", Channel: "sms", RecipientCount: 1})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestRoute_NegativeRecipientCount verifie que 400 est renvoye pour recipient_count < 0.
func TestRoute_NegativeRecipientCount(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.Route, "/route", RouteRequest{WorkspaceID: "ws-1", Channel: "sms", Country: "CM", RecipientCount: -1})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestRoute_ErrNoProviderAvailable verifie que 422 est renvoye.
func TestRoute_ErrNoProviderAvailable(t *testing.T) {
	h := newHandler(domain.ErrNoProviderAvailable, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.Route, "/route", RouteRequest{WorkspaceID: "ws-1", Channel: "sms", Country: "CM", RecipientCount: 1})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("attendu 422, obtenu %d", rr.Code)
	}
}

// TestRoute_ErrInvalidStrategy verifie que 400 est renvoye.
func TestRoute_ErrInvalidStrategy(t *testing.T) {
	h := newHandler(domain.ErrInvalidStrategy, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.Route, "/route", RouteRequest{WorkspaceID: "ws-1", Channel: "sms", Country: "CM", RecipientCount: 1})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestRoute_InternalError verifie que 500 est renvoye pour une erreur inconnue.
func TestRoute_InternalError(t *testing.T) {
	h := newHandler(errors.New("db down"), domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.Route, "/route", RouteRequest{WorkspaceID: "ws-1", Channel: "sms", Country: "CM", RecipientCount: 1})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("attendu 500, obtenu %d", rr.Code)
	}
}

// TestRoute_InvalidJSON verifie que 400 est renvoye pour un corps non-JSON.
func TestRoute_InvalidJSON(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/route", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Route(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// --- Tests POST /scores ---

// TestScoreFeedback_HappyPath verifie que 204 est renvoye si tout est correct.
func TestScoreFeedback_HappyPath(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.ScoreFeedback, "/scores", ScoreFeedbackRequest{
		ProviderID: "p1",
		Channel:    "sms",
		Score:      75,
	})
	if rr.Code != http.StatusNoContent {
		t.Errorf("attendu 204, obtenu %d — body: %s", rr.Code, rr.Body.String())
	}
}

// TestScoreFeedback_MissingProviderID verifie que 400 est renvoye.
func TestScoreFeedback_MissingProviderID(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.ScoreFeedback, "/scores", ScoreFeedbackRequest{Channel: "sms", Score: 50})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestScoreFeedback_MissingChannel verifie que 400 est renvoye.
func TestScoreFeedback_MissingChannel(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.ScoreFeedback, "/scores", ScoreFeedbackRequest{ProviderID: "p1", Score: 50})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestScoreFeedback_ScoreOutOfRange_TooHigh verifie que 400 est renvoye pour score > 100.
func TestScoreFeedback_ScoreOutOfRange_TooHigh(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.ScoreFeedback, "/scores", ScoreFeedbackRequest{ProviderID: "p1", Channel: "sms", Score: 101})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400 (score > 100), obtenu %d", rr.Code)
	}
}

// TestScoreFeedback_ScoreOutOfRange_Negative verifie que 400 est renvoye pour score < 0.
func TestScoreFeedback_ScoreOutOfRange_Negative(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	rr := postJSON(t, h.ScoreFeedback, "/scores", ScoreFeedbackRequest{ProviderID: "p1", Channel: "sms", Score: -1})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400 (score < 0), obtenu %d", rr.Code)
	}
}

// TestScoreFeedback_InternalError verifie que 500 est renvoye pour une erreur repo.
func TestScoreFeedback_InternalError(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, errors.New("db error"))
	rr := postJSON(t, h.ScoreFeedback, "/scores", ScoreFeedbackRequest{ProviderID: "p1", Channel: "sms", Score: 80})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("attendu 500, obtenu %d", rr.Code)
	}
}

// TestScoreFeedback_InvalidJSON verifie que 400 est renvoye pour un corps non-JSON.
func TestScoreFeedback_InvalidJSON(t *testing.T) {
	h := newHandler(nil, domain.RoutingDecision{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/scores", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ScoreFeedback(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d", rr.Code)
	}
}

// TestScoreFeedback_ScoreBoundaries_ValidExtremes verifie que 0 et 100 sont acceptes.
func TestScoreFeedback_ScoreBoundaries_ValidExtremes(t *testing.T) {
	for _, score := range []int{0, 100} {
		h := newHandler(nil, domain.RoutingDecision{}, nil)
		rr := postJSON(t, h.ScoreFeedback, "/scores", ScoreFeedbackRequest{ProviderID: "p1", Channel: "sms", Score: score})
		if rr.Code != http.StatusNoContent {
			t.Errorf("score=%d : attendu 204, obtenu %d", score, rr.Code)
		}
	}
}

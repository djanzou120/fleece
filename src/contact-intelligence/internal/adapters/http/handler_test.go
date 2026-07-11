package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fleece/src/contact-intelligence/internal/application/usecases"
	"fleece/src/contact-intelligence/internal/domain"
)

// ─── Mocks ──────────────────────────────────────────────────────────────────

// mockGetScore implementeMockContactScoreUseCase pour les tests.
type mockGetScore struct {
	result *usecases.ContactScoreResult
	err    error
}

func (m *mockGetScore) Execute(_ context.Context, phone string) (*usecases.ContactScoreResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	// par defaut : contact inconnu → score neutre
	return &usecases.ContactScoreResult{
		Phone:         phone,
		DeliveryScore: domain.NewDeliveryScore(50),
		Found:         false,
		ChannelScores: nil,
	}, nil
}

// mockRecordOutcome implementeRecordDeliveryOutcomeUseCase pour les tests.
type mockRecordOutcome struct {
	err error
}

func (m *mockRecordOutcome) Execute(_ context.Context, phone string, ch domain.Channel, success bool) error {
	return m.err
}

// ─── Tests GetScore ───────────────────────────────────────────────────────────

// TestGetScore_ContactInconnu_Found_False verifie que GET /contacts/{phone}/score
// retourne 200 + found=false + score=50 pour un contact inconnu (critere C5).
func TestGetScore_ContactInconnu_Found_False(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	req := httptest.NewRequest(http.MethodGet, "/contacts/%2B237690000000/score", nil)
	req.SetPathValue("phone", "+237690000000")
	w := httptest.NewRecorder()

	handler.GetScore(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, obtenu %d — body: %s", w.Code, w.Body.String())
	}

	var resp ContactScoreResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp.Found {
		t.Error("attendu Found=false pour contact inconnu")
	}
	if resp.DeliveryScore != 50 {
		t.Errorf("attendu delivery_score=50 (prior neutre), obtenu %d", resp.DeliveryScore)
	}
	if resp.Phone != "+237690000000" {
		t.Errorf("attendu phone=+237690000000, obtenu %s", resp.Phone)
	}
	if resp.ChannelScores == nil {
		// channel_scores vide acceptable (nil ou [])
	}
}

// TestGetScore_ContactConnu_Found_True verifie que GET /contacts/{phone}/score
// retourne 200 + found=true + les channel_scores pour un contact connu.
func TestGetScore_ContactConnu_Found_True(t *testing.T) {
	t0 := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	cs, _ := domain.NewChannelScore("+237690000001", domain.ChannelWhatsApp)
	cs.RecordOutcome(true, t0)
	cs.RecordOutcome(true, t0)

	mock := &mockGetScore{
		result: &usecases.ContactScoreResult{
			Phone:         "+237690000001",
			DeliveryScore: domain.NewDeliveryScore(75),
			Found:         true,
			ChannelScores: []*domain.ChannelScore{cs},
		},
	}
	handler := NewContactIntelHandler(mock, &mockRecordOutcome{})

	req := httptest.NewRequest(http.MethodGet, "/contacts/%2B237690000001/score", nil)
	req.SetPathValue("phone", "+237690000001")
	w := httptest.NewRecorder()

	handler.GetScore(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, obtenu %d — body: %s", w.Code, w.Body.String())
	}

	var resp ContactScoreResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if !resp.Found {
		t.Error("attendu Found=true pour contact connu")
	}
	if resp.DeliveryScore != 75 {
		t.Errorf("attendu delivery_score=75, obtenu %d", resp.DeliveryScore)
	}
	if len(resp.ChannelScores) != 1 {
		t.Errorf("attendu 1 channel_score, obtenu %d", len(resp.ChannelScores))
	}
	if resp.ChannelScores[0].Channel != "whatsapp" {
		t.Errorf("attendu channel=whatsapp, obtenu %s", resp.ChannelScores[0].Channel)
	}
}

// TestGetScore_PhoneVide_400 verifie que GET sans phone dans le path retourne 400.
func TestGetScore_PhoneVide_400(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	req := httptest.NewRequest(http.MethodGet, "/contacts//score", nil)
	// PathValue non positionne → phone=""
	req.SetPathValue("phone", "")
	w := httptest.NewRecorder()

	handler.GetScore(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d — body: %s", w.Code, w.Body.String())
	}
}

// TestGetScore_Pas404_ContactInconnu verifie que le handler ne retourne jamais 404
// pour un contact inconnu (critere C5 : found=false, pas 404).
func TestGetScore_Pas404_ContactInconnu(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	req := httptest.NewRequest(http.MethodGet, "/contacts/inconnu/score", nil)
	req.SetPathValue("phone", "+33600000000")
	w := httptest.NewRecorder()

	handler.GetScore(w, req)

	if w.Code == http.StatusNotFound {
		t.Error("le handler NE DOIT PAS retourner 404 pour un contact inconnu")
	}
	if w.Code != http.StatusOK {
		t.Errorf("attendu 200 (found=false), obtenu %d — body: %s", w.Code, w.Body.String())
	}
}

// ─── Tests RecordOutcome ──────────────────────────────────────────────────────

// TestRecordOutcome_OK_204 verifie que POST /outcomes retourne 204 sur payload valide.
func TestRecordOutcome_OK_204(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	body, _ := json.Marshal(map[string]any{
		"phone":   "+237690000010",
		"channel": "whatsapp",
		"success": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/outcomes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RecordOutcome(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("attendu 204, obtenu %d — body: %s", w.Code, w.Body.String())
	}
}

// TestRecordOutcome_PhoneManquant_400 verifie que POST /outcomes retourne 400 si phone absent.
func TestRecordOutcome_PhoneManquant_400(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	body, _ := json.Marshal(map[string]any{
		"channel": "sms",
		"success": false,
	})
	req := httptest.NewRequest(http.MethodPost, "/outcomes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RecordOutcome(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d — body: %s", w.Code, w.Body.String())
	}
}

// TestRecordOutcome_ChannelManquant_400 verifie que POST /outcomes retourne 400 si channel absent.
func TestRecordOutcome_ChannelManquant_400(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	body, _ := json.Marshal(map[string]any{
		"phone":   "+237690000011",
		"success": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/outcomes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RecordOutcome(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d — body: %s", w.Code, w.Body.String())
	}
}

// TestRecordOutcome_JsonInvalide_400 verifie que POST /outcomes retourne 400 sur JSON malformed.
func TestRecordOutcome_JsonInvalide_400(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	req := httptest.NewRequest(http.MethodPost, "/outcomes", bytes.NewReader([]byte(`{not valid json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RecordOutcome(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d — body: %s", w.Code, w.Body.String())
	}
}

// TestRecordOutcome_CanalInconnu_400 verifie que POST /outcomes retourne 400 si canal invalide.
func TestRecordOutcome_CanalInconnu_400(t *testing.T) {
	errCh := domain.ErrInvalidChannel
	mock := &mockRecordOutcome{err: errCh}
	handler := NewContactIntelHandler(&mockGetScore{}, mock)

	body, _ := json.Marshal(map[string]any{
		"phone":   "+237690000012",
		"channel": "pigeon-voyageur",
		"success": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/outcomes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.RecordOutcome(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("attendu 400, obtenu %d — body: %s", w.Code, w.Body.String())
	}
}

// TestGetScore_ResponseJSON_Fields verifie que la reponse JSON contient exactement
// les champs documentes : phone, delivery_score, found, channel_scores.
func TestGetScore_ResponseJSON_Fields(t *testing.T) {
	handler := NewContactIntelHandler(&mockGetScore{}, &mockRecordOutcome{})

	req := httptest.NewRequest(http.MethodGet, "/contacts/+237690000020/score", nil)
	req.SetPathValue("phone", "+237690000020")
	w := httptest.NewRecorder()

	handler.GetScore(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("attendu 200, obtenu %d", w.Code)
	}

	var raw map[string]any
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	for _, field := range []string{"phone", "delivery_score", "found", "channel_scores"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("champ JSON manquant : %q", field)
		}
	}
}

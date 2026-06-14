package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fleece/src/webhook/internal/domain"
)

// --- Mocks ---

type mockRegisterEndpointUC struct {
	result *domain.WebhookEndpoint
	err    error
}

func (m *mockRegisterEndpointUC) Execute(_ context.Context, ep *domain.WebhookEndpoint) (*domain.WebhookEndpoint, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	// Par defaut : retourner l'endpoint tel quel avec un secret genere.
	ep.Active = true
	if ep.Secret == "" {
		ep.Secret = "generated-secret-abcdef"
	}
	return ep, nil
}

type mockEndpointRepoHTTP struct {
	endpoints []*domain.WebhookEndpoint
	saveErr   error
	deleteErr error
}

func (m *mockEndpointRepoHTTP) Save(_ context.Context, e *domain.WebhookEndpoint) error {
	return m.saveErr
}

func (m *mockEndpointRepoHTTP) FindByID(_ context.Context, id string) (*domain.WebhookEndpoint, error) {
	for _, ep := range m.endpoints {
		if ep.ID == id {
			return ep, nil
		}
	}
	return nil, domain.ErrEndpointNotFound
}

func (m *mockEndpointRepoHTTP) FindByWorkspaceAndEvent(_ context.Context, _, _ string) ([]*domain.WebhookEndpoint, error) {
	return nil, nil
}

func (m *mockEndpointRepoHTTP) ListByWorkspace(_ context.Context, workspaceID string) ([]*domain.WebhookEndpoint, error) {
	var result []*domain.WebhookEndpoint
	for _, ep := range m.endpoints {
		if ep.WorkspaceID == workspaceID {
			result = append(result, ep)
		}
	}
	return result, nil
}

func (m *mockEndpointRepoHTTP) Delete(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, ep := range m.endpoints {
		if ep.ID == id {
			m.endpoints = append(m.endpoints[:i], m.endpoints[i+1:]...)
			return nil
		}
	}
	return domain.ErrEndpointNotFound
}

// --- Helpers ---

func newTestHandler(uc registerEndpointUseCase, repo *mockEndpointRepoHTTP) *WebhookHandler {
	return NewWebhookHandler(uc, repo)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return b
}

func decodeJSON(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("unmarshal JSON: %v — body: %s", err, string(body))
	}
}

// --- Tests POST /endpoints (Register) ---

func TestHandler_Register_HappyPath(t *testing.T) {
	uc := &mockRegisterEndpointUC{}
	repo := &mockEndpointRepoHTTP{}
	h := newTestHandler(uc, repo)

	body := mustJSON(t, map[string]any{
		"workspace_id": "ws-1",
		"url":          "https://example.com/hook",
		"events":       []string{"message.delivered"},
	})

	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("code HTTP attendu %d, obtenu %d — body: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp EndpointResponse
	decodeJSON(t, w.Body.Bytes(), &resp)

	if resp.URL != "https://example.com/hook" {
		t.Errorf("URL attendu %q, obtenu %q", "https://example.com/hook", resp.URL)
	}
	if !resp.Active {
		t.Error("l'endpoint doit etre actif a la creation")
	}
}

func TestHandler_Register_URLVide_400(t *testing.T) {
	h := newTestHandler(&mockRegisterEndpointUC{}, &mockEndpointRepoHTTP{})

	body := mustJSON(t, map[string]any{
		"workspace_id": "ws-1",
		"url":          "",
		"events":       []string{"message.delivered"},
	})

	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_Register_AucunEvenement_400(t *testing.T) {
	h := newTestHandler(&mockRegisterEndpointUC{}, &mockEndpointRepoHTTP{})

	body := mustJSON(t, map[string]any{
		"workspace_id": "ws-1",
		"url":          "https://example.com/hook",
		"events":       []string{},
	})

	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_Register_WorkspaceIDVide_400(t *testing.T) {
	h := newTestHandler(&mockRegisterEndpointUC{}, &mockEndpointRepoHTTP{})

	body := mustJSON(t, map[string]any{
		"workspace_id": "",
		"url":          "https://example.com/hook",
		"events":       []string{"message.delivered"},
	})

	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_Register_CorpsInvalide_400(t *testing.T) {
	h := newTestHandler(&mockRegisterEndpointUC{}, &mockEndpointRepoHTTP{})

	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader([]byte("not-json")))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_Register_ErreurUseCase_500(t *testing.T) {
	uc := &mockRegisterEndpointUC{err: fmt.Errorf("erreur interne")}
	h := newTestHandler(uc, &mockEndpointRepoHTTP{})

	body := mustJSON(t, map[string]any{
		"workspace_id": "ws-1",
		"url":          "https://example.com/hook",
		"events":       []string{"message.delivered"},
	})

	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Register(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusInternalServerError, w.Code)
	}
}

// --- Tests GET /endpoints (List) ---

func TestHandler_List_HappyPath(t *testing.T) {
	repo := &mockEndpointRepoHTTP{
		endpoints: []*domain.WebhookEndpoint{
			{ID: "ep-1", WorkspaceID: "ws-list", URL: "https://a.com/hook", Events: []string{"message.delivered"}, Active: true},
			{ID: "ep-2", WorkspaceID: "ws-list", URL: "https://b.com/hook", Events: []string{"wallet.debited"}, Active: true},
		},
	}
	h := newTestHandler(&mockRegisterEndpointUC{}, repo)

	req := httptest.NewRequest(http.MethodGet, "/endpoints?workspace_id=ws-list", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code HTTP attendu %d, obtenu %d — body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp ListEndpointsResponse
	decodeJSON(t, w.Body.Bytes(), &resp)

	if len(resp.Endpoints) != 2 {
		t.Errorf("attendu 2 endpoints, obtenu %d", len(resp.Endpoints))
	}
}

func TestHandler_List_WorkspaceIDAbsent_400(t *testing.T) {
	h := newTestHandler(&mockRegisterEndpointUC{}, &mockEndpointRepoHTTP{})

	req := httptest.NewRequest(http.MethodGet, "/endpoints", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandler_List_WorkspaceInconnu_RetourneListeVide(t *testing.T) {
	repo := &mockEndpointRepoHTTP{
		endpoints: []*domain.WebhookEndpoint{
			{ID: "ep-autre", WorkspaceID: "autre-ws", URL: "https://x.com", Events: []string{"message.delivered"}, Active: true},
		},
	}
	h := newTestHandler(&mockRegisterEndpointUC{}, repo)

	req := httptest.NewRequest(http.MethodGet, "/endpoints?workspace_id=ws-inconnu", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusOK, w.Code)
	}

	var resp ListEndpointsResponse
	decodeJSON(t, w.Body.Bytes(), &resp)
	if len(resp.Endpoints) != 0 {
		t.Errorf("attendu 0 endpoint pour workspace inconnu, obtenu %d", len(resp.Endpoints))
	}
}

// --- Tests DELETE /endpoints/{id} ---

func TestHandler_Delete_HappyPath(t *testing.T) {
	repo := &mockEndpointRepoHTTP{
		endpoints: []*domain.WebhookEndpoint{
			{ID: "ep-del-1", WorkspaceID: "ws-del", URL: "https://del.com/hook", Events: []string{"message.delivered"}, Active: true},
		},
	}
	h := newTestHandler(&mockRegisterEndpointUC{}, repo)

	req := httptest.NewRequest(http.MethodDelete, "/endpoints/ep-del-1", nil)
	// Simuler PathValue (Go 1.22+)
	req.SetPathValue("id", "ep-del-1")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("code HTTP attendu %d, obtenu %d — body: %s", http.StatusNoContent, w.Code, w.Body.String())
	}
}

func TestHandler_Delete_EndpointInexistant_404(t *testing.T) {
	repo := &mockEndpointRepoHTTP{}
	h := newTestHandler(&mockRegisterEndpointUC{}, repo)

	req := httptest.NewRequest(http.MethodDelete, "/endpoints/inexistant", nil)
	req.SetPathValue("id", "inexistant")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusNotFound, w.Code)
	}
}

func TestHandler_Delete_IDAbsent_400(t *testing.T) {
	h := newTestHandler(&mockRegisterEndpointUC{}, &mockEndpointRepoHTTP{})

	req := httptest.NewRequest(http.MethodDelete, "/endpoints/", nil)
	// PathValue non renseigne → id = ""
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code HTTP attendu %d, obtenu %d", http.StatusBadRequest, w.Code)
	}
}

// --- Test Content-Type JSON ---

func TestHandler_Register_ContentTypeJSON(t *testing.T) {
	h := newTestHandler(&mockRegisterEndpointUC{}, &mockEndpointRepoHTTP{})

	body := mustJSON(t, map[string]any{
		"workspace_id": "ws-ct",
		"url":          "https://example.com/hook",
		"events":       []string{"message.delivered"},
	})

	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.Register(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type attendu %q, obtenu %q", "application/json", ct)
	}
}

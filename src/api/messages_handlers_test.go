package api

// Ce fichier contient les tests des handlers HTTP messages via net/http/httptest.
//
// Perimetre teste (sans base de donnees) :
//   - Validation des inputs (body JSON invalide, champs manquants, strategy invalide,
//     UUID malformes) → retours 400 attendus.
//   - Verification du Content-Type application/json dans les reponses d'erreur.
//   - Methodes HTTP incorrectes (405 implicite via le mux — verifie indirectement).
//   - GET /messages/{id} avec UUID invalide → 400.
//   - GET /messages avec limit/offset invalides → 400.
//
// Perimetre NON teste (necessite une base de donnees reelle) :
//   - Insertion effective dans messaging.messages.
//   - Lecture depuis messaging.messages (GET /{id} 200/404, GET / filtres).
//   - Selection et appel de provider reel.
//   - Publication AMQP.
// Ces chemins sont couverts par les tests d'acceptance (QA) avec une DB de test.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestService cree un *Service minimal (sans DB, sans AMQP, sans providers)
// suffisant pour tester la validation et le parsing des handlers.
// Les tests qui atteignent s.DB ou s.Providers paniqueront — c'est attendu et
// documente : ces chemins sont marques "necessite DB".
func newTestService() *Service {
	return &Service{
		Providers: make(map[string]Provider),
		// DB et AMQP laisses nil : tout acces DB plantera le test.
		// On n'atteint pas ces chemins dans les cas de validation.
	}
}

// ---- POST /messages ----

func TestHandleSendMessage_invalidBody(t *testing.T) {
	svc := newTestService()
	body := bytes.NewBufferString(`not valid json`)
	req := httptest.NewRequest(http.MethodPost, "/messages", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleSendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleSendMessage_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"recipient": "+237600000001",
		"body":      "Bonjour",
		"strategy":  "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/messages", payload)
	rr := httptest.NewRecorder()

	svc.HandleSendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleSendMessage_missingRecipient(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"workspace_id": "550e8400-e29b-41d4-a716-446655440000",
		"body":         "Bonjour",
		"strategy":     "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/messages", payload)
	rr := httptest.NewRecorder()

	svc.HandleSendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (recipient manquant)", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleSendMessage_missingBody(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"workspace_id": "550e8400-e29b-41d4-a716-446655440000",
		"recipient":    "+237600000001",
		"strategy":     "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/messages", payload)
	rr := httptest.NewRecorder()

	svc.HandleSendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body manquant)", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleSendMessage_invalidStrategy(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"workspace_id": "550e8400-e29b-41d4-a716-446655440000",
		"recipient":    "+237600000001",
		"body":         "Bonjour",
		"strategy":     "best_price", // invalide
	}
	req := jsonRequest(t, http.MethodPost, "/messages", payload)
	rr := httptest.NewRecorder()

	svc.HandleSendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (strategy invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleSendMessage_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"workspace_id": "not-a-uuid",
		"recipient":    "+237600000001",
		"body":         "Bonjour",
		"strategy":     "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/messages", payload)
	rr := httptest.NewRecorder()

	svc.HandleSendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleSendMessage_emptyBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleSendMessage(rr, req)

	// body vide = workspace_id manquant → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body JSON vide)", rr.Code, http.StatusBadRequest)
	}
}

// ---- GET /messages/{id} ----

func TestHandleGetMessage_invalidUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/messages/not-a-uuid", nil)
	// Simuler le PathValue que le mux Go 1.22 injecte.
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()

	svc.HandleGetMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetMessage_missingID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/messages/", nil)
	// PathValue vide = id non fourni.
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	svc.HandleGetMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id manquant)", rr.Code, http.StatusBadRequest)
	}
}

// ---- GET /messages ----

func TestHandleListMessages_invalidLimit(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/messages?limit=abc", nil)
	rr := httptest.NewRecorder()

	svc.HandleListMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (limit non-numerique)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListMessages_limitZero(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/messages?limit=0", nil)
	rr := httptest.NewRecorder()

	svc.HandleListMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (limit=0)", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListMessages_negativeOffset(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/messages?offset=-1", nil)
	rr := httptest.NewRecorder()

	svc.HandleListMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (offset negatif)", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListMessages_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/messages?workspace_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()

	svc.HandleListMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleListMessages_invalidOffset(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/messages?offset=notanint", nil)
	rr := httptest.NewRecorder()

	svc.HandleListMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (offset non-numerique)", rr.Code, http.StatusBadRequest)
	}
}

// ---- helpers de test ----

// jsonRequest cree une requete HTTP avec un corps JSON encode depuis payload.
func jsonRequest(t *testing.T, method, url string, payload any) *http.Request {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("jsonRequest: marshal: %v", err)
	}
	req := httptest.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// assertJSONError verifie que la reponse a un Content-Type application/json
// et un corps JSON contenant la cle "error".
func assertJSONError(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, voulu application/json", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("corps JSON invalide : %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("cle \"error\" absente du corps JSON : %v", body)
	}
}

package api

// Ce fichier contient les tests des handlers HTTP routing via net/http/httptest.
//
// Perimetre teste (sans base de donnees) :
//   - POST /routing/select : corps JSON invalide → 400 ; workspace_id manquant → 400 ;
//     workspace_id format invalide → 400 ; strategy invalide → 400.
//   - POST /routing/estimate : corps JSON invalide → 400 ; recipient manquant → 400 ;
//     body manquant → 400 ; strategy invalide → 400.
//   - Verification Content-Type application/json dans les reponses d'erreur.
//
// Perimetre NON teste (necessite une base de donnees reelle) :
//   - Chemins nominaux (lecture routing.provider_scores / routing.provider_pricing).
//   - 422 ErrNoProvider (la table doit etre vide — testable avec DB).
//   - Enrichissement contact-intelligence (necessite le service /contacts/{phone}/score).
//   - Affinement cout via provider.EstimateCost (necessite un adapter configure).
// Ces cas sont couverts par les tests d'acceptance (QA) avec une DB de test.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---- POST /routing/select ----

func TestHandleRoutingSelect_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/routing/select",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleRoutingSelect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingSelect_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"strategy": "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/routing/select", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingSelect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingSelect_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"workspace_id": "not-a-uuid",
		"strategy":     "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/routing/select", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingSelect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingSelect_invalidStrategy(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"workspace_id": "550e8400-e29b-41d4-a716-446655440000",
		"strategy":     "best_price", // invalide
	}
	req := jsonRequest(t, http.MethodPost, "/routing/select", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingSelect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (strategy invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingSelect_emptyBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/routing/select",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleRoutingSelect(rr, req)

	// body vide = workspace_id manquant → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body JSON vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingSelect_missingStrategy(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"workspace_id": "550e8400-e29b-41d4-a716-446655440000",
		// strategy absente → "" → isValidStrategy("") = false → 400
	}
	req := jsonRequest(t, http.MethodPost, "/routing/select", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingSelect(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (strategy manquante)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- POST /routing/estimate ----

func TestHandleRoutingEstimate_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/routing/estimate",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleRoutingEstimate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingEstimate_missingRecipient(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"body":     "Bonjour",
		"strategy": "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/routing/estimate", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingEstimate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (recipient manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingEstimate_missingBody(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"recipient": "+237600000001",
		"strategy":  "lowest_cost",
	}
	req := jsonRequest(t, http.MethodPost, "/routing/estimate", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingEstimate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingEstimate_invalidStrategy(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"recipient": "+237600000001",
		"body":      "Bonjour",
		"strategy":  "cheapest_ever", // invalide
	}
	req := jsonRequest(t, http.MethodPost, "/routing/estimate", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingEstimate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (strategy invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingEstimate_emptyBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/routing/estimate",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleRoutingEstimate(rr, req)

	// body vide = recipient manquant → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body JSON vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleRoutingEstimate_missingStrategy(t *testing.T) {
	svc := newTestService()
	payload := map[string]string{
		"recipient": "+237600000001",
		"body":      "Bonjour",
		// strategy absente → "" → invalide → 400
	}
	req := jsonRequest(t, http.MethodPost, "/routing/estimate", payload)
	rr := httptest.NewRecorder()

	svc.HandleRoutingEstimate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (strategy manquante)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

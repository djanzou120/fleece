package api

// campaigns_endpoints_test.go — Tests des handlers HTTP campaigns via net/http/httptest.
//
// Perimetre teste (sans base de donnees) :
//   POST /campaigns :
//     - Corps JSON invalide → 400.
//     - workspace_id manquant → 400.
//     - workspace_id UUID invalide → 400.
//     - name manquant → 400.
//     - channel_strategy invalide → 400.
//   PATCH /campaigns/{id}/schedule :
//     - id UUID invalide → 400.
//     - Corps JSON invalide → 400.
//     - scheduled_at manquant → 400.
//     - scheduled_at format invalide → 400.
//   PATCH /campaigns/{id}/cancel :
//     - id UUID invalide → 400.
//   POST /campaigns/{id}/recipients :
//     - id UUID invalide → 400.
//     - Corps JSON invalide → 400.
//     - recipients vide → 400.
//   GET /campaigns/{id} :
//     - id UUID invalide → 400.
//   GET /campaigns :
//     - workspace_id UUID invalide → 400.
//     - limit non numerique → 400.
//     - limit = 0 → 400.
//     - offset negatif → 400.
//     - offset non numerique → 400.
//
// Perimetre NON teste (necessite une base de donnees reelle) :
//   - POST /campaigns 201 : insertion DB reelle.
//   - PATCH /campaigns/{id}/schedule 200 : charge campagne, calcul cout, transition, UPDATE.
//   - PATCH /campaigns/{id}/schedule 409 (ErrInvalidTransition) : campagne deja scheduled.
//   - PATCH /campaigns/{id}/schedule 422 (ErrMissingMessageBody) : message_body vide.
//   - PATCH /campaigns/{id}/cancel 200 : transition valide draft → cancelled.
//   - PATCH /campaigns/{id}/cancel 409 : transition interdite (ex. completed).
//   - POST /campaigns/{id}/recipients 200 : import, comptage inseres vs doublons.
//   - GET /campaigns/{id} 200/404 : lecture DB.
//   - GET /campaigns 200 avec donnees et filtres.
// Ces cas sont couverts par les tests d'acceptance (QA) avec une DB de test.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	validCampaignUUID   = "550e8400-e29b-41d4-a716-446655440000"
	invalidCampaignUUID = "not-a-uuid"
)

// ---- POST /campaigns ----

func TestHandleCreateCampaign_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/campaigns",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleCreateCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateCampaign_emptyBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/campaigns",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleCreateCampaign(rr, req)

	// body vide → workspace_id manquant → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body JSON vide → workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateCampaign_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"name":         "Black Friday",
		"message_body": "Offre speciale !",
	}
	req := jsonRequest(t, http.MethodPost, "/campaigns", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateCampaign_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"workspace_id": invalidCampaignUUID,
		"name":         "Black Friday",
	}
	req := jsonRequest(t, http.MethodPost, "/campaigns", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateCampaign_missingName(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"workspace_id": validCampaignUUID,
		// name absent
	}
	req := jsonRequest(t, http.MethodPost, "/campaigns", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (name manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateCampaign_invalidStrategy(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"workspace_id":     validCampaignUUID,
		"name":             "Campagne Test",
		"channel_strategy": "best_price", // invalide
	}
	req := jsonRequest(t, http.MethodPost, "/campaigns", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (channel_strategy invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- PATCH /campaigns/{id}/schedule ----

func TestHandleScheduleCampaign_invalidUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"scheduled_at": "2026-08-01T10:00:00Z",
	}
	req := jsonRequest(t, http.MethodPatch, "/campaigns/not-a-uuid/schedule", payload)
	req.SetPathValue("id", invalidCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleScheduleCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleScheduleCampaign_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPatch, "/campaigns/"+validCampaignUUID+"/schedule",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", validCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleScheduleCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleScheduleCampaign_missingScheduledAt(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{} // scheduled_at absent
	req := jsonRequest(t, http.MethodPatch, "/campaigns/"+validCampaignUUID+"/schedule", payload)
	req.SetPathValue("id", validCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleScheduleCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (scheduled_at manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleScheduleCampaign_invalidScheduledAtFormat(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"scheduled_at": "01/08/2026 10:00", // format invalide (pas RFC3339)
	}
	req := jsonRequest(t, http.MethodPatch, "/campaigns/"+validCampaignUUID+"/schedule", payload)
	req.SetPathValue("id", validCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleScheduleCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (scheduled_at format invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- PATCH /campaigns/{id}/cancel ----

func TestHandleCancelCampaign_invalidUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPatch, "/campaigns/not-a-uuid/cancel", nil)
	req.SetPathValue("id", invalidCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleCancelCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCancelCampaign_emptyID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPatch, "/campaigns//cancel", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	svc.HandleCancelCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- POST /campaigns/{id}/recipients ----

func TestHandleImportRecipients_invalidUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"recipients": []string{"+237600000001"},
	}
	req := jsonRequest(t, http.MethodPost, "/campaigns/not-a-uuid/recipients", payload)
	req.SetPathValue("id", invalidCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleImportRecipients(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleImportRecipients_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/campaigns/"+validCampaignUUID+"/recipients",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", validCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleImportRecipients(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleImportRecipients_emptyRecipients(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"recipients": []string{}, // liste vide
	}
	req := jsonRequest(t, http.MethodPost, "/campaigns/"+validCampaignUUID+"/recipients", payload)
	req.SetPathValue("id", validCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleImportRecipients(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (recipients vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleImportRecipients_missingRecipientsKey(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{} // cle recipients absente → slice nil → vide
	req := jsonRequest(t, http.MethodPost, "/campaigns/"+validCampaignUUID+"/recipients", payload)
	req.SetPathValue("id", validCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleImportRecipients(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (recipients absent → nil → 400)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- GET /campaigns/{id} ----

func TestHandleGetCampaign_invalidUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/not-a-uuid", nil)
	req.SetPathValue("id", invalidCampaignUUID)
	rr := httptest.NewRecorder()

	svc.HandleGetCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetCampaign_emptyID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/campaigns/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	svc.HandleGetCampaign(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- GET /campaigns ----

func TestHandleListCampaigns_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/campaigns?workspace_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()

	svc.HandleListCampaigns(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListCampaigns_invalidLimit(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/campaigns?limit=abc", nil)
	rr := httptest.NewRecorder()

	svc.HandleListCampaigns(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (limit non-numerique)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListCampaigns_limitZero(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/campaigns?limit=0", nil)
	rr := httptest.NewRecorder()

	svc.HandleListCampaigns(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (limit=0)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListCampaigns_negativeOffset(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/campaigns?offset=-1", nil)
	rr := httptest.NewRecorder()

	svc.HandleListCampaigns(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (offset negatif)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListCampaigns_invalidOffset(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/campaigns?offset=notanint", nil)
	rr := httptest.NewRecorder()

	svc.HandleListCampaigns(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (offset non-numerique)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- Test de la fonction pure deriveChannelFromStrategy ----

func TestDeriveChannelFromStrategy(t *testing.T) {
	cases := []struct {
		strategy string
		expected string
	}{
		{"highest_delivery", "whatsapp"},
		{"lowest_cost", "sms"},
		{"round_robin", "sms"},
		{"", "sms"},
		{"custom", "sms"},
	}
	for _, tc := range cases {
		got := deriveChannelFromStrategy(tc.strategy)
		if got != tc.expected {
			t.Errorf("deriveChannelFromStrategy(%q) = %q, voulu %q", tc.strategy, got, tc.expected)
		}
	}
}

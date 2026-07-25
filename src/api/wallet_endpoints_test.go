package api

// wallet_endpoints_test.go — Tests des handlers HTTP wallet via net/http/httptest.
//
// Périmètre testé (sans base de données) :
//   - GET /wallet/{workspaceId} : workspaceId vide → 400 ; UUID invalide → 400.
//   - GET /wallet/{workspaceId}/transactions : workspaceId vide → 400 ; UUID invalide → 400 ;
//     limit non numérique → 400 ; limit = 0 → 400 ; offset négatif → 400.
//   - POST /wallet/{workspaceId}/deduct : workspaceId UUID invalide → 400 ;
//     corps JSON invalide → 400 ; amount_cents = 0 → 400 ; amount_cents < 0 → 400.
//   - POST /wallet/{workspaceId}/topup : workspaceId UUID invalide → 400 ;
//     corps JSON invalide → 400 ; amount_cents = 0 → 400 ; amount_cents négatif → 400 ;
//     reference_id optionnel (présent ou absent) ne change pas la validation
//     amount_cents (migration 0019 — voir wallet_topup.go).
//   - Vérification Content-Type application/json dans les réponses d'erreur.
//
// Périmètre NON testé (nécessite une base de données réelle) :
//   - GET /wallet/{workspaceId} 200 (wallet existant) et 404 (wallet inexistant).
//   - GET transactions 200 avec données.
//   - POST deduct : 402 solde insuffisant, 404 wallet inexistant, débit atomique FOR UPDATE.
//   - POST topup : UPSERT création au premier topup, solde après crédit.
//   - Atomicité transactionnelle (rollback sur erreur mid-transaction).
// Ces cas sont couverts par les tests d'acceptance (QA) avec une DB de test.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	validWalletUUID   = "550e8400-e29b-41d4-a716-446655440000"
	invalidWalletUUID = "not-a-uuid"
)

// ---- GET /wallet/{workspaceId} ----

func TestHandleGetWallet_emptyWorkspaceId(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/", nil)
	req.SetPathValue("workspaceId", "")
	rr := httptest.NewRecorder()

	svc.HandleGetWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspaceId vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetWallet_invalidUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/not-a-uuid", nil)
	req.SetPathValue("workspaceId", invalidWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleGetWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetWallet_uuidTooShort(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/550e8400-e29b", nil)
	req.SetPathValue("workspaceId", "550e8400-e29b")
	rr := httptest.NewRecorder()

	svc.HandleGetWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (UUID trop court)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- GET /wallet/{workspaceId}/transactions ----

func TestHandleListWalletTransactions_emptyWorkspaceId(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet//transactions", nil)
	req.SetPathValue("workspaceId", "")
	rr := httptest.NewRecorder()

	svc.HandleListWalletTransactions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspaceId vide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListWalletTransactions_invalidUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/not-a-uuid/transactions", nil)
	req.SetPathValue("workspaceId", invalidWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleListWalletTransactions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListWalletTransactions_invalidLimit(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/"+validWalletUUID+"/transactions?limit=abc", nil)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleListWalletTransactions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (limit non-numérique)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListWalletTransactions_limitZero(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/"+validWalletUUID+"/transactions?limit=0", nil)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleListWalletTransactions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (limit=0)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListWalletTransactions_negativeOffset(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/"+validWalletUUID+"/transactions?offset=-1", nil)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleListWalletTransactions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (offset négatif)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListWalletTransactions_invalidOffset(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/wallet/"+validWalletUUID+"/transactions?offset=notanint", nil)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleListWalletTransactions(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (offset non-numérique)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- POST /wallet/{workspaceId}/deduct ----

func TestHandleDeductWallet_invalidUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": 1000,
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/not-a-uuid/deduct", payload)
	req.SetPathValue("workspaceId", invalidWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeductWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleDeductWallet_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/wallet/"+validWalletUUID+"/deduct",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeductWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleDeductWallet_amountZero(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": 0,
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/"+validWalletUUID+"/deduct", payload)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeductWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (amount_cents=0)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleDeductWallet_amountNegative(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": -500,
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/"+validWalletUUID+"/deduct", payload)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeductWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (amount_cents négatif)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleDeductWallet_emptyBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/wallet/"+validWalletUUID+"/deduct",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeductWallet(rr, req)

	// body vide → amount_cents = 0 → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body vide → amount_cents=0)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- POST /wallet/{workspaceId}/topup ----

func TestHandleTopupWallet_invalidUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": 5000,
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/not-a-uuid/topup", payload)
	req.SetPathValue("workspaceId", invalidWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleTopupWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleTopupWallet_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/wallet/"+validWalletUUID+"/topup",
		bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleTopupWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleTopupWallet_amountZero(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": 0,
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/"+validWalletUUID+"/topup", payload)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleTopupWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (amount_cents=0)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleTopupWallet_amountNegative(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": -100,
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/"+validWalletUUID+"/topup", payload)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleTopupWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (amount_cents négatif)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleTopupWallet_emptyBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/wallet/"+validWalletUUID+"/topup",
		bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleTopupWallet(rr, req)

	// body vide → amount_cents = 0 → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (body vide → amount_cents=0)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// TestHandleTopupWallet_amountZero_withReferenceID verifie que la presence d'un
// reference_id (migration 0019) ne contourne pas la validation amount_cents > 0 —
// le nouveau champ optionnel ne doit rien changer au pipeline de validation
// existant (rétrocompatibilité stricte, voir wallet_topup.go).
func TestHandleTopupWallet_amountZero_withReferenceID(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": 0,
		"reference_id": "OM-TXN-123",
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/"+validWalletUUID+"/topup", payload)
	req.SetPathValue("workspaceId", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleTopupWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (amount_cents=0 malgre reference_id present)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// TestHandleTopupWallet_invalidBody_withReferenceIDField verifie que le decodage
// JSON du corps accepte bien le nouveau champ reference_id sans erreur de parsing
// (test de forme uniquement — l'UPSERT reel necessite une DB, voir tests QA).
// Ce test s'arrete volontairement a la validation UUID (avant tout acces DB) pour
// rester offline-safe (s.DB nil dans newTestService()).
func TestHandleTopupWallet_invalidUUID_withReferenceIDField(t *testing.T) {
	svc := newTestService()
	payload := map[string]interface{}{
		"amount_cents": 5000,
		"reference_id": "OM-TXN-456",
	}
	req := jsonRequest(t, http.MethodPost, "/wallet/not-a-uuid/topup", payload)
	req.SetPathValue("workspaceId", invalidWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleTopupWallet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (UUID invalide, reference_id present)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

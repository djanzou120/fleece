package api

// webhook_endpoints_test.go — Tests des handlers HTTP CRUD des webhooks SORTANTS
// (D-M40 / T-005 : POST/GET/DELETE /webhook-endpoints) via net/http/httptest.
//
// Deux familles de tests :
//
//   - Validation pure (newTestService, sans DB) : corps/paramètres invalides → 400.
//   - Chemin DB réellement exécuté, via le faux driver database/sql/driver défini
//     dans qa_m019_dbpath_test.go (fakeConn/fakeRows/newFakeGosqlDB, même package) :
//     vérifie la FORME du SQL émis (table, colonnes, clauses, placeholders, ordre
//     des arguments) et les branchements 0/1/>1 lignes affectées. En particulier :
//       - le secret est présent dans la réponse 201 de création, absent de la liste ;
//       - le WHERE workspace_id=$N est bien présent sur List ET Delete (scoping,
//         D-M41 — ne pas reproduire la fuite inter-workspaces de /messages/{id}) ;
//       - Delete émet un UPDATE ... SET active = false (suppression logique), jamais
//         un DELETE physique.

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	golog "fleece/src/go/log"
)

// ============================================================
// Validation pure — POST /webhook-endpoints
// ============================================================

func TestHandleCreateWebhookEndpoint_invalidBody(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/webhook-endpoints", bytes.NewBufferString(`not valid json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (corps invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateWebhookEndpoint_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	payload := map[string]any{
		"url":    "https://example.com/hook",
		"events": []string{"message.delivered"},
	}
	req := jsonRequest(t, http.MethodPost, "/webhook-endpoints", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateWebhookEndpoint_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	payload := map[string]any{
		"workspace_id": "not-a-uuid",
		"url":          "https://example.com/hook",
		"events":       []string{"message.delivered"},
	}
	req := jsonRequest(t, http.MethodPost, "/webhook-endpoints", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateWebhookEndpoint_missingURL(t *testing.T) {
	svc := newTestService()
	payload := map[string]any{
		"workspace_id": validWalletUUID,
		"events":       []string{"message.delivered"},
	}
	req := jsonRequest(t, http.MethodPost, "/webhook-endpoints", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (url manquante)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleCreateWebhookEndpoint_noEvents(t *testing.T) {
	svc := newTestService()
	payload := map[string]any{
		"workspace_id": validWalletUUID,
		"url":          "https://example.com/hook",
		"events":       []string{},
	}
	req := jsonRequest(t, http.MethodPost, "/webhook-endpoints", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (0 evenement)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ============================================================
// Chemin DB réel — POST /webhook-endpoints
// ============================================================

func TestHandleCreateWebhookEndpoint_nominal_generatesSecret(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 1}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	payload := map[string]any{
		"workspace_id": validWalletUUID,
		"url":          "https://example.com/hook",
		"events":       []string{"message.delivered", "message.failed"},
	}
	req := jsonRequest(t, http.MethodPost, "/webhook-endpoints", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, voulu %d : %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	if !strings.Contains(conn.lastQuery, "INSERT INTO webhook.webhook_endpoints") {
		t.Fatalf("requete SQL inattendue (table) : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "events, active, created_at") {
		t.Fatalf("requete SQL inattendue (colonnes) : %q", conn.lastQuery)
	}
	// active=true est un literal SQL (pas un placeholder) — voir webhook_endpoints_create.go.
	// Args attendus : id($1), workspace_id($2), url($3), secret($4), events($5), created_at($6).
	if len(conn.lastArgs) != 6 {
		t.Fatalf("nombre d'arguments = %d, voulu 6 : %+v", len(conn.lastArgs), conn.lastArgs)
	}
	if got := conn.lastArgs[1].Value; got != validWalletUUID {
		t.Errorf("arg[1] (workspace_id) = %v, voulu %q", got, validWalletUUID)
	}
	if got, ok := conn.lastArgs[4].Value.(string); !ok || !strings.Contains(got, "message.delivered") {
		t.Errorf("arg[4] (events, pq.Array) = %v, voulu un literal Postgres contenant message.delivered", conn.lastArgs[4].Value)
	}
	if !strings.Contains(conn.lastQuery, "VALUES ($1, $2, $3, $4, $5::text[], true, $6)") {
		t.Fatalf("requete SQL : active devrait etre un literal true (pas un placeholder) : %q", conn.lastQuery)
	}

	var resp createWebhookEndpointResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode reponse : %v", err)
	}
	if resp.Secret == "" {
		t.Error("secret absent de la reponse 201 (devrait etre genere via crypto/rand)")
	}
	if len(resp.Secret) != 64 { // 32 octets hex = 64 caracteres
		t.Errorf("longueur du secret genere = %d, voulu 64 (32 octets hex)", len(resp.Secret))
	}
	if !resp.Active {
		t.Error("Active = false, voulu true (actif par defaut a la creation)")
	}
	if len(resp.Events) != 2 {
		t.Errorf("Events = %v, voulu 2 elements", resp.Events)
	}
}

func TestHandleCreateWebhookEndpoint_providedSecret_isPreserved(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 1}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	payload := map[string]any{
		"workspace_id": validWalletUUID,
		"url":          "https://example.com/hook",
		"secret":       "mon-secret-explicite",
		"events":       []string{"message.delivered"},
	}
	req := jsonRequest(t, http.MethodPost, "/webhook-endpoints", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("code = %d, voulu %d", rr.Code, http.StatusCreated)
	}
	var resp createWebhookEndpointResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode reponse : %v", err)
	}
	if resp.Secret != "mon-secret-explicite" {
		t.Errorf("secret = %q, voulu le secret fourni explicitement", resp.Secret)
	}
	if got := conn.lastArgs[3].Value; got != "mon-secret-explicite" {
		t.Errorf("arg[3] (secret) = %v, voulu le secret fourni", got)
	}
}

func TestHandleCreateWebhookEndpoint_dbError(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return nil, errors.New("connexion perdue")
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	payload := map[string]any{
		"workspace_id": validWalletUUID,
		"url":          "https://example.com/hook",
		"events":       []string{"message.delivered"},
	}
	req := jsonRequest(t, http.MethodPost, "/webhook-endpoints", payload)
	rr := httptest.NewRecorder()

	svc.HandleCreateWebhookEndpoint(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, voulu %d (erreur DB)", rr.Code, http.StatusInternalServerError)
	}
	assertJSONError(t, rr)
}

// ============================================================
// Validation pure — GET /webhook-endpoints
// ============================================================

func TestHandleListWebhookEndpoints_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/webhook-endpoints", nil)
	rr := httptest.NewRecorder()

	svc.HandleListWebhookEndpoints(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleListWebhookEndpoints_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/webhook-endpoints?workspace_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()

	svc.HandleListWebhookEndpoints(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ============================================================
// Chemin DB réel — GET /webhook-endpoints
// ============================================================

func TestHandleListWebhookEndpoints_nominal_scopedToWorkspaceAndNoSecret(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{
			columns: []string{"id", "workspace_id", "url", "events", "active", "created_at"},
			data: [][]driver.Value{
				{
					"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
					validWalletUUID,
					"https://example.com/hook",
					"{message.delivered,message.failed}",
					true,
					time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC),
				},
			},
		}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/webhook-endpoints?workspace_id="+validWalletUUID, nil)
	rr := httptest.NewRecorder()

	svc.HandleListWebhookEndpoints(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, voulu %d : %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	if !strings.Contains(conn.lastQuery, "FROM webhook.webhook_endpoints") {
		t.Fatalf("requete SQL inattendue (table) : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "WHERE workspace_id = $1") {
		t.Fatalf("requete SQL sans scoping workspace_id (D-M41) : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "AND active = true") {
		t.Fatalf("requete SQL sans filtre active=true (suppression logique) : %q", conn.lastQuery)
	}
	if got := conn.lastArgs[0].Value; got != validWalletUUID {
		t.Errorf("arg[0] (workspace_id) = %v, voulu %q", got, validWalletUUID)
	}

	// Le corps JSON brut ne doit JAMAIS contenir la cle "secret" (D-M40).
	if strings.Contains(rr.Body.String(), "secret") {
		t.Fatalf("la reponse de liste contient le mot \"secret\" (fuite potentielle) : %s", rr.Body.String())
	}

	var rows []webhookEndpointListRow
	if err := json.Unmarshal(rr.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode reponse : %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("nombre de resultats = %d, voulu 1", len(rows))
	}
	if len(rows[0].Events) != 2 {
		t.Errorf("Events = %v, voulu 2 elements (text[] decode via pq.StringArray)", rows[0].Events)
	}
	if !rows[0].Active {
		t.Error("Active = false, voulu true")
	}
}

func TestHandleListWebhookEndpoints_empty(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"id", "workspace_id", "url", "events", "active", "created_at"}}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/webhook-endpoints?workspace_id="+validWalletUUID, nil)
	rr := httptest.NewRecorder()

	svc.HandleListWebhookEndpoints(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d, voulu %d", rr.Code, http.StatusOK)
	}
	if strings.TrimSpace(rr.Body.String()) != "[]" {
		t.Errorf("body = %q, voulu un tableau JSON vide (jamais null)", rr.Body.String())
	}
}

func TestHandleListWebhookEndpoints_dbError(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return nil, errors.New("connexion perdue")
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/webhook-endpoints?workspace_id="+validWalletUUID, nil)
	rr := httptest.NewRecorder()

	svc.HandleListWebhookEndpoints(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, voulu %d (erreur DB)", rr.Code, http.StatusInternalServerError)
	}
	assertJSONError(t, rr)
}

// ============================================================
// Validation pure — DELETE /webhook-endpoints/{id}
// ============================================================

func TestHandleDeleteWebhookEndpoint_missingID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodDelete, "/webhook-endpoints/", nil)
	req.SetPathValue("id", "")
	rr := httptest.NewRecorder()

	svc.HandleDeleteWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleDeleteWebhookEndpoint_invalidID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodDelete, "/webhook-endpoints/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rr := httptest.NewRecorder()

	svc.HandleDeleteWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleDeleteWebhookEndpoint_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodDelete, "/webhook-endpoints/"+validWalletUUID, nil)
	req.SetPathValue("id", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeleteWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleDeleteWebhookEndpoint_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodDelete, "/webhook-endpoints/"+validWalletUUID+"?workspace_id=not-a-uuid", nil)
	req.SetPathValue("id", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeleteWebhookEndpoint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ============================================================
// Chemin DB réel — DELETE /webhook-endpoints/{id}
// ============================================================

func TestHandleDeleteWebhookEndpoint_nominal_logicalDeleteScopedToWorkspace(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 1}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	otherWorkspace := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	req := httptest.NewRequest(http.MethodDelete, "/webhook-endpoints/"+validWalletUUID+"?workspace_id="+otherWorkspace, nil)
	req.SetPathValue("id", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeleteWebhookEndpoint(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("code = %d, voulu %d : %s", rr.Code, http.StatusNoContent, rr.Body.String())
	}

	if !strings.Contains(conn.lastQuery, "UPDATE webhook.webhook_endpoints") {
		t.Fatalf("requete SQL inattendue (devrait etre un UPDATE, suppression logique) : %q", conn.lastQuery)
	}
	if strings.Contains(conn.lastQuery, "DELETE FROM") {
		t.Fatalf("requete SQL contient un DELETE physique — la decision D-M40 impose une suppression logique : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "SET active = false") {
		t.Fatalf("requete SQL sans SET active = false : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "WHERE id = $1") || !strings.Contains(conn.lastQuery, "AND workspace_id = $2") {
		t.Fatalf("requete SQL sans scoping workspace_id (D-M41) : %q", conn.lastQuery)
	}
	if len(conn.lastArgs) != 2 {
		t.Fatalf("nombre d'arguments = %d, voulu 2 : %+v", len(conn.lastArgs), conn.lastArgs)
	}
	if got := conn.lastArgs[0].Value; got != validWalletUUID {
		t.Errorf("arg[0] (id) = %v, voulu %q", got, validWalletUUID)
	}
	if got := conn.lastArgs[1].Value; got != otherWorkspace {
		t.Errorf("arg[1] (workspace_id) = %v, voulu %q", got, otherWorkspace)
	}
}

func TestHandleDeleteWebhookEndpoint_notFound_zeroRows(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 0}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodDelete, "/webhook-endpoints/"+validWalletUUID+"?workspace_id="+validWalletUUID, nil)
	req.SetPathValue("id", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeleteWebhookEndpoint(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("code = %d, voulu %d (0 ligne affectee — id inconnu ou autre workspace)", rr.Code, http.StatusNotFound)
	}
	assertJSONError(t, rr)
}

func TestHandleDeleteWebhookEndpoint_dbError(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return nil, errors.New("connexion perdue")
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodDelete, "/webhook-endpoints/"+validWalletUUID+"?workspace_id="+validWalletUUID, nil)
	req.SetPathValue("id", validWalletUUID)
	rr := httptest.NewRecorder()

	svc.HandleDeleteWebhookEndpoint(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, voulu %d (erreur DB)", rr.Code, http.StatusInternalServerError)
	}
	assertJSONError(t, rr)
}

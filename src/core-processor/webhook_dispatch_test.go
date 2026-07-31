package coreprocessor

// webhook_dispatch_test.go — teste le fan-out de webhooks sortants a la
// reception d'un evenement (D-M43) : construction du payload, selection des
// endpoints abonnes, dispatch HTTP signe (httptest.NewServer), persistance de
// la delivery, et la garde d'idempotence anti-doublon (non-regression B1).
import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// Fonctions pures
// ============================================================

func TestWebhookEventStatus(t *testing.T) {
	cases := []struct {
		event string
		want  string
	}{
		{"message.delivered", "delivered"},
		{"message.failed", "failed"},
		{"message.sent", "message.sent"}, // pas gere par ce worker, retombe sur l'event tel quel
	}
	for _, tc := range cases {
		if got := webhookEventStatus(tc.event); got != tc.want {
			t.Errorf("webhookEventStatus(%q) = %q, voulu %q", tc.event, got, tc.want)
		}
	}
}

func TestIsSuccessStatusCode(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{301, false},
		{400, false},
		{404, false},
		{500, false},
		{100, false},
	}
	for _, tc := range cases {
		if got := isSuccessStatusCode(tc.code); got != tc.want {
			t.Errorf("isSuccessStatusCode(%d) = %v, voulu %v", tc.code, got, tc.want)
		}
	}
}

func TestBuildWebhookPayload(t *testing.T) {
	fixedNow := func() time.Time { return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC) }
	evt := messageEvent{
		MessageID:  "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		ExternalID: "42",
		ProviderID: "telegram-bot",
		Status:     "delivered",
		Source:     "telegram",
	}

	raw, err := buildWebhookPayload(fixedNow, "message.delivered", evt)
	if err != nil {
		t.Fatalf("buildWebhookPayload() erreur inattendue : %v", err)
	}

	var decoded webhookEventPayload
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("payload invalide : %v", err)
	}
	if decoded.Event != "message.delivered" {
		t.Errorf("Event = %q, voulu message.delivered", decoded.Event)
	}
	if decoded.MessageID != evt.MessageID {
		t.Errorf("MessageID = %q, voulu %q", decoded.MessageID, evt.MessageID)
	}
	if decoded.ExternalID != "42" {
		t.Errorf("ExternalID = %q, voulu 42", decoded.ExternalID)
	}
	if decoded.ProviderID != "telegram-bot" {
		t.Errorf("ProviderID = %q, voulu telegram-bot", decoded.ProviderID)
	}
	// Status est DEDUIT du nom d'evenement, pas recopie de evt.Status (D-M21 :
	// le payload AMQP n'est jamais une source de verite pour une decision).
	if decoded.Status != "delivered" {
		t.Errorf("Status = %q, voulu delivered (deduit de l'evenement, pas de evt.Status)", decoded.Status)
	}
	if decoded.OccurredAt != "2026-07-28T12:00:00Z" {
		t.Errorf("OccurredAt = %q, voulu 2026-07-28T12:00:00Z", decoded.OccurredAt)
	}
}

// ============================================================
// postWebhook — httptest.NewServer
// ============================================================

func TestPostWebhook_success_signatureHeaderCorrect(t *testing.T) {
	const secret = "un-secret-de-test"
	payload := []byte(`{"event":"message.delivered"}`)
	wantSig := signWebhookPayload(secret, payload)

	var gotSig string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Fleece-Signature")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{Logger: testLogger()}
	code, err := svc.postWebhook(context.Background(), srv.URL, payload, wantSig)
	if err != nil {
		t.Fatalf("postWebhook() erreur inattendue : %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("code = %d, voulu 200", code)
	}
	if gotSig != wantSig {
		t.Errorf("X-Fleece-Signature recu = %q, voulu %q", gotSig, wantSig)
	}
	// Le serveur de test recalcule la signature attendue a partir du corps
	// RECU (et non du payload envoye par le client) : preuve que le corps
	// n'est pas altere en transit et que la signature correspond bien a CE
	// corps.
	if recomputed := signWebhookPayload(secret, gotBody); recomputed != gotSig {
		t.Errorf("signature recue ne correspond pas au HMAC du corps recu : recompute=%q recu=%q", recomputed, gotSig)
	}
	if string(gotBody) != string(payload) {
		t.Errorf("corps recu = %q, voulu %q", gotBody, payload)
	}
}

func TestPostWebhook_nonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := &Service{Logger: testLogger()}
	code, err := svc.postWebhook(context.Background(), srv.URL, []byte(`{}`), "sig")
	if err != nil {
		t.Fatalf("postWebhook() erreur inattendue (statut HTTP non-2xx n'est pas une erreur reseau) : %v", err)
	}
	if code != http.StatusInternalServerError {
		t.Errorf("code = %d, voulu 500", code)
	}
}

func TestPostWebhook_invalidResponseBody_doesNotFail(t *testing.T) {
	// Un corps de reponse illisible/invalide ne doit jamais faire echouer
	// postWebhook : seul le code HTTP est exploite (voir http_dispatcher de
	// reference, meme comportement).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ceci n'est pas du JSON valide {{{"))
	}))
	defer srv.Close()

	svc := &Service{Logger: testLogger()}
	code, err := svc.postWebhook(context.Background(), srv.URL, []byte(`{}`), "sig")
	if err != nil {
		t.Fatalf("postWebhook() erreur inattendue : %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("code = %d, voulu 200", code)
	}
}

func TestPostWebhook_timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Client a timeout tres court (5ms << 100ms de latence serveur) — evite
	// d'attendre les 10s reelles de webhookDispatchTimeout en production.
	svc := &Service{Logger: testLogger(), WebhookHTTPClient: &http.Client{Timeout: 5 * time.Millisecond}}
	code, err := svc.postWebhook(context.Background(), srv.URL, []byte(`{}`), "sig")
	if err == nil {
		t.Fatalf("postWebhook() attendu en erreur (timeout), code = %d", code)
	}
	if code != 0 {
		t.Errorf("code = %d, voulu 0 sur erreur reseau", code)
	}
}

// ============================================================
// dispatchToNewEndpoint — persistance (fakeConn)
//
// B2 (BLOQUANT, revue architecture Phase 3) — ORDRE INSERT-AVANT-POST :
// dispatchToNewEndpoint emet desormais 2 requetes dans l'ordre : [0] INSERT
// 'pending' (insertPendingWebhookDelivery, AVANT le POST), [1] UPDATE du
// resultat (finalizeNewWebhookDelivery, APRES le POST). Le faux driver
// (fakeConn) ne repond au INSERT ... RETURNING id que via querySteps (c'est
// un Query, pas un Exec — meme motif que les autres RETURNING de ce depot).
// ============================================================

func TestDispatchToNewEndpoint_recordsDeliveredOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOf("id", int64(101))}, // INSERT ... RETURNING id
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	ep := webhookEndpointRow{ID: "ep-1", URL: srv.URL, Secret: "s3cr3t"}
	svc.dispatchToNewEndpoint(context.Background(), ep, "message.delivered", []byte(`{"event":"message.delivered"}`))

	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2 (B2 : INSERT pending AVANT le POST, puis UPDATE du resultat) : %+v", len(conn.queries), conn.queries)
	}
	if !strings.Contains(conn.queries[0], "INSERT INTO webhook.webhook_deliveries") {
		t.Fatalf("1ere requete inattendue (devrait etre l'INSERT pending, B2) : %q", conn.queries[0])
	}
	insertArgs := conn.args[0]
	if got := insertArgs[0].Value; got != "ep-1" {
		t.Errorf("endpoint_id (INSERT) = %v, voulu ep-1", got)
	}
	if got := insertArgs[2].Value; got != webhookStatusPending {
		t.Errorf("status (INSERT) = %v, voulu %q (B2 : pending AVANT le POST)", got, webhookStatusPending)
	}

	if !strings.Contains(conn.queries[1], "UPDATE webhook.webhook_deliveries") ||
		!strings.Contains(conn.queries[1], "SET status = $1, next_retry_at = $2") {
		t.Fatalf("2e requete inattendue (devrait etre l'UPDATE de finalisation) : %q", conn.queries[1])
	}
	finalizeArgs := conn.args[1]
	if got := finalizeArgs[0].Value; got != webhookStatusDelivered {
		t.Errorf("status (UPDATE) = %v, voulu %q", got, webhookStatusDelivered)
	}
	if finalizeArgs[1].Value != nil {
		t.Errorf("next_retry_at (UPDATE) = %v, voulu NULL (livraison reussie)", finalizeArgs[1].Value)
	}
	if got := finalizeArgs[2].Value; got != int64(101) {
		t.Errorf("id finalise = %v, voulu 101 (id retourne par l'INSERT)", got)
	}
}

func TestDispatchToNewEndpoint_recordsFailedWithBackoffOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOf("id", int64(202))},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	ep := webhookEndpointRow{ID: "ep-1", URL: srv.URL, Secret: "s3cr3t"}
	before := time.Now()
	svc.dispatchToNewEndpoint(context.Background(), ep, "message.failed", []byte(`{"event":"message.failed"}`))

	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2 (INSERT pending + UPDATE resultat)", len(conn.queries))
	}
	finalizeArgs := conn.args[1]
	if got := finalizeArgs[0].Value; got != webhookStatusFailed {
		t.Errorf("status = %v, voulu %q", got, webhookStatusFailed)
	}
	nrt, ok := finalizeArgs[1].Value.(time.Time)
	if !ok {
		t.Fatalf("next_retry_at absent ou de mauvais type : %v (%T)", finalizeArgs[1].Value, finalizeArgs[1].Value)
	}
	// backoffFor(1) == 1 minute (premiere tentative).
	if diff := nrt.Sub(before); diff < 59*time.Second || diff > 61*time.Second {
		t.Errorf("next_retry_at = %v apres l'echec (%v), voulu ~1 minute (backoffFor(1))", nrt, diff)
	}
}

// TestInsertPendingWebhookDelivery_setsBailNextRetryAt verifie que l'INSERT
// pending (B2) positionne bien next_retry_at au bail de reservation (B1,
// webhookRetryClaimBail) — pour qu'une ligne 'pending' orpheline (worker mort
// avant finalizeNewWebhookDelivery) soit rattrapee par le scheduler de retry
// sans aucune intervention (voir doc de tete de fichier).
func TestInsertPendingWebhookDelivery_setsBailNextRetryAt(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOf("id", int64(1))},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	if _, err := svc.insertPendingWebhookDelivery(context.Background(), "ep-1", "message.delivered", []byte(`{}`)); err != nil {
		t.Fatalf("insertPendingWebhookDelivery() erreur inattendue : %v", err)
	}

	// attempts=1 est un LITERAL SQL, pas un placeholder ; et le bail est
	// applique a now() COTE POSTGRES (N4, meme raison que dans
	// runWebhookRetryTick : le scheduler de retry compare next_retry_at au
	// now() de Postgres, un timestamp calcule cote Go derive) — voir
	// insertPendingWebhookDelivery.
	if !strings.Contains(conn.queries[0], "VALUES ($1, $2, $3, 1, $4, now() + make_interval(secs => $5))") {
		t.Fatalf("requete INSERT inattendue (attempts devrait etre le literal 1, et le bail calcule cote serveur) : %q", conn.queries[0])
	}
	args := conn.args[0]
	if got := args[2].Value; got != webhookStatusPending {
		t.Errorf("status = %v, voulu %q", got, webhookStatusPending)
	}
	if _, isTime := args[4].Value.(time.Time); isTime {
		t.Fatalf("le bail est passe comme timestamp calcule cote Go (%v) — N4 : il doit etre un nombre de secondes applique a now() cote Postgres", args[4].Value)
	}
	bailSecs, ok := args[4].Value.(float64)
	if !ok {
		t.Fatalf("next_retry_at absent ou de mauvais type : %v (%T)", args[4].Value, args[4].Value)
	}
	if bailSecs != webhookRetryClaimBail.Seconds() {
		t.Errorf("next_retry_at = now() + %v s, voulu %v s (webhookRetryClaimBail)", bailSecs, webhookRetryClaimBail.Seconds())
	}
}

// TestFinalizeNewWebhookDelivery_survivesCanceledContext est le pendant, pour
// la PREMIERE tentative (B2), de TestFinalizeWebhookRetry_survivesCanceledContext :
// le POST a deja eu lieu (statut connu) quand cette fonction est appelee — un
// ctx appelant deja annule (SIGTERM pendant handleMessageDelivered/
// handleMessageFailed) ne doit JAMAIS empecher la persistance du resultat
// (contexte detache, B1).
func TestFinalizeNewWebhookDelivery_survivesCanceledContext(t *testing.T) {
	conn := &fakeConn{}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := svc.finalizeNewWebhookDelivery(ctx, 7, webhookStatusDelivered, nil); err != nil {
		t.Fatalf("finalizeNewWebhookDelivery() erreur inattendue avec ctx appelant annule : %v", err)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("requete de finalisation non executee malgre le contexte detache : %d requetes", len(conn.queries))
	}
}

// ============================================================
// fanOutWebhookEvent — selection des endpoints + fan-out concurrent borne
// ============================================================

func TestFanOutWebhookEvent_emptyWorkspaceID_noQuery(t *testing.T) {
	conn := &fakeConn{}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.fanOutWebhookEvent(context.Background(), "message.delivered", messageEvent{MessageID: testMessageID}, "")

	if len(conn.queries) != 0 {
		t.Fatalf("nombre de requetes = %d, voulu 0 (workspace_id vide -> annule avant toute requete) : %+v", len(conn.queries), conn.queries)
	}
}

func TestFanOutWebhookEvent_noSubscribedEndpoints_noHTTPCall(t *testing.T) {
	var httpCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&httpCalls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"id", "url", "secret"})}, // 0 endpoint abonne
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.fanOutWebhookEvent(context.Background(), "message.delivered", messageEvent{MessageID: testMessageID}, testWorkspaceID)

	if atomic.LoadInt32(&httpCalls) != 0 {
		t.Errorf("appels HTTP = %d, voulu 0 (aucun endpoint abonne)", httpCalls)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (selection des endpoints, aucune persistance de delivery)", len(conn.queries))
	}
}

func TestFanOutWebhookEvent_loadEndpointsQuery_scopedAndFiltered(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"id", "url", "secret"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.fanOutWebhookEvent(context.Background(), "message.failed", messageEvent{MessageID: testMessageID}, testWorkspaceID)

	q := conn.queries[0]
	if !strings.Contains(q, "FROM webhook.webhook_endpoints") {
		t.Fatalf("table inattendue : %q", q)
	}
	if !strings.Contains(q, "workspace_id = $1") || !strings.Contains(q, "active = true") || !strings.Contains(q, "ANY(events)") {
		t.Fatalf("requete de selection inattendue (scoping/active/events) : %q", q)
	}
	args := conn.args[0]
	if args[0].Value != testWorkspaceID || args[1].Value != "message.failed" {
		t.Fatalf("arguments inattendus : %+v", args)
	}
}

// TestFanOutWebhookEvent_oneEndpointFails_otherStillDelivered est LE test de
// la regle "l'echec de l'un ne doit pas empecher les autres" : un endpoint
// injoignable (port ferme) et un endpoint qui repond 200 sont tous deux
// abonnes -> les DEUX doivent avoir une delivery persistee (le compte
// d'INSERT vaut 2, quel que soit l'ordre — le fan-out est concurrent borne
// via syncx.Map, cf. fakeConn.mu pour la sequence thread-safe).
func TestFanOutWebhookEvent_oneEndpointFails_otherStillDelivered(t *testing.T) {
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okSrv.Close()

	// Port TCP ferme (personne n'ecoute) : la connexion est refusee
	// immediatement, pas besoin d'attendre un timeout.
	const deadURL = "http://127.0.0.1:1"

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"id", "url", "secret"},
			[]driver.Value{"ep-ok", okSrv.URL, "secret-ok"},
			[]driver.Value{"ep-dead", deadURL, "secret-dead"},
		)},
		// B2 : chaque dispatchToNewEndpoint insere D'ABORD une ligne 'pending'
		// (INSERT ... RETURNING id, un Query) AVANT le POST — il en faut donc
		// une par endpoint ici, peu importe l'ordre (fan-out concurrent).
		{rows: fakeRowsOf("id", int64(1))},
		{rows: fakeRowsOf("id", int64(2))},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.fanOutWebhookEvent(context.Background(), "message.delivered", messageEvent{MessageID: testMessageID}, testWorkspaceID)

	// 1 selection + 2 INSERT pending (Query) + 2 UPDATE de finalisation (Exec) = 5.
	if len(conn.queries) != 5 {
		t.Fatalf("nombre de requetes = %d, voulu 5 (selection + 2 INSERT pending + 2 UPDATE finalisation) : %+v", len(conn.queries), conn.queries)
	}
	insertCount := 0
	for _, q := range conn.queries {
		if strings.Contains(q, "INSERT INTO webhook.webhook_deliveries") {
			insertCount++
		}
	}
	if insertCount != 2 {
		t.Errorf("nombre d'INSERT pending = %d, voulu 2 (un par endpoint, succes ET echec)", insertCount)
	}
	seenStatuses := map[string]int{}
	for i, q := range conn.queries {
		if !strings.Contains(q, "UPDATE webhook.webhook_deliveries") {
			continue
		}
		status, _ := conn.args[i][0].Value.(string)
		seenStatuses[status]++
	}
	if seenStatuses[webhookStatusDelivered] != 1 {
		t.Errorf("deliveries finalisees en statut %q = %d, voulu 1 (endpoint disponible)", webhookStatusDelivered, seenStatuses[webhookStatusDelivered])
	}
	if seenStatuses[webhookStatusFailed] != 1 {
		t.Errorf("deliveries finalisees en statut %q = %d, voulu 1 (endpoint injoignable, mais persiste quand meme)", webhookStatusFailed, seenStatuses[webhookStatusFailed])
	}
}

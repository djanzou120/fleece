package coreprocessor

// webhook_retry_scheduler_test.go — teste le ticker de retry webhook (D-M43) :
// backoffFor (chaque palier + au-dela du max), l'atomicite de la capture
// (UNE SEULE requete UPDATE...RETURNING, jamais un SELECT separe), le bail de
// reservation (B1) et l'arithmetique attempts (incrementee UNE SEULE FOIS, a
// la capture — voir doc de tete de webhook_retry_scheduler.go), et le
// dispatch/finalisation d'une livraison retentee.
import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================
// backoffFor — fonction pure, chaque palier + au-dela du max
// ============================================================

func TestBackoffFor(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{1, 1 * time.Minute},
		{2, 5 * time.Minute},
		{3, 30 * time.Minute},
		{4, 2 * time.Hour},
		{5, 8 * time.Hour},
		{6, 8 * time.Hour}, // au-dela du max : plafond 8h
		{100, 8 * time.Hour},
	}
	for _, tc := range cases {
		if got := backoffFor(tc.attempts); got != tc.want {
			t.Errorf("backoffFor(%d) = %v, voulu %v", tc.attempts, got, tc.want)
		}
	}
}

// ============================================================
// runWebhookRetryTick — atomicite (critere de QA M-022 applique ici)
// ============================================================

// TestRunWebhookRetryTick_singleQuery_notSelectThenUpdate verifie que la
// capture des livraisons eligibles est UNE SEULE requete UPDATE...RETURNING,
// jamais un SELECT prealable — core-processor est deploye a 3 replicas
// (deploy/k8s/services/core-processor.yaml) : un SELECT puis UPDATE separes
// laisserait une fenetre ou deux replicas selectionneraient la meme
// livraison et la retenteraient deux fois.
func TestRunWebhookRetryTick_singleQuery_notSelectThenUpdate(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"id", "event", "payload", "attempts", "endpoint_id", "url", "secret"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runWebhookRetryTick(context.Background())

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu EXACTEMENT 1 (aucune livraison due, pas de SELECT prealable) : %+v", len(conn.queries), conn.queries)
	}
	q := strings.ToUpper(strings.TrimSpace(conn.queries[0]))
	if strings.HasPrefix(q, "SELECT") {
		t.Fatalf("la premiere requete est un SELECT (%q) — viole l'anti-double-traitement (doit etre UNE SEULE requete UPDATE...RETURNING)", conn.queries[0])
	}
	if !strings.HasPrefix(q, "UPDATE") {
		t.Fatalf("la premiere requete devrait etre un UPDATE...RETURNING, obtenu : %q", conn.queries[0])
	}
	if !strings.Contains(conn.queries[0], "RETURNING") {
		t.Fatalf("requete sans RETURNING (capture non-atomique) : %q", conn.queries[0])
	}
	// B1 : le marquage pose desormais un BAIL futur, PLUS JAMAIS NULL (une
	// capture a NULL etait irreversible en cas de mort du worker avant
	// finalisation — voir doc de tete de fichier).
	if strings.Contains(conn.queries[0], "next_retry_at = NULL") {
		t.Fatalf("requete pose encore next_retry_at a NULL (B1 : devrait poser un bail futur, jamais NULL, pour rester reversible) : %q", conn.queries[0])
	}
	// N4 : le bail est calcule COTE SERVEUR, dans la MEME expression SQL que
	// le now() du filtre de selection — jamais un timestamp calcule cote Go
	// (une derive d'horloge pod/DB rendrait le bail inoperant).
	if !strings.Contains(conn.queries[0], "next_retry_at = now() + make_interval(secs => $2)") {
		t.Fatalf("requete sans marquage next_retry_at = now() + make_interval(secs => $2) (bail de reservation calcule cote serveur, B1/N4) : %q", conn.queries[0])
	}
	// B1 : attempts est incremente A LA CAPTURE (voir doc de tete de fichier —
	// retryWebhookDelivery/finalizeWebhookRetry ne le reincrementent plus).
	if !strings.Contains(conn.queries[0], "SET attempts = wd.attempts + 1") {
		t.Fatalf("requete sans increment atomique de attempts a la capture (B1) : %q", conn.queries[0])
	}
	if !strings.Contains(conn.queries[0], "status IN ('pending', 'failed')") {
		t.Fatalf("requete sans filtre de statut attendu : %q", conn.queries[0])
	}
	// E6 : un endpoint desactive (active=false) ne doit plus jamais etre
	// retente.
	if !strings.Contains(conn.queries[0], "we.active = true") {
		t.Fatalf("requete sans filtre we.active = true (E6 : un endpoint desactive ne doit plus etre retente) : %q", conn.queries[0])
	}
	if !strings.Contains(conn.queries[0], "attempts < $1") {
		t.Fatalf("requete sans filtre attempts < maxAttempts : %q", conn.queries[0])
	}
	// E2 : la capture est BORNEE (LIMIT) et non bloquante entre replicas
	// (FOR UPDATE SKIP LOCKED) — sans cette borne, un lot volumineux depasse
	// la duree du bail et se fait recapturer par un autre replica (double
	// livraison chez le client final). La sous-requete reste PARTIE de
	// l'UNIQUE UPDATE...RETURNING verifie ci-dessus.
	if !strings.Contains(conn.queries[0], "FOR UPDATE SKIP LOCKED") {
		t.Fatalf("requete sans FOR UPDATE SKIP LOCKED (E2 : contention inter-replicas) : %q", conn.queries[0])
	}
	if !strings.Contains(conn.queries[0], "LIMIT $3") {
		t.Fatalf("requete sans LIMIT $3 (E2 : capture non bornee -> lot plus long que le bail) : %q", conn.queries[0])
	}
}

// TestRunWebhookRetryTick_bail_computedServerSide verifie que le bail posé par
// la capture (B1) est bien un decalage FUTUR de ~webhookRetryClaimBail, jamais
// NULL et jamais dans le passé — c'est ce qui rend une ligne orpheline (worker
// mort avant finalizeWebhookRetry) éligible de nouveau après expiration du
// bail, sans jamais retomber dans le piège "NULL éternel" (D-M31).
//
// N4 : ce décalage est transmis en SECONDES et appliqué à now() COTE POSTGRES
// (`now() + make_interval(secs => $2)`), jamais comme un timestamp calculé
// côté Go — le filtre de sélection compare next_retry_at au now() de Postgres,
// une dérive d'horloge pod/DB rendrait un bail calculé côté Go inopérant.
// C'est donc le NOMBRE DE SECONDES qui est asserté ici, pas un time.Time : un
// timestamp Go dans cet argument serait précisément la régression à empêcher.
func TestRunWebhookRetryTick_bail_computedServerSide(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"id", "event", "payload", "attempts", "endpoint_id", "url", "secret"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runWebhookRetryTick(context.Background())

	args := conn.args[0]
	if _, isTime := args[1].Value.(time.Time); isTime {
		t.Fatalf("le bail est passe comme timestamp calcule cote Go (%v) — N4 : il doit etre un nombre de secondes applique a now() cote Postgres", args[1].Value)
	}
	bailSecs, ok := args[1].Value.(float64)
	if !ok {
		t.Fatalf("2e argument de la capture n'est pas un nombre de secondes : %v (%T)", args[1].Value, args[1].Value)
	}
	if bailSecs != webhookRetryClaimBail.Seconds() {
		t.Errorf("bail = %v s, voulu %v s (webhookRetryClaimBail)", bailSecs, webhookRetryClaimBail.Seconds())
	}
	// E2 : la borne de capture est transmise en 3e argument (LIMIT $3).
	if got := args[2].Value; got != int64(webhookRetryCaptureLimit) {
		t.Errorf("borne de capture = %v, voulu %d (webhookRetryCaptureLimit, E2)", got, webhookRetryCaptureLimit)
	}
}

func TestRunWebhookRetryTick_dbError_doesNotCrash(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runWebhookRetryTick(context.Background()) // ne doit pas paniquer
}

// TestRunWebhookRetryTick_claimedDelivery_dispatchesAndFinalizes verifie
// qu'une livraison capturee est bien retentee (POST signe) puis finalisee
// (UPDATE status/attempts/next_retry_at).
//
// B1 : la valeur "attempts" mockee ici represente la valeur POST-CAPTURE
// (RETURNING renvoie la valeur APRES le "SET attempts = wd.attempts + 1" —
// voir doc de tete de webhook_retry_scheduler.go) — 2 signifie "1 tentative
// precedente deja faite, cette capture vient d'incrementer a 2 pour la
// tentative en vol". retryWebhookDelivery NE DOIT PLUS ajouter +1 : la
// finalisation doit ecrire EXACTEMENT cette meme valeur (2), jamais 3.
func TestRunWebhookRetryTick_claimedDelivery_dispatchesAndFinalizes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols(
			[]string{"id", "event", "payload", "attempts", "endpoint_id", "url", "secret"},
			[]driver.Value{int64(42), "message.delivered", []byte(`{"event":"message.delivered"}`), int64(2), "ep-1", srv.URL, "s3cr3t"},
		)},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runWebhookRetryTick(context.Background())

	// [0] capture (Query/RETURNING), [1] finalisation (Exec UPDATE).
	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2 (capture + finalisation) : %+v", len(conn.queries), conn.queries)
	}
	if !strings.Contains(conn.queries[1], "UPDATE webhook.webhook_deliveries") ||
		!strings.Contains(conn.queries[1], "SET status = $1, attempts = $2, next_retry_at = $3") {
		t.Fatalf("requete de finalisation inattendue : %q", conn.queries[1])
	}
	args := conn.args[1]
	if got := args[0].Value; got != webhookStatusDelivered {
		t.Errorf("status finalise = %v, voulu %q (livraison reussie)", got, webhookStatusDelivered)
	}
	if got := args[1].Value; got != int64(2) {
		t.Errorf("attempts finalise = %v, voulu 2 (deja incremente par la capture, PAS de +1 supplementaire ici — B1)", got)
	}
	if args[3].Value != int64(42) {
		t.Errorf("id de la delivery finalisee = %v, voulu 42", args[3].Value)
	}
}

// TestRunWebhookRetryTick_stillFailing_reschedulesWithBackoff verifie le
// palier de backoff applique lors d'un echec en cours de sequence de retry.
//
// B1 : attempts=3 mocke = valeur POST-CAPTURE (2 tentatives precedentes, la
// capture vient d'incrementer a 3 pour cette tentative en vol) ->
// backoffFor(3) = 30 min, ecrite TELLE QUELLE (pas de +1 supplementaire).
func TestRunWebhookRetryTick_stillFailing_reschedulesWithBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols(
			[]string{"id", "event", "payload", "attempts", "endpoint_id", "url", "secret"},
			[]driver.Value{int64(7), "message.failed", []byte(`{"event":"message.failed"}`), int64(3), "ep-1", srv.URL, "s3cr3t"},
		)},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	before := time.Now()
	svc.runWebhookRetryTick(context.Background())

	args := conn.args[1]
	if got := args[0].Value; got != webhookStatusFailed {
		t.Fatalf("status finalise = %v, voulu %q (toujours en echec, sous le plafond de tentatives)", got, webhookStatusFailed)
	}
	if got := args[1].Value; got != int64(3) {
		t.Fatalf("attempts finalise = %v, voulu 3 (deja incremente par la capture — B1, pas de +1 supplementaire)", got)
	}
	nrt, ok := args[2].Value.(time.Time)
	if !ok {
		t.Fatalf("next_retry_at absent ou de mauvais type : %v (%T)", args[2].Value, args[2].Value)
	}
	if diff := nrt.Sub(before); diff < 29*time.Minute || diff > 31*time.Minute {
		t.Errorf("next_retry_at = %v (delta %v), voulu ~30 minutes (backoffFor(3))", nrt, diff)
	}
}

// TestRunWebhookRetryTick_lastAttemptFails_exhausted verifie le passage en
// 'exhausted' quand la tentative capturee est la derniere autorisee.
//
// B1 : attempts=5 mocke = valeur POST-CAPTURE (4 tentatives precedentes, la
// capture vient d'incrementer a 5) -> 5 >= maxAttempts(5) -> exhausted,
// ecrite TELLE QUELLE.
func TestRunWebhookRetryTick_lastAttemptFails_exhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols(
			[]string{"id", "event", "payload", "attempts", "endpoint_id", "url", "secret"},
			[]driver.Value{int64(9), "message.failed", []byte(`{"event":"message.failed"}`), int64(5), "ep-1", srv.URL, "s3cr3t"},
		)},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runWebhookRetryTick(context.Background())

	args := conn.args[1]
	if got := args[0].Value; got != webhookStatusExhausted {
		t.Fatalf("status finalise = %v, voulu %q (5e tentative epuisee, maxAttempts=5)", got, webhookStatusExhausted)
	}
	if got := args[1].Value; got != int64(5) {
		t.Fatalf("attempts finalise = %v, voulu 5 (deja incremente par la capture — B1, pas de +1 supplementaire)", got)
	}
	if args[2].Value != nil {
		t.Errorf("next_retry_at = %v, voulu NULL (exhausted, plus jamais retentee)", args[2].Value)
	}
}

// TestRunWebhookRetryTick_emptyPayload_exhaustedWithoutDispatch verifie la
// garde de surete : un payload vide (anomalie) ne doit JAMAIS etre redispatche
// — passage direct en exhausted, aucun appel HTTP.
func TestRunWebhookRetryTick_emptyPayload_exhaustedWithoutDispatch(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols(
			[]string{"id", "event", "payload", "attempts", "endpoint_id", "url", "secret"},
			[]driver.Value{int64(11), "message.failed", []byte(nil), int64(1), "ep-1", srv.URL, "s3cr3t"},
		)},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.runWebhookRetryTick(context.Background())

	if called {
		t.Fatal("un payload vide a ete redispatche (le serveur HTTP a ete appele), voulu : aucun appel")
	}
	args := conn.args[1]
	if got := args[0].Value; got != webhookStatusExhausted {
		t.Fatalf("status finalise = %v, voulu %q (payload vide -> exhausted immediat)", got, webhookStatusExhausted)
	}
	if got := args[1].Value; got != int64(1) {
		t.Fatalf("attempts finalise = %v, voulu 1 (valeur POST-CAPTURE mockee ici — la capture a deja incremente attempts avant meme que ce payload vide soit detecte ; retryWebhookDelivery ne la reincremente pas)", got)
	}
}

// ============================================================
// RunWebhookRetryScheduler — arret propre sur annulation de contexte
// ============================================================

func TestRunWebhookRetryScheduler_stopsOnContextCancel(t *testing.T) {
	conn := &fakeConn{} // aucun step programme : comportement par defaut (0 ligne), tolere.
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunWebhookRetryScheduler(ctx, time.Millisecond)
		close(done)
	}()

	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunWebhookRetryScheduler() n'a pas retourne apres annulation du contexte")
	}
}

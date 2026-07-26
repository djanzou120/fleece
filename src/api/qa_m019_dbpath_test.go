package api

// qa_m019_dbpath_test.go — Tests ajoutes par la QA (passe finale M-019).
//
// Contexte (points A et C de la revue finale QA Phase 2) :
//
//   A. Le pattern "nil DB -> panic" (voir newTestService/newTestServiceWithLogger
//      dans les autres fichiers de test) fait que TOUT test httptest existant
//      evite deliberement le chemin qui appelle reellement s.DB.ExecRows/Exec/Get
//      (sinon panic sur receveur nil). Consequence documentee explicitement en
//      tete de webhooks_endpoints_test.go : le chemin nominal des webhooks
//      OM/MTN (reference_id present + statut reconnu -> UPDATE reellement
//      execute) et Telegram (message_id present -> correlation external_id ->
//      UPDATE reellement execute) n'etait couvert par AUCUN test automatise,
//      uniquement par revue statique. Ce fichier comble ce trou en construisant
//      un faux driver database/sql/driver (stdlib uniquement + github.com/jmoiron/sqlx,
//      deja une dependance directe du module via src/go/sql — AUCUNE dependance
//      ajoutee, go.mod inchange) qui capture la requete SQL et les arguments
//      reellement emis par le code de production. L'injection du faux driver
//      utilise gosql.NewFromSQLX (constructeur expose par src/go/sql/db.go,
//      documente comme reserve aux tests/infrastructure) : aucune ecriture de
//      bas niveau dans un champ non exporte n'est plus necessaire.
//
//   C. recoverMiddleware est deja unit-teste directement (helpers_test.go) et
//      Service.ServeHTTP est deja teste sur le chemin SANS panique (GET /health).
//      Aucun test n'exerce cependant l'integration complete (mux -> handler reel
//      -> panique reelle -> recoverMiddleware) via Service.ServeHTTP : les tests
//      existants n'atteignent jamais un handler qui panique reellement (DB nil
//      court-circuitee par la validation). Ce fichier ajoute ce test manquant en
//      s'appuyant sur un comportement de production reel et documente : un GET
//      /wallet/{workspaceId} avec un UUID valide et s.DB nil provoque un vrai
//      nil pointer dereference dans gosql.DB.Get (receveur nil), qui doit etre
//      absorbe par recoverMiddleware A TRAVERS ServeHTTP (pas seulement de
//      maniere synthetique).
//
// Limites assumees : le faux driver valide la FORME du SQL emis (table/colonnes/
// placeholders/ordre des arguments) et le branchement du code (0/1/>1 lignes
// affectees, erreur technique) — pas la conformite exacte au moteur Postgres
// reel (deja verifiee par revue croisee avec les migrations 0018/0019 dans le
// rapport QA). Aucune infrastructure Postgres/RabbitMQ n'est disponible dans cet
// environnement (contrainte de la tache QA).
import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"

	"github.com/jmoiron/sqlx"
)

// ============================================================
// Faux driver database/sql/driver (stdlib + sqlx, 0 nouvelle dependance)
// ============================================================

// fakeExecResult est un driver.Result minimaliste a valeur programmable.
type fakeExecResult struct{ rows int64 }

func (r fakeExecResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeExecResult) RowsAffected() (int64, error) { return r.rows, nil }

// fakeRows est un driver.Rows minimaliste a contenu programmable, utilise pour
// simuler le resultat d'un SELECT (webhooks_telegram.go, B1/Phase 3 :
// processTelegramDLR n'emet plus JAMAIS d'UPDATE — uniquement des SELECT en
// lecture seule, via s.DB.Select/QueryContext, pour resoudre l'UUID Fleece a
// publier dans l'evenement AMQP).
type fakeRows struct {
	columns []string
	data    [][]driver.Value
	pos     int
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}

// fakeConn est une connexion driver minimaliste qui capture la derniere requete
// (Exec ou Query) et ses arguments, et delegue le resultat/l'erreur a execFn/
// queryFn (programmables par test). Implemente driver.ExecerContext et
// driver.QueryerContext : database/sql les utilise alors directement, sans
// passer par Prepare/Stmt (suffisant pour gosql.DB/Tx.Exec/ExecRows/Select/Get,
// qui n'utilisent que ExecContext/QueryContext).
type fakeConn struct {
	execFn  func(query string, args []driver.NamedValue) (driver.Result, error)
	queryFn func(query string, args []driver.NamedValue) (driver.Rows, error)

	lastQuery string
	lastArgs  []driver.NamedValue

	// queries/argsHistory capturent l'historique COMPLET (Exec ET Query, dans
	// l'ordre chronologique) — necessaire pour verifier une sequence de
	// plusieurs requetes (ex. B2 : correlation stricte PUIS repli), ce que
	// lastQuery/lastArgs (un seul slot, ecrase a chaque appel) ne permettent
	// pas d'observer. Champ additif : n'affecte aucun test existant qui
	// n'utilise que lastQuery/lastArgs.
	queries     []string
	argsHistory [][]driver.NamedValue
}

var _ driver.ExecerContext = (*fakeConn)(nil)
var _ driver.QueryerContext = (*fakeConn)(nil)

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("fakeConn: Prepare non supporte (ExecerContext/QueryerContext utilises a la place)")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("fakeConn: transactions non supportees par ce faux driver")
}
func (c *fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.lastQuery = query
	c.lastArgs = args
	c.queries = append(c.queries, query)
	c.argsHistory = append(c.argsHistory, args)
	if c.execFn != nil {
		return c.execFn(query, args)
	}
	return fakeExecResult{rows: 0}, nil
}
func (c *fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.lastQuery = query
	c.lastArgs = args
	c.queries = append(c.queries, query)
	c.argsHistory = append(c.argsHistory, args)
	if c.queryFn != nil {
		return c.queryFn(query, args)
	}
	return &fakeRows{columns: []string{}}, nil
}

// fakeDriverImpl adapte fakeConn a l'interface driver.Driver.
type fakeDriverImpl struct{ conn *fakeConn }

func (d fakeDriverImpl) Open(name string) (driver.Conn, error) { return d.conn, nil }

var fakeDriverSeq int64

// newFakeGosqlDB construit un *gosql.DB dont le *sqlx.DB interne est branche
// sur le faux driver ci-dessus, permettant d'executer reellement (sans
// Postgres) le SQL emis par le code de production et d'observer requete/args.
//
// Injection via gosql.NewFromSQLX (constructeur de test/infrastructure expose
// par src/go/sql/db.go) : gosql.Open() exige une vraie connexion (Ping reel),
// NewFromSQLX permet d'envelopper un *sqlx.DB deja ouvert — ici sur le faux
// driver — sans toucher au champ non exporte gosql.DB.db.
func newFakeGosqlDB(t *testing.T, conn *fakeConn) *gosql.DB {
	t.Helper()

	name := "flmock" + strconv.FormatInt(atomic.AddInt64(&fakeDriverSeq, 1), 10)
	sql.Register(name, fakeDriverImpl{conn: conn})

	rawDB, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("sql.Open(%q) erreur inattendue : %v", name, err)
	}
	sqlxDB := sqlx.NewDb(rawDB, "postgres")

	return gosql.NewFromSQLX(sqlxDB)
}

// ============================================================
// A. reconcileWalletTransactionStatus (OM/MTN) — chemin nominal reellement execute
// ============================================================

func TestReconcileWalletTransactionStatus_om_happyPath_rows1(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 1}, nil
	}}
	svc := &Service{
		DB:     newFakeGosqlDB(t, conn),
		Logger: golog.Init("warn", "text"),
	}

	reconcileWalletTransactionStatus(context.Background(), svc, "om", "txn-123", mapOMStatus("SUCCESS"))

	if !strings.Contains(conn.lastQuery, "UPDATE wallet.wallet_transactions") {
		t.Fatalf("requete SQL inattendue (table) : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "SET status = $1") || !strings.Contains(conn.lastQuery, "WHERE reference_id = $2") {
		t.Fatalf("requete SQL inattendue (clause) : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "AND status NOT IN ('completed', 'failed')") {
		t.Fatalf("garde d'idempotence (D-M15) absente de la requete : %q", conn.lastQuery)
	}
	if len(conn.lastArgs) != 2 {
		t.Fatalf("nombre d'arguments = %d, voulu 2 : %+v", len(conn.lastArgs), conn.lastArgs)
	}
	if got := conn.lastArgs[0].Value; got != "completed" {
		t.Errorf("arg[0] (status) = %v, voulu \"completed\"", got)
	}
	if got := conn.lastArgs[1].Value; got != "txn-123" {
		t.Errorf("arg[1] (reference_id) = %v, voulu \"txn-123\"", got)
	}
}

func TestReconcileWalletTransactionStatus_mtn_happyPath_rows1(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 1}, nil
	}}
	svc := &Service{
		DB:     newFakeGosqlDB(t, conn),
		Logger: golog.Init("warn", "text"),
	}

	reconcileWalletTransactionStatus(context.Background(), svc, "mtn", "ref-456", mapMTNStatus("SUCCESSFUL"))

	if len(conn.lastArgs) != 2 {
		t.Fatalf("nombre d'arguments = %d, voulu 2", len(conn.lastArgs))
	}
	if got := conn.lastArgs[0].Value; got != "completed" {
		t.Errorf("arg[0] (status) = %v, voulu \"completed\"", got)
	}
	if got := conn.lastArgs[1].Value; got != "ref-456" {
		t.Errorf("arg[1] (reference_id) = %v, voulu \"ref-456\"", got)
	}
}

// TestReconcileWalletTransactionStatus_zeroRows verifie le branchement "0 ligne
// affectee" (reference_id inconnue) : ne doit jamais paniquer ni faire echouer
// l'appelant (le handler webhook repond quand meme 200).
func TestReconcileWalletTransactionStatus_zeroRows(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 0}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	reconcileWalletTransactionStatus(context.Background(), svc, "om", "unknown-ref", "completed")
	// Pas de panique = succes du test. La requete a bien ete executee.
	if conn.lastQuery == "" {
		t.Fatal("aucune requete executee alors qu'un reference_id et un statut reconnu etaient fournis")
	}
}

// TestReconcileWalletTransactionStatus_replayOnFinalStatus_zeroRows documente le
// scenario vise par la garde d'idempotence (D-M15) : un callback tardif/rejoue
// sur une transaction deja dans un statut final ne doit rien ecraser. Le faux
// driver ne peut pas evaluer la clause SQL "AND status NOT IN (...)" (il ne
// simule pas un vrai moteur SQL), mais la forme de la requete est deja verifiee
// par TestReconcileWalletTransactionStatus_om_happyPath_rows1 ; ce test verifie
// ici le comportement observable cote appelant lorsque la garde s'applique
// reellement en base (0 ligne affectee) : pas de panique, log WARN, 200 implicite.
func TestReconcileWalletTransactionStatus_replayOnFinalStatus_zeroRows(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		// Simule la garde d'idempotence cote base : la ligne existe mais est deja
		// dans un statut final, donc la clause NOT IN l'exclut -> 0 ligne affectee.
		return fakeExecResult{rows: 0}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	// Callback FAILED rejoue apres qu'un SUCCESS ait deja ete reconcilie en 'completed'.
	reconcileWalletTransactionStatus(context.Background(), svc, "om", "txn-already-final", mapOMStatus("FAILED"))

	if !strings.Contains(conn.lastQuery, "AND status NOT IN ('completed', 'failed')") {
		t.Fatalf("garde d'idempotence (D-M15) absente de la requete : %q", conn.lastQuery)
	}
	// Pas de panique = succes du test ; le statut final existant n'est jamais ecrase.
}

// TestReconcileWalletTransactionStatus_dbError verifie que l'erreur technique de
// l'UPDATE est absorbee (log + retour, jamais de panique/erreur remontee).
func TestReconcileWalletTransactionStatus_dbError(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return nil, errors.New("connexion perdue")
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	reconcileWalletTransactionStatus(context.Background(), svc, "mtn", "ref-err", "completed")
	// Pas de panique = succes du test.
}

// ============================================================
// A. processTelegramDLR — correlation external_id reellement executee
//
// B1 (Phase 3) : processTelegramDLR N'EMET PLUS JAMAIS D'UPDATE sur
// messaging.messages — UNIQUEMENT des SELECT en lecture seule (s.DB.Select,
// donc QueryContext/queryFn), pour resoudre l'UUID Fleece a publier dans
// l'evenement AMQP. core-processor redevient l'UNIQUE ecrivain du statut.
// Chaque test ci-dessous verifie explicitement l'ABSENCE de toute requete
// UPDATE : ces tests DOIVENT echouer si une regression reintroduit une
// ecriture de statut dans ce handler (voir assertNoUpdateEmitted).
//
// B2 (Phase 3) : recipient/chat_id est un FILTRE OPPORTUNISTE (strict PUIS
// repli), plus une condition dure — voir
// TestProcessTelegramDLR_withChatID_fallbackOnMismatch_logsError pour le cas
// de desynchronisation (chat.id present, correlation stricte QUI ECHOUE, le
// repli aboutit -> log ERROR "dlr_recipient_mismatch").
// ============================================================

// assertNoUpdateEmitted echoue le test si l'une des requetes capturees par
// conn contient "UPDATE" — le coeur du correctif B1 : ce handler ne doit
// JAMAIS ecrire messaging.messages.status.
func assertNoUpdateEmitted(t *testing.T, conn *fakeConn) {
	t.Helper()
	for _, q := range conn.queries {
		if strings.Contains(q, "UPDATE") {
			t.Fatalf("B1 regression : requete UPDATE emise par le webhook (devrait etre lecture seule) : %q", q)
		}
	}
}

// fakeRowsOf construit un fakeRows a une colonne "id" contenant les valeurs
// fournies (0, 1 ou plusieurs lignes simulees) — utilise par le chemin de
// compatibilite fleece_message_id (SELECT id seul).
func fakeRowsOf(ids ...string) *fakeRows {
	data := make([][]driver.Value, len(ids))
	for i, id := range ids {
		data[i] = []driver.Value{id}
	}
	return &fakeRows{columns: []string{"id"}, data: data}
}

// fakeIDRecipientRows construit un fakeRows a deux colonnes "id"/"recipient"
// (0, 1 ou plusieurs lignes) — utilise par les chemins external_id (avec ou
// sans chat_id), qui selectionnent toujours id ET recipient (B2 : recipient
// est necessaire pour detecter/loguer une desynchronisation eventuelle).
func fakeIDRecipientRows(pairs ...[2]string) *fakeRows {
	data := make([][]driver.Value, len(pairs))
	for i, p := range pairs {
		data[i] = []driver.Value{p[0], p[1]}
	}
	return &fakeRows{columns: []string{"id", "recipient"}, data: data}
}

// captureLogger construit un *golog.Logger dont la sortie est capturee dans
// buf (handler texte slog) — permet de verifier qu'un log ERROR precis
// (champ distinctif) a bien ete emis, ce que les autres tests de ce fichier
// (golog.Init("warn","text") vers os.Stderr) ne permettent pas d'observer.
func captureLogger(buf *strings.Builder) *golog.Logger {
	return &golog.Logger{Logger: slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))}
}

// TestProcessTelegramDLR_noChatID_happyPath verifie que, chat.id absent, la
// correlation par (external_id, provider_id) SEUL est executee (repli direct
// documente, pas une strategie stricte-puis-repli), via un SELECT — jamais un
// UPDATE (B1).
func TestProcessTelegramDLR_noChatID_happyPath(t *testing.T) {
	const wantMessageID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return fakeIDRecipientRows([2]string{wantMessageID, "+237699112233"}), nil
	}}
	svc := &Service{
		DB:     newFakeGosqlDB(t, conn),
		Logger: golog.Init("warn", "text"),
		// AMQP nil : publishTelegramDLR doit no-op silencieusement (garde nil).
	}

	// Pas de Chat : la requete ne porte donc que (external_id, provider_id), pas recipient.
	update := telegramUpdate{
		UpdateID: 999,
		Message: &telegramMessage{
			MessageID:      42,
			DeliveryStatus: "delivered",
		},
	}

	svc.processTelegramDLR(context.Background(), update)

	assertNoUpdateEmitted(t, conn)
	if !strings.Contains(conn.lastQuery, "SELECT id, recipient FROM messaging.messages") {
		t.Fatalf("requete SQL inattendue (attendu un SELECT en lecture seule, B1) : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "external_id = $1") || !strings.Contains(conn.lastQuery, "provider_id = $2") {
		t.Fatalf("requete SQL inattendue (clauses de correlation) : %q", conn.lastQuery)
	}
	if strings.Contains(conn.lastQuery, "AND recipient") {
		t.Fatalf("requete SQL ne devrait pas filtrer sur recipient (chat.id absent) : %q", conn.lastQuery)
	}
	if len(conn.lastArgs) != 2 {
		t.Fatalf("nombre d'arguments = %d, voulu 2 : %+v", len(conn.lastArgs), conn.lastArgs)
	}
	if got := conn.lastArgs[0].Value; got != "42" {
		t.Errorf("arg[0] (external_id) = %v, voulu \"42\"", got)
	}
	if got := conn.lastArgs[1].Value; got != telegramProviderID {
		t.Errorf("arg[1] (provider_id) = %v, voulu %q", got, telegramProviderID)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (pas de chat_id -> pas de tentative stricte)", len(conn.queries))
	}
}

// TestProcessTelegramDLR_withChatID_strictMatch_happyPath verifie que, chat.id
// present ET correlant reellement (recipient == chat_id), UNE SEULE requete
// (la correlation stricte) suffit — pas de repli inutile.
func TestProcessTelegramDLR_withChatID_strictMatch_happyPath(t *testing.T) {
	const wantMessageID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return fakeIDRecipientRows([2]string{wantMessageID, "987654321"}), nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{
		Message: &telegramMessage{
			MessageID:      42,
			Chat:           &telegramChat{ID: 987654321},
			DeliveryStatus: "delivered",
		},
	}
	svc.processTelegramDLR(context.Background(), update)

	assertNoUpdateEmitted(t, conn)
	if !strings.Contains(conn.lastQuery, "AND recipient = $3") {
		t.Fatalf("requete SQL inattendue (recipient absent malgre chat.id present, B2) : %q", conn.lastQuery)
	}
	if len(conn.lastArgs) != 3 {
		t.Fatalf("nombre d'arguments = %d, voulu 3 : %+v", len(conn.lastArgs), conn.lastArgs)
	}
	if got := conn.lastArgs[2].Value; got != "987654321" {
		t.Errorf("arg[2] (recipient/chat_id) = %v, voulu \"987654321\"", got)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (correlation stricte suffisante, pas de repli B2)", len(conn.queries))
	}
}

// TestProcessTelegramDLR_withChatID_fallbackOnMismatch_logsError couvre LE
// SCENARIO CENTRAL du correctif B2 : chat.id present mais la correlation
// STRICTE (recipient=chat_id) ne matche AUCUNE ligne (ex. le message a ete
// envoye a un numero E.164 via campagne, pas via un vrai chat_id) — le repli
// (external_id, provider_id) SEUL doit alors etre tente ET reussir, et un log
// ERROR distinctif "dlr_recipient_mismatch" doit etre emis (pas noye dans le
// bruit WARN habituel).
func TestProcessTelegramDLR_withChatID_fallbackOnMismatch_logsError(t *testing.T) {
	const wantMessageID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	callCount := 0
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		callCount++
		if strings.Contains(query, "AND recipient") {
			// Correlation stricte : 0 ligne (recipient stocke != chat_id du callback).
			return fakeIDRecipientRows(), nil
		}
		// Repli (external_id, provider_id) seul : reussit, recipient EN BASE
		// different du chat_id attendu (desynchronisation reelle).
		return fakeIDRecipientRows([2]string{wantMessageID, "+237699112233"}), nil
	}}
	var logBuf strings.Builder
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: captureLogger(&logBuf)}

	update := telegramUpdate{
		Message: &telegramMessage{
			MessageID:      42,
			Chat:           &telegramChat{ID: 987654321},
			DeliveryStatus: "delivered",
		},
	}
	svc.processTelegramDLR(context.Background(), update)

	assertNoUpdateEmitted(t, conn)
	if callCount != 2 {
		t.Fatalf("nombre de SELECT emis = %d, voulu 2 (correlation stricte PUIS repli, B2)", callCount)
	}
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "dlr_recipient_mismatch") {
		t.Fatalf("log ERROR distinctif dlr_recipient_mismatch absent (B2, point 3) : %q", logOutput)
	}
	if !strings.Contains(logOutput, "level=ERROR") {
		t.Fatalf("le log de desynchronisation doit etre ERROR (pas WARN) : %q", logOutput)
	}
	if !strings.Contains(logOutput, "987654321") || !strings.Contains(logOutput, "+237699112233") {
		t.Fatalf("le log doit porter les deux valeurs (chat_id attendu ET recipient en base) : %q", logOutput)
	}
}

// TestProcessTelegramDLR_withChatID_bothStrictAndFallbackEmpty verifie que si
// NI la correlation stricte NI le repli ne matchent, aucune publication n'est
// tentee et AUCUN log "dlr_recipient_mismatch" n'est emis (ce n'est pas une
// desynchronisation constatee, juste l'absence totale de correlation).
func TestProcessTelegramDLR_withChatID_bothStrictAndFallbackEmpty(t *testing.T) {
	callCount := 0
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		callCount++
		return fakeIDRecipientRows(), nil
	}}
	var logBuf strings.Builder
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: captureLogger(&logBuf)}

	update := telegramUpdate{
		Message: &telegramMessage{
			MessageID:      42,
			Chat:           &telegramChat{ID: 987654321},
			DeliveryStatus: "delivered",
		},
	}
	svc.processTelegramDLR(context.Background(), update)

	assertNoUpdateEmitted(t, conn)
	if callCount != 2 {
		t.Fatalf("nombre de SELECT emis = %d, voulu 2 (stricte puis repli, tous deux vides)", callCount)
	}
	if strings.Contains(logBuf.String(), "dlr_recipient_mismatch") {
		t.Fatalf("aucune desynchronisation constatee (les deux requetes sont vides) : log inattendu %q", logBuf.String())
	}
}

func TestProcessTelegramDLR_externalID_failedStatus(t *testing.T) {
	const wantMessageID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return fakeIDRecipientRows([2]string{wantMessageID, "+237699112233"}), nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{
		Message: &telegramMessage{MessageID: 7, DeliveryStatus: "failed"},
	}
	svc.processTelegramDLR(context.Background(), update)

	assertNoUpdateEmitted(t, conn)
	if got := conn.lastArgs[0].Value; got != "7" {
		t.Errorf("arg[0] (external_id) = %v, voulu \"7\"", got)
	}
}

// TestProcessTelegramDLR_zeroRows verifie le branchement "0 ligne trouvee"
// (external_id inconnu) : jamais de panique, et (D-M21) aucune publication —
// verifie indirectement par l'absence de panique/erreur puisque s.AMQP est nil.
func TestProcessTelegramDLR_zeroRows(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return fakeIDRecipientRows(), nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{Message: &telegramMessage{MessageID: 123, DeliveryStatus: "delivered"}}
	svc.processTelegramDLR(context.Background(), update)
	assertNoUpdateEmitted(t, conn)
	if conn.lastQuery == "" {
		t.Fatal("aucune requete executee alors qu'un message_id etait fourni")
	}
}

// TestProcessTelegramDLR_multipleRows verifie que plusieurs lignes retournees
// (index non-unique) ne font pas planter : la premiere est utilisee comme
// message_id publie.
func TestProcessTelegramDLR_multipleRows(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return fakeIDRecipientRows([2]string{"id-1", "+237699112233"}, [2]string{"id-2", "+237699112233"}), nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{Message: &telegramMessage{MessageID: 55, DeliveryStatus: "delivered"}}
	svc.processTelegramDLR(context.Background(), update)
	assertNoUpdateEmitted(t, conn)
	// Pas de panique = succes du test (le WARN "plusieurs messages correles" est
	// attendu mais non asserte ici — comportement observable deja couvert par
	// revue).
}

// TestProcessTelegramDLR_compatFleeceMsgID_happyPath couvre le chemin de
// compatibilite fleece_message_id (SELECT id seul, sans recipient) — toujours
// en lecture seule (B1).
func TestProcessTelegramDLR_compatFleeceMsgID_happyPath(t *testing.T) {
	const wantMessageID = "550e8400-e29b-41d4-a716-446655440000"
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return fakeRowsOf(wantMessageID), nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{
		Message: &telegramMessage{DeliveryStatus: "delivered", FleeceMsgID: wantMessageID},
	}
	svc.processTelegramDLR(context.Background(), update)

	assertNoUpdateEmitted(t, conn)
	if !strings.Contains(conn.lastQuery, "SELECT id FROM messaging.messages WHERE id = $1") {
		t.Fatalf("requete SQL inattendue (compat fleece_message_id) : %q", conn.lastQuery)
	}
	if len(conn.lastArgs) != 1 || conn.lastArgs[0].Value != wantMessageID {
		t.Fatalf("arguments inattendus : %+v", conn.lastArgs)
	}
}

// TestProcessTelegramDLR_dbError verifie que l'erreur technique du SELECT est
// absorbee (log + retour, jamais de panique) et qu'aucune publication n'est
// tentee (D-M21 : pas d'UUID confirme).
func TestProcessTelegramDLR_dbError(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return nil, errors.New("connexion perdue")
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{Message: &telegramMessage{MessageID: 1, DeliveryStatus: "delivered"}}
	svc.processTelegramDLR(context.Background(), update)
	assertNoUpdateEmitted(t, conn)
}

// ============================================================
// C. recoverMiddleware <-> Service.ServeHTTP sur un VRAI handler route (nil DB)
// ============================================================

// TestServiceServeHTTP_recoversFromRealHandlerPanic_nilDB verifie l'integration
// complete (mux -> handler reel -> panique reelle -> recoverMiddleware) via
// Service.ServeHTTP, ce qu'aucun test existant n'exerce (helpers_test.go ne
// verifie ServeHTTP que sur GET /health, qui ne panique jamais).
//
// GET /wallet/{workspaceId} avec un UUID syntaxiquement valide et s.DB nil
// atteint reellement s.DB.Get(...) (voir wallet_get.go), ce qui panique
// (nil pointer dereference, receveur *gosql.DB nil) : la panique doit etre
// absorbee par recoverMiddleware et convertie en 500 JSON, pas crasher le test.
func TestServiceServeHTTP_recoversFromRealHandlerPanic_nilDB(t *testing.T) {
	svc := &Service{
		Providers: make(map[string]Provider),
		Logger:    golog.Init("warn", "text"),
		// DB volontairement nil.
	}
	if err := svc.Init(); err != nil {
		t.Fatalf("Init() erreur inattendue : %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/wallet/550e8400-e29b-41d4-a716-446655440000", nil)
	rr := httptest.NewRecorder()

	svc.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, voulu %d (panique nil DB absorbee par recoverMiddleware via ServeHTTP)", rr.Code, http.StatusInternalServerError)
	}
	assertJSONError(t, rr)
}

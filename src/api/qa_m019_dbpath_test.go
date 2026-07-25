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

// fakeConn est une connexion driver minimaliste qui capture la derniere requete
// Exec et ses arguments, et delegue le resultat/l'erreur a execFn (programmable
// par test). Implemente driver.ExecerContext : database/sql l'utilise alors
// directement pour ExecContext, sans passer par Prepare/Stmt (suffisant pour
// gosql.DB.Exec/ExecRows, qui n'utilisent que ExecContext).
type fakeConn struct {
	execFn func(query string, args []driver.NamedValue) (driver.Result, error)

	lastQuery string
	lastArgs  []driver.NamedValue
}

var _ driver.ExecerContext = (*fakeConn)(nil)

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("fakeConn: Prepare non supporte (ExecerContext utilise a la place)")
}
func (c *fakeConn) Close() error { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("fakeConn: transactions non supportees par ce faux driver")
}
func (c *fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.lastQuery = query
	c.lastArgs = args
	if c.execFn != nil {
		return c.execFn(query, args)
	}
	return fakeExecResult{rows: 0}, nil
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
// ============================================================

func TestProcessTelegramDLR_externalID_happyPath_rows1(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 1}, nil
	}}
	svc := &Service{
		DB:     newFakeGosqlDB(t, conn),
		Logger: golog.Init("warn", "text"),
		// AMQP nil : publishTelegramDLR doit no-op silencieusement (garde nil).
	}

	update := telegramUpdate{
		UpdateID: 999,
		Message: &telegramMessage{
			MessageID:      42,
			DeliveryStatus: "delivered",
		},
	}

	svc.processTelegramDLR(context.Background(), update)

	if !strings.Contains(conn.lastQuery, "UPDATE messaging.messages") {
		t.Fatalf("requete SQL inattendue (table) : %q", conn.lastQuery)
	}
	if !strings.Contains(conn.lastQuery, "external_id = $2") || !strings.Contains(conn.lastQuery, "provider_id = $3") {
		t.Fatalf("requete SQL inattendue (clauses de correlation) : %q", conn.lastQuery)
	}
	if len(conn.lastArgs) != 3 {
		t.Fatalf("nombre d'arguments = %d, voulu 3 : %+v", len(conn.lastArgs), conn.lastArgs)
	}
	if got := conn.lastArgs[0].Value; got != string(MessageDelivered) {
		t.Errorf("arg[0] (status) = %v, voulu %q", got, string(MessageDelivered))
	}
	if got := conn.lastArgs[1].Value; got != "42" {
		t.Errorf("arg[1] (external_id) = %v, voulu \"42\"", got)
	}
	if got := conn.lastArgs[2].Value; got != telegramProviderID {
		t.Errorf("arg[2] (provider_id) = %v, voulu %q", got, telegramProviderID)
	}
}

func TestProcessTelegramDLR_externalID_failedStatus(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 1}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{
		Message: &telegramMessage{MessageID: 7, DeliveryStatus: "failed"},
	}
	svc.processTelegramDLR(context.Background(), update)

	if got := conn.lastArgs[0].Value; got != string(MessageFailed) {
		t.Errorf("arg[0] (status) = %v, voulu %q", got, string(MessageFailed))
	}
	if got := conn.lastArgs[1].Value; got != "7" {
		t.Errorf("arg[1] (external_id) = %v, voulu \"7\"", got)
	}
}

// TestProcessTelegramDLR_zeroRows verifie le branchement "0 ligne affectee"
// (external_id inconnu) : jamais de panique.
func TestProcessTelegramDLR_zeroRows(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return fakeExecResult{rows: 0}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{Message: &telegramMessage{MessageID: 123, DeliveryStatus: "delivered"}}
	svc.processTelegramDLR(context.Background(), update)
	if conn.lastQuery == "" {
		t.Fatal("aucune requete executee alors qu'un message_id etait fourni")
	}
}

// TestProcessTelegramDLR_dbError verifie que l'erreur technique de l'UPDATE est
// absorbee (log + retour, jamais de panique).
func TestProcessTelegramDLR_dbError(t *testing.T) {
	conn := &fakeConn{execFn: func(query string, args []driver.NamedValue) (driver.Result, error) {
		return nil, errors.New("connexion perdue")
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	update := telegramUpdate{Message: &telegramMessage{MessageID: 1, DeliveryStatus: "delivered"}}
	svc.processTelegramDLR(context.Background(), update)
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

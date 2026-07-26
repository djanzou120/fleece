package intelligenceprocessor

// fakedriver_test.go — faux driver database/sql/driver (stdlib + sqlx, 0
// nouvelle dependance), COPIE ADAPTEE de src/core-processor/fakedriver_test.go
// (M-020) — reutilisation explicitement demandee par le plan M-021 ("reutilise
// le faux driver ... src/core-processor/fakedriver_test.go"). Adaptation :
// noms de mock prefixes "flintelproc..." (au lieu de "flcoreprocmock...")
// pour eviter toute collision de nom de driver enregistre entre packages Go
// distincts au sein du meme binaire de test (sql.Register est un registre
// process-global).
//
// Permet d'executer reellement (sans Postgres ni RabbitMQ, environnement
// offline) le SQL emis par ce worker, y COMPRIS les transactions
// (Begin/Exec/Query/Commit/Rollback) exigees par delivery_outcome.go et
// on_campaign_run.go (markRecipientSent).
//
// Limites assumees (identiques a src/core-processor/fakedriver_test.go) : ce
// faux driver valide la FORME du SQL emis (table/colonnes/placeholders/ordre
// des arguments) et le branchement du code (0/1 ligne RETURNING, erreur
// technique) — pas la conformite exacte au moteur Postgres reel (confrontee
// separement aux migrations 0007/0008/0009/0014/0016/0017/0018 dans le
// rapport de tache). Aucune infrastructure Postgres/RabbitMQ n'est disponible
// dans cet environnement.
import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strconv"
	"sync/atomic"
	"testing"

	gosql "fleece/src/go/sql"

	"github.com/jmoiron/sqlx"
)

// ============================================================
// driver.Result / driver.Rows programmables
// ============================================================

type fakeExecResult struct{ rows int64 }

func (r fakeExecResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeExecResult) RowsAffected() (int64, error) { return r.rows, nil }

// fakeRows est un driver.Rows minimaliste a contenu programmable, utilise
// pour simuler le resultat d'un SELECT / UPDATE ... RETURNING / INSERT ...
// RETURNING (via tx.Get/tx.Select/s.DB.Get/s.DB.Select).
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

// ============================================================
// fakeConn : driver.Conn + Exec/QueryerContext + transactions
// ============================================================

// queryStep decrit une reponse programmee pour UNE requete Query (dans
// l'ordre d'appel).
type queryStep struct {
	rows *fakeRows
	err  error
}

// execStep decrit une reponse programmee pour UNE requete Exec (dans l'ordre
// d'appel).
type execStep struct {
	result driver.Result
	err    error
}

// fakeConn capture chaque requete Exec/Query executee (dans l'ordre) et
// consomme les steps programmes sequentiellement. Si aucun step n'est
// programme pour un appel donne, retombe sur un comportement par defaut
// (0 ligne / rows vides) plutot que de paniquer.
type fakeConn struct {
	querySteps []queryStep
	execSteps  []execStep
	queryPos   int
	execPos    int

	// queries/args capturent l'historique complet (une entree par appel),
	// dans l'ordre chronologique, quel que soit Exec ou Query.
	queries []string
	args    [][]driver.NamedValue

	beginCount    int
	commitCount   int
	rollbackCount int
}

var _ driver.ExecerContext = (*fakeConn)(nil)
var _ driver.QueryerContext = (*fakeConn)(nil)

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("fakeConn: Prepare non supporte (Exec/QueryerContext utilises a la place)")
}
func (c *fakeConn) Close() error { return nil }

// Begin implemente driver.Conn : database/sql l'invoque pour DB.BeginTx quand
// le driver n'implemente pas driver.ConnBeginTx. Les appels Exec/Query
// suivants, jusqu'a Commit/Rollback, sont achemines vers CE MEME fakeConn.
func (c *fakeConn) Begin() (driver.Tx, error) {
	c.beginCount++
	return &fakeTx{conn: c}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.queries = append(c.queries, query)
	c.args = append(c.args, args)

	if c.execPos < len(c.execSteps) {
		step := c.execSteps[c.execPos]
		c.execPos++
		return step.result, step.err
	}
	return fakeExecResult{rows: 0}, nil
}

func (c *fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.queries = append(c.queries, query)
	c.args = append(c.args, args)

	if c.queryPos < len(c.querySteps) {
		step := c.querySteps[c.queryPos]
		c.queryPos++
		if step.err != nil {
			return nil, step.err
		}
		return step.rows, nil
	}
	return &fakeRows{columns: []string{}}, nil
}

// lastQuery/lastArgs — la derniere requete executee (Exec OU Query).
func (c *fakeConn) lastQuery() string {
	if len(c.queries) == 0 {
		return ""
	}
	return c.queries[len(c.queries)-1]
}
func (c *fakeConn) lastArgs() []driver.NamedValue {
	if len(c.args) == 0 {
		return nil
	}
	return c.args[len(c.args)-1]
}

// fakeTx adapte fakeConn a driver.Tx. Commit/Rollback ne font que compter les
// appels.
type fakeTx struct{ conn *fakeConn }

func (t *fakeTx) Commit() error   { t.conn.commitCount++; return nil }
func (t *fakeTx) Rollback() error { t.conn.rollbackCount++; return nil }

// fakeDriverImpl adapte fakeConn a l'interface driver.Driver.
type fakeDriverImpl struct{ conn *fakeConn }

func (d fakeDriverImpl) Open(name string) (driver.Conn, error) { return d.conn, nil }

var fakeDriverSeq int64

// newFakeGosqlDB construit un *gosql.DB dont le *sqlx.DB interne est branche
// sur le faux driver ci-dessus (via gosql.NewFromSQLX, constructeur de
// test/infrastructure expose par src/go/sql/db.go).
func newFakeGosqlDB(t *testing.T, conn *fakeConn) *gosql.DB {
	t.Helper()

	name := "flintelprocmock" + strconv.FormatInt(atomic.AddInt64(&fakeDriverSeq, 1), 10)
	sql.Register(name, fakeDriverImpl{conn: conn})

	rawDB, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("sql.Open(%q) erreur inattendue : %v", name, err)
	}
	sqlxDB := sqlx.NewDb(rawDB, "postgres")

	return gosql.NewFromSQLX(sqlxDB)
}

// fakeRowsOf construit un fakeRows a une colonne unique "col" contenant les
// valeurs fournies (0, 1 ou plusieurs lignes simulees).
func fakeRowsOf(col string, values ...driver.Value) *fakeRows {
	data := make([][]driver.Value, len(values))
	for i, v := range values {
		data[i] = []driver.Value{v}
	}
	return &fakeRows{columns: []string{col}, data: data}
}

// fakeRowsOfCols construit un fakeRows a plusieurs colonnes, une ligne par
// entree de rows (chaque entree doit avoir len(columns) valeurs, dans l'ordre).
func fakeRowsOfCols(columns []string, rows ...[]driver.Value) *fakeRows {
	return &fakeRows{columns: columns, data: rows}
}

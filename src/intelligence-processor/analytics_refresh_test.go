package intelligenceprocessor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// analytics_refresh_test.go — teste isNotPopulatedError (pure) et refreshTick
// (dette D-M03 : REFRESH CONCURRENTLY en steady-state, repli non-concurrent
// tant que la MV n'a jamais ete peuplee ; un echec ne tue jamais le worker).

func TestIsNotPopulatedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"erreur non liee", errors.New("connexion perdue"), false},
		{"message exact Postgres", errors.New(`materialized view "analytics.kpi_daily" has not been populated`), true},
		{"insensible a la casse", errors.New("HAS NOT BEEN POPULATED"), true},
		{"sous-chaine au milieu d'un message plus long", errors.New("pq: error: materialized view has not been populated yet, refresh first"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotPopulatedError(tc.err); got != tc.want {
				t.Errorf("isNotPopulatedError(%v) = %v, voulu %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestRefreshTick_concurrentSuccess couvre le cas nominal steady-state : un
// seul REFRESH CONCURRENTLY, pas de repli.
func TestRefreshTick_concurrentSuccess(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{}}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.refreshTick(context.Background())

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (pas de repli attendu)", len(conn.queries))
	}
	if !strings.Contains(conn.queries[0], "REFRESH MATERIALIZED VIEW CONCURRENTLY analytics.kpi_daily") {
		t.Errorf("requete inattendue : %q", conn.queries[0])
	}
}

// TestRefreshTick_notPopulated_fallbackSucceeds couvre D-M03 : premier
// refresh CONCURRENTLY echoue (MV jamais peuplee), repli non-concurrent qui
// reussit.
//
// CORRECTIF M-026 — CE TEST MOCKAIT LE MAUVAIS MESSAGE. Il utilisait
// `... has not been populated`, qui est la reponse de Postgres a un **SELECT**
// sur une MV vide, pour exercer le chemin d'un **REFRESH**. Un REFRESH ...
// CONCURRENTLY sur une MV vide repond en realite tout autre chose ("is not
// populated"). Le test passait donc au vert alors que le repli ne se
// declenchait JAMAIS en production (MV vide indefiniment, /analytics/kpis en
// 500 permanent sur tout deploiement neuf). Le message ci-dessous est celui
// REELLEMENT OBSERVE contre un PostgreSQL 16 pendant M-026 : ne jamais le
// remplacer par un message invente ou "simplifie" — c'est exactement ce qui
// avait masque le bug. Voir aussi TestIsNotPopulatedError_realPostgresMessages.
func TestRefreshTick_notPopulated_fallbackSucceeds(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{
		{err: errors.New(`pq: CONCURRENTLY cannot be used when the materialized view is not populated`)},
		{result: fakeExecResult{}},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.refreshTick(context.Background())

	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2 (CONCURRENTLY puis repli)", len(conn.queries))
	}
	if !strings.Contains(conn.queries[0], "CONCURRENTLY") {
		t.Errorf("1ere requete devrait tenter CONCURRENTLY : %q", conn.queries[0])
	}
	if strings.Contains(conn.queries[1], "CONCURRENTLY") {
		t.Errorf("2e requete (repli) ne doit PAS contenir CONCURRENTLY (MV jamais peuplee) : %q", conn.queries[1])
	}
	if !strings.Contains(conn.queries[1], "REFRESH MATERIALIZED VIEW analytics.kpi_daily") {
		t.Errorf("requete de repli inattendue : %q", conn.queries[1])
	}
}

// TestRefreshTick_notPopulated_fallbackAlsoFails_doesNotPanic verifie qu'un
// double echec (CONCURRENTLY ET le repli) est journalise ERROR sans jamais
// paniquer -- le worker continue de tourner, le tick suivant retentera.
func TestRefreshTick_notPopulated_fallbackAlsoFails_doesNotPanic(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{
		{err: errors.New("has not been populated")},
		{err: errors.New("connexion perdue")},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.refreshTick(context.Background()) // ne doit pas paniquer

	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2", len(conn.queries))
	}
}

// TestRefreshTick_genericError_doesNotFallback verifie qu'une erreur SANS
// rapport avec "not populated" (ex. panne DB generique) ne declenche PAS de
// tentative de repli -- une seule requete emise, log ERROR, pas de panique.
func TestRefreshTick_genericError_doesNotFallback(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	svc.refreshTick(context.Background()) // ne doit pas paniquer

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (pas de repli pour une erreur generique)", len(conn.queries))
	}
}

// TestRunAnalyticsRefresh_stopsOnContextCancel verifie l'arret propre du
// ticker, avec un intervalle tres court (jamais de sleep bloquant de test
// > ~100ms).
func TestRunAnalyticsRefresh_stopsOnContextCancel(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{}}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		svc.RunAnalyticsRefresh(ctx, time.Millisecond)
		close(done)
	}()

	time.Sleep(5 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunAnalyticsRefresh() n'a pas retourne apres annulation du contexte")
	}
}

// TestIsNotPopulatedError_realPostgresMessages est la garde de non-regression
// du bug trouve en M-026 : la detection portait sur le message du SELECT
// ("has not been populated") alors que ce ticker n'execute que des REFRESH,
// auxquels Postgres repond "is not populated" — les deux chaines ne se
// contiennent PAS l'une l'autre ("has not BEEN populated"), donc le repli
// non-concurrent ne se declenchait jamais et la MV restait vide indefiniment.
//
// Les deux premiers messages sont ceux REELLEMENT OBSERVES contre un
// PostgreSQL 16. Ne pas les reformuler : ce test n'a de valeur que s'il
// compare aux chaines exactes que Postgres emet.
func TestIsNotPopulatedError_realPostgresMessages(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			// LE cas de ce ticker : REFRESH ... CONCURRENTLY sur MV vide.
			name: "refresh concurrently sur MV vide (message reel PG16)",
			err:  errors.New(`pq: CONCURRENTLY cannot be used when the materialized view is not populated`),
			want: true,
		},
		{
			// Conserve par prudence : message d'un SELECT sur MV vide.
			name: "select sur MV vide (message reel PG16)",
			err:  errors.New(`pq: materialized view "kpi_daily" has not been populated`),
			want: true,
		},
		{
			name: "erreur sans rapport ne doit pas declencher le repli",
			err:  errors.New(`pq: permission denied for materialized view kpi_daily`),
			want: false,
		},
		{name: "nil", err: nil, want: false},
	}
	for _, tc := range cases {
		if got := isNotPopulatedError(tc.err); got != tc.want {
			t.Errorf("%s : isNotPopulatedError(%v) = %v, voulu %v", tc.name, tc.err, got, tc.want)
		}
	}
}

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
// refresh CONCURRENTLY echoue ("has not been populated"), repli non-concurrent
// qui reussit.
func TestRefreshTick_notPopulated_fallbackSucceeds(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{
		{err: errors.New(`materialized view "analytics.kpi_daily" has not been populated`)},
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

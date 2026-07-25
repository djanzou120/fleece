package api

// analytics_endpoints_test.go — Tests des handlers HTTP analytics via net/http/httptest.
//
// Perimetre teste (sans base de donnees) :
//
//   GET /analytics/kpis :
//     - workspace_id absent → 400.
//     - workspace_id UUID invalide → 400.
//     - from format invalide → 400.
//     - to format invalide → 400.
//     - from > to → 400.
//
//   GET /analytics/timeseries :
//     - workspace_id absent → 400.
//     - workspace_id UUID invalide → 400.
//     - from format invalide → 400.
//     - to format invalide → 400.
//     - from > to → 400.
//
//   buildKpiQuery (fonction pure) :
//     - sans filtres optionnels : 3 args, ORDER BY day DESC.
//     - avec channel : 4 args, AND channel = $4.
//     - avec country : 4 args, AND country = $4.
//     - avec channel + country : 5 args.
//
//   buildTimeseriesQuery (fonction pure) :
//     - sans filtre channel : 3 args, GROUP BY day, ORDER BY day ASC.
//     - avec channel : 4 args, AND channel = $4.
//
// Perimetre NON teste (necessite une base de donnees reelle) :
//   - GET /analytics/kpis 200 : lecture reelle de analytics.kpi_daily.
//   - Gestion NULL avg_delivery_ms : scan *float64, serialisation JSON null.
//   - GET /analytics/timeseries 200 : lecture reelle de analytics.message_daily avec SUM/GROUP BY.
//   - Filtres from/to effectifs sur les donnees.
//   - Filtres channel et country effectifs sur les donnees.
//   - Tableau vide quand aucun resultat.
// Ces cas sont couverts par les tests d'acceptance (QA) avec une DB de test.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	validAnalyticsUUID   = "550e8400-e29b-41d4-a716-446655440000"
	invalidAnalyticsUUID = "not-a-uuid"
)

// ---- GET /analytics/kpis — workspace_id ----

func TestHandleGetKpis_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/analytics/kpis", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetKpis(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id absent)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetKpis_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/analytics/kpis?workspace_id="+invalidAnalyticsUUID, nil)
	rr := httptest.NewRecorder()

	svc.HandleGetKpis(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- GET /analytics/kpis — from/to ----

func TestHandleGetKpis_invalidFromDate(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet,
		"/analytics/kpis?workspace_id="+validAnalyticsUUID+"&from=01/01/2026", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetKpis(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (from format invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetKpis_invalidToDate(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet,
		"/analytics/kpis?workspace_id="+validAnalyticsUUID+"&from=2026-01-01&to=not-a-date", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetKpis(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (to format invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetKpis_fromAfterTo(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet,
		"/analytics/kpis?workspace_id="+validAnalyticsUUID+"&from=2026-06-01&to=2026-01-01", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetKpis(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (from > to)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- GET /analytics/timeseries — workspace_id ----

func TestHandleGetTimeseries_missingWorkspaceID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet, "/analytics/timeseries", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetTimeseries(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id absent)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetTimeseries_invalidWorkspaceUUID(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet,
		"/analytics/timeseries?workspace_id="+invalidAnalyticsUUID, nil)
	rr := httptest.NewRecorder()

	svc.HandleGetTimeseries(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (workspace_id UUID invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- GET /analytics/timeseries — from/to ----

func TestHandleGetTimeseries_invalidFromDate(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet,
		"/analytics/timeseries?workspace_id="+validAnalyticsUUID+"&from=2026/01/01", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetTimeseries(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (from format invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetTimeseries_invalidToDate(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet,
		"/analytics/timeseries?workspace_id="+validAnalyticsUUID+"&from=2026-01-01&to=2026-13-01", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetTimeseries(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (to format invalide)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

func TestHandleGetTimeseries_fromAfterTo(t *testing.T) {
	svc := newTestService()
	req := httptest.NewRequest(http.MethodGet,
		"/analytics/timeseries?workspace_id="+validAnalyticsUUID+"&from=2026-12-01&to=2026-01-01", nil)
	rr := httptest.NewRecorder()

	svc.HandleGetTimeseries(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code = %d, voulu %d (from > to)", rr.Code, http.StatusBadRequest)
	}
	assertJSONError(t, rr)
}

// ---- buildKpiQuery (fonction pure) ----

func TestBuildKpiQuery_noOptionalFilters(t *testing.T) {
	from := mustParseDate(t, "2026-01-01")
	to := mustParseDate(t, "2026-01-31")

	q, args := buildKpiQuery(validAnalyticsUUID, from, to, "", "")

	if !strings.Contains(q, "FROM analytics.kpi_daily") {
		t.Errorf("requete sans FROM analytics.kpi_daily : %q", q)
	}
	if !strings.Contains(q, "workspace_id = $1") {
		t.Errorf("requete sans workspace_id = $1 : %q", q)
	}
	if !strings.Contains(q, "BETWEEN $2 AND $3") {
		t.Errorf("requete sans BETWEEN $2 AND $3 : %q", q)
	}
	if !strings.Contains(q, "ORDER BY day DESC") {
		t.Errorf("requete sans ORDER BY day DESC : %q", q)
	}
	// 3 args : workspace_id, from, to.
	if len(args) != 3 {
		t.Errorf("nombre d'args = %d, voulu 3 (sans filtres optionnels)", len(args))
	}
	if args[0] != validAnalyticsUUID {
		t.Errorf("args[0] = %v, voulu workspace_id", args[0])
	}
}

func TestBuildKpiQuery_withChannel(t *testing.T) {
	from := mustParseDate(t, "2026-01-01")
	to := mustParseDate(t, "2026-01-31")

	q, args := buildKpiQuery(validAnalyticsUUID, from, to, "sms", "")

	if !strings.Contains(q, "channel = $4") {
		t.Errorf("requete sans channel = $4 : %q", q)
	}
	// 4 args : workspace_id, from, to, channel.
	if len(args) != 4 {
		t.Errorf("nombre d'args = %d, voulu 4 (avec channel)", len(args))
	}
	if args[3] != "sms" {
		t.Errorf("args[3] = %v, voulu \"sms\"", args[3])
	}
}

func TestBuildKpiQuery_withCountry(t *testing.T) {
	from := mustParseDate(t, "2026-01-01")
	to := mustParseDate(t, "2026-01-31")

	q, args := buildKpiQuery(validAnalyticsUUID, from, to, "", "CM")

	if !strings.Contains(q, "country = $4") {
		t.Errorf("requete sans country = $4 : %q", q)
	}
	// 4 args : workspace_id, from, to, country.
	if len(args) != 4 {
		t.Errorf("nombre d'args = %d, voulu 4 (avec country)", len(args))
	}
	if args[3] != "CM" {
		t.Errorf("args[3] = %v, voulu \"CM\"", args[3])
	}
}

func TestBuildKpiQuery_withChannelAndCountry(t *testing.T) {
	from := mustParseDate(t, "2026-01-01")
	to := mustParseDate(t, "2026-01-31")

	q, args := buildKpiQuery(validAnalyticsUUID, from, to, "whatsapp", "SN")

	if !strings.Contains(q, "channel = $4") {
		t.Errorf("requete sans channel = $4 : %q", q)
	}
	if !strings.Contains(q, "country = $5") {
		t.Errorf("requete sans country = $5 : %q", q)
	}
	// 5 args : workspace_id, from, to, channel, country.
	if len(args) != 5 {
		t.Errorf("nombre d'args = %d, voulu 5 (avec channel + country)", len(args))
	}
	if args[3] != "whatsapp" {
		t.Errorf("args[3] = %v, voulu \"whatsapp\"", args[3])
	}
	if args[4] != "SN" {
		t.Errorf("args[4] = %v, voulu \"SN\"", args[4])
	}
}

// ---- buildTimeseriesQuery (fonction pure) ----

func TestBuildTimeseriesQuery_noChannel(t *testing.T) {
	from := mustParseDate(t, "2026-01-01")
	to := mustParseDate(t, "2026-01-31")

	q, args := buildTimeseriesQuery(validAnalyticsUUID, from, to, "")

	if !strings.Contains(q, "FROM analytics.message_daily") {
		t.Errorf("requete sans FROM analytics.message_daily : %q", q)
	}
	if !strings.Contains(q, "SUM(sent)") {
		t.Errorf("requete sans SUM(sent) : %q", q)
	}
	if !strings.Contains(q, "GROUP BY day") {
		t.Errorf("requete sans GROUP BY day : %q", q)
	}
	if !strings.Contains(q, "ORDER BY day ASC") {
		t.Errorf("requete sans ORDER BY day ASC : %q", q)
	}
	// 3 args : workspace_id, from, to.
	if len(args) != 3 {
		t.Errorf("nombre d'args = %d, voulu 3 (sans filtre channel)", len(args))
	}
}

func TestBuildTimeseriesQuery_withChannel(t *testing.T) {
	from := mustParseDate(t, "2026-01-01")
	to := mustParseDate(t, "2026-01-31")

	q, args := buildTimeseriesQuery(validAnalyticsUUID, from, to, "telegram")

	if !strings.Contains(q, "channel = $4") {
		t.Errorf("requete sans channel = $4 : %q", q)
	}
	// 4 args : workspace_id, from, to, channel.
	if len(args) != 4 {
		t.Errorf("nombre d'args = %d, voulu 4 (avec channel)", len(args))
	}
	if args[3] != "telegram" {
		t.Errorf("args[3] = %v, voulu \"telegram\"", args[3])
	}
}

// ---- helpers locaux ----

// mustParseDate parse une date "2006-01-02" et echoue le test si invalide.
func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("mustParseDate(%q): %v", s, err)
	}
	return d
}

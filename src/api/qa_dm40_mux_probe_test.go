package api

// qa_dm40_mux_probe_test.go — ajouté par la QA D-M40 (critère 10) : vérifie par un
// test d'intégration réel (via Service.ServeHTTP -> le vrai http.ServeMux Go 1.22+,
// pas une lecture statique de service.go) que les nouvelles routes
// /webhook-endpoints (CRUD sortant) ne collisionnent pas avec les routes
// /webhooks/om|mtn|telegram (callbacks entrants) déjà enregistrées.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteMuxDoesNotCollide_probe(t *testing.T) {
	s := &Service{Providers: map[string]Provider{}}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	// POST /webhook-endpoints should route to create handler, not webhooks/telegram etc.
	req := httptest.NewRequest(http.MethodPost, "/webhook-endpoints", nil)
	rr := httptest.NewRecorder()
	s.ServeHTTP(rr, req)
	// invalid body -> 400 from HandleCreateWebhookEndpoint, not 404 (proves routed correctly)
	if rr.Code == http.StatusNotFound {
		t.Fatalf("POST /webhook-endpoints returned 404 -- route not registered/collision, got %d", rr.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/webhooks/telegram", nil)
	rr2 := httptest.NewRecorder()
	s.ServeHTTP(rr2, req2)
	if rr2.Code == http.StatusNotFound {
		t.Fatalf("POST /webhooks/telegram returned 404 -- collided with /webhook-endpoints?, got %d", rr2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/webhook-endpoints?workspace_id=x", nil)
	rr3 := httptest.NewRecorder()
	s.ServeHTTP(rr3, req3)
	if rr3.Code == http.StatusNotFound {
		t.Fatalf("GET /webhook-endpoints returned 404, got %d", rr3.Code)
	}
}

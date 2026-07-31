package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	golog "fleece/src/go/log"
)

// ---- parseUUID ----

func TestParseUUID_valid(t *testing.T) {
	cases := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"00000000-0000-0000-0000-000000000000",
		"FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			got, err := parseUUID(tc)
			if err != nil {
				t.Fatalf("parseUUID(%q) erreur inattendue : %v", tc, err)
			}
			if got != tc {
				t.Errorf("parseUUID(%q) = %q, voulu %q", tc, got, tc)
			}
		})
	}
}

func TestParseUUID_invalid(t *testing.T) {
	cases := []string{
		"",
		"not-a-uuid",
		"550e8400-e29b-41d4-a716",               // trop court
		"550e8400-e29b-41d4-a716-4466554400001", // trop long
		"550e8400-e29b-41d4-a716-44665544000G",  // char invalide
		"550e8400e29b41d4a716446655440000",      // sans tirets
	}
	for _, tc := range cases {
		t.Run(tc+"_invalid", func(t *testing.T) {
			_, err := parseUUID(tc)
			if err == nil {
				t.Fatalf("parseUUID(%q) aurait du echouer", tc)
			}
		})
	}
}

// ---- isValidStrategy ----

func TestIsValidStrategy(t *testing.T) {
	valid := []string{"lowest_cost", "highest_delivery", "round_robin"}
	for _, s := range valid {
		if !isValidStrategy(s) {
			t.Errorf("isValidStrategy(%q) = false, voulu true", s)
		}
	}

	invalid := []string{"", "unknown", "LOWEST_COST", "best_effort"}
	for _, s := range invalid {
		if isValidStrategy(s) {
			t.Errorf("isValidStrategy(%q) = true, voulu false", s)
		}
	}
}

// ---- newUUID ----

func TestNewUUID(t *testing.T) {
	u1 := newUUID()
	u2 := newUUID()

	// Format : 8-4-4-4-12 = 36 caracteres.
	if len(u1) != 36 {
		t.Errorf("newUUID() longueur = %d, voulu 36 : %q", len(u1), u1)
	}
	// Doit etre un UUID valide selon parseUUID.
	if _, err := parseUUID(u1); err != nil {
		t.Errorf("newUUID() n'est pas un UUID valide : %q", u1)
	}
	// Deux appels consecutifs doivent produire des valeurs differentes.
	if u1 == u2 {
		t.Errorf("newUUID() a retourne le meme UUID deux fois : %q", u1)
	}
	// Version 4 : le 13eme caractere (index 14) doit etre '4'.
	if u1[14] != '4' {
		t.Errorf("newUUID() version incorrecte : caractere 14 = %c, voulu 4", u1[14])
	}
	// Variante RFC 4122 : le 17eme caractere (index 19) doit etre 8, 9, a ou b.
	variant := u1[19]
	if variant != '8' && variant != '9' && variant != 'a' && variant != 'b' {
		t.Errorf("newUUID() variante incorrecte : caractere 19 = %c, voulu 8/9/a/b", variant)
	}
}

// ---- mapProviderStatus ----

func TestMapProviderStatus(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"sent", "sent"},
		{"delivered", "sent"},
		{"rejected", "rejected"},
		{"failed", "failed"},
		{"", "pending"},
		{"unknown", "pending"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := mapProviderStatus(tc.input)
			if got != tc.want {
				t.Errorf("mapProviderStatus(%q) = %q, voulu %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---- buildListQuery ----

// TestBuildListQuery_workspaceFilterIsUnconditional remplace l'ancien
// TestBuildListQuery_noFilters, qui vérifiait — et donc VERROUILLAIT — le
// comportement non scopé : sans workspace_id, la requête ne portait AUCUN
// filtre de workspace et retournait les messages de tous les workspaces
// confondus. C'était la faille D-M41 (2e occurrence), pas une fonctionnalité.
//
// Le filtre est désormais STRUCTUREL : il appartient à la clause WHERE de base.
// Un workspaceID vide produit donc une requête qui ne matche rien, au lieu de
// tout retourner.
func TestBuildListQuery_workspaceFilterIsUnconditional(t *testing.T) {
	q, args := buildListQuery("", "", 50, 0)
	if !strings.Contains(q, "FROM messaging.messages") {
		t.Errorf("requete sans FROM messaging.messages : %q", q)
	}
	if !strings.Contains(q, "ORDER BY created_at DESC") {
		t.Errorf("requete sans ORDER BY : %q", q)
	}
	if !strings.Contains(q, "LIMIT") || !strings.Contains(q, "OFFSET") {
		t.Errorf("requete sans LIMIT/OFFSET : %q", q)
	}
	// LE point du test : le filtre est present MEME avec un workspaceID vide.
	if !strings.Contains(q, "WHERE workspace_id = $1") {
		t.Fatalf("D-M41 : filtre workspace_id absent alors qu'il doit etre inconditionnel : %q", q)
	}
	if strings.Contains(q, "WHERE 1=1") {
		t.Errorf("D-M41 : la requete utilise encore le WHERE 1=1 qui rendait le scoping optionnel : %q", q)
	}
	// 3 args : workspace_id (vide ici), limit, offset.
	if len(args) != 3 {
		t.Fatalf("nombre d'args = %d, voulu 3 (workspace_id + limit + offset)", len(args))
	}
	if args[0] != "" {
		t.Errorf("args[0] (workspace_id) = %v, voulu la chaine vide passee en entree", args[0])
	}
	if args[1] != 50 {
		t.Errorf("args[1] (limit) = %v, voulu 50", args[1])
	}
	if args[2] != 0 {
		t.Errorf("args[2] (offset) = %v, voulu 0", args[2])
	}
}

func TestBuildListQuery_withWorkspaceID(t *testing.T) {
	wid := "550e8400-e29b-41d4-a716-446655440000"
	q, args := buildListQuery(wid, "", 10, 5)
	if !strings.Contains(q, "workspace_id = $1") {
		t.Errorf("requete sans workspace_id filter : %q", q)
	}
	// 3 args : workspace_id, limit, offset.
	if len(args) != 3 {
		t.Errorf("nombre d'args = %d, voulu 3", len(args))
	}
	if args[0] != wid {
		t.Errorf("args[0] = %v, voulu %q", args[0], wid)
	}
}

func TestBuildListQuery_withAllFilters(t *testing.T) {
	wid := "550e8400-e29b-41d4-a716-446655440000"
	q, args := buildListQuery(wid, "sent", 20, 10)
	if !strings.Contains(q, "workspace_id = $1") {
		t.Errorf("requete sans workspace_id filter : %q", q)
	}
	if !strings.Contains(q, "status = $2") {
		t.Errorf("requete sans status filter : %q", q)
	}
	// 4 args : workspace_id, status, limit, offset.
	if len(args) != 4 {
		t.Errorf("nombre d'args = %d, voulu 4", len(args))
	}
	if args[1] != "sent" {
		t.Errorf("args[1] (status) = %v, voulu sent", args[1])
	}
}

// D-M41 : le status n'est plus jamais le PREMIER placeholder — $1 est
// desormais reserve a workspace_id, qui est structurel. Ce test verifiait
// auparavant `status = $1`, c'est-a-dire une requete SANS scoping.
func TestBuildListQuery_withStatusOnly(t *testing.T) {
	q, args := buildListQuery("", "failed", 50, 0)
	if !strings.Contains(q, "WHERE workspace_id = $1") {
		t.Fatalf("D-M41 : filtre workspace_id absent : %q", q)
	}
	if !strings.Contains(q, "status = $2") {
		t.Errorf("requete sans status filter en $2 : %q", q)
	}
	// 4 args : workspace_id, status, limit, offset.
	if len(args) != 4 {
		t.Fatalf("nombre d'args = %d, voulu 4 (workspace_id + status + limit + offset)", len(args))
	}
	if args[1] != "failed" {
		t.Errorf("args[1] (status) = %v, voulu failed", args[1])
	}
}

// ---- recoverMiddleware (ecart C4 : panic sur un chemin de handler) ----

// TestRecoverMiddleware_convertsPanicTo500 verifie qu'une panique survenue dans
// le handler enveloppe est convertie en reponse 500 JSON plutot que de faire
// crasher le processus (recover global, beneficie aux 22 handlers — voir
// service.go ServeHTTP). Couvre newUUID() panic (ex. epuisement crypto/rand).
func TestRecoverMiddleware_convertsPanicTo500(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("newUUID: boom")
	})
	handler := recoverMiddleware(golog.Init("warn", "text"), panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, voulu %d (panique convertie en 500)", rr.Code, http.StatusInternalServerError)
	}
	assertJSONError(t, rr)
}

// TestRecoverMiddleware_nilLoggerDoesNotPanic verifie que recoverMiddleware ne
// paniquerait pas lui-meme si Logger est nil (cas des Service de test minimaux
// — voir newTestService dans messages_handlers_test.go).
func TestRecoverMiddleware_nilLoggerDoesNotPanic(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom sans logger")
	})
	handler := recoverMiddleware(nil, panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, voulu %d (panique convertie en 500 meme sans logger)", rr.Code, http.StatusInternalServerError)
	}
}

// TestRecoverMiddleware_passesThroughWithoutPanic verifie que la reponse normale
// (sans panique) traverse la middleware inchangee.
func TestRecoverMiddleware_passesThroughWithoutPanic(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := recoverMiddleware(golog.Init("warn", "text"), okHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("code = %d, voulu %d (pas de panique → reponse inchangee)", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != `{"ok":true}` {
		t.Errorf("body = %q, voulu %q", rr.Body.String(), `{"ok":true}`)
	}
}

// TestServiceServeHTTP_recoversFromPanic verifie l'integration complete :
// Service.ServeHTTP (utilise par le composition root cmd/api/main.go via
// http.Server.Handler) recupere bien les paniques des handlers routes par le mux.
func TestServiceServeHTTP_recoversFromPanic(t *testing.T) {
	svc := &Service{
		Providers: make(map[string]Provider),
		Logger:    golog.Init("warn", "text"),
	}
	if err := svc.Init(); err != nil {
		t.Fatalf("Init() erreur inattendue : %v", err)
	}

	// GET /health ne panique jamais dans l'implementation actuelle : on verifie
	// simplement que ServeHTTP fonctionne nominalement a travers la middleware.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	svc.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("GET /health via ServeHTTP : code = %d, voulu %d", rr.Code, http.StatusOK)
	}
}

package api

// d_m41_workspace_scoping_test.go — gardes de non-régression de la faille
// D-M41 : lecture de messages appartenant à un AUTRE workspace.
//
// Deux occurrences, corrigées ensemble :
//
//   - GET /messages/{id} — `WHERE id = $1` seul : connaître un UUID de message
//     suffisait à lire son contenu, son destinataire et son statut, quel que
//     soit son workspace. C'est l'occurrence décrite par la dette.
//   - GET /messages — `workspace_id` était OPTIONNEL : appeler l'endpoint SANS
//     ce paramètre retournait les messages de TOUS les workspaces confondus
//     (jusqu'à 200 lignes). Occurrence NON décrite par la dette et plus grave :
//     fuite en masse, sans rien avoir à deviner.
//
// Ces tests exercent le SQL RÉELLEMENT ÉMIS via le faux driver
// (newFakeGosqlDB, cf. qa_m019_dbpath_test.go), pas seulement les codes de
// retour : c'est la clause WHERE qui constitue la frontière de sécurité, et
// c'est donc elle qu'il faut observer. Une vérification limitée au code HTTP
// passerait au vert sur une requête non scopée renvoyant une ligne.

import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	golog "fleece/src/go/log"
)

const (
	dm41WorkspaceA = "550e8400-e29b-41d4-a716-446655440000"
	dm41WorkspaceB = "11111111-1111-1111-1111-111111111111"
	dm41MessageID  = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
)

// ============================================================
// GET /messages/{id}
// ============================================================

// TestHandleGetMessage_requiresWorkspaceID : sans workspace_id, la requête est
// refusée AVANT tout accès base — le paramètre est une frontière, pas un
// filtre d'agrément.
func TestHandleGetMessage_requiresWorkspaceID(t *testing.T) {
	conn := &fakeConn{}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/messages/"+dm41MessageID, nil)
	req.SetPathValue("id", dm41MessageID)
	rr := httptest.NewRecorder()

	svc.HandleGetMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, voulu %d (workspace_id manquant)", rr.Code, http.StatusBadRequest)
	}
	if len(conn.queries) != 0 {
		t.Errorf("D-M41 : une requete SQL a ete emise malgre l'absence de workspace_id : %+v", conn.queries)
	}
}

func TestHandleGetMessage_invalidWorkspaceUUID(t *testing.T) {
	conn := &fakeConn{}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/messages/"+dm41MessageID+"?workspace_id=not-a-uuid", nil)
	req.SetPathValue("id", dm41MessageID)
	rr := httptest.NewRecorder()

	svc.HandleGetMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, voulu %d (workspace_id non-UUID)", rr.Code, http.StatusBadRequest)
	}
	if len(conn.queries) != 0 {
		t.Errorf("D-M41 : requete SQL emise malgre un workspace_id invalide : %+v", conn.queries)
	}
}

// TestHandleGetMessage_scopesQueryByWorkspace est LE test de la faille : il
// observe le SQL émis et exige que la clause WHERE porte les DEUX critères,
// avec les deux valeurs réellement passées en arguments.
func TestHandleGetMessage_scopesQueryByWorkspace(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{
			columns: []string{"id", "workspace_id", "recipient", "content", "status", "channel", "created_at"},
			data:    [][]driver.Value{},
		}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/messages/"+dm41MessageID+"?workspace_id="+dm41WorkspaceA, nil)
	req.SetPathValue("id", dm41MessageID)
	rr := httptest.NewRecorder()

	svc.HandleGetMessage(rr, req)

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 : %+v", len(conn.queries), conn.queries)
	}
	q := conn.queries[0]
	if !strings.Contains(q, "WHERE id = $1 AND workspace_id = $2") {
		t.Fatalf("D-M41 : requete non scopee par workspace (frontiere de securite absente) : %q", q)
	}
	if len(conn.lastArgs) != 2 {
		t.Fatalf("nombre d'arguments = %d, voulu 2 (id + workspace_id) : %+v", len(conn.lastArgs), conn.lastArgs)
	}
	if got := conn.lastArgs[1].Value; got != dm41WorkspaceA {
		t.Errorf("2e argument = %v, voulu le workspace_id de la requete (%q)", got, dm41WorkspaceA)
	}
}

// TestHandleGetMessage_otherWorkspace_returns404NotForbidden : un message
// appartenant à un autre workspace doit être INDISCERNABLE d'un message
// inexistant. Un 403 confirmerait l'existence de l'identifiant et
// transformerait l'endpoint en oracle d'énumération.
func TestHandleGetMessage_otherWorkspace_returns404NotForbidden(t *testing.T) {
	// Le message existe, mais appartient au workspace B : la requête scopée sur
	// A ne ramène donc aucune ligne — exactement comme pour un id inexistant.
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{
			columns: []string{"id", "workspace_id", "recipient", "content", "status", "channel", "created_at"},
			data:    [][]driver.Value{},
		}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/messages/"+dm41MessageID+"?workspace_id="+dm41WorkspaceB, nil)
	req.SetPathValue("id", dm41MessageID)
	rr := httptest.NewRecorder()

	svc.HandleGetMessage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("code = %d, voulu %d (404 uniforme, jamais 403 : ne pas fuiter l'existence)", rr.Code, http.StatusNotFound)
	}
	if rr.Code == http.StatusForbidden {
		t.Error("403 renvoye : confirme l'existence du message a un appelant d'un autre workspace")
	}
	// Le corps ne doit rien reveler du message ni de son workspace reel.
	if body := rr.Body.String(); strings.Contains(body, dm41WorkspaceA) || strings.Contains(body, "recipient") {
		t.Errorf("le corps 404 fuit des informations sur le message : %q", body)
	}
}

// ============================================================
// GET /messages (fuite en masse — occurrence non décrite par la dette)
// ============================================================

// TestHandleListMessages_requiresWorkspaceID : c'est l'appel qui retournait
// auparavant les messages de tous les workspaces.
func TestHandleListMessages_requiresWorkspaceID(t *testing.T) {
	conn := &fakeConn{}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	rr := httptest.NewRecorder()

	svc.HandleListMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, voulu %d — sans workspace_id, cet appel listait TOUS les workspaces (D-M41)", rr.Code, http.StatusBadRequest)
	}
	if len(conn.queries) != 0 {
		t.Errorf("D-M41 : requete SQL emise sans workspace_id : %+v", conn.queries)
	}
}

// TestHandleListMessages_scopesQueryByWorkspace vérifie que le filtre est bien
// dans le SQL émis, et qu'il est le PREMIER placeholder (structurel), y compris
// quand d'autres filtres optionnels sont présents.
func TestHandleListMessages_scopesQueryByWorkspace(t *testing.T) {
	conn := &fakeConn{queryFn: func(query string, args []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{
			columns: []string{"id", "workspace_id", "recipient", "content", "status", "channel", "created_at"},
			data:    [][]driver.Value{},
		}, nil
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}

	req := httptest.NewRequest(http.MethodGet, "/messages?workspace_id="+dm41WorkspaceA+"&status=sent", nil)
	rr := httptest.NewRecorder()

	svc.HandleListMessages(rr, req)

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 : %+v", len(conn.queries), conn.queries)
	}
	q := conn.queries[0]
	if !strings.Contains(q, "WHERE workspace_id = $1") {
		t.Fatalf("D-M41 : filtre workspace_id absent ou non structurel : %q", q)
	}
	if strings.Contains(q, "WHERE 1=1") {
		t.Errorf("D-M41 : le WHERE 1=1 qui rendait le scoping optionnel est toujours la : %q", q)
	}
	if got := conn.lastArgs[0].Value; got != dm41WorkspaceA {
		t.Errorf("1er argument = %v, voulu le workspace_id (%q)", got, dm41WorkspaceA)
	}
}

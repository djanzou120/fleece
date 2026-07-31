package coreprocessor

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	golog "fleece/src/go/log"
)

func testLogger() *golog.Logger { return golog.Init("warn", "text") }

// TestHandleMessageDelivered_nominal couvre le chemin nominal : le
// UPDATE ... RETURNING workspace_id renvoie une ligne (transition reelle) ->
// succes, pas d'erreur, workspace_id recupere SANS seconde requete (E2).
func TestHandleMessageDelivered_nominal(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id"}, []driver.Value{testWorkspaceID})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", ExternalID: "42", ProviderID: "telegram-bot"}
	if err := handleMessageDelivered(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageDelivered() erreur inattendue : %v", err)
	}

	// D-M43 (fan-out webhooks sortants) : depuis workspace_id connu (transition
	// reelle), ce handler emet une requete SUPPLEMENTAIRE apres la transition
	// (chargement des endpoints abonnes, voir webhook_dispatch.go) — cette
	// assertion cible donc explicitement conn.queries[0] (la toute PREMIERE
	// requete, celle de la transition de statut elle-meme, RETURNING
	// workspace_id EN UNE SEULE requete — E2, plus de seconde lecture) plutot
	// que conn.lastQuery(), pour rester correcte independamment de ce qui suit.
	if len(conn.queries) == 0 {
		t.Fatalf("aucune requete emise")
	}
	statusUpdateQuery := conn.queries[0]
	if !strings.Contains(statusUpdateQuery, "UPDATE messaging.messages") {
		t.Fatalf("requete SQL inattendue (table) : %q", statusUpdateQuery)
	}
	if !strings.Contains(statusUpdateQuery, "status = 'delivered'") {
		t.Fatalf("requete SQL inattendue (statut cible) : %q", statusUpdateQuery)
	}
	// B1/Q3 (Phase 3) : la garde SQL est le miroir exact de messageTransitions
	// (src/api/message.go) — "sent" est le SEUL etat depuis lequel "delivered"
	// est une transition autorisee (draft/pending -> delivered est INTERDIT).
	if !strings.Contains(statusUpdateQuery, "status = 'sent'") {
		t.Fatalf("garde d'idempotence absente de la requete (attendu status = 'sent', miroir de Message.Transition()) : %q", statusUpdateQuery)
	}
	// E2 : workspace_id doit etre lu DIRECTEMENT dans le RETURNING de CETTE
	// requete (plus de seconde lecture separee de messaging.messages).
	if !strings.Contains(statusUpdateQuery, "RETURNING workspace_id") {
		t.Fatalf("requete SQL sans RETURNING workspace_id (E2, plus de seconde requete de lecture) : %q", statusUpdateQuery)
	}
	if len(conn.args[0]) != 1 || conn.args[0][0].Value != evt.MessageID {
		t.Fatalf("arguments inattendus : %+v", conn.args[0])
	}
	// 2e requete : chargement des endpoints abonnes a "message.delivered"
	// (workspace_id reutilise depuis le RETURNING ci-dessus, jamais une
	// deuxieme lecture de messaging.messages).
	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2 (transition + fan-out webhook, E2 sans lecture workspace_id separee) : %+v", len(conn.queries), conn.queries)
	}
	if !strings.Contains(conn.queries[1], "FROM webhook.webhook_endpoints") {
		t.Fatalf("2e requete inattendue (fan-out webhook) : %q", conn.queries[1])
	}
	if conn.args[1][0].Value != testWorkspaceID {
		t.Errorf("workspace_id utilise pour le fan-out = %v, voulu %q (reutilise depuis le RETURNING, E2)", conn.args[1][0].Value, testWorkspaceID)
	}
}

// TestHandleMessageDelivered_doubleTraitement verifie l'idempotence : 0 ligne
// affectee (deja delivered/failed/rejected, ou introuvable) -> succes, pas
// d'erreur (pas de requeue, pas de crash), AUCUN fan-out webhook.
func TestHandleMessageDelivered_doubleTraitement(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id"})}, // 0 ligne -> sql.ErrNoRows
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"}
	if err := handleMessageDelivered(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageDelivered() erreur inattendue sur double-traitement : %v", err)
	}
}

// TestHandleMessageDelivered_dbError verifie qu'une panne technique remonte
// une erreur "nue" (donc requeue cote consumer.go — non permanente).
func TestHandleMessageDelivered_dbError(t *testing.T) {
	wantErr := errors.New("connexion perdue")
	conn := &fakeConn{querySteps: []queryStep{{err: wantErr}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"}
	err := handleMessageDelivered(context.Background(), svc, evt)
	if err == nil {
		t.Fatal("handleMessageDelivered() attendu en erreur sur panne DB")
	}
	if isPermanentError(err) {
		t.Errorf("handleMessageDelivered() erreur DB classee permanente a tort (devrait etre requeue) : %v", err)
	}
}

// TestHandleMessageDelivered_rows0_neverFansOutWebhook est LE test de
// non-regression B1/D-M26 applique au volet webhook (D-M43) : quand
// l'UPDATE ne transitionne AUCUNE ligne (rejeu Telegram — D-M25, Telegram
// rejoue ses updates jusqu'a 24h — ou tout autre double-traitement), le
// fan-out webhook NE DOIT JAMAIS se declencher, sous peine d'envoyer le MEME
// evenement en double au client final a chaque rejeu. On le prouve en
// verifiant qu'AUCUNE requete SQL supplementaire (le chargement des endpoints
// abonnes) n'est emise au-dela de l'UPDATE lui-meme : une seule requete au
// total.
func TestHandleMessageDelivered_rows0_neverFansOutWebhook(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id"})}, // 0 ligne
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", ExternalID: "42", ProviderID: "telegram-bot"}
	if err := handleMessageDelivered(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageDelivered() erreur inattendue : %v", err)
	}

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu EXACTEMENT 1 (0 ligne -> AUCUNE requete de fan-out webhook, donc AUCUN webhook envoye) : %+v", len(conn.queries), conn.queries)
	}
}

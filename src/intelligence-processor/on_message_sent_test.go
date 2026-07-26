package intelligenceprocessor

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

// TestHandleMessageSent_payloadIllisible / messageIDVide / messageIDInvalide
// verifient les rejets permanents AVANT toute requete SQL.
func TestHandleMessageSent_payloadIllisible(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageSent(context.Background(), svc, []byte("not json"))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageSent() sur JSON illisible = %v, voulu une permanentError", err)
	}
}

func TestHandleMessageSent_messageIDVide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageSent(context.Background(), svc, []byte(`{}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageSent() sur message_id vide = %v, voulu une permanentError", err)
	}
}

func TestHandleMessageSent_messageIDInvalide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageSent(context.Background(), svc, []byte(`{"message_id":"pas-un-uuid"}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageSent() sur message_id invalide = %v, voulu une permanentError", err)
	}
}

// TestHandleMessageSent_messageIntrouvable verifie le rejet permanent quand
// messaging.messages ne contient pas l'id (evenement force/rejoue).
func TestHandleMessageSent_messageIntrouvable(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id", "recipient", "channel", "cost", "created_at"})}, // 0 ligne
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	err := handleMessageSent(context.Background(), svc, body)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageSent() sur message introuvable = %v, voulu une permanentError", err)
	}
}

// TestHandleMessageSent_dbErrorOnLoad verifie qu'une panne technique remonte
// une erreur "nue" (requeue).
func TestHandleMessageSent_dbErrorOnLoad(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	err := handleMessageSent(context.Background(), svc, body)
	if err == nil {
		t.Fatal("handleMessageSent() attendu en erreur sur panne DB")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort (devrait etre requeue) : %v", err)
	}
}

// TestHandleMessageSent_dbErrorOnUpsert verifie qu'une panne technique sur
// l'UPSERT analytics remonte aussi une erreur "nue".
func TestHandleMessageSent_dbErrorOnUpsert(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: messageRowFixture()}},
		execSteps:  []execStep{{err: errors.New("connexion perdue")}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	err := handleMessageSent(context.Background(), svc, body)
	if err == nil {
		t.Fatal("handleMessageSent() attendu en erreur sur panne DB (upsert)")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort (devrait etre requeue) : %v", err)
	}
}

// TestHandleMessageSent_nominal_incrementsSentAndCostOnly verifie la
// repartition stricte des compteurs (analytics_aggregate.go) : sent+cost
// incrementes, delivered/failed/latence JAMAIS touches par ce handler.
func TestHandleMessageSent_nominal_incrementsSentAndCostOnly(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: messageRowFixture()}}, // cost=10, created_at=testMessageCreatedAt
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	if err := handleMessageSent(context.Background(), svc, body); err != nil {
		t.Fatalf("handleMessageSent() erreur inattendue : %v", err)
	}

	if !strings.Contains(conn.lastQuery(), "analytics.message_daily") {
		t.Fatalf("requete inattendue : %q", conn.lastQuery())
	}
	args := conn.lastArgs()
	assertArg := func(idx int, want driver.Value, label string) {
		if args[idx].Value != want {
			t.Errorf("%s = %v, voulu %v", label, args[idx].Value, want)
		}
	}
	assertArg(4, int64(1), "sentDelta")
	assertArg(5, int64(0), "deliveredDelta")
	assertArg(6, int64(0), "failedDelta")
	assertArg(7, int64(10), "costDelta (relu depuis messaging.messages.cost, jamais le payload)")
	assertArg(8, int64(0), "latencyDelta")

	// day = jour d'ENVOI (created_at de la fixture), jamais time.Now().
	if got := args[0].Value; got != "2024-03-03" {
		t.Errorf("day = %v, voulu %q (derive de created_at)", got, "2024-03-03")
	}
	// country derive du recipient de la fixture (+237... -> CM).
	if got := args[2].Value; got != "CM" {
		t.Errorf("country = %v, voulu CM", got)
	}
}

// TestHandleMessageSent_costNull verifie que cost NULL en base est traite
// comme costDelta=0 (jamais une erreur de scan).
func TestHandleMessageSent_costNull(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: messageRowFixtureWithCost(nil, testMessageCreatedAt)}},
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	if err := handleMessageSent(context.Background(), svc, body); err != nil {
		t.Fatalf("handleMessageSent() erreur inattendue (cost NULL) : %v", err)
	}
	if got := conn.lastArgs()[7].Value; got != int64(0) {
		t.Errorf("costDelta (cost NULL) = %v, voulu 0", got)
	}
}

// TestHandleMessageSent_channelNull couvre la regression E2 : messaging.messages.channel
// NULL (donnee heritee) ne doit jamais produire une erreur de scan classee
// transitoire (requeue infini) — le handler doit reussir et utiliser
// unknownChannel ("unknown") comme valeur de repli dans l'UPSERT analytics
// (colonne NOT NULL, membre de la cle primaire).
func TestHandleMessageSent_channelNull(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: messageRowFixtureWithChannel(nil)}},
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	if err := handleMessageSent(context.Background(), svc, body); err != nil {
		t.Fatalf("handleMessageSent() erreur inattendue (channel NULL, regression E2) : %v", err)
	}
	if got := conn.lastArgs()[3].Value; got != unknownChannel {
		t.Errorf("channel utilise = %v, voulu %q (repli sur channel NULL)", got, unknownChannel)
	}
}

// TestHandleMessageSent_ignoresPayloadFields verifie l'autonomie stricte
// (§Autonomie, consumer.go) : un evenement dont le payload contient un
// workspace_id/recipient DIFFERENT de ce qui est en base n'influence jamais
// l'agregat — seules les valeurs relues depuis messaging.messages comptent.
func TestHandleMessageSent_ignoresPayloadFields(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: messageRowFixture()}},
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	// Le payload pretend un tout autre workspace/recipient : ne doit jamais
	// se retrouver dans l'UPSERT (toujours row.WorkspaceID/row.Recipient de la
	// fixture DB).
	body := []byte(`{"message_id":"` + testMessageID + `","workspace_id":"forged","recipient":"+33000000000"}`)
	if err := handleMessageSent(context.Background(), svc, body); err != nil {
		t.Fatalf("handleMessageSent() erreur inattendue : %v", err)
	}
	args := conn.lastArgs()
	if got := args[1].Value; got != testWorkspaceID {
		t.Errorf("workspace_id utilise = %v, voulu %q (relu en base, jamais le payload)", got, testWorkspaceID)
	}
	if got := args[2].Value; got != "CM" {
		t.Errorf("country derive = %v, voulu CM (base sur le recipient EN BASE, pas le payload)", got)
	}
}

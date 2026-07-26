package intelligenceprocessor

import (
	"context"
	"strings"
	"testing"
)

// on_message_failed_test.go — tests specifiques a handleMessageFailed
// (parsing/validation, delegation a processDeliveryOutcome), ET le
// garde-fou anti-double-remboursement (Decision C, PM M-021) : ce worker ne
// doit JAMAIS emettre de requete touchant wallet.* — cette responsabilite
// appartient EXCLUSIVEMENT a src/core-processor.

func TestHandleMessageFailed_payloadIllisible(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageFailed(context.Background(), svc, []byte("not json"))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageFailed() sur JSON illisible = %v, voulu une permanentError", err)
	}
}

func TestHandleMessageFailed_messageIDVide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageFailed(context.Background(), svc, []byte(`{}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageFailed() sur message_id vide = %v, voulu une permanentError", err)
	}
}

func TestHandleMessageFailed_messageIDInvalide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageFailed(context.Background(), svc, []byte(`{"message_id":"pas-un-uuid"}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageFailed() sur message_id invalide = %v, voulu une permanentError", err)
	}
}

// TestHandleMessageFailed_nominal_incrementsFailedNeverDelivered verifie la
// repartition stricte des compteurs pour ce handler precis.
func TestHandleMessageFailed_nominal_incrementsFailedNeverDelivered(t *testing.T) {
	// cost=500 (> 0) : verifie explicitement qu'un message facture n'entraine
	// AUCUN mouvement wallet ici (voir garde-fou ci-dessous), meme si le cout
	// est positif -- c'est le cas le plus a risque de confusion avec
	// core-processor.
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: messageRowFixtureWithCost(int64(500), testMessageCreatedAt)},
			{rows: fakeRowsOfCols([]string{"score", "sample_size"})},
			{rows: fakeRowsOfCols([]string{"campaign_id"})},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `","external_id":"42","provider_id":"telegram-bot"}`)
	if err := handleMessageFailed(context.Background(), svc, body); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue : %v", err)
	}

	_, args, found := findQueryContaining(conn, "analytics.message_daily")
	if !found {
		t.Fatal("aucune requete analytics.message_daily emise")
	}
	if got := args[5].Value; got != int64(0) {
		t.Errorf("deliveredDelta = %v, voulu 0 (message.failed n'incremente jamais delivered)", got)
	}
	if got := args[6].Value; got != int64(1) {
		t.Errorf("failedDelta = %v, voulu 1", got)
	}
	if got := args[8].Value; got != int64(0) {
		t.Errorf("delivery_latency_ms_sum = %v, voulu 0 (pas de latence sur un echec)", got)
	}
}

// TestHandleMessageFailed_neverTouchesWalletTables est LE garde-fou explicite
// anti-double-remboursement demande par le PM : quel que soit le scenario
// (ici le cas le plus a risque -- message avec cost > 0), AUCUNE requete
// emise par ce worker ne doit jamais mentionner un objet du schema wallet.
// Le remboursement reste l'exclusivite de src/core-processor (Decision C).
func TestHandleMessageFailed_neverTouchesWalletTables(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: messageRowFixtureWithCost(int64(500), testMessageCreatedAt)},
			{rows: fakeRowsOfCols([]string{"score", "sample_size"})},
			{rows: fakeRowsOfCols([]string{"campaign_id"})},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
			{result: fakeExecResult{rows: 1}},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	if err := handleMessageFailed(context.Background(), svc, body); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue : %v", err)
	}

	if len(conn.queries) == 0 {
		t.Fatal("aucune requete emise, le test ne verifierait rien")
	}
	for i, q := range conn.queries {
		if strings.Contains(strings.ToLower(q), "wallet.") {
			t.Fatalf("requete #%d touche le schema wallet (INTERDIT, Decision C -- remboursement exclusif a core-processor) : %q", i, q)
		}
	}
}

// TestHandleMessageFailed_messageIntrouvable_permanentRejection verifie le
// rejet permanent quand messaging.messages ne connait pas l'id.
func TestHandleMessageFailed_messageIntrouvable_permanentRejection(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id", "recipient", "channel", "cost", "created_at"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	err := handleMessageFailed(context.Background(), svc, body)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageFailed() sur message introuvable = %v, voulu une permanentError", err)
	}
}

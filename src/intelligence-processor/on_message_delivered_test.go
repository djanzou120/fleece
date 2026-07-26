package intelligenceprocessor

import (
	"context"
	"testing"
)

// on_message_delivered_test.go — tests specifiques a handleMessageDelivered
// (parsing/validation, puis delegation a processDeliveryOutcome deja couvert
// en detail par delivery_outcome_test.go).

func TestHandleMessageDelivered_payloadIllisible(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageDelivered(context.Background(), svc, []byte("not json"))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageDelivered() sur JSON illisible = %v, voulu une permanentError", err)
	}
}

func TestHandleMessageDelivered_messageIDVide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageDelivered(context.Background(), svc, []byte(`{}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageDelivered() sur message_id vide = %v, voulu une permanentError", err)
	}
}

func TestHandleMessageDelivered_messageIDInvalide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	err := handleMessageDelivered(context.Background(), svc, []byte(`{"message_id":"pas-un-uuid"}`))
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageDelivered() sur message_id invalide = %v, voulu une permanentError", err)
	}
}

// TestHandleMessageDelivered_nominal_incrementsDeliveredNeverSent verifie la
// repartition stricte des compteurs pour ce handler precis (sent JAMAIS
// touche, delivered incremente).
func TestHandleMessageDelivered_nominal_incrementsDeliveredNeverSent(t *testing.T) {
	conn := newContactFixtureConn(testMessageCreatedAt)
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `","external_id":"42","provider_id":"telegram-bot"}`)
	if err := handleMessageDelivered(context.Background(), svc, body); err != nil {
		t.Fatalf("handleMessageDelivered() erreur inattendue : %v", err)
	}

	_, args, found := findQueryContaining(conn, "analytics.message_daily")
	if !found {
		t.Fatal("aucune requete analytics.message_daily emise")
	}
	if got := args[4].Value; got != int64(0) {
		t.Errorf("sentDelta = %v, voulu 0 (message.delivered n'incremente jamais sent)", got)
	}
	if got := args[5].Value; got != int64(1) {
		t.Errorf("deliveredDelta = %v, voulu 1", got)
	}
	if got := args[8].Value.(int64); got < 0 {
		t.Errorf("delivery_latency_ms_sum = %v, jamais negatif", got)
	}
}

// TestHandleMessageDelivered_messageIntrouvable_permanentRejection verifie le
// rejet permanent quand messaging.messages ne connait pas l'id.
func TestHandleMessageDelivered_messageIntrouvable_permanentRejection(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id", "recipient", "channel", "cost", "created_at"})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	body := []byte(`{"message_id":"` + testMessageID + `"}`)
	err := handleMessageDelivered(context.Background(), svc, body)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("handleMessageDelivered() sur message introuvable = %v, voulu une permanentError", err)
	}
}

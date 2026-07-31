package coreprocessor

// qa_dm43_fanout_robustness_test.go — ajouté par la QA (D-M43, critère 15) :
// couvre un trou de couverture identifié lors de la revue — aucun test
// n'exerçait le cas où loadSubscribedWebhookEndpoints (la toute PREMIÈRE
// requête du fan-out) échoue avec une erreur technique DB, à l'échelle du
// HANDLER complet (handleMessageDelivered/handleMessageFailed), pas seulement
// de fanOutWebhookEvent isolément. La règle 4/critère 15 du rapport de tâche
// exige qu'un échec de fan-out ne fasse JAMAIS échouer le traitement du statut
// ni provoquer de requeue AMQP : ce test le prouve de bout en bout (le handler
// retourne nil malgré la panne DB survenue APRÈS le commit de la transition).
import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
)

// TestHandleMessageDelivered_fanOutEndpointLoadDBError_stillReturnsNil
// simule une panne DB sur la requête de chargement des endpoints abonnés
// (2e requête, après la transition de statut RETURNING workspace_id) :
// handleMessageDelivered doit tout de même retourner nil (pas de requeue),
// exactement comme documenté par fanOutWebhookEvent (void, erreur absorbée en
// interne).
func TestHandleMessageDelivered_fanOutEndpointLoadDBError_stillReturnsNil(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id"}, []driver.Value{testWorkspaceID})}, // transition réelle
		{err: errors.New("connexion perdue (chargement endpoints)")},                      // panne DB sur le fan-out
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := handleMessageDelivered(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageDelivered() erreur inattendue malgré une panne DB isolée au fan-out webhook (ne doit JAMAIS provoquer de requeue) : %v", err)
	}
	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requêtes = %d, voulu 2 (transition + tentative de fan-out, même en échec)", len(conn.queries))
	}
}

// TestHandleMessageFailed_fanOutEndpointLoadDBError_stillReturnsNil est le
// symétrique pour handleMessageFailed, sur le chemin cost NULL (le plus
// fréquent aujourd'hui, Telegram gratuit) : le remboursement (absent ici) et
// la transition de statut sont déjà commités quand le fan-out échoue.
func TestHandleMessageFailed_fanOutEndpointLoadDBError_stillReturnsNil(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id", "cost"},
			[]driver.Value{testWorkspaceID, nil},
		)},
		{err: errors.New("connexion perdue (chargement endpoints)")},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := handleMessageFailed(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue malgré une panne DB isolée au fan-out webhook : %v", err)
	}
	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requêtes = %d, voulu 2 (transition + tentative de fan-out, même en échec)", len(conn.queries))
	}
}

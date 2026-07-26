package coreprocessor

import (
	"context"
	"errors"
	"strings"
	"testing"

	golog "fleece/src/go/log"
)

func testLogger() *golog.Logger { return golog.Init("warn", "text") }

// TestHandleMessageDelivered_nominal couvre le chemin nominal : l'UPDATE
// affecte 1 ligne (transition reelle) -> succes, pas d'erreur.
func TestHandleMessageDelivered_nominal(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 1}}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", ExternalID: "42", ProviderID: "telegram-bot"}
	if err := handleMessageDelivered(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageDelivered() erreur inattendue : %v", err)
	}

	if !strings.Contains(conn.lastQuery(), "UPDATE messaging.messages") {
		t.Fatalf("requete SQL inattendue (table) : %q", conn.lastQuery())
	}
	if !strings.Contains(conn.lastQuery(), "status = 'delivered'") {
		t.Fatalf("requete SQL inattendue (statut cible) : %q", conn.lastQuery())
	}
	// B1/Q3 (Phase 3) : la garde SQL est le miroir exact de messageTransitions
	// (src/api/message.go) — "sent" est le SEUL etat depuis lequel "delivered"
	// est une transition autorisee (draft/pending -> delivered est INTERDIT).
	if !strings.Contains(conn.lastQuery(), "status = 'sent'") {
		t.Fatalf("garde d'idempotence absente de la requete (attendu status = 'sent', miroir de Message.Transition()) : %q", conn.lastQuery())
	}
	if len(conn.lastArgs()) != 1 || conn.lastArgs()[0].Value != evt.MessageID {
		t.Fatalf("arguments inattendus : %+v", conn.lastArgs())
	}
}

// TestHandleMessageDelivered_doubleTraitement verifie l'idempotence : 0 ligne
// affectee (deja delivered/failed/rejected, ou introuvable) -> succes, pas
// d'erreur (pas de requeue, pas de crash).
func TestHandleMessageDelivered_doubleTraitement(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 0}}}}
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
	conn := &fakeConn{execSteps: []execStep{{err: wantErr}}}
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

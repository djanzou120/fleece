package coreprocessor

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

// ============================================================
// fonctions pures : parseMessageEvent, resolveHandler, uuidLike
// ============================================================

func TestParseMessageEvent(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    messageEvent
		wantErr bool
	}{
		{
			name: "evenement complet (contrat D-M21)",
			body: `{"message_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","external_id":"42","provider_id":"telegram-bot","status":"delivered","source":"telegram"}`,
			want: messageEvent{
				MessageID:  "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
				ExternalID: "42",
				ProviderID: "telegram-bot",
				Status:     "delivered",
				Source:     "telegram",
			},
		},
		{
			name: "champs optionnels absents",
			body: `{"message_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8"}`,
			want: messageEvent{MessageID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
		},
		{
			name:    "JSON illisible",
			body:    `not json`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseMessageEvent([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseMessageEvent(%q) attendu en erreur", tc.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMessageEvent(%q) erreur inattendue : %v", tc.body, err)
			}
			if got != tc.want {
				t.Errorf("parseMessageEvent(%q) = %+v, voulu %+v", tc.body, got, tc.want)
			}
		})
	}
}

func TestResolveHandler(t *testing.T) {
	if _, ok := resolveHandler("message.delivered"); !ok {
		t.Error(`resolveHandler("message.delivered") devrait exister`)
	}
	if _, ok := resolveHandler("message.failed"); !ok {
		t.Error(`resolveHandler("message.failed") devrait exister`)
	}
	if _, ok := resolveHandler("message.unknown"); ok {
		t.Error(`resolveHandler("message.unknown") ne devrait pas exister`)
	}
}

// TestBoundRoutingKeys verifie que BoundRoutingKeys() (D-M29, Phase 3) est la
// PROJECTION EXACTE, triee, des cles de handlersByRoutingKey — SOURCE UNIQUE
// utilisee par le composition root pour lier la queue "core-processor" a
// l'exchange "fleece" : ce test echouerait si un routing key etait ajoute/
// retire de handlersByRoutingKey sans que la topologie declaree au demarrage
// ne suive.
func TestBoundRoutingKeys(t *testing.T) {
	got := BoundRoutingKeys()
	want := []string{"message.delivered", "message.failed"}
	if len(got) != len(want) {
		t.Fatalf("BoundRoutingKeys() = %v, voulu %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BoundRoutingKeys()[%d] = %q, voulu %q", i, got[i], want[i])
		}
	}
	// Coherence avec handlersByRoutingKey : chaque cle retournee doit
	// resoudre un handler reel (et reciproquement).
	if len(got) != len(handlersByRoutingKey) {
		t.Errorf("BoundRoutingKeys() renvoie %d cles, handlersByRoutingKey en a %d", len(got), len(handlersByRoutingKey))
	}
	for _, k := range got {
		if _, ok := handlersByRoutingKey[k]; !ok {
			t.Errorf("BoundRoutingKeys() contient %q, absent de handlersByRoutingKey", k)
		}
	}
}

func TestUUIDLike(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c8", true},
		{"550E8400-E29B-41D4-A716-446655440000", true},
		{"", false},
		{"42", false},
		{"not-a-uuid", false},
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c", false}, // trop court
	}
	for _, tc := range cases {
		if got := uuidLike.MatchString(tc.in); got != tc.want {
			t.Errorf("uuidLike.MatchString(%q) = %v, voulu %v", tc.in, got, tc.want)
		}
	}
}

// ============================================================
// dispatch — sans I/O AMQP reel (amqp091.Delivery est un struct de valeurs)
// ============================================================

func TestDispatch_payloadIllisible(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	delivery := amqp091.Delivery{RoutingKey: "message.delivered", Body: []byte("not json")}

	err := svc.dispatch(context.Background(), delivery)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("dispatch() sur JSON illisible = %v, voulu une permanentError", err)
	}
}

func TestDispatch_messageIDVide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	body, _ := json.Marshal(messageEvent{})
	delivery := amqp091.Delivery{RoutingKey: "message.delivered", Body: body}

	err := svc.dispatch(context.Background(), delivery)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("dispatch() sur message_id vide = %v, voulu une permanentError", err)
	}
}

func TestDispatch_messageIDInvalide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	body, _ := json.Marshal(messageEvent{MessageID: "pas-un-uuid"})
	delivery := amqp091.Delivery{RoutingKey: "message.delivered", Body: body}

	err := svc.dispatch(context.Background(), delivery)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("dispatch() sur message_id invalide = %v, voulu une permanentError", err)
	}
}

func TestDispatch_routingKeyInconnue(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{RoutingKey: "message.unknown", Body: body}

	err := svc.dispatch(context.Background(), delivery)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("dispatch() sur routing key inconnue = %v, voulu une permanentError", err)
	}
}

func TestDispatch_nominal_delivered(t *testing.T) {
	// E2 (D-M43) : handleMessageDelivered emet desormais un UPDATE ... RETURNING
	// workspace_id (Query), plus un ExecStep. workspace_id="" fait que le
	// fan-out webhook (D-M43) s'arrete immediatement (voir fanOutWebhookEvent,
	// garde "workspace_id vide"), sans requete supplementaire — ce test ne
	// s'interesse qu'au dispatch lui-meme.
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id"}, []driver.Value{""})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{RoutingKey: "message.delivered", Body: body}

	if err := svc.dispatch(context.Background(), delivery); err != nil {
		t.Fatalf("dispatch() nominal message.delivered erreur inattendue : %v", err)
	}
}

// ============================================================
// fakeAcknowledger — capture Ack/Nack sans AMQP reel
// ============================================================

type fakeAcknowledger struct {
	acked   []uint64
	nacked  []uint64
	requeue []bool
}

func (f *fakeAcknowledger) Ack(tag uint64, multiple bool) error {
	f.acked = append(f.acked, tag)
	return nil
}
func (f *fakeAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error {
	f.nacked = append(f.nacked, tag)
	f.requeue = append(f.requeue, requeue)
	return nil
}
func (f *fakeAcknowledger) Reject(tag uint64, requeue bool) error { return nil }

var _ amqp091.Acknowledger = (*fakeAcknowledger)(nil)

// ============================================================
// processDelivery — Ack/Nack selon le resultat de dispatch
// ============================================================

func TestProcessDelivery_success_acks(t *testing.T) {
	// E2 (D-M43) : handleMessageDelivered emet un UPDATE ... RETURNING
	// workspace_id (Query, pas Exec) — workspace_id="" court-circuite le
	// fan-out webhook sans requete supplementaire (voir fanOutWebhookEvent).
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id"}, []driver.Value{""})},
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	ack := &fakeAcknowledger{}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.delivered", Body: body, DeliveryTag: 1}

	svc.processDelivery(context.Background(), delivery)

	if len(ack.acked) != 1 {
		t.Fatalf("Ack appele %d fois, voulu 1", len(ack.acked))
	}
	if len(ack.nacked) != 0 {
		t.Fatalf("Nack appele %d fois, voulu 0", len(ack.nacked))
	}
}

func TestProcessDelivery_permanentError_nacksWithoutRequeue(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	ack := &fakeAcknowledger{}
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.delivered", Body: []byte("not json"), DeliveryTag: 2}

	svc.processDelivery(context.Background(), delivery)

	if len(ack.acked) != 0 {
		t.Fatalf("Ack appele %d fois, voulu 0 (erreur permanente)", len(ack.acked))
	}
	if len(ack.nacked) != 1 || ack.requeue[0] != false {
		t.Fatalf("Nack = %+v requeue=%+v, voulu 1 Nack sans requeue", ack.nacked, ack.requeue)
	}
}

func TestProcessDelivery_dbError_nacksWithRequeue(t *testing.T) {
	// E2 (D-M43) : handleMessageDelivered emet desormais un Query (UPDATE ...
	// RETURNING workspace_id), pas un Exec — la panne technique doit donc
	// etre programmee sur querySteps.
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	ack := &fakeAcknowledger{}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.delivered", Body: body, DeliveryTag: 3}

	svc.processDelivery(context.Background(), delivery)

	if len(ack.acked) != 0 {
		t.Fatalf("Ack appele %d fois, voulu 0 (erreur DB transitoire)", len(ack.acked))
	}
	if len(ack.nacked) != 1 || ack.requeue[0] != true {
		t.Fatalf("Nack = %+v requeue=%+v, voulu 1 Nack AVEC requeue", ack.nacked, ack.requeue)
	}
}

func TestProcessDelivery_doubleTraitement_uneSeuleTransition(t *testing.T) {
	// 2 deliveries identiques (redelivery AMQP) : la garde d'idempotence SQL
	// (simulee ici par le fakeConn qui retourne rows=1 puis rows=0) fait que
	// la 2e delivery est quand meme Ack (succes no-op), jamais requeue ni
	// traitee comme une erreur.
	// E2 (D-M43) : handleMessageDelivered emet un Query (UPDATE ... RETURNING
	// workspace_id), pas un Exec — workspace_id="" sur la 1ere delivery
	// court-circuite le fan-out webhook (garde "workspace_id vide") pour que
	// ce test ne s'interesse qu'a l'idempotence du dispatch AMQP lui-meme.
	conn := &fakeConn{querySteps: []queryStep{
		{rows: fakeRowsOfCols([]string{"workspace_id"}, []driver.Value{""})}, // 1ere delivery : transition reelle
		{rows: fakeRowsOfCols([]string{"workspace_id"})},                     // 2e delivery (redelivery) : 0 ligne -> idempotence
	}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	ack := &fakeAcknowledger{}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})

	svc.processDelivery(context.Background(), amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.delivered", Body: body, DeliveryTag: 1})
	svc.processDelivery(context.Background(), amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.delivered", Body: body, DeliveryTag: 2})

	if len(ack.acked) != 2 {
		t.Fatalf("Ack appele %d fois, voulu 2 (les deux deliveries sont acquittees, la 2e en no-op idempotent)", len(ack.acked))
	}
	if len(ack.nacked) != 0 {
		t.Fatalf("Nack appele %d fois, voulu 0", len(ack.nacked))
	}
	if conn.queryPos != 2 {
		t.Fatalf("nombre d'UPDATE...RETURNING executes = %d, voulu 2 (une transition + un no-op)", conn.queryPos)
	}
}

// TestProcessDelivery_panicRecovered verifie que la panique d'un handler ne
// se propage jamais hors de processDelivery (D-M18) : le message est Nack
// sans requeue, le worker continue de tourner.
func TestProcessDelivery_panicRecovered(t *testing.T) {
	// Injecte temporairement un handler paniquant pour la duree du test.
	original := handlersByRoutingKey["message.delivered"]
	handlersByRoutingKey["message.delivered"] = func(ctx context.Context, s *Service, evt messageEvent) error {
		panic("boom")
	}
	defer func() { handlersByRoutingKey["message.delivered"] = original }()

	svc := &Service{Logger: testLogger()}
	ack := &fakeAcknowledger{}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.delivered", Body: body, DeliveryTag: 9}

	// Ne doit PAS paniquer hors de processDelivery.
	svc.processDelivery(context.Background(), delivery)

	if len(ack.acked) != 0 {
		t.Fatalf("Ack appele %d fois, voulu 0 (panique recuperee)", len(ack.acked))
	}
	if len(ack.nacked) != 1 || ack.requeue[0] != false {
		t.Fatalf("Nack = %+v requeue=%+v, voulu 1 Nack sans requeue (panique)", ack.nacked, ack.requeue)
	}
}

// ============================================================
// Consume — arret sur annulation de contexte
// ============================================================

// TestConsume_stopsOnContextCancel verifie que Consume() retourne (sans
// erreur) quand ctx est annule, meme si aucune delivery n'arrive jamais —
// s.AMQP.Consume() ne peut pas etre appele sans connexion reelle, ce test
// exerce donc directement la boucle de selection via un canal de delivery
// construit a la main (meme forme que celui retourne par goamqp.Conn.Consume).
func TestConsume_stopsOnContextCancel(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	ctx, cancel := context.WithCancel(context.Background())

	deliveries := make(chan amqp091.Delivery)
	done := make(chan error, 1)
	go func() {
		done <- svc.consumeLoop(ctx, deliveries)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("consumeLoop() erreur inattendue a l'arret : %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consumeLoop() n'a pas retourne apres annulation du contexte")
	}
}

// TestConsume_stopsOnChannelClosed verifie que consumeLoop() retourne
// (sans erreur) quand le canal de delivery est ferme (ex. perte de connexion
// AMQP cote broker), sans attendre l'annulation du contexte.
func TestConsume_stopsOnChannelClosed(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	deliveries := make(chan amqp091.Delivery)
	done := make(chan error, 1)
	go func() {
		done <- svc.consumeLoop(context.Background(), deliveries)
	}()

	close(deliveries)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("consumeLoop() erreur inattendue a la fermeture du canal : %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consumeLoop() n'a pas retourne apres fermeture du canal")
	}
}

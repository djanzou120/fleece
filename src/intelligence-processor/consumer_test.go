package intelligenceprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	golog "fleece/src/go/log"

	amqp091 "github.com/rabbitmq/amqp091-go"
)

func testLogger() *golog.Logger { return golog.Init("warn", "text") }

// ============================================================
// parseMessageEvent / validateMessageID / uuidLike / resolveHandler —
// fonctions pures.
// ============================================================

func TestParseMessageEvent(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    messageEvent
		wantErr bool
	}{
		{
			name: "message.sent (contrat src/api/messages_send.go)",
			body: `{"message_id":"6ba7b810-9dad-11d1-80b4-00c04fd430c8","workspace_id":"550e8400-e29b-41d4-a716-446655440000","recipient":"+237699112233","provider_id":"telegram-bot","status":"sent"}`,
			want: messageEvent{
				MessageID:   "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
				WorkspaceID: "550e8400-e29b-41d4-a716-446655440000",
				Recipient:   "+237699112233",
				ProviderID:  "telegram-bot",
				Status:      "sent",
			},
		},
		{
			name: "message.delivered/failed (contrat D-M21)",
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
		{name: "JSON illisible", body: `not json`, wantErr: true},
		{name: "JSON illisible (tableau au lieu d'objet)", body: `[1,2,3]`, wantErr: true},
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

func TestValidateMessageID(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"UUID valide", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", false},
		{"UUID valide majuscules", "550E8400-E29B-41D4-A716-446655440000", false},
		{"vide", "", true},
		{"non-UUID", "42", true},
		{"forme presque valide (trop court)", "6ba7b810-9dad-11d1-80b4-00c04fd430c", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMessageID(tc.id)
			if tc.wantErr && err == nil {
				t.Errorf("validateMessageID(%q) attendu en erreur", tc.id)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateMessageID(%q) erreur inattendue : %v", tc.id, err)
			}
		})
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
		{"6ba7b810-9dad-11d1-80b4-00c04fd430c", false},
	}
	for _, tc := range cases {
		if got := uuidLike.MatchString(tc.in); got != tc.want {
			t.Errorf("uuidLike.MatchString(%q) = %v, voulu %v", tc.in, got, tc.want)
		}
	}
}

func TestResolveHandler(t *testing.T) {
	for _, key := range []string{"message.sent", "message.delivered", "message.failed", "campaign.run"} {
		if _, ok := resolveHandler(key); !ok {
			t.Errorf("resolveHandler(%q) devrait exister", key)
		}
	}
	if _, ok := resolveHandler("message.unknown"); ok {
		t.Error(`resolveHandler("message.unknown") ne devrait pas exister`)
	}
	if _, ok := resolveHandler(""); ok {
		t.Error(`resolveHandler("") ne devrait pas exister`)
	}
}

// TestBoundRoutingKeys verifie que BoundRoutingKeys() (D-M29, Phase 3) est la
// PROJECTION EXACTE, triee, des cles de handlersByRoutingKey — SOURCE UNIQUE
// utilisee par le composition root pour lier la queue "intelligence-processor"
// a l'exchange "fleece".
func TestBoundRoutingKeys(t *testing.T) {
	got := BoundRoutingKeys()
	want := []string{"campaign.run", "message.delivered", "message.failed", "message.sent"}
	if len(got) != len(want) {
		t.Fatalf("BoundRoutingKeys() = %v, voulu %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BoundRoutingKeys()[%d] = %q, voulu %q", i, got[i], want[i])
		}
	}
	if len(got) != len(handlersByRoutingKey) {
		t.Errorf("BoundRoutingKeys() renvoie %d cles, handlersByRoutingKey en a %d", len(got), len(handlersByRoutingKey))
	}
	for _, k := range got {
		if _, ok := handlersByRoutingKey[k]; !ok {
			t.Errorf("BoundRoutingKeys() contient %q, absent de handlersByRoutingKey", k)
		}
	}
}

// ============================================================
// dispatch — sans I/O AMQP reel.
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
	delivery := amqp091.Delivery{RoutingKey: "message.sent", Body: body}

	err := svc.dispatch(context.Background(), delivery)
	if err == nil || !isPermanentError(err) {
		t.Fatalf("dispatch() sur message_id vide = %v, voulu une permanentError", err)
	}
}

func TestDispatch_messageIDInvalide(t *testing.T) {
	svc := &Service{Logger: testLogger()}
	body, _ := json.Marshal(messageEvent{MessageID: "pas-un-uuid"})
	delivery := amqp091.Delivery{RoutingKey: "message.failed", Body: body}

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

func TestDispatch_nominal_messageDelivered(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: messageRowFixture()},
			{rows: fakeRowsOfCols([]string{"score", "sample_size"})}, // pas de score existant
			{rows: fakeRowsOfCols([]string{"campaign_id"})},          // pas de correlation campagne
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // upsert channel score
			{result: fakeExecResult{rows: 1}}, // upsert contacts
			{result: fakeExecResult{rows: 1}}, // upsert analytics
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{RoutingKey: "message.delivered", Body: body}

	if err := svc.dispatch(context.Background(), delivery); err != nil {
		t.Fatalf("dispatch() nominal message.delivered erreur inattendue : %v", err)
	}
}

// ============================================================
// fakeAcknowledger — capture Ack/Nack sans AMQP reel.
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

// signalingAcknowledger notifie l'acquittement (Ack) via un channel bufferise
// — utilise UNIQUEMENT par les tests qui exercent consumeLoop depuis une
// goroutine separee (TestConsumeLoop_processesDeliveryThenStops), pour
// eviter toute lecture non synchronisee d'un champ partage depuis la
// goroutine de test pendant que la goroutine du worker y ecrit (data race).
type signalingAcknowledger struct {
	acked chan uint64
}

func (s *signalingAcknowledger) Ack(tag uint64, multiple bool) error {
	s.acked <- tag
	return nil
}
func (s *signalingAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error { return nil }
func (s *signalingAcknowledger) Reject(tag uint64, requeue bool) error              { return nil }

var _ amqp091.Acknowledger = (*signalingAcknowledger)(nil)

// ============================================================
// processDelivery — Ack/Nack selon le resultat de dispatch, recuperation de
// panique (D-M18).
// ============================================================

func TestProcessDelivery_success_acks(t *testing.T) {
	// Succes reel via message.sent (message trouve, un seul UPSERT analytics).
	conn := &fakeConn{
		querySteps: []queryStep{{rows: messageRowFixture()}},
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	ack := &fakeAcknowledger{}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.sent", Body: body, DeliveryTag: 1}

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
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.sent", Body: []byte("not json"), DeliveryTag: 2}

	svc.processDelivery(context.Background(), delivery)

	if len(ack.acked) != 0 {
		t.Fatalf("Ack appele %d fois, voulu 0 (erreur permanente)", len(ack.acked))
	}
	if len(ack.nacked) != 1 || ack.requeue[0] != false {
		t.Fatalf("Nack = %+v requeue=%+v, voulu 1 Nack sans requeue", ack.nacked, ack.requeue)
	}
}

func TestProcessDelivery_dbError_nacksWithRequeue(t *testing.T) {
	conn := &fakeConn{querySteps: []queryStep{{err: errors.New("connexion perdue")}}}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	ack := &fakeAcknowledger{}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.sent", Body: body, DeliveryTag: 3}

	svc.processDelivery(context.Background(), delivery)

	if len(ack.acked) != 0 {
		t.Fatalf("Ack appele %d fois, voulu 0 (erreur DB transitoire)", len(ack.acked))
	}
	if len(ack.nacked) != 1 || ack.requeue[0] != true {
		t.Fatalf("Nack = %+v requeue=%+v, voulu 1 Nack AVEC requeue", ack.nacked, ack.requeue)
	}
}

// TestProcessDelivery_panicRecovered verifie que la panique d'un handler ne se
// propage jamais hors de processDelivery (D-M18) : le message est Nack sans
// requeue, le worker continue de tourner.
func TestProcessDelivery_panicRecovered(t *testing.T) {
	original := handlersByRoutingKey["message.sent"]
	handlersByRoutingKey["message.sent"] = func(ctx context.Context, s *Service, body []byte) error {
		panic("boom")
	}
	defer func() { handlersByRoutingKey["message.sent"] = original }()

	svc := &Service{Logger: testLogger()}
	ack := &fakeAcknowledger{}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	delivery := amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.sent", Body: body, DeliveryTag: 9}

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
// Consume / consumeLoop — arret sur annulation de contexte / canal ferme.
// ============================================================

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

// TestConsumeLoop_processesDeliveryThenStops verifie que consumeLoop()
// traite bien une delivery recue avant de s'arreter sur annulation ulterieure
// du contexte (exerce processDelivery depuis la boucle reelle, pas seulement
// directement).
func TestConsumeLoop_processesDeliveryThenStops(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: messageRowFixture()}},
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}
	ctx, cancel := context.WithCancel(context.Background())

	deliveries := make(chan amqp091.Delivery, 1)
	// signalingAcknowledger notifie acked via un channel (plutot qu'un champ
	// de slice partage relu sans synchronisation depuis la goroutine de
	// test) : evite toute race data entre la goroutine consumeLoop (qui
	// appelle Ack) et la goroutine de test (qui attend l'acquittement).
	acked := make(chan uint64, 1)
	ack := &signalingAcknowledger{acked: acked}
	body, _ := json.Marshal(messageEvent{MessageID: testMessageID})
	deliveries <- amqp091.Delivery{Acknowledger: ack, RoutingKey: "message.sent", Body: body, DeliveryTag: 1}

	done := make(chan error, 1)
	go func() { done <- svc.consumeLoop(ctx, deliveries) }()

	// Attend que la delivery soit traitee (Ack observe), puis arrete la boucle.
	select {
	case <-acked:
	case <-time.After(2 * time.Second):
		t.Fatal("la delivery n'a jamais ete acquittee")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("consumeLoop() erreur inattendue : %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consumeLoop() n'a pas retourne apres annulation du contexte")
	}
}

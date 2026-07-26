package intelligenceprocessor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// ============================================================
// deliveryLatencyMs — fonction pure, bornee a >= 0 (y compris derive
// d'horloge : now < createdAt).
// ============================================================

func TestDeliveryLatencyMs(t *testing.T) {
	base := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		createdAt time.Time
		now       time.Time
		want      int64
	}{
		{"latence nominale de 2500ms", base, base.Add(2500 * time.Millisecond), 2500},
		{"latence nulle (meme instant)", base, base, 0},
		{"derive d'horloge : now avant created_at -> bornee a 0", base, base.Add(-time.Hour), 0},
		{"grande latence (plusieurs heures, ex. DLR Telegram tardif)", base, base.Add(6 * time.Hour), (6 * time.Hour).Milliseconds()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deliveryLatencyMs(tc.createdAt, tc.now)
			if got != tc.want {
				t.Errorf("deliveryLatencyMs(%v, %v) = %d, voulu %d", tc.createdAt, tc.now, got, tc.want)
			}
			if got < 0 {
				t.Errorf("deliveryLatencyMs(%v, %v) = %d, jamais negatif", tc.createdAt, tc.now, got)
			}
		})
	}
}

// ============================================================
// upsertMessageDaily — chemin DB via faux driver (hors transaction, *gosql.DB
// directement, comme utilise par on_message_sent.go).
// ============================================================

func TestUpsertMessageDaily_sqlShapeAndDayFormat(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 1}}}}
	db := newFakeGosqlDB(t, conn)

	day := time.Date(2024, time.March, 3, 23, 59, 0, 0, time.UTC)
	err := upsertMessageDaily(context.Background(), db, day, testWorkspaceID, "CM", "sms", 1, 0, 0, 10, 0)
	if err != nil {
		t.Fatalf("upsertMessageDaily() erreur inattendue : %v", err)
	}

	if !strings.Contains(conn.lastQuery(), "analytics.message_daily") {
		t.Fatalf("table inattendue : %q", conn.lastQuery())
	}
	if !strings.Contains(conn.lastQuery(), "ON CONFLICT (day, workspace_id, country, channel) DO UPDATE") {
		t.Fatalf("clause ON CONFLICT absente : %q", conn.lastQuery())
	}
	args := conn.lastArgs()
	if len(args) != 9 {
		t.Fatalf("nombre d'arguments = %d, voulu 9", len(args))
	}
	if got := args[0].Value; got != "2024-03-03" {
		t.Errorf("day formate = %v, voulu %q (format YYYY-MM-DD, colonne `date`)", got, "2024-03-03")
	}
}

func TestUpsertMessageDaily_additiveDeltasNeverMixed(t *testing.T) {
	// Garde de non-regression documentaire : sent/cost et delivered/failed ne
	// sont jamais incrementes simultanement par le MEME appelant (repartition
	// stricte imposee par le PM, voir doc de tete de fichier). Ce test verifie
	// que la fonction elle-meme transmet fidelement les deltas fournis sans
	// les alterer (elle ne connait pas la regle de repartition, c'est aux
	// appelants de la respecter).
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 1}}}}
	db := newFakeGosqlDB(t, conn)

	if err := upsertMessageDaily(context.Background(), db, time.Now(), testWorkspaceID, "unknown", "sms", 0, 1, 0, 0, 4200); err != nil {
		t.Fatalf("upsertMessageDaily() erreur inattendue : %v", err)
	}
	args := conn.lastArgs()
	if args[4].Value != int64(0) { // sent
		t.Errorf("sentDelta = %v, voulu 0", args[4].Value)
	}
	if args[5].Value != int64(1) { // delivered
		t.Errorf("deliveredDelta = %v, voulu 1", args[5].Value)
	}
	if args[8].Value != int64(4200) { // delivery_latency_ms_sum
		t.Errorf("latencyDelta = %v, voulu 4200", args[8].Value)
	}
}

func TestUpsertMessageDaily_dbError(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{err: errors.New("connexion perdue")}}}
	db := newFakeGosqlDB(t, conn)

	if err := upsertMessageDaily(context.Background(), db, time.Now(), testWorkspaceID, "CM", "sms", 1, 0, 0, 0, 0); err == nil {
		t.Fatal("upsertMessageDaily() attendu en erreur sur panne DB")
	}
}

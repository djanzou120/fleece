package intelligenceprocessor

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

// ============================================================
// Laplace / clamp — fonctions pures dupliquees de src/api/scorer.go /
// src/api/routing.go (voir doc de tete de fichier contact_score.go). Cas
// canoniques alignes sur src/api/scorer_test.go.
// ============================================================

func TestLaplace(t *testing.T) {
	cases := []struct {
		name    string
		success int
		failure int
		want    int
	}{
		{"prior neutre (0,0) = 50", 0, 0, 50},
		{"(1,0) = 66", 1, 0, 66},
		{"(10,0) = 91", 10, 0, 91},
		{"(0,10) = 8", 0, 10, 8},
		{"(5,5) = 50", 5, 5, 50},
		{"entrees negatives bornees a 0 : (-1,0) = 50", -1, 0, 50},
		{"entrees negatives bornees a 0 : (0,-5) = 50", 0, -5, 50},
		{"grand nombre (100,0) = 99", 100, 0, 99},
		{"grand nombre (0,100) = 0", 0, 100, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Laplace(tc.success, tc.failure); got != tc.want {
				t.Errorf("Laplace(%d, %d) = %d, voulu %d", tc.success, tc.failure, got, tc.want)
			}
			if got := Laplace(tc.success, tc.failure); got < 0 || got > 100 {
				t.Errorf("Laplace(%d, %d) = %d hors bornes [0,100]", tc.success, tc.failure, got)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	cases := []struct {
		v, lo, hi, want int
	}{
		{-5, 0, 100, 0},
		{150, 0, 100, 100},
		{50, 0, 100, 50},
		{0, 0, 100, 0},
		{100, 0, 100, 100},
	}
	for _, tc := range cases {
		if got := clamp(tc.v, tc.lo, tc.hi); got != tc.want {
			t.Errorf("clamp(%d, %d, %d) = %d, voulu %d", tc.v, tc.lo, tc.hi, got, tc.want)
		}
	}
}

// ============================================================
// reconstructSuccess — dette D26 (precision +-1pt), fonction pure.
// ============================================================

func TestReconstructSuccess(t *testing.T) {
	cases := []struct {
		name       string
		score      int
		sampleSize int
		want       int
	}{
		{"sampleSize=0 -> toujours 0 quel que soit le score", 0, 0, 0},
		{"sampleSize=0 -> toujours 0 (score eleve)", 50, 0, 0},
		{"score=100, sampleSize=5 -> clampe au plafond sampleSize", 100, 5, 5},
		{"score=0, sampleSize=5 -> clampe au plancher 0", 0, 5, 0},
		{"score=66, sampleSize=1 -> 0 (dette D26, precision +-1pt)", 66, 1, 0},
		{"score=91, sampleSize=10 -> 9 (dette D26, precision +-1pt)", 91, 10, 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reconstructSuccess(tc.score, tc.sampleSize); got != tc.want {
				t.Errorf("reconstructSuccess(%d, %d) = %d, voulu %d", tc.score, tc.sampleSize, got, tc.want)
			}
		})
	}
}

// ============================================================
// upsertChannelScore / upsertContactCounters — chemin DB via faux driver,
// testes directement (isoles de processDeliveryOutcome).
// ============================================================

const testPhone = "+237699112233"
const testChannel = "sms"

// TestUpsertChannelScore_newRow_success couvre le cas "premier contact avec ce
// canal" (SELECT -> 0 ligne) + succes : sample_size=1, INSERT AVEC
// last_success_at (colonne presente dans la requete).
func TestUpsertChannelScore_newRow_success(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: fakeRowsOfCols([]string{"score", "sample_size"})}}, // 0 ligne -> sql.ErrNoRows
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertChannelScore(context.Background(), tx, testPhone, testChannel, true, time.Now().UTC()); err != nil {
		t.Fatalf("upsertChannelScore() erreur inattendue : %v", err)
	}
	if !strings.Contains(conn.lastQuery(), "contact_intel.contact_channel_scores") {
		t.Fatalf("table inattendue : %q", conn.lastQuery())
	}
	if !strings.Contains(conn.lastQuery(), "last_success_at") {
		t.Fatalf("INSERT succes devrait inclure last_success_at : %q", conn.lastQuery())
	}
}

// TestUpsertChannelScore_newRow_failure couvre le meme cas mais un ECHEC :
// INSERT SANS colonne last_success_at.
func TestUpsertChannelScore_newRow_failure(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: fakeRowsOfCols([]string{"score", "sample_size"})}},
		execSteps:  []execStep{{result: fakeExecResult{rows: 1}}},
	}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertChannelScore(context.Background(), tx, testPhone, testChannel, false, time.Now().UTC()); err != nil {
		t.Fatalf("upsertChannelScore() erreur inattendue : %v", err)
	}
	if strings.Contains(conn.lastQuery(), "last_success_at") {
		t.Fatalf("INSERT echec ne devrait PAS inclure last_success_at : %q", conn.lastQuery())
	}
}

// TestUpsertChannelScore_existingRow couvre la reconstruction (score/sample_size
// deja presents) avant l'UPSERT.
func TestUpsertChannelScore_existingRow(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{{rows: fakeRowsOfCols([]string{"score", "sample_size"},
			[]driver.Value{int64(66), int64(1)},
		)}},
		execSteps: []execStep{{result: fakeExecResult{rows: 1}}},
	}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertChannelScore(context.Background(), tx, testPhone, testChannel, true, time.Now().UTC()); err != nil {
		t.Fatalf("upsertChannelScore() erreur inattendue : %v", err)
	}
	if conn.execPos != 1 {
		t.Fatalf("nombre d'UPSERT executes = %d, voulu 1", conn.execPos)
	}
}

// TestUpsertChannelScore_dbErrorOnSelect verifie qu'une panne technique sur le
// SELECT remonte une erreur "nue".
func TestUpsertChannelScore_dbErrorOnSelect(t *testing.T) {
	wantErr := errors.New("connexion perdue")
	conn := &fakeConn{querySteps: []queryStep{{err: wantErr}}}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertChannelScore(context.Background(), tx, testPhone, testChannel, true, time.Now().UTC()); err == nil {
		t.Fatal("upsertChannelScore() attendu en erreur")
	}
}

// TestUpsertContactCounters_success/failure verifient le choix de requete
// (branche success vs failure) sans lecture prealable (INSERT ... ON CONFLICT
// direct).
func TestUpsertContactCounters_success(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 1}}}}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertContactCounters(context.Background(), tx, testPhone, testChannel, true, time.Now().UTC()); err != nil {
		t.Fatalf("upsertContactCounters() erreur inattendue : %v", err)
	}
	if !strings.Contains(conn.lastQuery(), "contact_intel.contacts.success_count + 1") {
		t.Fatalf("requete succes inattendue : %q", conn.lastQuery())
	}
}

func TestUpsertContactCounters_failure(t *testing.T) {
	conn := &fakeConn{execSteps: []execStep{{result: fakeExecResult{rows: 1}}}}
	db := newFakeGosqlDB(t, conn)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertContactCounters(context.Background(), tx, testPhone, testChannel, false, time.Now().UTC()); err != nil {
		t.Fatalf("upsertContactCounters() erreur inattendue : %v", err)
	}
	if !strings.Contains(conn.lastQuery(), "failure_count = contact_intel.contacts.failure_count + 1") {
		t.Fatalf("requete echec inattendue : %q", conn.lastQuery())
	}
}

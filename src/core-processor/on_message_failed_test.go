package coreprocessor

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
)

const testMessageID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
const testWorkspaceID = "550e8400-e29b-41d4-a716-446655440000"

// ============================================================
// shouldRefundCost — fonction pure (D-M12/regle transverse n°9)
// ============================================================

func TestShouldRefundCost(t *testing.T) {
	cases := []struct {
		name string
		cost sql.NullInt64
		want bool
	}{
		{"cost NULL -> pas de remboursement", sql.NullInt64{Valid: false}, false},
		{"cost 0 (provider gratuit, ex. Telegram) -> pas de remboursement", sql.NullInt64{Valid: true, Int64: 0}, false},
		{"cost > 0 -> remboursement", sql.NullInt64{Valid: true, Int64: 500}, true},
		{"cost negatif (ne devrait jamais arriver) -> pas de remboursement", sql.NullInt64{Valid: true, Int64: -1}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRefundCost(tc.cost); got != tc.want {
				t.Errorf("shouldRefundCost(%+v) = %v, voulu %v", tc.cost, got, tc.want)
			}
		})
	}
}

// ============================================================
// handleMessageFailed — chemin DB reel via fakedriver_test.go
// ============================================================

// TestHandleMessageFailed_costNull couvre le cas majoritaire aujourd'hui
// (Telegram, gratuit) : cost NULL -> transition de statut appliquee, AUCUN
// remboursement (une seule requete Query dans la tx, pas de deuxieme UPDATE
// wallet), commit.
func TestHandleMessageFailed_costNull(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: fakeRowsOfCols([]string{"workspace_id", "cost"},
				[]driver.Value{testWorkspaceID, nil},
			)},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := handleMessageFailed(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue (cost NULL) : %v", err)
	}

	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (aucun remboursement attendu) : %+v", len(conn.queries), conn.queries)
	}
	if !strings.Contains(conn.queries[0], "UPDATE messaging.messages") ||
		!strings.Contains(conn.queries[0], "RETURNING workspace_id, cost") {
		t.Fatalf("requete SQL inattendue : %q", conn.queries[0])
	}
	if conn.commitCount != 1 {
		t.Fatalf("commitCount = %d, voulu 1", conn.commitCount)
	}
}

// TestHandleMessageFailed_costZero couvre le cas cost=0 (provider gratuit,
// distinct de NULL) : meme comportement que cost NULL, aucun remboursement.
func TestHandleMessageFailed_costZero(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: fakeRowsOfCols([]string{"workspace_id", "cost"},
				[]driver.Value{testWorkspaceID, int64(0)},
			)},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := handleMessageFailed(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue (cost=0) : %v", err)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (aucun remboursement attendu)", len(conn.queries))
	}
}

// TestHandleMessageFailed_costPositive_refund couvre le remboursement reel
// (cost > 0) : 3 requetes attendues dans l'ordre — UPDATE messages RETURNING,
// UPDATE wallets RETURNING, INSERT wallet_transactions — puis commit.
func TestHandleMessageFailed_costPositive_refund(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: fakeRowsOfCols([]string{"workspace_id", "cost"},
				[]driver.Value{testWorkspaceID, int64(500)},
			)},
			{rows: fakeRowsOf("balance", int64(1500))},
		},
		execSteps: []execStep{
			{result: fakeExecResult{rows: 1}}, // INSERT wallet_transactions
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := handleMessageFailed(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue (remboursement) : %v", err)
	}

	// c.queries/c.args capturent Query ET Exec dans l'ordre chronologique :
	// [0] UPDATE messaging.messages RETURNING (Query), [1] UPDATE wallet.wallets
	// RETURNING (Query), [2] INSERT wallet_transactions (Exec).
	if len(conn.queries) != 3 {
		t.Fatalf("nombre total de requetes = %d, voulu 3 (messages + wallets + ledger) : %+v", len(conn.queries), conn.queries)
	}
	if !strings.Contains(conn.queries[0], "UPDATE messaging.messages") {
		t.Fatalf("1ere requete inattendue : %q", conn.queries[0])
	}
	if !strings.Contains(conn.queries[1], "UPDATE wallet.wallets") ||
		!strings.Contains(conn.queries[1], "balance = balance + $1") ||
		!strings.Contains(conn.queries[1], "RETURNING balance") {
		t.Fatalf("2e requete inattendue (credit wallet) : %q", conn.queries[1])
	}
	if got := conn.args[1][0].Value; got != int64(500) {
		t.Errorf("montant credite = %v, voulu 500 (source de verite = cost relu en base, pas le payload)", got)
	}

	if conn.execPos != 1 {
		t.Fatalf("nombre d'Exec (ledger) execute = %d, voulu 1", conn.execPos)
	}
	if !strings.Contains(conn.queries[2], "INSERT INTO wallet.wallet_transactions") ||
		!strings.Contains(conn.queries[2], "'refund'") {
		t.Fatalf("requete ledger inattendue : %q", conn.queries[2])
	}
	if got := conn.args[2][2].Value; got != testMessageID {
		t.Errorf("message_id du ledger = %v, voulu %q", got, testMessageID)
	}
	if conn.commitCount != 1 {
		t.Fatalf("commitCount = %d, voulu 1", conn.commitCount)
	}
}

// TestHandleMessageFailed_doubleTraitement_alreadyTerminal couvre
// l'idempotence : l'UPDATE messages ne retourne aucune ligne (garde de statut
// terminal deja atteinte) -> AUCUN remboursement (meme si le message avait un
// cout > 0 lors du premier traitement), pas d'erreur, commit (rien a annuler).
func TestHandleMessageFailed_doubleTraitement_alreadyTerminal(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: fakeRowsOfCols([]string{"workspace_id", "cost"})}, // 0 ligne
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := handleMessageFailed(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue sur double-traitement : %v", err)
	}
	if len(conn.queries) != 1 {
		t.Fatalf("nombre de requetes = %d, voulu 1 (aucune tentative de remboursement) : %+v", len(conn.queries), conn.queries)
	}
	if conn.commitCount != 1 {
		t.Fatalf("commitCount = %d, voulu 1 (rien a annuler, commit propre)", conn.commitCount)
	}
}

// TestHandleMessageFailed_dbErrorOnStatusUpdate verifie qu'une panne technique
// sur l'UPDATE de statut remonte une erreur "nue" (requeue cote consumer.go).
func TestHandleMessageFailed_dbErrorOnStatusUpdate(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{err: errors.New("connexion perdue")},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	err := handleMessageFailed(context.Background(), svc, evt)
	if err == nil {
		t.Fatal("handleMessageFailed() attendu en erreur sur panne DB")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort (devrait etre requeue) : %v", err)
	}
	if conn.commitCount != 0 {
		t.Errorf("commitCount = %d, voulu 0 (tx doit etre abandonnee sur erreur)", conn.commitCount)
	}
}

// TestHandleMessageFailed_dbErrorOnWalletCredit verifie qu'une panne
// technique survenant PENDANT le remboursement (apres la transition de
// statut) remonte aussi une erreur "nue" : la transaction entiere sera
// rollback (rien n'est jamais commite partiellement), donc au prochain
// requeue la transition de statut sera retentee proprement (D-M12 : le
// faux driver ne simule pas un vrai rollback physique, mais verifie que
// Commit n'est jamais appele dans ce cas).
func TestHandleMessageFailed_dbErrorOnWalletCredit(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: fakeRowsOfCols([]string{"workspace_id", "cost"},
				[]driver.Value{testWorkspaceID, int64(500)},
			)},
			{err: errors.New("connexion perdue")},
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	err := handleMessageFailed(context.Background(), svc, evt)
	if err == nil {
		t.Fatal("handleMessageFailed() attendu en erreur sur panne DB (credit wallet)")
	}
	if isPermanentError(err) {
		t.Errorf("erreur DB classee permanente a tort (devrait etre requeue) : %v", err)
	}
	if conn.commitCount != 0 {
		t.Errorf("commitCount = %d, voulu 0 (aucun commit partiel)", conn.commitCount)
	}
}

// TestHandleMessageFailed_walletNotFound couvre l'anomalie "wallet
// introuvable" (message facture mais wallet absent) : log ERROR (non
// verifiable directement ici) mais PAS d'erreur remontee — la transition de
// statut deja appliquee dans la tx est commitee (comportement documente dans
// on_message_failed.go : requeuer ne creerait jamais le wallet manquant).
func TestHandleMessageFailed_walletNotFound(t *testing.T) {
	conn := &fakeConn{
		querySteps: []queryStep{
			{rows: fakeRowsOfCols([]string{"workspace_id", "cost"},
				[]driver.Value{testWorkspaceID, int64(500)},
			)},
			{rows: fakeRowsOf("balance")}, // 0 ligne -> sql.ErrNoRows
		},
	}
	svc := &Service{DB: newFakeGosqlDB(t, conn), Logger: testLogger()}

	evt := messageEvent{MessageID: testMessageID}
	if err := handleMessageFailed(context.Background(), svc, evt); err != nil {
		t.Fatalf("handleMessageFailed() erreur inattendue (wallet introuvable, anomalie geree) : %v", err)
	}
	if conn.commitCount != 1 {
		t.Fatalf("commitCount = %d, voulu 1 (transition de statut commitee malgre l'anomalie)", conn.commitCount)
	}
	// Aucune tentative d'INSERT wallet_transactions.
	if conn.execPos != 0 {
		t.Errorf("execPos = %d, voulu 0 (pas de ledger ecrit sans wallet)", conn.execPos)
	}
}

// TestHandleMessageFailed_beginError verifie qu'une panne technique a
// l'ouverture de la transaction remonte une erreur "nue" (requeue).
func TestHandleMessageFailed_beginError(t *testing.T) {
	// Ce cas ne peut pas etre simule avec fakeConn.Begin (qui ne retourne
	// jamais d'erreur) : documente comme non couvert (voir rapport final,
	// section "ce que les tests ne couvrent pas").
	t.Skip("fakeConn.Begin() ne simule pas d'echec — non couvert, voir rapport final")
}

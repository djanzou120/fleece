package api

// d_m36_wallet_debit_test.go — gardes du débit wallet à l'envoi (D-M36).
//
// Le défaut corrigé : `POST /messages` ne débitait rien, alors que
// core-processor CRÉDITE le wallet sur échec à partir de
// `messaging.messages.cost`. Chaque échec de message payant créditait donc un
// wallet jamais débité — de la monnaie apparaissait, et le ledger affichait des
// `refund` sans `debit` correspondant.
//
// Ces tests exercent le SQL réellement émis (faux driver, cf.
// qa_m019_dbpath_test.go) : c'est l'atomicité INSERT+débit et la présence de la
// ligne de ledger qui constituent l'invariant, pas le code HTTP.

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	golog "fleece/src/go/log"
	gosql "fleece/src/go/sql"
)

const (
	dm36Workspace = "550e8400-e29b-41d4-a716-446655440000"
	dm36MessageID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
)

func dm36Service(t *testing.T, conn *fakeConn) *Service {
	t.Helper()
	return &Service{DB: newFakeGosqlDB(t, conn), Logger: golog.Init("warn", "text")}
}

// ============================================================
// Pré-vol : refuser AVANT d'envoyer
// ============================================================

// TestAssertSufficientBalance_freeChannel_noQuery : un canal gratuit
// (Telegram, estimation 0) ne doit ni interroger le wallet ni pouvoir être
// refusé. Sans ce court-circuit, tout envoi Telegram exigerait un wallet
// approvisionné pour un coût nul.
func TestAssertSufficientBalance_freeChannel_noQuery(t *testing.T) {
	conn := &fakeConn{}
	svc := dm36Service(t, conn)

	if err := svc.assertSufficientBalance(context.Background(), dm36Workspace, 0); err != nil {
		t.Fatalf("erreur inattendue pour un canal gratuit : %v", err)
	}
	if len(conn.queries) != 0 {
		t.Errorf("le wallet a ete interroge pour un cout nul : %+v", conn.queries)
	}
}

func TestAssertSufficientBalance_enough_ok(t *testing.T) {
	conn := &fakeConn{queryFn: func(q string, a []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"balance"}, data: [][]driver.Value{{int64(1000)}}}, nil
	}}
	svc := dm36Service(t, conn)

	if err := svc.assertSufficientBalance(context.Background(), dm36Workspace, 25); err != nil {
		t.Fatalf("erreur inattendue avec un solde suffisant : %v", err)
	}
	// Le pré-vol est une LECTURE : il ne doit poser aucun verrou (pas de
	// FOR UPDATE), sans quoi il sérialiserait tous les envois d'un workspace
	// pendant toute la durée de l'appel provider qui suit.
	if strings.Contains(strings.ToUpper(conn.queries[0]), "FOR UPDATE") {
		t.Errorf("le pre-vol pose un verrou (FOR UPDATE) alors qu'il precede l'appel provider : %q", conn.queries[0])
	}
}

func TestAssertSufficientBalance_insufficient_isSentinel(t *testing.T) {
	conn := &fakeConn{queryFn: func(q string, a []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"balance"}, data: [][]driver.Value{{int64(10)}}}, nil
	}}
	svc := dm36Service(t, conn)

	err := svc.assertSufficientBalance(context.Background(), dm36Workspace, 25)
	if !errors.Is(err, errInsufficientBalance) {
		t.Fatalf("err = %v, voulu errInsufficientBalance (traduit en 402 par le handler)", err)
	}
}

func TestAssertSufficientBalance_noWallet_isSentinel(t *testing.T) {
	conn := &fakeConn{queryFn: func(q string, a []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"balance"}, data: [][]driver.Value{}}, nil
	}}
	svc := dm36Service(t, conn)

	err := svc.assertSufficientBalance(context.Background(), dm36Workspace, 25)
	if !errors.Is(err, errWalletNotFound) {
		t.Fatalf("err = %v, voulu errWalletNotFound", err)
	}
}

// ============================================================
// Débit : le miroir exact du remboursement
// ============================================================

// TestDebitWalletForMessage_freeMessage_noWrite : cost = 0 ne doit produire ni
// débit ni ligne de ledger (une ligne à 0 polluerait le journal comptable).
func TestDebitWalletForMessage_freeMessage_noWrite(t *testing.T) {
	conn := &fakeConn{}
	tx := dm36BeginTx(t, conn)

	if err := debitWalletForMessage(context.Background(), tx, dm36Workspace, dm36MessageID, 0); err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if len(conn.queries) != 0 {
		t.Errorf("ecritures emises pour un message gratuit : %+v", conn.queries)
	}
}

// TestDebitWalletForMessage_writesBalanceAndLedger est LE test de l'invariant
// comptable : le débit doit écrire les DEUX faces — le solde ET la ligne de
// ledger `kind='debit'` rattachée au message — exactement comme le
// remboursement de core-processor écrit `kind='refund'`. Sans la ligne de
// ledger, le journal afficherait des refunds sans debit correspondant, ce qui
// est le symptôme comptable du bug corrigé.
func TestDebitWalletForMessage_writesBalanceAndLedger(t *testing.T) {
	conn := &fakeConn{queryFn: func(q string, a []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"balance"}, data: [][]driver.Value{{int64(975)}}}, nil
	}}
	tx := dm36BeginTx(t, conn)

	if err := debitWalletForMessage(context.Background(), tx, dm36Workspace, dm36MessageID, 25); err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if len(conn.queries) != 2 {
		t.Fatalf("nombre de requetes = %d, voulu 2 (solde + ledger) : %+v", len(conn.queries), conn.queries)
	}

	// 1. Décrément RELATIF, jamais un SET d'une valeur lue : sous concurrence,
	// `balance = <valeur lue> - cost` perdrait les débits simultanés.
	if !strings.Contains(conn.queries[0], "balance = balance - $1") {
		t.Errorf("le solde n'est pas decremente relativement (perte d'ecritures concurrentes possible) : %q", conn.queries[0])
	}
	if !strings.Contains(conn.queries[0], "RETURNING balance") {
		t.Errorf("sans RETURNING, un wallet absent passerait inapercu : %q", conn.queries[0])
	}

	// 2. Ligne de ledger, miroir du refund.
	if !strings.Contains(conn.queries[1], "INSERT INTO wallet.wallet_transactions") {
		t.Fatalf("aucune ligne de ledger ecrite : %q", conn.queries[1])
	}
	if !strings.Contains(conn.queries[1], "'debit'") {
		t.Errorf("le ledger n'enregistre pas kind='debit' (miroir du 'refund' de core-processor) : %q", conn.queries[1])
	}
	args := conn.argsHistory[1]
	if got := args[2].Value; got != dm36MessageID {
		t.Errorf("ligne de ledger non rattachee au message : message_id = %v, voulu %q", got, dm36MessageID)
	}
}

// TestDebitWalletForMessage_noWallet_isSentinel : le message étant déjà parti,
// l'appelant doit pouvoir distinguer l'anomalie « wallet disparu » d'une panne
// technique, pour préserver la trace du message au lieu de tout annuler.
func TestDebitWalletForMessage_noWallet_isSentinel(t *testing.T) {
	conn := &fakeConn{queryFn: func(q string, a []driver.NamedValue) (driver.Rows, error) {
		return &fakeRows{columns: []string{"balance"}, data: [][]driver.Value{}}, nil
	}}
	tx := dm36BeginTx(t, conn)

	err := debitWalletForMessage(context.Background(), tx, dm36Workspace, dm36MessageID, 25)
	if !errors.Is(err, errWalletNotFound) {
		t.Fatalf("err = %v, voulu errWalletNotFound", err)
	}
}

// ============================================================
// Atomicité INSERT + débit
// ============================================================

// TestPersistSentMessage_insertAndDebitShareOneTransaction est l'invariant
// central de D-M36 : une ligne messaging.messages avec cost > 0 existe si et
// seulement si le débit a été committé. Les deux écritures doivent donc être
// encadrées par UN SEUL BEGIN/COMMIT.
func TestPersistSentMessage_insertAndDebitShareOneTransaction(t *testing.T) {
	conn := &fakeConn{
		txAware: true,
		queryFn: func(q string, a []driver.NamedValue) (driver.Rows, error) {
			return &fakeRows{columns: []string{"balance"}, data: [][]driver.Value{{int64(975)}}}, nil
		},
	}
	svc := dm36Service(t, conn)

	req := sendMessageRequest{WorkspaceID: dm36Workspace, Recipient: "+237690000000", Body: "hello"}
	res := ProviderResult{ExternalID: "ext-1", Status: "sent", Cost: 25}

	if err := svc.persistSentMessage(context.Background(), dm36MessageID, req, "sms-twilio", "sms", "sent", res); err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}

	joined := strings.Join(conn.queries, " || ")
	if !strings.Contains(joined, "INSERT INTO messaging.messages") {
		t.Fatalf("le message n'a pas ete insere : %q", joined)
	}
	if !strings.Contains(joined, "balance = balance - $1") {
		t.Fatalf("D-M36 : aucun debit emis — le remboursement crediterait un wallet jamais debite : %q", joined)
	}
	if !strings.Contains(joined, "INSERT INTO wallet.wallet_transactions") {
		t.Fatalf("D-M36 : aucune ligne de ledger 'debit' : %q", joined)
	}
	if conn.beginCount != 1 {
		t.Errorf("nombre de BEGIN = %d, voulu exactement 1 (INSERT et debit doivent partager LA MEME transaction)", conn.beginCount)
	}
	if conn.commitCount != 1 {
		t.Errorf("nombre de COMMIT = %d, voulu 1", conn.commitCount)
	}
}

// TestPersistSentMessage_freeMessage_insertOnly : un message gratuit est
// inséré sans qu'aucune écriture wallet ne soit émise.
func TestPersistSentMessage_freeMessage_insertOnly(t *testing.T) {
	conn := &fakeConn{txAware: true}
	svc := dm36Service(t, conn)

	req := sendMessageRequest{WorkspaceID: dm36Workspace, Recipient: "@user", Body: "hello"}
	res := ProviderResult{ExternalID: "tg-1", Status: "sent", Cost: 0}

	if err := svc.persistSentMessage(context.Background(), dm36MessageID, req, "telegram-bot", "telegram", "sent", res); err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}

	joined := strings.Join(conn.queries, " || ")
	if !strings.Contains(joined, "INSERT INTO messaging.messages") {
		t.Fatalf("le message n'a pas ete insere : %q", joined)
	}
	if strings.Contains(joined, "wallet.") {
		t.Errorf("ecriture wallet emise pour un message gratuit : %q", joined)
	}
}

// TestInsertMessage_bothPathsShareQueryAndArgs verrouille la factorisation :
// le chemin transactionnel et le chemin direct doivent émettre EXACTEMENT la
// même requête et les mêmes arguments. Une colonne ajoutée d'un seul côté
// produirait des messages différemment formés selon qu'ils sont payants ou non.
func TestInsertMessage_bothPathsShareQueryAndArgs(t *testing.T) {
	if !strings.Contains(insertMessageQuery, "INSERT INTO messaging.messages") {
		t.Fatalf("requete partagee inattendue : %q", insertMessageQuery)
	}
	a := insertMessageArgs("id", "ws", "to", "body", "prov", "sms", "sent", "ext", 25)
	if len(a) != 10 {
		t.Fatalf("nombre d'arguments = %d, voulu 10 (colonnes de insertMessageQuery)", len(a))
	}
	// cost = 0 doit devenir NULL, pas 0 (index partiels de la migration 0018).
	b := insertMessageArgs("id", "ws", "to", "body", "prov", "telegram", "sent", "", 0)
	if v, ok := b[8].(interface{ Value() (driver.Value, error) }); ok {
		got, _ := v.Value()
		if got != nil {
			t.Errorf("cost = 0 doit etre insere en NULL, obtenu %v", got)
		}
	}
}

// dm36BeginTx ouvre une transaction sur le faux driver.
func dm36BeginTx(t *testing.T, conn *fakeConn) *gosql.Tx {
	t.Helper()
	conn.txAware = true
	tx, err := newFakeGosqlDB(t, conn).Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() erreur inattendue : %v", err)
	}
	return tx
}

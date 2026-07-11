package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleece/src/wallet/internal/domain"
)

// mockListTxnRepo est un mock de output.TransactionRepository dedie aux tests
// de ListTransactions. Il est distinct de mockTxnRepo (debit_test.go) afin de
// pouvoir simuler une erreur sur ListByWorkspace sans toucher les mocks existants.
type mockListTxnRepo struct {
	txns    []*domain.WalletTransaction
	listErr error
}

func (m *mockListTxnRepo) Append(_ context.Context, _ *domain.WalletTransaction) error {
	return nil
}

func (m *mockListTxnRepo) ListByWorkspace(_ context.Context, _ string) ([]*domain.WalletTransaction, error) {
	return m.txns, m.listErr
}

// TestListTransactions_NonEmpty verifie que le use case retourne la slice fournie
// par le repo dans l'ordre recu (tri descendant delegu e au repo/DB).
func TestListTransactions_NonEmpty(t *testing.T) {
	now := time.Now().UTC()
	txns := []*domain.WalletTransaction{
		{ID: 2, WorkspaceID: "ws-1", Kind: domain.KindDebit, Amount: 100, CreatedAt: now},
		{ID: 1, WorkspaceID: "ws-1", Kind: domain.KindCredit, Amount: 200, CreatedAt: now.Add(-time.Hour)},
	}
	repo := &mockListTxnRepo{txns: txns}
	uc := ListTransactions{Txns: repo}

	result, err := uc.Execute(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("attendu 2 transactions, obtenu %d", len(result))
	}
	// L'ordre retourne par le repo doit etre preserve (le plus recent en premier).
	if result[0].ID != 2 {
		t.Errorf("attendu ID=2 en premier (plus recent), obtenu %d", result[0].ID)
	}
	if result[1].ID != 1 {
		t.Errorf("attendu ID=1 en second, obtenu %d", result[1].ID)
	}
	if result[0].Kind != domain.KindDebit {
		t.Errorf("attendu kind=%s, obtenu %s", domain.KindDebit, result[0].Kind)
	}
}

// TestListTransactions_Empty verifie que le use case retourne une slice vide
// (non nil) quand le workspace n'a aucune transaction.
func TestListTransactions_Empty(t *testing.T) {
	repo := &mockListTxnRepo{txns: nil}
	uc := ListTransactions{Txns: repo}

	result, err := uc.Execute(context.Background(), "ws-empty")
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}
	if result == nil {
		t.Error("attendu slice non nil pour workspace sans transaction, obtenu nil")
	}
	if len(result) != 0 {
		t.Errorf("attendu slice vide, obtenu %d elements", len(result))
	}
}

// TestListTransactions_RepoError verifie que l'erreur du repository est propagee
// correctement (errors.Is doit retrouver la sentinelle d'origine).
func TestListTransactions_RepoError(t *testing.T) {
	sentinel := errors.New("base de donnees indisponible")
	repo := &mockListTxnRepo{listErr: sentinel}
	uc := ListTransactions{Txns: repo}

	_, err := uc.Execute(context.Background(), "ws-1")
	if err == nil {
		t.Fatal("attendu une erreur, obtenu nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("attendu l'erreur sentinelle via errors.Is, obtenu %v", err)
	}
}

package usecases

import (
	"context"
	"errors"
	"testing"

	"fleece/src/wallet/internal/domain"
)

// --- Mocks des ports ---

type mockWalletRepo struct {
	wallet   *domain.Wallet
	getErr   error
	saved    []*domain.Wallet
	saveErr  error
}

func (m *mockWalletRepo) Get(_ context.Context, _ string) (*domain.Wallet, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.wallet == nil {
		return nil, domain.ErrWalletNotFound
	}
	// Retourner une copie pour eviter les mutations entre appels.
	w := *m.wallet
	return &w, nil
}

func (m *mockWalletRepo) Save(_ context.Context, w *domain.Wallet) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, w)
	// Mettre a jour le wallet interne pour simuler la persistance.
	wCopy := *w
	m.wallet = &wCopy
	return nil
}

type mockTxnRepo struct {
	appended []*domain.WalletTransaction
	appendErr error
}

func (m *mockTxnRepo) Append(_ context.Context, t *domain.WalletTransaction) error {
	if m.appendErr != nil {
		return m.appendErr
	}
	m.appended = append(m.appended, t)
	return nil
}

func (m *mockTxnRepo) ListByWorkspace(_ context.Context, _ string) ([]*domain.WalletTransaction, error) {
	return m.appended, nil
}

type mockPublisher struct {
	events []string
}

func (m *mockPublisher) Publish(_ context.Context, event string, _ *domain.WalletTransaction) error {
	m.events = append(m.events, event)
	return nil
}

// --- Tests DebitWallet ---

func TestDebitWallet_Success(t *testing.T) {
	wallet := &domain.Wallet{
		WorkspaceID: "ws-1",
		Balance:     domain.Money{Amount: 1000, Currency: "XAF"},
	}
	walletRepo := &mockWalletRepo{wallet: wallet}
	txnRepo := &mockTxnRepo{}
	pub := &mockPublisher{}

	uc := DebitWallet{
		Wallets:   walletRepo,
		Txns:      txnRepo,
		Publisher: pub,
	}

	txn, err := uc.Execute(context.Background(), "ws-1", 300, "msg-1")
	if err != nil {
		t.Fatalf("Execute inattendu: %v", err)
	}

	// La transaction doit etre retournee avec les bonnes valeurs.
	if txn == nil {
		t.Fatal("transaction attendue, obtenu nil")
	}
	if txn.Kind != domain.KindDebit {
		t.Errorf("kind attendu %s, obtenu %s", domain.KindDebit, txn.Kind)
	}
	if txn.Amount != 300 {
		t.Errorf("amount attendu 300, obtenu %d", txn.Amount)
	}
	if txn.MessageID != "msg-1" {
		t.Errorf("message_id attendu msg-1, obtenu %s", txn.MessageID)
	}

	// Le solde doit etre decremente.
	if walletRepo.wallet.Balance.Amount != 700 {
		t.Errorf("solde attendu 700, obtenu %d", walletRepo.wallet.Balance.Amount)
	}

	// La transaction doit etre enregistree dans le ledger.
	if len(txnRepo.appended) != 1 {
		t.Errorf("attendu 1 transaction, obtenu %d", len(txnRepo.appended))
	}

	// L'evenement doit etre publie.
	if len(pub.events) != 1 || pub.events[0] != "wallet.debited" {
		t.Errorf("attendu evenement wallet.debited, obtenu %v", pub.events)
	}
}

func TestDebitWallet_InsufficientFunds(t *testing.T) {
	wallet := &domain.Wallet{
		WorkspaceID: "ws-1",
		Balance:     domain.Money{Amount: 100, Currency: "XAF"},
	}
	walletRepo := &mockWalletRepo{wallet: wallet}
	txnRepo := &mockTxnRepo{}
	pub := &mockPublisher{}

	uc := DebitWallet{
		Wallets:   walletRepo,
		Txns:      txnRepo,
		Publisher: pub,
	}

	_, err := uc.Execute(context.Background(), "ws-1", 500, "msg-2")
	if err == nil {
		t.Fatal("attendu ErrInsufficientFunds")
	}
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Errorf("attendu ErrInsufficientFunds, obtenu %v", err)
	}

	// Aucune transaction ne doit avoir ete enregistree.
	if len(txnRepo.appended) != 0 {
		t.Errorf("aucune transaction attendue, obtenu %d", len(txnRepo.appended))
	}

	// Aucun evenement ne doit avoir ete publie.
	if len(pub.events) != 0 {
		t.Errorf("aucun evenement attendu, obtenu %v", pub.events)
	}

	// Le solde ne doit pas avoir change.
	if walletRepo.wallet.Balance.Amount != 100 {
		t.Errorf("solde ne devrait pas avoir change, obtenu %d", walletRepo.wallet.Balance.Amount)
	}
}

func TestDebitWallet_WalletNotFound(t *testing.T) {
	walletRepo := &mockWalletRepo{
		getErr: domain.ErrWalletNotFound,
	}
	txnRepo := &mockTxnRepo{}
	pub := &mockPublisher{}

	uc := DebitWallet{
		Wallets:   walletRepo,
		Txns:      txnRepo,
		Publisher: pub,
	}

	_, err := uc.Execute(context.Background(), "ws-inexistant", 100, "msg-3")
	if err == nil {
		t.Fatal("attendu ErrWalletNotFound")
	}
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Errorf("attendu ErrWalletNotFound, obtenu %v", err)
	}

	// Aucune transaction.
	if len(txnRepo.appended) != 0 {
		t.Errorf("aucune transaction attendue, obtenu %d", len(txnRepo.appended))
	}
}

func TestDebitWallet_InvalidAmount(t *testing.T) {
	wallet := &domain.Wallet{
		WorkspaceID: "ws-1",
		Balance:     domain.Money{Amount: 1000, Currency: "XAF"},
	}
	walletRepo := &mockWalletRepo{wallet: wallet}
	txnRepo := &mockTxnRepo{}
	pub := &mockPublisher{}

	uc := DebitWallet{
		Wallets:   walletRepo,
		Txns:      txnRepo,
		Publisher: pub,
	}

	// Montant negatif : doit retourner ErrInvalidAmount.
	_, err := uc.Execute(context.Background(), "ws-1", -100, "msg-4")
	if err == nil {
		t.Fatal("attendu ErrInvalidAmount pour montant negatif")
	}
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("attendu ErrInvalidAmount, obtenu %v", err)
	}
}

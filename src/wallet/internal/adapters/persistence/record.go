// Package persistence implemente le port output.WalletRepository et
// output.TransactionRepository via *sql.DB.
// Couche 3 (Interface Adapters, driven).
package persistence

import (
	"database/sql"
	"time"

	"fleece/src/wallet/internal/domain"
)

// walletRecord est le modele de la table wallet.wallets.
// Il assure la separation entre la representation base et l'entite domaine.
type walletRecord struct {
	WorkspaceID string
	Balance     int64
	Currency    string
	UpdatedAt   time.Time
}

// toEntity convertit un walletRecord en entite domaine.
func (r walletRecord) toEntity() *domain.Wallet {
	return &domain.Wallet{
		WorkspaceID: r.WorkspaceID,
		Balance: domain.Money{
			Amount:   r.Balance,
			Currency: r.Currency,
		},
	}
}

// fromWallet convertit une entite domaine en walletRecord.
func fromWallet(w *domain.Wallet) walletRecord {
	return walletRecord{
		WorkspaceID: w.WorkspaceID,
		Balance:     w.Balance.Amount,
		Currency:    w.Balance.Currency,
	}
}

// transactionRecord est le modele de la table wallet.wallet_transactions.
type transactionRecord struct {
	ID          int64
	WorkspaceID string
	Kind        string
	Amount      int64
	MessageID   sql.NullString
	CreatedAt   time.Time
}

// toEntity convertit un transactionRecord en entite domaine.
func (r transactionRecord) toEntity() *domain.WalletTransaction {
	messageID := ""
	if r.MessageID.Valid {
		messageID = r.MessageID.String
	}
	return &domain.WalletTransaction{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Kind:        domain.TransactionKind(r.Kind),
		Amount:      r.Amount,
		MessageID:   messageID,
		CreatedAt:   r.CreatedAt,
	}
}

// fromTransaction convertit une entite domaine en transactionRecord.
func fromTransaction(t *domain.WalletTransaction) transactionRecord {
	var msgID sql.NullString
	if t.MessageID != "" {
		msgID = sql.NullString{String: t.MessageID, Valid: true}
	}
	return transactionRecord{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		Kind:        string(t.Kind),
		Amount:      t.Amount,
		MessageID:   msgID,
		CreatedAt:   t.CreatedAt,
	}
}

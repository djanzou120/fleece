package domain

import "time"

// TransactionKind est le type d'une transaction dans le ledger.
type TransactionKind string

const (
	// KindDebit represente un debit (consommation de credit).
	KindDebit TransactionKind = "debit"
	// KindCredit represente un credit (rechargement).
	KindCredit TransactionKind = "credit"
	// KindRefund represente un remboursement apres echec d'envoi.
	KindRefund TransactionKind = "refund"
)

// WalletTransaction est une entree append-only dans le ledger du wallet.
// Elle trace chaque mouvement de fonds avec son contexte.
type WalletTransaction struct {
	// ID est l'identifiant sequentiel de la transaction (bigserial cote Postgres).
	ID int64
	// WorkspaceID est le workspace concerne.
	WorkspaceID string
	// Kind est le type de la transaction : debit, credit ou refund.
	Kind TransactionKind
	// Amount est le montant en centimes de la transaction.
	Amount int64
	// MessageID est l'identifiant du message associe (peut etre vide pour credit).
	MessageID string
	// CreatedAt est l'horodatage de creation de la transaction.
	CreatedAt time.Time
}

// NewDebitTransaction cree une transaction de debit.
func NewDebitTransaction(workspaceID string, amount int64, messageID string) *WalletTransaction {
	return &WalletTransaction{
		WorkspaceID: workspaceID,
		Kind:        KindDebit,
		Amount:      amount,
		MessageID:   messageID,
		CreatedAt:   time.Now().UTC(),
	}
}

// NewCreditTransaction cree une transaction de credit.
func NewCreditTransaction(workspaceID string, amount int64) *WalletTransaction {
	return &WalletTransaction{
		WorkspaceID: workspaceID,
		Kind:        KindCredit,
		Amount:      amount,
		CreatedAt:   time.Now().UTC(),
	}
}

// NewRefundTransaction cree une transaction de remboursement.
func NewRefundTransaction(workspaceID string, amount int64, messageID string) *WalletTransaction {
	return &WalletTransaction{
		WorkspaceID: workspaceID,
		Kind:        KindRefund,
		Amount:      amount,
		MessageID:   messageID,
		CreatedAt:   time.Now().UTC(),
	}
}

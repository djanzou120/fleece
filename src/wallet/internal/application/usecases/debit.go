package usecases

import (
	"context"
	"fmt"

	"fleece/src/wallet/internal/application/ports/output"
	"fleece/src/wallet/internal/domain"
)

// DebitWallet orchestre le debit du wallet d'un workspace.
// Il ne depend que des ports (interfaces) : aucune dependance vers un framework.
type DebitWallet struct {
	Wallets   output.WalletRepository
	Txns      output.TransactionRepository
	Publisher output.EventPublisher
}

// Execute debite workspaceID de amount centimes. messageID lie la transaction au message.
//
// Flux : charge le wallet → applique Debit domaine → Save wallet → Append transaction → publie evenement.
// La publication est best-effort (erreur ignoree).
func (uc DebitWallet) Execute(ctx context.Context, workspaceID string, amount int64, messageID string) (*domain.WalletTransaction, error) {
	// 1. Charger le wallet (ErrWalletNotFound propagee).
	w, err := uc.Wallets.Get(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("debit: charger wallet: %w", err)
	}

	// 2. Construire le montant a debiter dans la devise du wallet.
	debitAmount, err := domain.NewMoney(amount, w.Balance.Currency)
	if err != nil {
		return nil, fmt.Errorf("debit: montant invalide: %w", err)
	}

	// 3. Appliquer la regle metier de debit (ErrInsufficientFunds propagee).
	if err := w.Debit(debitAmount); err != nil {
		return nil, fmt.Errorf("debit: %w", err)
	}

	// 4. Persister le nouveau solde.
	if err := uc.Wallets.Save(ctx, w); err != nil {
		return nil, fmt.Errorf("debit: sauvegarder wallet: %w", err)
	}

	// 5. Enregistrer la transaction dans le ledger.
	txn := domain.NewDebitTransaction(workspaceID, amount, messageID)
	if err := uc.Txns.Append(ctx, txn); err != nil {
		return nil, fmt.Errorf("debit: enregistrer transaction: %w", err)
	}

	// 6. Publier l'evenement (best-effort).
	_ = uc.Publisher.Publish(ctx, "wallet.debited", txn)

	return txn, nil
}

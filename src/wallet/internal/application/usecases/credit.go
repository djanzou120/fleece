package usecases

import (
	"context"
	"fmt"

	"fleece/src/wallet/internal/application/ports/output"
	"fleece/src/wallet/internal/domain"
)

// CreditWallet orchestre le credit (rechargement) du wallet d'un workspace.
// Si le wallet n'existe pas, il est cree a la volee.
type CreditWallet struct {
	Wallets   output.WalletRepository
	Txns      output.TransactionRepository
	Publisher output.EventPublisher
}

// Execute credite workspaceID de amount centimes dans la devise currency.
//
// Flux : charger ou creer le wallet → appliquer Credit domaine → Save wallet →
// Append transaction → publier evenement.
// La publication est best-effort (erreur ignoree).
func (uc CreditWallet) Execute(ctx context.Context, workspaceID string, amount int64, currency string) (*domain.WalletTransaction, error) {
	// 1. Charger ou creer le wallet.
	w, err := uc.Wallets.Get(ctx, workspaceID)
	if err != nil {
		if isWalletNotFound(err) {
			w = domain.NewWallet(workspaceID, currency)
		} else {
			return nil, fmt.Errorf("credit: charger wallet: %w", err)
		}
	}

	// 2. Construire le montant a crediter dans la devise du wallet.
	creditAmount, err := domain.NewMoney(amount, w.Balance.Currency)
	if err != nil {
		return nil, fmt.Errorf("credit: montant invalide: %w", err)
	}

	// 3. Appliquer la regle metier de credit.
	if err := w.Credit(creditAmount); err != nil {
		return nil, fmt.Errorf("credit: %w", err)
	}

	// 4. Persister le nouveau solde (upsert).
	if err := uc.Wallets.Save(ctx, w); err != nil {
		return nil, fmt.Errorf("credit: sauvegarder wallet: %w", err)
	}

	// 5. Enregistrer la transaction dans le ledger.
	txn := domain.NewCreditTransaction(workspaceID, amount)
	if err := uc.Txns.Append(ctx, txn); err != nil {
		return nil, fmt.Errorf("credit: enregistrer transaction: %w", err)
	}

	// 6. Publier l'evenement (best-effort).
	_ = uc.Publisher.Publish(ctx, "wallet.credited", txn)

	return txn, nil
}

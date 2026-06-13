package usecases

import (
	"context"
	"fmt"

	"fleece/src/wallet/internal/application/ports/output"
	"fleece/src/wallet/internal/domain"
)

// Refund orchestre le remboursement du wallet apres un echec d'envoi de message.
// Il re-credite le workspace du montant debite precedemment.
type Refund struct {
	Wallets   output.WalletRepository
	Txns      output.TransactionRepository
	Publisher output.EventPublisher
}

// Execute re-credite workspaceID de amount centimes. messageID identifie le message
// dont l'envoi a echoue.
//
// Flux : charger le wallet → appliquer Credit domaine → Save wallet →
// Append transaction (kind=refund) → publier evenement.
// La publication est best-effort (erreur ignoree).
func (uc Refund) Execute(ctx context.Context, workspaceID string, amount int64, messageID string) (*domain.WalletTransaction, error) {
	// 1. Charger le wallet (ErrWalletNotFound propagee).
	w, err := uc.Wallets.Get(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("refund: charger wallet: %w", err)
	}

	// 2. Construire le montant a re-crediter dans la devise du wallet.
	refundAmount, err := domain.NewMoney(amount, w.Balance.Currency)
	if err != nil {
		return nil, fmt.Errorf("refund: montant invalide: %w", err)
	}

	// 3. Appliquer la regle metier de credit.
	if err := w.Credit(refundAmount); err != nil {
		return nil, fmt.Errorf("refund: %w", err)
	}

	// 4. Persister le nouveau solde.
	if err := uc.Wallets.Save(ctx, w); err != nil {
		return nil, fmt.Errorf("refund: sauvegarder wallet: %w", err)
	}

	// 5. Enregistrer la transaction dans le ledger.
	txn := domain.NewRefundTransaction(workspaceID, amount, messageID)
	if err := uc.Txns.Append(ctx, txn); err != nil {
		return nil, fmt.Errorf("refund: enregistrer transaction: %w", err)
	}

	// 6. Publier l'evenement (best-effort).
	_ = uc.Publisher.Publish(ctx, "wallet.refunded", txn)

	return txn, nil
}

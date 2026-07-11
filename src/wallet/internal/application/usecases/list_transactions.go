package usecases

import (
	"context"
	"fmt"

	"fleece/src/wallet/internal/application/ports/output"
	"fleece/src/wallet/internal/domain"
)

// ListTransactions retourne toutes les transactions d'un workspace, dans l'ordre
// retourne par le repository (tri descendant par date, delegue au repo/DB).
// Retourne une slice vide (non nil) si aucune transaction n'existe.
// Ne depend que du port output.TransactionRepository ; zero dependance framework.
type ListTransactions struct {
	Txns output.TransactionRepository
}

// Execute retourne les transactions de workspaceID.
// La slice de retour est toujours non nil : elle est vide si le workspace
// n'a aucune transaction.
// En cas d'erreur du repository, l'erreur est propagee wrappee.
func (uc ListTransactions) Execute(ctx context.Context, workspaceID string) ([]*domain.WalletTransaction, error) {
	txns, err := uc.Txns.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list_transactions: %w", err)
	}
	if txns == nil {
		txns = []*domain.WalletTransaction{}
	}
	return txns, nil
}

package usecases

import (
	"context"
	"errors"
	"fmt"

	"fleece/src/wallet/internal/application/ports/output"
	"fleece/src/wallet/internal/domain"
)

// GetBalance retourne le wallet courant d'un workspace.
type GetBalance struct {
	Wallets output.WalletRepository
}

// Execute retourne le wallet de workspaceID.
// Retourne domain.ErrWalletNotFound (non wrappe) si le workspace n'a pas de wallet.
func (uc GetBalance) Execute(ctx context.Context, workspaceID string) (*domain.Wallet, error) {
	w, err := uc.Wallets.Get(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get_balance: %w", err)
	}
	return w, nil
}

// isWalletNotFound est un helper partage par les use cases pour tester si une erreur
// est (ou enveloppe) domain.ErrWalletNotFound.
func isWalletNotFound(err error) bool {
	return errors.Is(err, domain.ErrWalletNotFound)
}

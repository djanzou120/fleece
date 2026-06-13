// Package input — ports pilotants : interfaces exposees par le service (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package input

import (
	"context"

	"fleece/src/wallet/internal/domain"
)

// DebitWalletUseCase debite le wallet d'un workspace du montant donne.
// Utilise lors de l'envoi d'un message par le Messaging Service.
type DebitWalletUseCase interface {
	// Execute debite workspaceID de amount centimes dans la devise du wallet.
	// messageID lie la transaction au message correspondant.
	// Retourne domain.ErrWalletNotFound si le workspace n'a pas de wallet.
	// Retourne domain.ErrInsufficientFunds si le solde est insuffisant.
	Execute(ctx context.Context, workspaceID string, amount int64, messageID string) (*domain.WalletTransaction, error)
}

// CreditWalletUseCase credite le wallet d'un workspace (rechargement).
// Si le wallet n'existe pas, il est cree.
type CreditWalletUseCase interface {
	// Execute credite workspaceID de amount centimes dans la devise donnee.
	Execute(ctx context.Context, workspaceID string, amount int64, currency string) (*domain.WalletTransaction, error)
}

// RefundUseCase recedite un workspace apres l'echec d'un envoi de message.
type RefundUseCase interface {
	// Execute re-credite workspaceID de amount centimes.
	// messageID identifie le message dont l'envoi a echoue.
	Execute(ctx context.Context, workspaceID string, amount int64, messageID string) (*domain.WalletTransaction, error)
}

// GetBalanceUseCase retourne le wallet courant d'un workspace.
type GetBalanceUseCase interface {
	// Execute retourne le wallet de workspaceID.
	// Retourne domain.ErrWalletNotFound si le workspace n'a pas de wallet.
	Execute(ctx context.Context, workspaceID string) (*domain.Wallet, error)
}

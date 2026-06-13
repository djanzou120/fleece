// Package output — ports pilotes : interfaces requises par les use cases (couche 2).
//
// Clean Architecture : voir .ia/ARCHITECTURE.md.
package output

import (
	"context"

	"fleece/src/wallet/internal/domain"
)

// WalletRepository persiste les wallets. Implemente en couche 3 (Postgres,
// schema "wallet"). Chaque appel a Save est un upsert : il cree le wallet
// s'il n'existe pas, ou met a jour son solde.
type WalletRepository interface {
	// Get recupere le wallet d'un workspace.
	// Retourne domain.ErrWalletNotFound si le workspace n'a pas de wallet.
	Get(ctx context.Context, workspaceID string) (*domain.Wallet, error)

	// Save persiste (upsert) le wallet.
	Save(ctx context.Context, w *domain.Wallet) error
}

// TransactionRepository persiste le ledger append-only des transactions.
// Implemente en couche 3 (Postgres, schema "wallet").
type TransactionRepository interface {
	// Append insere une nouvelle transaction dans le ledger.
	Append(ctx context.Context, t *domain.WalletTransaction) error

	// ListByWorkspace retourne toutes les transactions d'un workspace,
	// ordonnees par date decroissante.
	ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.WalletTransaction, error)
}

// EventPublisher publie les evenements de domaine wallet (RabbitMQ en couche 3).
type EventPublisher interface {
	// Publish serialise et publie un evenement pour la transaction donnee.
	Publish(ctx context.Context, event string, t *domain.WalletTransaction) error
}

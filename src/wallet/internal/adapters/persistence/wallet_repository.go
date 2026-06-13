package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"fleece/src/wallet/internal/domain"
)

// WalletRepository implemente output.WalletRepository via *sql.DB.
// Il n'accede qu'au schema "wallet" (tables prefixees wallet.).
type WalletRepository struct {
	db *sql.DB
}

// NewWalletRepository cree un WalletRepository avec la connexion fournie.
func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

// Get recupere le wallet d'un workspace par son workspace_id.
// Retourne domain.ErrWalletNotFound si le workspace n'a pas de wallet.
func (r *WalletRepository) Get(ctx context.Context, workspaceID string) (*domain.Wallet, error) {
	const query = `
		SELECT workspace_id, balance, currency, updated_at
		  FROM wallet.wallets
		 WHERE workspace_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, workspaceID)
	var rec walletRecord
	err := row.Scan(
		&rec.WorkspaceID,
		&rec.Balance,
		&rec.Currency,
		&rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("persistence: Get wallet %s: %w", workspaceID, domain.ErrWalletNotFound)
		}
		return nil, fmt.Errorf("persistence: Get wallet %s: %w", workspaceID, err)
	}
	return rec.toEntity(), nil
}

// Save persiste (upsert) un wallet dans wallet.wallets.
// ON CONFLICT (workspace_id) DO UPDATE met a jour le solde, la devise et updated_at.
func (r *WalletRepository) Save(ctx context.Context, w *domain.Wallet) error {
	rec := fromWallet(w)
	const query = `
		INSERT INTO wallet.wallets (workspace_id, balance, currency, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (workspace_id) DO UPDATE
		  SET balance    = EXCLUDED.balance,
		      currency   = EXCLUDED.currency,
		      updated_at = now()
	`
	_, err := r.db.ExecContext(ctx, query,
		rec.WorkspaceID,
		rec.Balance,
		rec.Currency,
	)
	if err != nil {
		return fmt.Errorf("persistence: Save wallet %s: %w", w.WorkspaceID, err)
	}
	return nil
}

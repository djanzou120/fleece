package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"fleece/src/wallet/internal/domain"
)

// TransactionRepository implemente output.TransactionRepository via *sql.DB.
// Il n'accede qu'au schema "wallet" (tables prefixees wallet.).
type TransactionRepository struct {
	db *sql.DB
}

// NewTransactionRepository cree un TransactionRepository avec la connexion fournie.
func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Append insere une nouvelle transaction dans le ledger wallet.wallet_transactions.
// message_id est NULL si vide (sql.NullString).
func (r *TransactionRepository) Append(ctx context.Context, t *domain.WalletTransaction) error {
	rec := fromTransaction(t)
	const query = `
		INSERT INTO wallet.wallet_transactions (workspace_id, kind, amount, message_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	row := r.db.QueryRowContext(ctx, query,
		rec.WorkspaceID,
		rec.Kind,
		rec.Amount,
		rec.MessageID,
	)
	if err := row.Scan(&t.ID); err != nil {
		return fmt.Errorf("persistence: Append transaction workspace=%s kind=%s: %w", t.WorkspaceID, t.Kind, err)
	}
	return nil
}

// ListByWorkspace retourne toutes les transactions d'un workspace, ordonnees par
// date decroissante.
func (r *TransactionRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]*domain.WalletTransaction, error) {
	const query = `
		SELECT id, workspace_id, kind, amount, message_id, created_at
		  FROM wallet.wallet_transactions
		 WHERE workspace_id = $1
		 ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("persistence: ListByWorkspace %s: %w", workspaceID, err)
	}
	defer rows.Close()

	var txns []*domain.WalletTransaction
	for rows.Next() {
		var rec transactionRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.WorkspaceID,
			&rec.Kind,
			&rec.Amount,
			&rec.MessageID,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("persistence: ListByWorkspace scan: %w", err)
		}
		txns = append(txns, rec.toEntity())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("persistence: ListByWorkspace rows: %w", err)
	}
	return txns, nil
}

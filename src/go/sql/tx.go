package gosql

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// Tx is a wrapper around sqlx.Tx.
type Tx struct {
	tx *sqlx.Tx
}

// Select runs a query within the transaction and scans all resulting rows into dest.
func (t *Tx) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return t.tx.SelectContext(ctx, dest, query, args...)
}

// Get runs a query within the transaction and scans the single resulting row into dest.
func (t *Tx) Get(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return t.tx.GetContext(ctx, dest, query, args...)
}

// Exec executes a query within the transaction, discarding the sql.Result.
func (t *Tx) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := t.tx.ExecContext(ctx, query, args...)
	return err
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	return t.tx.Commit()
}

// Rollback aborts the transaction.
func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

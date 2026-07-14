package gosql

import (
	"context"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// DB is a wrapper around sqlx.DB.
type DB struct {
	db *sqlx.DB
}

// Open opens a new connection to a PostgreSQL database identified by dsn,
// verifies the connection with a Ping, and returns the DB or an error.
func Open(dsn string) (*DB, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

// Close closes the underlying database connection pool.
func (d *DB) Close() error {
	return d.db.Close()
}

// Select runs a query and scans all resulting rows into dest.
func (d *DB) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return d.db.SelectContext(ctx, dest, query, args...)
}

// Get runs a query and scans the single resulting row into dest.
func (d *DB) Get(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return d.db.GetContext(ctx, dest, query, args...)
}

// Exec executes a query that does not return rows, discarding the sql.Result.
func (d *DB) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := d.db.ExecContext(ctx, query, args...)
	return err
}

// Begin starts a new transaction and returns a Tx wrapping it.
func (d *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

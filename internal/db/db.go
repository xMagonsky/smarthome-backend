package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps pgxpool.Pool for database operations
type DB struct {
	pool *pgxpool.Pool
}

// NewDB creates a new DB connection pool
func NewDB(url string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return &DB{pool: pool}, nil
}

// Close closes the connection pool
func (d *DB) Close(ctx context.Context) error {
	d.pool.Close()
	return nil
}

// Pool returns the underlying pgxpool.Pool
func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// Add query methods as needed, e.g., Exec, Query

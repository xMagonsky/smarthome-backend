package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// DB wraps pgx.Conn for database operations
type DB struct {
	conn *pgx.Conn
}

// NewDB creates a new DB connection
func NewDB(url string) (*DB, error) {
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}

// Close closes the connection
func (d *DB) Close(ctx context.Context) error {
	return d.conn.Close(ctx)
}

// Add query methods as needed, e.g., Exec, Query

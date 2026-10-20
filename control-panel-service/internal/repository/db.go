// Package repository owns the control plane's Postgres connection and
// every hand-written pgx query the rest of the service needs. It is the
// only package that imports the pgx driver.
package repository

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open connects to Postgres via the pgx stdlib driver and verifies the
// connection with a ping.
func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// Package storetest gives repository, usecase, and handler tests a
// migrated, clean control Postgres connection. It skips the calling test
// if no test database is configured, so `go test ./...` stays green on
// machines without Postgres and exercises the real database in CI and
// local dev.
package storetest

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/migrations"
)

// Open returns a database connection migrated inside its own fresh
// Postgres schema, dropped on test cleanup. Each caller gets an isolated
// schema (rather than a shared one that gets truncated) because
// `go test ./...` runs the store and api packages' tests concurrently
// against the same Postgres instance, and a shared schema would race.
func Open(t *testing.T) *sql.DB {
	t.Helper()

	url := os.Getenv("DATABASE_URL_TEST")
	if url == "" {
		t.Skip("DATABASE_URL_TEST not set; skipping test that needs Postgres")
	}

	db, err := repository.Open(url)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	// A single connection keeps the search_path set below in effect for
	// every query this test issues; a pool would risk other connections
	// falling back to the public schema.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	schema := "test_" + randomHex(t)
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	if err := migrations.Run(db); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return db
}

func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	return hex.EncodeToString(b)
}

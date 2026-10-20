package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
)

func insertDatabase(t *testing.T, db *sql.DB, userID, name string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status)
		VALUES ($1, $2, 10, 'provisioning')
		RETURNING id
	`, userID, name).Scan(&id)
	if err != nil {
		t.Fatalf("insert database fixture: %v", err)
	}
	return id
}

func TestListAndGetDatabasesScopedToOwner(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	other, err := repository.CreateUser(ctx, db, "other@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	dbID := insertDatabase(t, db, owner.ID, "mine")
	insertDatabase(t, db, other.ID, "not-mine")

	rows, err := repository.ListDatabasesByUser(ctx, db, owner.ID)
	if err != nil {
		t.Fatalf("ListDatabasesByUser: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "mine" {
		t.Fatalf("ListDatabasesByUser = %+v, want exactly one row named %q", rows, "mine")
	}

	got, err := repository.DatabaseByIDForUser(ctx, db, dbID, owner.ID)
	if err != nil {
		t.Fatalf("DatabaseByIDForUser: %v", err)
	}
	if got.ID != dbID {
		t.Fatalf("DatabaseByIDForUser id = %q, want %q", got.ID, dbID)
	}

	_, err = repository.DatabaseByIDForUser(ctx, db, dbID, other.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DatabaseByIDForUser for non-owner error = %v, want sql.ErrNoRows", err)
	}
}

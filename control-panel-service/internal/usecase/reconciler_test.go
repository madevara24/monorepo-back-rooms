package usecase_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

func insertReconcilerFixtures(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO nodes (id, name, address, token_hash)
		VALUES ('node-reconcile', 'node-reconcile', 'https://node.example:8443', 'reconcile-token')
	`); err != nil {
		t.Fatalf("insert node fixture: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO databases (user_id, name, size_gb, status, node_id, slug, host, port)
		VALUES ($1, 'demo', 5, 'running', 'node-reconcile', 'demo-ab12', '10.0.0.5', 21000)
	`, owner.ID); err != nil {
		t.Fatalf("insert database fixture: %v", err)
	}
}

func TestReconcilerWritesRunningRoutes(t *testing.T) {
	db := storetest.Open(t)
	insertReconcilerFixtures(t, db)

	path := filepath.Join(t.TempDir(), "mbr-routes.map")
	r := &usecase.Reconciler{DB: db, Path: path}

	changed, err := r.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !changed {
		t.Fatalf("changed = false, want true for the first reconcile")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read routes file: %v", err)
	}
	want := "demo-ab12.db.mbr.madevara.com 10.0.0.5:21000;\n"
	if string(got) != want {
		t.Fatalf("routes file = %q, want %q", got, want)
	}

	changed, err = r.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if changed {
		t.Fatalf("changed = true, want false when nothing moved")
	}
}

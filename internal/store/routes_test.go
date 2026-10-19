package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/internal/store"
	"github.com/madevara24/monorepo-back-rooms/internal/storetest"
)

func insertRouteNode(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO nodes (id, name, address, token_hash)
		VALUES ($1, $1, 'https://node.example:8443', $1 || '-token')
	`, id); err != nil {
		t.Fatalf("insert node fixture: %v", err)
	}
}

func insertRouteDatabase(t *testing.T, db *sql.DB, userID, nodeID, status string, slug, host *string, port *int) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO databases (user_id, name, size_gb, status, node_id, slug, host, port)
		VALUES ($1, 'demo', 5, $2, $3, $4, $5, $6)
	`, userID, status, nodeID, slug, host, port); err != nil {
		t.Fatalf("insert database fixture: %v", err)
	}
}

func strp(s string) *string { return &s }
func intp(n int) *int       { return &n }

func TestListRunningRoutesOnlyRunningWithSlugHostPort(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := store.CreateUser(ctx, db, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	insertRouteNode(t, db, "node-routes")

	// A running database with everything set: should appear.
	insertRouteDatabase(t, db, owner.ID, "node-routes", "running", strp("demo-ab12"), strp("10.0.0.5"), intp(21000))
	// Provisioning: not running yet, no slug collision issue but excluded.
	insertRouteDatabase(t, db, owner.ID, "node-routes", "provisioning", strp("demo-cd34"), nil, nil)
	// Deleting: was running, now torn down; excluded.
	insertRouteDatabase(t, db, owner.ID, "node-routes", "deleting", strp("demo-ef56"), strp("10.0.0.6"), intp(21001))
	// Failed: never finished; excluded.
	insertRouteDatabase(t, db, owner.ID, "node-routes", "failed", strp("demo-gh78"), nil, nil)
	// Running but missing slug/host/port (shouldn't happen in practice, but
	// the query must not crash or include it).
	insertRouteDatabase(t, db, owner.ID, "node-routes", "running", nil, nil, nil)

	got, err := store.ListRunningRoutes(ctx, db)
	if err != nil {
		t.Fatalf("ListRunningRoutes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("routes = %d, want 1: %+v", len(got), got)
	}
	want := store.RunningRoute{Slug: "demo-ab12", Host: "10.0.0.5", Port: 21000}
	if got[0] != want {
		t.Fatalf("route = %+v, want %+v", got[0], want)
	}
}

func TestListRunningRoutesEmpty(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	got, err := store.ListRunningRoutes(ctx, db)
	if err != nil {
		t.Fatalf("ListRunningRoutes: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("routes = %d, want 0", len(got))
	}
}

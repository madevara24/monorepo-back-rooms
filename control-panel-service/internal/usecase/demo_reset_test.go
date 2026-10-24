package usecase_test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

type fakeDemoAgent struct {
	counts integration.ResetCounts
	err    error
	calls  int
}

func (f *fakeDemoAgent) ResetDemo(context.Context, string) (integration.ResetCounts, error) {
	f.calls++
	return f.counts, f.err
}

func (f *fakeDemoAgent) ListDatabases(context.Context, string) ([]integration.Database, error) {
	return nil, nil
}

// openResetPool opens a second pool on the same test schema. The reset
// holds one connection for the advisory lock, so it needs a pool with
// more than the single connection storetest gives the test body.
func openResetPool(t *testing.T, db *sql.DB) *sql.DB {
	t.Helper()
	var schema string
	if err := db.QueryRow(`SHOW search_path`).Scan(&schema); err != nil {
		t.Fatalf("read search_path: %v", err)
	}
	dsn := os.Getenv("DATABASE_URL_TEST") + "&options=" + url.QueryEscape("-c search_path="+schema)
	pool, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open reset pool: %v", err)
	}
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

func seedResetFixtures(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()

	if _, err := db.Exec(`
		INSERT INTO nodes (id, name, address, token_hash, last_heartbeat)
		VALUES ('node-1', 'node-1', 'https://agent.invalid:9443', 'token-hash', now())
	`); err != nil {
		t.Fatalf("insert node: %v", err)
	}
	if _, err := usecase.SeedAdmin(ctx, db, "admin@example.com", "password123"); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}
	user, err := repository.CreateUser(ctx, db, "visitor@example.com", "x")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := repository.CreateSession(ctx, db, "session-hash", user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	var databaseID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status, slug, node_id)
		VALUES ($1, 'demo', 1, 'running', 'demo-ab12', 'node-1')
		RETURNING id
	`, user.ID).Scan(&databaseID); err != nil {
		t.Fatalf("insert database: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jobs (kind, payload) VALUES ('provision', '{}')`); err != nil {
		t.Fatalf("insert job: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO database_events (database_id, to_status) VALUES ($1, 'running')`, databaseID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO database_metrics (database_id, active, idle, idle_in_transaction, max_connections, superuser_reserved_connections, memory_limit_bytes, collected_at)
		VALUES ($1, 0, 0, 0, 100, 3, 1000, now())
	`, databaseID); err != nil {
		t.Fatalf("insert metric: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO node_containers (node_id, database_id, status, port) VALUES ('node-1', $1, 'running', 21001)`, databaseID); err != nil {
		t.Fatalf("insert container: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO invoices (user_id, status, total_amount_idr, period_start, period_end)
		VALUES ($1, 'open', 10000, current_date, current_date)
	`, user.ID); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}
}

func TestDemoResetCleansBothPlanesAndKeepsSeed(t *testing.T) {
	db := storetest.Open(t)
	seedResetFixtures(t, db)
	pool := openResetPool(t, db)

	agent := &fakeDemoAgent{counts: integration.ResetCounts{Containers: 2, Volumes: 2, Orphans: 1}}
	reset := &usecase.DemoReset{
		DB:         pool,
		Agent:      agent,
		Reconciler: &usecase.Reconciler{DB: pool, Path: filepath.Join(t.TempDir(), "routes.map")},
	}
	result, err := reset.Reset(context.Background(), "manual")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if result.Counts != agent.counts {
		t.Fatalf("counts = %+v, want %+v", result.Counts, agent.counts)
	}
	if result.State.Status != "idle" || result.State.Maintenance || result.State.Generation != 1 {
		t.Fatalf("state = %+v, want idle, not in maintenance, generation 1", result.State)
	}
	if !result.State.NextResetAt.After(time.Now()) {
		t.Fatalf("next reset = %v, want a future time", result.State.NextResetAt)
	}

	for _, table := range []string{
		"sessions", "databases", "jobs", "database_events", "database_metrics",
		"node_containers", "invoices",
	} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s = %d, want 0", table, count)
		}
	}

	var visitorCount, seedCount, nodeCount, planCount int
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE NOT is_seed`).Scan(&visitorCount); err != nil {
		t.Fatalf("count visitors: %v", err)
	}
	if visitorCount != 0 {
		t.Fatalf("visitor users = %d, want 0", visitorCount)
	}
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE is_seed`).Scan(&seedCount); err != nil {
		t.Fatalf("count seed users: %v", err)
	}
	if seedCount != 1 {
		t.Fatalf("seed users = %d, want 1", seedCount)
	}
	if err := db.QueryRow(`SELECT count(*) FROM nodes`).Scan(&nodeCount); err != nil {
		t.Fatalf("count nodes: %v", err)
	}
	if nodeCount != 1 {
		t.Fatalf("nodes = %d, want 1", nodeCount)
	}
	if err := db.QueryRow(`SELECT count(*) FROM plans`).Scan(&planCount); err != nil {
		t.Fatalf("count plans: %v", err)
	}
	if planCount != 4 {
		t.Fatalf("plans = %d, want 4", planCount)
	}

	var purchasable bool
	if err := db.QueryRow(`SELECT purchasable FROM plans WHERE slug = 'sandbox'`).Scan(&purchasable); err != nil {
		t.Fatalf("read sandbox: %v", err)
	}
	if !purchasable {
		t.Fatalf("sandbox purchasable = false, want true")
	}
	if err := db.QueryRow(`SELECT purchasable FROM plans WHERE slug = 'production'`).Scan(&purchasable); err != nil {
		t.Fatalf("read production: %v", err)
	}
	if purchasable {
		t.Fatalf("production purchasable = true, want false")
	}

	second, err := reset.Reset(context.Background(), "manual")
	if err != nil {
		t.Fatalf("second Reset: %v", err)
	}
	if second.State.Generation != 2 {
		t.Fatalf("second generation = %d, want 2", second.State.Generation)
	}
}

func TestDemoResetAgentFailureLeavesRowsAndMaintenance(t *testing.T) {
	db := storetest.Open(t)
	seedResetFixtures(t, db)
	pool := openResetPool(t, db)

	reset := &usecase.DemoReset{
		DB:    pool,
		Agent: &fakeDemoAgent{err: errors.New("agent unreachable")},
	}
	if _, err := reset.Reset(context.Background(), "manual"); err == nil {
		t.Fatal("Reset succeeded, want an agent error")
	}

	state, err := repository.GetDemoState(context.Background(), db)
	if err != nil {
		t.Fatalf("GetDemoState: %v", err)
	}
	if !state.Maintenance || state.Status != "failed" {
		t.Fatalf("state = %+v, want failed and in maintenance", state)
	}
	var visitors int
	if err := db.QueryRow(`SELECT count(*) FROM users WHERE NOT is_seed`).Scan(&visitors); err != nil {
		t.Fatalf("count visitors: %v", err)
	}
	if visitors != 1 {
		t.Fatalf("visitor users = %d, want 1 after a failed reset", visitors)
	}
}

func TestDemoResetRefusesWithoutANode(t *testing.T) {
	db := storetest.Open(t)
	pool := openResetPool(t, db)
	reset := &usecase.DemoReset{DB: pool, Agent: &fakeDemoAgent{}}

	if _, err := reset.Reset(context.Background(), "manual"); err == nil {
		t.Fatal("Reset succeeded without a node, want an error")
	}
	state, err := repository.GetDemoState(context.Background(), db)
	if err != nil {
		t.Fatalf("GetDemoState: %v", err)
	}
	if !state.Maintenance {
		t.Fatalf("maintenance = false, want true after a refused reset")
	}
}

func TestNextBoundaryUsesTheWallClock(t *testing.T) {
	at := time.Date(2026, 10, 24, 14, 30, 0, 0, time.UTC)
	if got, want := usecase.NextBoundary(at), at.Add(30*time.Minute); !got.Equal(want) {
		t.Fatalf("NextBoundary(%v) = %v, want %v", at, got, want)
	}
	at = time.Date(2026, 10, 24, 14, 29, 59, 0, time.UTC)
	if got, want := usecase.NextBoundary(at), time.Date(2026, 10, 24, 14, 30, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("NextBoundary(%v) = %v, want %v", at, got, want)
	}
}

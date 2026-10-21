package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
)

func TestListActivePlansReturnsSeedRowsOrderedBySortOrder(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	plans, err := repository.ListActivePlans(ctx, db)
	if err != nil {
		t.Fatalf("ListActivePlans: %v", err)
	}
	if len(plans) != 4 {
		t.Fatalf("ListActivePlans returned %d plans, want 4", len(plans))
	}

	wantSlugs := []string{"sandbox", "side-project", "production", "heavy"}
	for i, want := range wantSlugs {
		if plans[i].Slug != want {
			t.Fatalf("plans[%d].Slug = %q, want %q", i, plans[i].Slug, want)
		}
	}
}

func TestListActivePlansExcludesInactivePlans(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	if _, err := db.Exec(`UPDATE plans SET active = false WHERE slug = 'sandbox'`); err != nil {
		t.Fatalf("deactivate sandbox plan: %v", err)
	}

	plans, err := repository.ListActivePlans(ctx, db)
	if err != nil {
		t.Fatalf("ListActivePlans: %v", err)
	}
	if len(plans) != 3 {
		t.Fatalf("ListActivePlans returned %d plans, want 3", len(plans))
	}
	for _, p := range plans {
		if p.Slug == "sandbox" {
			t.Fatalf("ListActivePlans still returned the deactivated sandbox plan")
		}
	}
}

func TestGetPlanByIDAndBySlug(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	want, err := repository.GetPlanBySlug(ctx, db, "production")
	if err != nil {
		t.Fatalf("GetPlanBySlug: %v", err)
	}
	if want.Name != "Production" || want.SizeGB != 10 || want.PriceIDRMonth != 100000 {
		t.Fatalf("GetPlanBySlug(production) = %+v, want name Production, size 10, price 100000", want)
	}

	got, err := repository.GetPlanByID(ctx, db, want.ID)
	if err != nil {
		t.Fatalf("GetPlanByID: %v", err)
	}
	if got.Slug != "production" {
		t.Fatalf("GetPlanByID(%s).Slug = %q, want production", want.ID, got.Slug)
	}
}

func TestListAllPlansOrderingAndIncludesInactive(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	if _, err := db.Exec(`UPDATE plans SET active = false WHERE slug = 'sandbox'`); err != nil {
		t.Fatalf("deactivate sandbox plan: %v", err)
	}

	plans, err := repository.ListAllPlans(ctx, db)
	if err != nil {
		t.Fatalf("ListAllPlans: %v", err)
	}
	if len(plans) != 4 {
		t.Fatalf("ListAllPlans returned %d plans, want 4 (including the inactive one)", len(plans))
	}

	wantSlugs := []string{"sandbox", "side-project", "production", "heavy"}
	for i, want := range wantSlugs {
		if plans[i].Slug != want {
			t.Fatalf("plans[%d].Slug = %q, want %q", i, plans[i].Slug, want)
		}
	}

	for _, p := range plans {
		if p.Slug == "sandbox" && p.Active {
			t.Fatalf("ListAllPlans reported sandbox as active after deactivating it")
		}
	}
}

func TestListAllPlansReportsDatabaseCount(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	production, err := repository.GetPlanBySlug(ctx, db, "production")
	if err != nil {
		t.Fatalf("GetPlanBySlug(production): %v", err)
	}

	var userID string
	if err := db.QueryRow(`
		INSERT INTO users (email, password_hash) VALUES ('plans-count@example.com', 'x') RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatalf("insert user fixture: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO databases (user_id, name, size_gb, status, plan_id) VALUES ($1, 'a', 10, 'running', $2), ($1, 'b', 10, 'running', $2)
	`, userID, production.ID); err != nil {
		t.Fatalf("insert database fixtures: %v", err)
	}

	plans, err := repository.ListAllPlans(ctx, db)
	if err != nil {
		t.Fatalf("ListAllPlans: %v", err)
	}
	for _, p := range plans {
		if p.Slug == "production" && p.DatabaseCount != 2 {
			t.Fatalf("production DatabaseCount = %d, want 2", p.DatabaseCount)
		}
		if p.Slug != "production" && p.DatabaseCount != 0 {
			t.Fatalf("%s DatabaseCount = %d, want 0", p.Slug, p.DatabaseCount)
		}
	}
}

// openConcurrent returns a second *sql.DB pointed at the same test schema
// db was migrated into, but with its own connection pool (storetest caps
// the original db at one connection, which would serialize any goroutines
// sharing it and defeat the point of a concurrency test). It pins the
// schema onto every connection this pool opens via the libpq "options"
// connection parameter, so there is no need to SET search_path per query.
func openConcurrent(t *testing.T, db *sql.DB) *sql.DB {
	t.Helper()

	var schema string
	if err := db.QueryRow(`SHOW search_path`).Scan(&schema); err != nil {
		t.Fatalf("read search_path: %v", err)
	}

	baseURL := os.Getenv("DATABASE_URL_TEST")
	dsn := baseURL + "&options=" + url.QueryEscape("-c search_path="+schema)
	concurrentDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open concurrent connection: %v", err)
	}
	t.Cleanup(func() { _ = concurrentDB.Close() })
	concurrentDB.SetMaxOpenConns(4)
	return concurrentDB
}

// TestUpdatePlanConcurrentDeactivationLeavesOneActivePlan is a regression
// test for a race in the "at least one active plan" guard: locking only
// the row being deactivated (FOR UPDATE) is not enough, because it never
// locks the *other* active rows the invariant depends on. Two concurrent
// requests deactivating the last two active plans could each read the
// other as still active and both commit, leaving zero active plans.
// UpdatePlan now holds a Postgres advisory lock for the duration of any
// deactivating transaction, so the two calls below must serialize:
// whichever commits second sees the first one's effect and is rejected.
func TestUpdatePlanConcurrentDeactivationLeavesOneActivePlan(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	// Narrow the seed data down to exactly two active plans so the two
	// concurrent calls below are deactivating the last two.
	for _, slug := range []string{"sandbox", "side-project"} {
		p, err := repository.GetPlanBySlug(ctx, db, slug)
		if err != nil {
			t.Fatalf("GetPlanBySlug(%s): %v", slug, err)
		}
		inactive := false
		if _, err := repository.UpdatePlan(ctx, db, p.ID, repository.PlanUpdate{Active: &inactive}); err != nil {
			t.Fatalf("deactivate %s: %v", slug, err)
		}
	}

	production, err := repository.GetPlanBySlug(ctx, db, "production")
	if err != nil {
		t.Fatalf("GetPlanBySlug(production): %v", err)
	}
	heavy, err := repository.GetPlanBySlug(ctx, db, "heavy")
	if err != nil {
		t.Fatalf("GetPlanBySlug(heavy): %v", err)
	}

	concurrentDB := openConcurrent(t, db)

	var wg sync.WaitGroup
	results := make([]error, 2)
	deactivate := func(idx int, id string) {
		defer wg.Done()
		inactive := false
		_, err := repository.UpdatePlan(ctx, concurrentDB, id, repository.PlanUpdate{Active: &inactive})
		results[idx] = err
	}

	wg.Add(2)
	go deactivate(0, production.ID)
	go deactivate(1, heavy.ID)
	wg.Wait()

	successes, rejections := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, repository.ErrLastActivePlan):
			rejections++
		default:
			t.Fatalf("unexpected UpdatePlan error: %v", err)
		}
	}
	if successes != 1 || rejections != 1 {
		t.Fatalf("concurrent deactivation results = %d succeeded, %d rejected; want exactly 1 and 1", successes, rejections)
	}

	plans, err := repository.ListAllPlans(ctx, db)
	if err != nil {
		t.Fatalf("ListAllPlans: %v", err)
	}
	activeCount := 0
	for _, p := range plans {
		if p.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("active plans after concurrent deactivation = %d, want exactly 1", activeCount)
	}
}

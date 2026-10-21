package repository_test

import (
	"context"
	"testing"

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

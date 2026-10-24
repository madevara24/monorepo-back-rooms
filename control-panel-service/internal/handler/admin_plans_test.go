package handler_test

import (
	"database/sql"
	"net/http"
	"testing"
)

func TestAdminPlansWriteRoutesRejectNonAdmin(t *testing.T) {
	server, client, _ := newTestServer(t)
	signUp(t, server, client, "plain-plans-user@example.com")

	resp := postJSON(t, client, server.URL+"/api/admin/plans", map[string]any{
		"slug": "nonadmin", "name": "Nonadmin", "size_gb": 5, "price_idr_month": 50000,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /api/admin/plans for non-admin = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	_ = resp.Body.Close()

	resp = patchJSON(t, client, server.URL+"/api/admin/plans/00000000-0000-0000-0000-000000000000", map[string]any{
		"name": "whatever",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("PATCH /api/admin/plans/{id} for non-admin = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	_ = resp.Body.Close()
}

func planID(t *testing.T, db *sql.DB, slug string) string {
	t.Helper()
	var id string
	if err := db.QueryRow(`SELECT id FROM plans WHERE slug = $1`, slug).Scan(&id); err != nil {
		t.Fatalf("look up plan id for %s: %v", slug, err)
	}
	return id
}

func TestAdminCreatePlanDuplicateSlugIsConflict(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-plans-dup@example.com")
	promoteToAdmin(t, db, "admin-plans-dup@example.com")

	resp := postJSON(t, client, server.URL+"/api/admin/plans", map[string]any{
		"slug": "sandbox", "name": "Sandbox clone", "size_gb": 1, "price_idr_month": 10000,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST /api/admin/plans with duplicate slug = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()
}

func TestAdminCreatePlanSucceeds(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-plans-create@example.com")
	promoteToAdmin(t, db, "admin-plans-create@example.com")

	resp := postJSON(t, client, server.URL+"/api/admin/plans", map[string]any{
		"slug": "test-pack", "name": "Test pack", "size_gb": 3, "price_idr_month": 30000,
		"tagline": "A test tagline", "badge": "New", "sort_order": 5,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/admin/plans = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created map[string]any
	decode(t, resp, &created)
	if created["slug"] != "test-pack" {
		t.Fatalf("created plan slug = %v, want test-pack", created["slug"])
	}
	if created["active"] != true {
		t.Fatalf("created plan active = %v, want true (default)", created["active"])
	}
	if created["database_count"] != float64(0) {
		t.Fatalf("created plan database_count = %v, want 0", created["database_count"])
	}
}

func TestAdminUpdatePlanWithSlugIsBadRequest(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-plans-slug@example.com")
	promoteToAdmin(t, db, "admin-plans-slug@example.com")

	id := planID(t, db, "sandbox")
	resp := patchJSON(t, client, server.URL+"/api/admin/plans/"+id, map[string]any{
		"slug": "renamed",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("PATCH with slug = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	_ = resp.Body.Close()
}

func TestAdminDeactivatePlanHidesItFromPublicListButNotAdminList(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-plans-deactivate@example.com")
	promoteToAdmin(t, db, "admin-plans-deactivate@example.com")

	id := planID(t, db, "sandbox")
	resp := patchJSON(t, client, server.URL+"/api/admin/plans/"+id, map[string]any{
		"active": false,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH active=false = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	resp, err := client.Get(server.URL + "/api/plans")
	if err != nil {
		t.Fatalf("GET /api/plans: %v", err)
	}
	var publicPlans []map[string]any
	decode(t, resp, &publicPlans)
	for _, p := range publicPlans {
		if p["slug"] == "sandbox" {
			t.Fatalf("GET /api/plans still listed the deactivated sandbox plan")
		}
	}

	resp, err = client.Get(server.URL + "/api/admin/plans")
	if err != nil {
		t.Fatalf("GET /api/admin/plans: %v", err)
	}
	var adminPlans []map[string]any
	decode(t, resp, &adminPlans)
	var found map[string]any
	for _, p := range adminPlans {
		if p["slug"] == "sandbox" {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("GET /api/admin/plans no longer listed the deactivated sandbox plan")
	}
	if found["active"] != false {
		t.Fatalf("admin plans row active = %v, want false", found["active"])
	}
}

func TestAdminDeactivateLastActivePlanIsConflict(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-plans-lastactive@example.com")
	promoteToAdmin(t, db, "admin-plans-lastactive@example.com")

	for _, slug := range []string{"sandbox", "side-project", "heavy"} {
		resp := patchJSON(t, client, server.URL+"/api/admin/plans/"+planID(t, db, slug), map[string]any{
			"active": false,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH active=false for %s = %d, want %d", slug, resp.StatusCode, http.StatusOK)
		}
		_ = resp.Body.Close()
	}

	resp := patchJSON(t, client, server.URL+"/api/admin/plans/"+planID(t, db, "production"), map[string]any{
		"active": false,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("PATCH active=false for the last active plan = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()
}

func TestAdminUpdatePlanPriceLeavesExistingDatabasePriceUnchanged(t *testing.T) {
	server, client, db := newTestServer(t)
	signUp(t, server, client, "admin-plans-price@example.com")
	promoteToAdmin(t, db, "admin-plans-price@example.com")
	insertNode(t, db, "node-admin-plans-price")
	if _, err := db.Exec(`UPDATE plans SET purchasable = TRUE WHERE slug = 'production'`); err != nil {
		t.Fatalf("make production buyable for the price snapshot test: %v", err)
	}

	var userID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, "admin-plans-price@example.com").Scan(&userID); err != nil {
		t.Fatalf("look up user id: %v", err)
	}

	productionID := planID(t, db, "production")
	resp := postJSON(t, client, server.URL+"/api/databases", map[string]any{
		"name": "priced-db", "plan_id": productionID,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /api/databases = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created map[string]any
	decode(t, resp, &created)

	var originalPrice int64
	if err := db.QueryRow(`SELECT price_idr_month FROM databases WHERE id = $1`, created["id"]).Scan(&originalPrice); err != nil {
		t.Fatalf("look up original price: %v", err)
	}
	if originalPrice != 100000 {
		t.Fatalf("original database price = %d, want 100000", originalPrice)
	}

	resp = patchJSON(t, client, server.URL+"/api/admin/plans/"+productionID, map[string]any{
		"price_idr_month": 999000,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH price_idr_month = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	var priceAfterPlanEdit int64
	if err := db.QueryRow(`SELECT price_idr_month FROM databases WHERE id = $1`, created["id"]).Scan(&priceAfterPlanEdit); err != nil {
		t.Fatalf("look up price after plan edit: %v", err)
	}
	if priceAfterPlanEdit != 100000 {
		t.Fatalf("database price after plan price edit = %d, want unchanged 100000", priceAfterPlanEdit)
	}
}

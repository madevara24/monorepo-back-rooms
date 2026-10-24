package handler_test

import (
	"database/sql"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/handler"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

func newDemoTestServer(t *testing.T) (*httptest.Server, *http.Client, *sql.DB) {
	t.Helper()
	db := storetest.Open(t)
	h := &handler.Handler{
		DB:     db,
		Auth:   &usecase.Auth{DB: db},
		Cipher: testCipher(t),
		Demo:   &usecase.DemoReset{DB: db},
	}
	server := httptest.NewServer(handler.NewMux(h))
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return server, &http.Client{Jar: jar}, db
}

func TestDemoStatusIsPublicOnlyInDemoMode(t *testing.T) {
	server, client, _ := newDemoTestServer(t)

	resp, err := client.Get(server.URL + "/api/demo/status")
	if err != nil {
		t.Fatalf("GET demo status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("demo status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var status map[string]any
	decode(t, resp, &status)
	if status["maintenance"] != false {
		t.Fatalf("maintenance = %v, want false", status["maintenance"])
	}
	if status["generation"] != float64(0) {
		t.Fatalf("generation = %v, want 0", status["generation"])
	}
	if _, ok := status["next_reset_at"]; !ok {
		t.Fatalf("status has no next_reset_at: %v", status)
	}

	plain, plainClient, _ := newTestServer(t)
	resp, err = plainClient.Get(plain.URL + "/api/demo/status")
	if err != nil {
		t.Fatalf("GET demo status without demo mode: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("demo status without demo mode = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestCreateIsRefusedDuringMaintenance(t *testing.T) {
	server, client, db := newDemoTestServer(t)
	signUp(t, server, client, "demo-maint@example.com")
	insertNode(t, db, "node-1")
	if _, err := db.Exec(`UPDATE demo_state SET maintenance = TRUE`); err != nil {
		t.Fatalf("set maintenance: %v", err)
	}

	resp := postJSON(t, client, server.URL+"/api/databases", map[string]any{"name": "demo", "size_gb": 1})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("create during maintenance = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	if got := resp.Header.Get("Retry-After"); got == "" {
		t.Fatalf("missing Retry-After on the maintenance response")
	}
	_ = resp.Body.Close()
}

func TestCreateStopsAtTwoNonDeletedDatabases(t *testing.T) {
	server, client, db := newDemoTestServer(t)
	signUp(t, server, client, "demo-limit@example.com")
	insertNode(t, db, "node-1")

	for i := range 2 {
		resp := postJSON(t, client, server.URL+"/api/databases", map[string]any{"name": "demo", "size_gb": 1})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create %d = %d, want %d", i+1, resp.StatusCode, http.StatusCreated)
		}
		_ = resp.Body.Close()
	}

	resp := postJSON(t, client, server.URL+"/api/databases", map[string]any{"name": "demo", "size_gb": 1})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("third create = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()
}

func TestSoldOutPlanIsListedAndRefused(t *testing.T) {
	server, client, _ := newDemoTestServer(t)

	resp, err := client.Get(server.URL + "/api/plans")
	if err != nil {
		t.Fatalf("GET /api/plans: %v", err)
	}
	var plans []map[string]any
	decode(t, resp, &plans)
	var productionID string
	for _, p := range plans {
		if p["slug"] == "production" {
			productionID, _ = p["id"].(string)
			if p["purchasable"] != false {
				t.Fatalf("production purchasable = %v, want false", p["purchasable"])
			}
		}
	}
	if productionID == "" {
		t.Fatal("production plan missing from the public list")
	}

	signUp(t, server, client, "demo-soldout@example.com")
	resp = postJSON(t, client, server.URL+"/api/databases", map[string]any{
		"name": "demo", "plan_id": productionID,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("sold-out plan create = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	_ = resp.Body.Close()
}

package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/handler"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/proxy"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
)

func TestHandleHealthz(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mbr-routes.map")
	if err := os.WriteFile(path, []byte("alpha.db.mbr.madevara.com 10.0.0.1:21000;\n"), 0o644); err != nil {
		t.Fatalf("write map: %v", err)
	}
	routes, err := routemap.New(path)
	if err != nil {
		t.Fatalf("routemap.New: %v", err)
	}

	metrics := &proxy.Metrics{}
	metrics.DirectTLS.Store(3)

	h := &handler.Handler{Commit: "abc123", Metrics: metrics, Routes: routes}
	mux := handler.NewMux(h)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Status       string `json:"status"`
		Commit       string `json:"commit"`
		RoutesLoaded int    `json:"routes_loaded"`
		Negotiations struct {
			DirectTLS int64 `json:"direct_tls"`
		} `json:"negotiations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "ok" || body.Commit != "abc123" {
		t.Fatalf("body = %+v", body)
	}
	if body.RoutesLoaded != 1 {
		t.Fatalf("RoutesLoaded = %d, want 1", body.RoutesLoaded)
	}
	if body.Negotiations.DirectTLS != 3 {
		t.Fatalf("Negotiations.DirectTLS = %d, want 3", body.Negotiations.DirectTLS)
	}
}

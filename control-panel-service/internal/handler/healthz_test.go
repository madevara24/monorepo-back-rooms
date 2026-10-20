package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/handler"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.NewMux(&handler.Handler{Commit: "abc123"}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	want := `{"status":"ok","commit":"abc123"}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

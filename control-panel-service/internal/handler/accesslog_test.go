package handler_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/handler"
)

func TestAccessLogWritesRouteStatusAndNoQueryOrBody(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/databases", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"connection_string":"postgres://secret"}`))
	})
	logged := handler.AccessLog(logger, mux)

	req := httptest.NewRequest(http.MethodPost, "/api/databases?password=secret", strings.NewReader(`{"password":"secret"}`))
	logged.ServeHTTP(httptest.NewRecorder(), req)

	line := buf.String()
	if !strings.Contains(line, "POST /api/databases 201") {
		t.Fatalf("access log = %q, want method, route, and status", line)
	}
	if strings.Contains(line, "secret") || strings.Contains(line, "password") {
		t.Fatalf("access log leaked the query or body: %q", line)
	}
}

func TestAccessLogSkipsHealthz(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	logged := handler.AccessLog(logger, mux)

	logged.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if buf.Len() != 0 {
		t.Fatalf("healthz wrote an access line: %q", buf.String())
	}
}

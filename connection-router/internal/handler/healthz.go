// Package handler implements the router's one HTTP endpoint,
// GET /healthz, served on a loopback-only listener separate from the
// TCP data path on :15432.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/proxy"
	"github.com/madevara24/monorepo-back-rooms/connection-router/internal/routemap"
)

// Handler serves the router's health and negotiation-counter endpoint.
type Handler struct {
	Commit  string
	Metrics *proxy.Metrics
	Routes  *routemap.Watcher
}

// NewMux builds the router's loopback-only HTTP mux.
func NewMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	return mux
}

type healthzResponse struct {
	Status       string         `json:"status"`
	Commit       string         `json:"commit"`
	RoutesLoaded int            `json:"routes_loaded"`
	Negotiations proxy.Snapshot `json:"negotiations"`
}

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthzResponse{
		Status:       "ok",
		Commit:       h.Commit,
		RoutesLoaded: len(h.Routes.Current()),
		Negotiations: h.Metrics.Snapshot(),
	})
}

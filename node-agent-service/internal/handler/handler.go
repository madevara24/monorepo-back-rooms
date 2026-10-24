// Package handler implements the agent's HTTP API: the net/http
// handlers and JSON wire types that sit behind cmd/node's mTLS listener.
// It does no authentication of its own, and owns the agent's own
// /healthz.
package handler

import (
	"errors"
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/node-agent-service/internal/usecase"
)

// Handler holds the usecase dependency the agent's HTTP handlers call.
type Handler struct {
	Node *usecase.Node
}

// NewMux builds the agent's HTTP router. It is meant to sit behind the
// mTLS listener cmd/node sets up.
func NewMux(n *usecase.Node) *http.ServeMux {
	h := &Handler{Node: n}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("GET /v1/node", h.handleGetNode)
	mux.HandleFunc("GET /v1/databases", h.handleListDatabases)
	mux.HandleFunc("POST /v1/databases", h.handleCreateDatabase)
	mux.HandleFunc("DELETE /v1/databases/{id}", h.handleDeleteDatabase)
	mux.HandleFunc("GET /v1/databases/{id}/snapshots", h.handleSnapshotsGated)
	mux.HandleFunc("POST /v1/databases/{id}/snapshots", h.handleSnapshotsGated)
	if n.DemoMode {
		mux.HandleFunc("POST /v1/demo/reset", h.handleResetDemo)
	}

	return mux
}

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type databaseResponse struct {
	ID     string `json:"id"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Status string `json:"status"`
}

func toDatabaseResponse(host string, c entities.Container) databaseResponse {
	return databaseResponse{ID: c.DatabaseID, Host: host, Port: c.Port, Status: c.Status}
}

type capacityResponse struct {
	PortRangeMin int `json:"port_range_min"`
	PortRangeMax int `json:"port_range_max"`
	PortsTotal   int `json:"ports_total"`
	PortsUsed    int `json:"ports_used"`
}

type nodeResponse struct {
	NodeID     string             `json:"node_id"`
	Host       string             `json:"host"`
	Version    string             `json:"version"`
	Capacity   capacityResponse   `json:"capacity"`
	UsedPorts  []int              `json:"used_ports"`
	Containers []databaseResponse `json:"containers"`
}

func (h *Handler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	status, err := h.Node.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not get node status")
		return
	}

	out := make([]databaseResponse, len(status.Containers))
	for i, c := range status.Containers {
		out[i] = toDatabaseResponse(status.Host, c)
	}

	writeJSON(w, http.StatusOK, nodeResponse{
		NodeID:  status.NodeID,
		Host:    status.Host,
		Version: status.Version,
		Capacity: capacityResponse{
			PortRangeMin: status.PortRangeMin,
			PortRangeMax: status.PortRangeMax,
			PortsTotal:   status.PortsTotal,
			PortsUsed:    status.PortsUsed,
		},
		UsedPorts:  status.UsedPorts,
		Containers: out,
	})
}

func (h *Handler) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	containers, err := h.Node.ListDatabases(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list containers")
		return
	}
	out := make([]databaseResponse, len(containers))
	for i, c := range containers {
		out[i] = toDatabaseResponse(h.Node.HostAddress, c)
	}
	writeJSON(w, http.StatusOK, out)
}

type createDatabaseRequest struct {
	ID       string `json:"id"`
	SizeGB   int    `json:"size_gb"`
	Password string `json:"password"`
}

func (h *Handler) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	var req createDatabaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" || req.SizeGB <= 0 || req.Password == "" {
		writeError(w, http.StatusBadRequest, "id, size_gb, and password are required")
		return
	}

	container, created, err := h.Node.CreateDatabase(r.Context(), req.ID, req.SizeGB, req.Password)
	if err != nil {
		if errors.Is(err, usecase.ErrNoFreePort) {
			writeError(w, http.StatusServiceUnavailable, "no free port available")
		} else {
			writeError(w, http.StatusInternalServerError, "could not create database")
		}
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, toDatabaseResponse(h.Node.HostAddress, container))
}

func (h *Handler) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.Node.DeleteDatabase(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete database")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleResetDemo(w http.ResponseWriter, r *http.Request) {
	counts, err := h.Node.ResetDemo(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "demo reset failed",
			"counts": counts,
		})
		return
	}
	writeJSON(w, http.StatusOK, counts)
}

// handleSnapshotsGated covers every snapshot route; snapshots are out of
// scope for this slice of the agent.
func (h *Handler) handleSnapshotsGated(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "snapshots are not implemented yet")
}

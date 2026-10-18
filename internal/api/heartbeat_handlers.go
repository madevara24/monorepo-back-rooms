package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/madevara24/monorepo-back-rooms/internal/auth"
	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

// heartbeatContainer and heartbeatCapacity mirror the shapes
// internal/node/server.go and internal/node/heartbeat.go send as
// databaseResponse and capacityResponse. heartbeatRequest mirrors
// internal/node/heartbeat.go's heartbeatRequest; notably it carries no
// agent version field, so a heartbeat never updates nodes.version.
type heartbeatContainer struct {
	ID     string `json:"id"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Status string `json:"status"`
}

type heartbeatCapacity struct {
	PortRangeMin int `json:"port_range_min"`
	PortRangeMax int `json:"port_range_max"`
	PortsTotal   int `json:"ports_total"`
	PortsUsed    int `json:"ports_used"`
}

type heartbeatRequest struct {
	NodeID     string               `json:"node_id"`
	Host       string               `json:"host"`
	Capacity   heartbeatCapacity    `json:"capacity"`
	Containers []heartbeatContainer `json:"containers"`
}

// handleAgentHeartbeat records an mbr-node agent's periodic report: its
// capacity and the tenant containers it currently sees. The agent
// authenticates with a bearer token; this looks up the node by the
// token's SHA-256 hash rather than trusting the body's node_id.
func (a *API) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}

	node, err := store.NodeByTokenHash(r.Context(), a.DB, auth.HashToken(token))
	switch {
	case isNotFound(err):
		writeError(w, http.StatusUnauthorized, "unknown agent token")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not look up node")
		return
	}

	var req heartbeatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	capacity, err := json.Marshal(req.Capacity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not encode capacity")
		return
	}
	if err := store.UpdateNodeHeartbeat(r.Context(), a.DB, node.ID, capacity); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record heartbeat")
		return
	}

	containers := make([]store.NodeContainer, len(req.Containers))
	for i, c := range req.Containers {
		containers[i] = store.NodeContainer{NodeID: node.ID, DatabaseID: c.ID, Status: c.Status, Port: c.Port}
	}
	if err := store.UpsertNodeContainers(r.Context(), a.DB, node.ID, containers); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record containers")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}

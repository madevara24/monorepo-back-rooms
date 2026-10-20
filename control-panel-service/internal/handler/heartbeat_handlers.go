package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/session"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

// heartbeatContainer and heartbeatCapacity mirror the shapes
// node-agent-service/internal/handler and node-agent-service/internal/usecase
// send as databaseResponse and capacityResponse. heartbeatRequest mirrors
// node-agent-service/internal/usecase's heartbeatRequest; notably it
// carries no agent version field, so a heartbeat never updates
// nodes.version.
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
func (h *Handler) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}

	node, err := repository.NodeByTokenHash(r.Context(), h.DB, session.HashToken(token))
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
	if err := repository.UpdateNodeHeartbeat(r.Context(), h.DB, node.ID, capacity); err != nil {
		writeError(w, http.StatusInternalServerError, "could not record heartbeat")
		return
	}

	containers := make([]entities.NodeContainer, len(req.Containers))
	for i, c := range req.Containers {
		containers[i] = entities.NodeContainer{NodeID: node.ID, DatabaseID: c.ID, Status: c.Status, Port: c.Port}
	}
	if err := repository.UpsertNodeContainers(r.Context(), h.DB, node.ID, containers); err != nil {
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

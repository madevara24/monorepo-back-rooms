package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/madevara24/monorepo-back-rooms/contracts"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/session"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

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

	var req contracts.HeartbeatRequest
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

	// A metric write failing (e.g. the database was deleted between
	// collection and this heartbeat landing) must not fail the
	// heartbeat itself: the container report above already succeeded.
	for _, c := range req.Containers {
		if c.Metrics == nil {
			continue
		}
		metric := entities.DatabaseMetric{
			DatabaseID:                   c.ID,
			Active:                       c.Metrics.Active,
			Idle:                         c.Metrics.Idle,
			IdleInTransaction:            c.Metrics.IdleInTransaction,
			MaxConnections:               c.Metrics.MaxConnections,
			SuperuserReservedConnections: c.Metrics.SuperuserReservedConnections,
			MemoryLimitBytes:             c.Metrics.MemoryLimitBytes,
			CollectedAt:                  c.Metrics.CollectedAt,
		}
		if err := repository.InsertDatabaseMetric(r.Context(), h.DB, metric); err != nil {
			log.Printf("heartbeat: insert metric for %s: %v", c.ID, err)
		}
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

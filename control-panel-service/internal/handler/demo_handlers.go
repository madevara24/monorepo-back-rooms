package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

type demoStatusResponse struct {
	NextResetAt time.Time `json:"next_reset_at"`
	Generation  int64     `json:"generation"`
	Maintenance bool      `json:"maintenance"`
	Status      string    `json:"status"`
}

func (h *Handler) handleDemoStatus(w http.ResponseWriter, r *http.Request) {
	if h.Demo == nil {
		writeError(w, http.StatusNotFound, "demo mode is not enabled")
		return
	}
	state, err := h.Demo.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load demo state")
		return
	}
	writeJSON(w, http.StatusOK, demoStatusResponse{
		NextResetAt: state.NextResetAt,
		Generation:  state.Generation,
		Maintenance: state.Maintenance,
		Status:      state.Status,
	})
}

type demoResetResponse struct {
	demoStatusResponse
	Counts resetCountsResponse `json:"counts"`
}

type resetCountsResponse struct {
	Containers int `json:"containers_deleted"`
	Volumes    int `json:"volumes_deleted"`
	Orphans    int `json:"orphans_deleted"`
}

func (h *Handler) handleAdminDemoReset(w http.ResponseWriter, r *http.Request) {
	if h.Demo == nil {
		writeError(w, http.StatusNotFound, "demo mode is not enabled")
		return
	}
	result, err := h.Demo.Reset(r.Context(), "manual")
	if errors.Is(err, usecase.ErrResetInProgress) {
		writeError(w, http.StatusConflict, "a demo reset is already running")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "demo reset failed; maintenance remains on")
		return
	}
	writeJSON(w, http.StatusOK, demoResetResponse{
		demoStatusResponse: demoStatusResponse{
			NextResetAt: result.State.NextResetAt,
			Generation:  result.State.Generation,
			Maintenance: result.State.Maintenance,
			Status:      result.State.Status,
		},
		Counts: resetCountsResponse{
			Containers: result.Counts.Containers,
			Volumes:    result.Counts.Volumes,
			Orphans:    result.Counts.Orphans,
		},
	})
}

// demoMaintenance reports whether a create or delete should be refused.
// A nil demo component (non-demo deployments and handler tests) is never
// in maintenance.
func (h *Handler) demoMaintenance(ctx context.Context) bool {
	if h.Demo == nil {
		return false
	}
	maintenance, err := h.Demo.IsMaintenance(ctx)
	return err == nil && maintenance
}

func writeMaintenance(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	writeError(w, http.StatusServiceUnavailable, "the demo is resetting; try again shortly")
}

package handler

import "net/http"

type healthzResponse struct {
	Status string `json:"status"`
	Commit string `json:"commit"`
}

// handleHealthz reports the build's commit SHA, so a deploy can confirm
// the live process matches the commit it dispatched.
func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthzResponse{Status: "ok", Commit: h.Commit})
}

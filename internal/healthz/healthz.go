// Package healthz provides a shared liveness handler for the API and node processes.
package healthz

import (
	"encoding/json"
	"net/http"
)

type response struct {
	Status string `json:"status"`
	Commit string `json:"commit"`
}

// NewHandler returns a liveness handler that reports the build's commit SHA.
func NewHandler(commit string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response{Status: "ok", Commit: commit})
	}
}

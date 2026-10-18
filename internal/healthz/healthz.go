// Package healthz provides a shared liveness handler for the API and node processes.
package healthz

import (
	"encoding/json"
	"net/http"
)

// Handler responds to GET requests with {"status":"ok"}.
func Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

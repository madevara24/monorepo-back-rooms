package api

import (
	"net/http"
	"time"

	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

type databaseResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SizeGB    int       `json:"size_gb"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func toDatabaseResponse(d store.Database) databaseResponse {
	return databaseResponse{ID: d.ID, Name: d.Name, SizeGB: d.SizeGB, Status: d.Status, CreatedAt: d.CreatedAt}
}

func (a *API) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	rows, err := store.ListDatabasesByUser(r.Context(), a.DB, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list databases")
		return
	}

	out := make([]databaseResponse, len(rows))
	for i, d := range rows {
		out[i] = toDatabaseResponse(d)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	d, err := store.DatabaseByIDForUser(r.Context(), a.DB, r.PathValue("id"), user.ID)
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load database")
		return
	}
	writeJSON(w, http.StatusOK, toDatabaseResponse(d))
}

// handleCreateDatabaseGated stands in for provisioning. This slice runs
// the control plane without the data-plane VPS or S3, so there is no
// agent to call and no container to create.
func (a *API) handleCreateDatabaseGated(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "database provisioning is not available yet")
}

// handleDeleteDatabaseGated stands in for teardown, gated for the same
// reason as handleCreateDatabaseGated.
func (a *API) handleDeleteDatabaseGated(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "database deletion is not available yet")
}

// handleSnapshotsGated covers every snapshot route. Snapshots need the
// data-plane agent and an S3 bucket, neither of which this slice has.
func (a *API) handleSnapshotsGated(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "snapshots are not implemented yet")
}

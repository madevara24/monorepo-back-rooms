package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

type adminOverviewResponse struct {
	Users             int            `json:"users"`
	DatabasesByStatus map[string]int `json:"databases_by_status"`
	NodesFresh        int            `json:"nodes_fresh"`
	NodesStale        int            `json:"nodes_stale"`
	JobsByStatus      map[string]int `json:"jobs_by_status"`
}

func (a *API) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := store.AdminGetOverview(r.Context(), a.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load overview")
		return
	}
	writeJSON(w, http.StatusOK, adminOverviewResponse{
		Users:             overview.UserCount,
		DatabasesByStatus: overview.DatabasesByStatus,
		NodesFresh:        overview.NodesFresh,
		NodesStale:        overview.NodesStale,
		JobsByStatus:      overview.JobsByStatus,
	})
}

type adminUserResponse struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"created_at"`
	DatabaseCount int       `json:"database_count"`
}

func (a *API) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := store.AdminListUsers(r.Context(), a.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list users")
		return
	}

	out := make([]adminUserResponse, len(rows))
	for i, u := range rows {
		out[i] = adminUserResponse{ID: u.ID, Email: u.Email, CreatedAt: u.CreatedAt, DatabaseCount: u.DatabaseCount}
	}
	writeJSON(w, http.StatusOK, out)
}

type adminDatabaseResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	SizeGB     int       `json:"size_gb"`
	Status     string    `json:"status"`
	OwnerEmail string    `json:"owner_email"`
	NodeID     *string   `json:"node_id"`
	Host       *string   `json:"host"`
	Port       *int      `json:"port"`
	CreatedAt  time.Time `json:"created_at"`
}

func (a *API) handleAdminDatabases(w http.ResponseWriter, r *http.Request) {
	rows, err := store.AdminListDatabases(r.Context(), a.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list databases")
		return
	}

	out := make([]adminDatabaseResponse, len(rows))
	for i, d := range rows {
		out[i] = adminDatabaseResponse{
			ID:         d.ID,
			Name:       d.Name,
			SizeGB:     d.SizeGB,
			Status:     d.Status,
			OwnerEmail: d.OwnerEmail,
			NodeID:     d.NodeID,
			Host:       d.Host,
			Port:       d.Port,
			CreatedAt:  d.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type adminContainerResponse struct {
	DatabaseID string    `json:"database_id"`
	Status     string    `json:"status"`
	Port       int       `json:"port"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type adminNodeResponse struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Address       string                   `json:"address"`
	Capacity      map[string]any           `json:"capacity"`
	Version       string                   `json:"version"`
	LastHeartbeat *time.Time               `json:"last_heartbeat"`
	Containers    []adminContainerResponse `json:"containers"`
}

func (a *API) handleAdminNodes(w http.ResponseWriter, r *http.Request) {
	rows, err := store.AdminListNodes(r.Context(), a.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list nodes")
		return
	}

	out := make([]adminNodeResponse, len(rows))
	for i, n := range rows {
		containers := make([]adminContainerResponse, len(n.Containers))
		for j, c := range n.Containers {
			containers[j] = adminContainerResponse{
				DatabaseID: c.DatabaseID,
				Status:     c.Status,
				Port:       c.Port,
				UpdatedAt:  c.UpdatedAt,
			}
		}

		var capacity map[string]any
		if err := json.Unmarshal(n.Capacity, &capacity); err != nil {
			capacity = map[string]any{}
		}

		out[i] = adminNodeResponse{
			ID:            n.ID,
			Name:          n.Name,
			Address:       n.Address,
			Capacity:      capacity,
			Version:       n.Version,
			LastHeartbeat: n.LastHeartbeat,
			Containers:    containers,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type adminJobResponse struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Attempts  int       `json:"attempts"`
	LastError *string   `json:"last_error"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *API) handleAdminJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := store.AdminListJobs(r.Context(), a.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list jobs")
		return
	}

	out := make([]adminJobResponse, len(rows))
	for i, j := range rows {
		out[i] = adminJobResponse{
			ID:        j.ID,
			Kind:      j.Kind,
			Status:    j.Status,
			Attempts:  j.Attempts,
			LastError: j.LastError,
			CreatedAt: j.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

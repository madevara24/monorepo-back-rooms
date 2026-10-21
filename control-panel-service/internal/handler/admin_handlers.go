package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

type adminOverviewResponse struct {
	Users             int            `json:"users"`
	DatabasesByStatus map[string]int `json:"databases_by_status"`
	NodesFresh        int            `json:"nodes_fresh"`
	NodesStale        int            `json:"nodes_stale"`
	JobsByStatus      map[string]int `json:"jobs_by_status"`
}

func (h *Handler) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := repository.AdminGetOverview(r.Context(), h.DB)
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

func (h *Handler) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := repository.AdminListUsers(r.Context(), h.DB)
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

func (h *Handler) handleAdminDatabases(w http.ResponseWriter, r *http.Request) {
	rows, err := repository.AdminListDatabases(r.Context(), h.DB)
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
	ID                   string                   `json:"id"`
	Name                 string                   `json:"name"`
	Address              string                   `json:"address"`
	Capacity             map[string]any           `json:"capacity"`
	Version              string                   `json:"version"`
	LastHeartbeat        *time.Time               `json:"last_heartbeat"`
	LastHeartbeatAgeSecs *int64                   `json:"last_heartbeat_age_seconds"`
	Containers           []adminContainerResponse `json:"containers"`
}

func (h *Handler) handleAdminNodes(w http.ResponseWriter, r *http.Request) {
	rows, err := repository.AdminListNodes(r.Context(), h.DB)
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

		var ageSecs *int64
		if n.LastHeartbeat != nil {
			age := int64(time.Since(*n.LastHeartbeat).Seconds())
			ageSecs = &age
		}

		out[i] = adminNodeResponse{
			ID:                   n.ID,
			Name:                 n.Name,
			Address:              n.Address,
			Capacity:             capacity,
			Version:              n.Version,
			LastHeartbeat:        n.LastHeartbeat,
			LastHeartbeatAgeSecs: ageSecs,
			Containers:           containers,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

type adminCreateDatabaseRequest struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	SizeGB int    `json:"size_gb"`
}

// handleAdminCreateDatabase provisions a database for any user through
// provisionDatabase, the same path POST /api/databases uses for the
// signed-in tenant. The plaintext password is returned here and only
// here, exactly once.
func (h *Handler) handleAdminCreateDatabase(w http.ResponseWriter, r *http.Request) {
	var req adminCreateDatabaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if msg := validateCreateDatabaseRequest(req.Name, req.SizeGB); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if _, err := repository.UserByID(r.Context(), h.DB, req.UserID); isNotFound(err) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "could not look up user")
		return
	}

	created, password, err := h.provisionDatabase(r.Context(), req.UserID, req.Name, req.SizeGB)
	if isNotFound(err) {
		writeError(w, http.StatusServiceUnavailable, "no node available")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create database")
		return
	}

	writeJSON(w, http.StatusCreated, createDatabaseResponse{
		ID:               created.ID,
		Host:             usecase.PublicHost(*created.Slug),
		Port:             usecase.PublicPort,
		Database:         "postgres",
		User:             "postgres",
		Password:         password,
		ConnectionString: usecase.ConnectionString(*created.Slug, password),
		JDBCURL:          usecase.JDBCConnectionString(*created.Slug),
	})
}

// handleAdminDeleteDatabase force-deletes any owner's database: it moves
// a "running" or "failed" row to "deleting" and queues a fresh delete
// job. There is no final snapshot: this is the same settled test-stage
// decision as the tenant delete path.
func (h *Handler) handleAdminDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	d, err := repository.AdminForceDeleteDatabase(r.Context(), h.DB, r.PathValue("id"))
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if errors.Is(err, repository.ErrInvalidState) {
		writeError(w, http.StatusConflict, "database is not in a deletable state")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete database")
		return
	}
	writeJSON(w, http.StatusAccepted, toDatabaseResponse(d))
}

// handleAdminRetryDatabase requeues a "failed" database's last
// operation: back to "provisioning" if it never finished provisioning,
// or back to "deleting" if it failed while being deleted.
func (h *Handler) handleAdminRetryDatabase(w http.ResponseWriter, r *http.Request) {
	d, err := repository.AdminRetryDatabase(r.Context(), h.DB, r.PathValue("id"))
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if errors.Is(err, repository.ErrInvalidState) {
		writeError(w, http.StatusConflict, "database is not in a retryable state")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not retry database")
		return
	}
	writeJSON(w, http.StatusAccepted, toDatabaseResponse(d))
}

// warningThresholdPercent and criticalThresholdPercent are the admin UI's
// connection-pressure warning cutoffs. v1 has no alert channel; they only
// drive the badge shown on GET /api/admin/database-metrics.
const (
	warningThresholdPercent  = 70.0
	criticalThresholdPercent = 90.0
)

func connectionWarningLevel(usedPercent float64) string {
	switch {
	case usedPercent >= criticalThresholdPercent:
		return "critical"
	case usedPercent >= warningThresholdPercent:
		return "warning"
	default:
		return "ok"
	}
}

type adminDatabaseMetricResponse struct {
	DatabaseID                   string    `json:"database_id"`
	DatabaseName                 string    `json:"database_name"`
	OwnerEmail                   string    `json:"owner_email"`
	Active                       int       `json:"active"`
	Idle                         int       `json:"idle"`
	IdleInTransaction            int       `json:"idle_in_transaction"`
	Used                         int       `json:"used"`
	MaxConnections               int       `json:"max_connections"`
	SuperuserReservedConnections int       `json:"superuser_reserved_connections"`
	UsedPercent                  float64   `json:"used_percent"`
	WarningLevel                 string    `json:"warning_level"`
	MemoryLimitBytes             int64     `json:"memory_limit_bytes"`
	CollectedAt                  time.Time `json:"collected_at"`
}

// handleAdminDatabaseMetrics returns each database's latest
// connection-pressure snapshot: used/max connections, the state
// breakdown, when it was collected, and a 70%/90% warning level. There is
// no alert channel in v1; this is the only place the warning surfaces.
func (h *Handler) handleAdminDatabaseMetrics(w http.ResponseWriter, r *http.Request) {
	rows, err := repository.LatestDatabaseMetrics(r.Context(), h.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load database metrics")
		return
	}

	out := make([]adminDatabaseMetricResponse, len(rows))
	for i, m := range rows {
		used := m.Active + m.Idle + m.IdleInTransaction
		var usedPercent float64
		if m.MaxConnections > 0 {
			usedPercent = float64(used) / float64(m.MaxConnections) * 100
		}
		out[i] = adminDatabaseMetricResponse{
			DatabaseID:                   m.DatabaseID,
			DatabaseName:                 m.DatabaseName,
			OwnerEmail:                   m.OwnerEmail,
			Active:                       m.Active,
			Idle:                         m.Idle,
			IdleInTransaction:            m.IdleInTransaction,
			Used:                         used,
			MaxConnections:               m.MaxConnections,
			SuperuserReservedConnections: m.SuperuserReservedConnections,
			UsedPercent:                  usedPercent,
			WarningLevel:                 connectionWarningLevel(usedPercent),
			MemoryLimitBytes:             m.MemoryLimitBytes,
			CollectedAt:                  m.CollectedAt,
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

func (h *Handler) handleAdminJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := repository.AdminListJobs(r.Context(), h.DB)
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

// Package handler wires the control plane's HTTP routes: auth, the
// databases API, the agent heartbeat endpoint, billing, and the admin
// API. Provisioning and deletion are carried out asynchronously by
// usecase.Worker, which calls the data-plane agent; this package only
// queues the work. It never touches S3 — snapshots are out of scope for
// this slice.
package handler

import (
	"database/sql"
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/credentials"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

// Handler holds the dependencies HTTP handlers need.
type Handler struct {
	DB   *sql.DB
	Auth *usecase.Auth
	// CookieSecure sets the Secure attribute on the session cookie. It
	// should be true behind TLS in production and false for local dev,
	// where the browser talks to the Vite proxy over plain HTTP.
	CookieSecure bool
	// Commit is the build's git SHA, reported by GET /healthz.
	Commit string
	// Cipher encrypts tenant database passwords at rest. It is required
	// by POST /api/databases.
	Cipher *credentials.Cipher
}

// NewMux builds the complete control-plane router.
func NewMux(h *Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", h.handleHealthz)

	mux.HandleFunc("POST /api/auth/signup", h.handleSignup)
	mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/me", h.requireAuth(h.handleMe))

	mux.HandleFunc("GET /api/databases", h.requireAuth(h.handleListDatabases))
	mux.HandleFunc("GET /api/databases/{id}", h.requireAuth(h.handleGetDatabase))
	mux.HandleFunc("POST /api/databases", h.requireAuth(h.handleCreateDatabase))
	mux.HandleFunc("DELETE /api/databases/{id}", h.requireAuth(h.handleDeleteDatabase))
	mux.HandleFunc("GET /api/databases/{id}/snapshots", h.requireAuth(h.handleSnapshotsGated))
	mux.HandleFunc("POST /api/databases/{id}/snapshots", h.requireAuth(h.handleSnapshotsGated))

	mux.HandleFunc("POST /api/agent/heartbeat", h.handleAgentHeartbeat)

	mux.HandleFunc("GET /api/billing/invoices", h.requireAuth(h.handleListInvoices))
	mux.HandleFunc("POST /api/billing/invoices/{id}/pay", h.requireAuth(h.handlePayInvoice))

	mux.HandleFunc("GET /api/admin/overview", h.requireAdmin(h.handleAdminOverview))
	mux.HandleFunc("GET /api/admin/users", h.requireAdmin(h.handleAdminUsers))
	mux.HandleFunc("GET /api/admin/databases", h.requireAdmin(h.handleAdminDatabases))
	mux.HandleFunc("POST /api/admin/databases", h.requireAdmin(h.handleAdminCreateDatabase))
	mux.HandleFunc("DELETE /api/admin/databases/{id}", h.requireAdmin(h.handleAdminDeleteDatabase))
	mux.HandleFunc("POST /api/admin/databases/{id}/retry", h.requireAdmin(h.handleAdminRetryDatabase))
	mux.HandleFunc("GET /api/admin/nodes", h.requireAdmin(h.handleAdminNodes))
	mux.HandleFunc("GET /api/admin/jobs", h.requireAdmin(h.handleAdminJobs))
	mux.HandleFunc("GET /api/admin/database-metrics", h.requireAdmin(h.handleAdminDatabaseMetrics))

	return mux
}

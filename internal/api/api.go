// Package api wires the control-plane HTTP routes: auth, the databases
// API, the agent heartbeat endpoint, and billing. Provisioning and
// deletion are carried out asynchronously by internal/jobs, which calls
// the data-plane agent; this package only queues the work. It never
// touches S3 — snapshots are out of scope for this slice.
package api

import (
	"database/sql"
	"net/http"

	credpkg "github.com/madevara24/monorepo-back-rooms/internal/credentials"
	"github.com/madevara24/monorepo-back-rooms/internal/healthz"
)

// API holds the dependencies HTTP handlers need.
type API struct {
	DB *sql.DB
	// CookieSecure sets the Secure attribute on the session cookie. It
	// should be true behind TLS in production and false for local dev,
	// where the browser talks to the Vite proxy over plain HTTP.
	CookieSecure bool
	// Commit is the build's git SHA, reported by GET /healthz.
	Commit string
	// Cipher encrypts tenant database passwords at rest. It is required
	// by POST /api/databases.
	Cipher *credpkg.Cipher
}

// NewMux builds the complete control-plane router.
func NewMux(a *API) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz.NewHandler(a.Commit))

	mux.HandleFunc("POST /api/auth/signup", a.handleSignup)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	mux.HandleFunc("GET /api/me", a.requireAuth(a.handleMe))

	mux.HandleFunc("GET /api/databases", a.requireAuth(a.handleListDatabases))
	mux.HandleFunc("GET /api/databases/{id}", a.requireAuth(a.handleGetDatabase))
	mux.HandleFunc("POST /api/databases", a.requireAuth(a.handleCreateDatabase))
	mux.HandleFunc("DELETE /api/databases/{id}", a.requireAuth(a.handleDeleteDatabase))
	mux.HandleFunc("GET /api/databases/{id}/snapshots", a.requireAuth(a.handleSnapshotsGated))
	mux.HandleFunc("POST /api/databases/{id}/snapshots", a.requireAuth(a.handleSnapshotsGated))

	mux.HandleFunc("POST /api/agent/heartbeat", a.handleAgentHeartbeat)

	mux.HandleFunc("GET /api/billing/invoices", a.requireAuth(a.handleListInvoices))
	mux.HandleFunc("POST /api/billing/invoices/{id}/pay", a.requireAuth(a.handlePayInvoice))

	mux.HandleFunc("GET /api/admin/overview", a.requireAdmin(a.handleAdminOverview))
	mux.HandleFunc("GET /api/admin/users", a.requireAdmin(a.handleAdminUsers))
	mux.HandleFunc("GET /api/admin/databases", a.requireAdmin(a.handleAdminDatabases))
	mux.HandleFunc("GET /api/admin/nodes", a.requireAdmin(a.handleAdminNodes))
	mux.HandleFunc("GET /api/admin/jobs", a.requireAdmin(a.handleAdminJobs))

	return mux
}

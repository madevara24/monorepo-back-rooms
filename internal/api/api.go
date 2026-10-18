// Package api wires the control-plane HTTP routes: auth, the gated
// databases API, and billing. It never calls the data-plane agent and
// never touches S3 — this slice runs without either.
package api

import (
	"database/sql"
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/internal/healthz"
)

// API holds the dependencies HTTP handlers need.
type API struct {
	DB *sql.DB
	// CookieSecure sets the Secure attribute on the session cookie. It
	// should be true behind TLS in production and false for local dev,
	// where the browser talks to the Vite proxy over plain HTTP.
	CookieSecure bool
}

// NewMux builds the complete control-plane router.
func NewMux(a *API) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz.Handler)

	mux.HandleFunc("POST /api/auth/signup", a.handleSignup)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	mux.HandleFunc("GET /api/me", a.requireAuth(a.handleMe))

	mux.HandleFunc("GET /api/databases", a.requireAuth(a.handleListDatabases))
	mux.HandleFunc("GET /api/databases/{id}", a.requireAuth(a.handleGetDatabase))
	mux.HandleFunc("POST /api/databases", a.requireAuth(a.handleCreateDatabaseGated))
	mux.HandleFunc("DELETE /api/databases/{id}", a.requireAuth(a.handleDeleteDatabaseGated))
	mux.HandleFunc("GET /api/databases/{id}/snapshots", a.requireAuth(a.handleSnapshotsGated))
	mux.HandleFunc("POST /api/databases/{id}/snapshots", a.requireAuth(a.handleSnapshotsGated))

	mux.HandleFunc("GET /api/billing/invoices", a.requireAuth(a.handleListInvoices))
	mux.HandleFunc("POST /api/billing/invoices/{id}/pay", a.requireAuth(a.handlePayInvoice))

	return mux
}

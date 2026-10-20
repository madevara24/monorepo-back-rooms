// Package usecase holds the control plane's business logic: auth,
// provisioning's background jobs, and the SNI route reconciler. It
// orchestrates repository (Postgres) and integration (the agent and the
// proxy's host-map file) without owning either directly.
package usecase

import (
	"bytes"
	"fmt"
	"sort"
)

// Domain is the parent zone every tenant's SNI hostname is a subdomain
// of. A wildcard certificate and DNS record cover it; provisioning those
// is out of scope here.
const Domain = "db.mbr.madevara.com"

// PublicPort is the port every tenant connects to; the SNI proxy, not
// Postgres, listens here and forwards to each tenant's real internal
// port based on the hostname the client asked for.
const PublicPort = 5432

// Route is one line of the proxy's host map: the public hostname a
// tenant connects to and the internal node address it forwards to.
type Route struct {
	Slug string
	Host string
	Port int
}

// PublicHost returns the hostname a tenant connects to for slug.
func PublicHost(slug string) string {
	return slug + "." + Domain
}

// ConnectionString returns the full client-facing connection string for
// slug and password. The password is only ever available to the caller
// once, at creation time.
//
// The connection router (connection-router/) replaced nginx's
// ssl_preread on :5432: it reads the SNI out of a client's normal
// SSLRequest-then-ClientHello handshake, so sslnegotiation=direct is
// no longer required and is dropped here. sslrootcert=system points
// libpq at the OS trust store instead of ~/.postgresql/root.crt, which
// postgres:17 clients don't have.
func ConnectionString(slug, password string) string {
	return fmt.Sprintf("postgres://postgres:%s@%s:%d/postgres?sslmode=verify-full&sslrootcert=system", password, PublicHost(slug), PublicPort)
}

// JDBCConnectionString returns the default DBeaver/pgJDBC connection URL
// for slug. DBeaver rejects the postgres:// scheme ConnectionString uses.
//
// As with ConnectionString, sslNegotiation=direct is dropped: the
// connection router handles pgJDBC's normal SSLRequest handshake.
//
// This default only sets sslmode=require: the connection is encrypted but
// the client does not verify the server's identity. The stronger
// sslmode=verify-full form needs a root certificate path pgJDBC can read
// (the ISRG Root X1 PEM; sslrootcert=system is a libpq-only keyword pgJDBC
// does not understand), so it is documented in the UI rather than built
// here.
func JDBCConnectionString(slug string) string {
	return fmt.Sprintf("jdbc:postgresql://%s:%d/postgres?sslmode=require", PublicHost(slug), PublicPort)
}

// Render renders routes as the proxy's map file, one line per route
// sorted by hostname so the output is deterministic and integration's
// WriteAtomic can detect a no-op reconcile.
func Render(routes []Route) []byte {
	sorted := make([]Route, len(routes))
	copy(sorted, routes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	var buf bytes.Buffer
	for _, r := range sorted {
		fmt.Fprintf(&buf, "%s %s:%d;\n", PublicHost(r.Slug), r.Host, r.Port)
	}
	return buf.Bytes()
}

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
// sslnegotiation=direct skips the plaintext Postgres SSLRequest, so
// nginx's ssl_preread sees the TLS ClientHello's SNI on the first packet
// instead of routing to the default backend. sslrootcert=system points
// libpq at the OS trust store instead of ~/.postgresql/root.crt, which
// postgres:17 clients don't have.
//
// sslnegotiation=direct requires a PostgreSQL 17+ aware client: psql
// 17+, libpq 17+ (so psycopg2/psycopg3 need a matching system libpq),
// jackc/pgx v5.7.5+, node-postgres (pg) v8.22.0+, pgjdbc v42.7.4+, or
// asyncpg v0.32.0+ (earlier asyncpg versions parse the DSN keyword but
// omit the ALPN PostgreSQL 17 requires, so direct negotiation silently
// fails). Older client versions fall back to indirect negotiation with
// no client-side error, which the proxy then misroutes.
func ConnectionString(slug, password string) string {
	return fmt.Sprintf("postgres://postgres:%s@%s:%d/postgres?sslmode=verify-full&sslnegotiation=direct&sslrootcert=system", password, PublicHost(slug), PublicPort)
}

// JDBCConnectionString returns the default DBeaver/pgJDBC connection URL
// for slug. DBeaver rejects the postgres:// scheme ConnectionString uses,
// and pgJDBC's own direct-negotiation keyword is the camelCase
// sslNegotiation, not libpq's sslnegotiation.
//
// This default only sets sslmode=require: the connection is encrypted but
// the client does not verify the server's identity. The stronger
// sslmode=verify-full form needs a root certificate path pgJDBC can read
// (the ISRG Root X1 PEM; sslrootcert=system is a libpq-only keyword pgJDBC
// does not understand), so it is documented in the UI rather than built
// here.
//
// sslNegotiation=direct requires pgJDBC 42.7.4 or newer; older versions
// silently fall back to indirect negotiation, which the SNI proxy then
// misroutes with no client-side error.
func JDBCConnectionString(slug string) string {
	return fmt.Sprintf("jdbc:postgresql://%s:%d/postgres?sslmode=require&sslNegotiation=direct", PublicHost(slug), PublicPort)
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

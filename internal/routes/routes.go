// Package routes renders the SNI proxy's host map and the public
// connection string tenants use to reach their database. The proxy on KKB
// reads the rendered file directly; this package only produces its bytes
// and writes them atomically, no sockets or systemd/nginx of its own.
package routes

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
func ConnectionString(slug, password string) string {
	return fmt.Sprintf("postgres://postgres:%s@%s:%d/postgres?sslmode=verify-full", password, PublicHost(slug), PublicPort)
}

// Render renders routes as the proxy's map file, one line per route
// sorted by hostname so the output is deterministic and WriteAtomic can
// detect a no-op reconcile.
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

// WriteAtomic writes data to path through a temp file and rename, so a
// concurrent reader (the proxy's path unit) never observes a partial
// write. It skips the write, and reports false, when path already holds
// exactly data. path's directory must already exist.
func WriteAtomic(path string, data []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".mbr-routes-*")
	if err != nil {
		return false, fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return false, fmt.Errorf("rename into %s: %w", path, err)
	}
	return true, nil
}

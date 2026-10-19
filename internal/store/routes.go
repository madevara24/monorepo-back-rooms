package store

import (
	"context"
	"database/sql"
)

// RunningRoute is a running tenant database's SNI route: the slug a
// client connects through and the internal node address the proxy
// forwards to.
type RunningRoute struct {
	Slug string
	Host string
	Port int
}

// ListRunningRoutes returns the SNI route for every database joined with
// its node that is currently "running" with a slug, host, and port set.
// A database outside "running", or missing any of those columns, is left
// out: it has nothing a client could connect to yet.
func ListRunningRoutes(ctx context.Context, db *sql.DB) ([]RunningRoute, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT d.slug, d.host, d.port
		FROM databases d
		JOIN nodes n ON n.id = d.node_id
		WHERE d.status = 'running'
		  AND d.slug IS NOT NULL
		  AND d.host IS NOT NULL
		  AND d.port IS NOT NULL
		ORDER BY d.slug
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []RunningRoute{}
	for rows.Next() {
		var r RunningRoute
		if err := rows.Scan(&r.Slug, &r.Host, &r.Port); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

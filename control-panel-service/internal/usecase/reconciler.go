package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

// Reconciler keeps the SNI proxy's host map file in sync with the
// control plane's view of which databases are running.
type Reconciler struct {
	DB *sql.DB
	// Path is the proxy's map file, e.g.
	// /etc/mbr/routes/mbr-routes.map. Its directory must already exist.
	Path string
}

// Reconcile rewrites Path from the current set of running databases. It
// reports whether the file's contents changed.
func (r *Reconciler) Reconcile(ctx context.Context) (bool, error) {
	rows, err := repository.ListRunningRoutes(ctx, r.DB)
	if err != nil {
		return false, fmt.Errorf("list running routes: %w", err)
	}

	out := make([]Route, len(rows))
	for i, row := range rows {
		out[i] = Route{Slug: row.Slug, Host: row.Host, Port: row.Port}
	}

	return integration.WriteAtomic(r.Path, Render(out))
}

// Run calls Reconcile immediately and then every interval, until ctx is
// canceled. A failed reconcile is logged, not fatal: the next tick tries
// again.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := r.Reconcile(ctx); err != nil {
			log.Printf("routes: reconcile: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

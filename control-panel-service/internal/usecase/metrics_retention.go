package usecase

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

// MetricsRetention periodically deletes database_metrics rows older than
// RetentionDays, so the connection-pressure history does not grow
// unbounded.
type MetricsRetention struct {
	DB            *sql.DB
	RetentionDays int
}

// Prune deletes every database_metrics row older than RetentionDays and
// reports how many it removed.
func (m *MetricsRetention) Prune(ctx context.Context) (int64, error) {
	return repository.PruneDatabaseMetrics(ctx, m.DB, m.RetentionDays)
}

// Run calls Prune immediately and then every interval, until ctx is
// canceled. A failed prune is logged, not fatal: the next tick tries
// again.
func (m *MetricsRetention) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if n, err := m.Prune(ctx); err != nil {
			log.Printf("metrics: prune: %v", err)
		} else if n > 0 {
			log.Printf("metrics: pruned %d snapshot(s) older than %d day(s)", n, m.RetentionDays)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

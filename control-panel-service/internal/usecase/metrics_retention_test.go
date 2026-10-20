package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

func TestMetricsRetentionPruneRemovesOnlyStaleRows(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "retention-owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	var databaseID string
	if err := db.QueryRow(`
		INSERT INTO databases (user_id, name, size_gb, status) VALUES ($1, 'retention-demo', 5, 'running') RETURNING id
	`, owner.ID).Scan(&databaseID); err != nil {
		t.Fatalf("insert database fixture: %v", err)
	}

	stale := entities.DatabaseMetric{DatabaseID: databaseID, MaxConnections: 100, CollectedAt: time.Now().Add(-20 * 24 * time.Hour)}
	fresh := entities.DatabaseMetric{DatabaseID: databaseID, MaxConnections: 100, CollectedAt: time.Now()}
	if err := repository.InsertDatabaseMetric(ctx, db, stale); err != nil {
		t.Fatalf("InsertDatabaseMetric (stale): %v", err)
	}
	if err := repository.InsertDatabaseMetric(ctx, db, fresh); err != nil {
		t.Fatalf("InsertDatabaseMetric (fresh): %v", err)
	}

	r := &usecase.MetricsRetention{DB: db, RetentionDays: 14}
	n, err := r.Prune(ctx)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Fatalf("Prune removed %d row(s), want exactly 1", n)
	}

	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM database_metrics WHERE database_id = $1`, databaseID).Scan(&remaining); err != nil {
		t.Fatalf("count remaining metrics: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining database_metrics rows = %d, want 1", remaining)
	}
}

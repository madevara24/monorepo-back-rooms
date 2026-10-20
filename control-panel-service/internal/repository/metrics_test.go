package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
)

func metricFixture(databaseID string, active, max int, collectedAt time.Time) entities.DatabaseMetric {
	return entities.DatabaseMetric{
		DatabaseID:                   databaseID,
		Active:                       active,
		Idle:                         1,
		IdleInTransaction:            0,
		MaxConnections:               max,
		SuperuserReservedConnections: 3,
		MemoryLimitBytes:             512 * 1024 * 1024,
		CollectedAt:                  collectedAt,
	}
}

func TestLatestDatabaseMetricsReturnsOnlyTheNewestRowPerDatabase(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "metrics-owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	dbID := insertDatabase(t, db, owner.ID, "metrics-demo")

	older := metricFixture(dbID, 2, 100, time.Now().Add(-time.Hour))
	newer := metricFixture(dbID, 71, 100, time.Now())

	if err := repository.InsertDatabaseMetric(ctx, db, older); err != nil {
		t.Fatalf("InsertDatabaseMetric (older): %v", err)
	}
	if err := repository.InsertDatabaseMetric(ctx, db, newer); err != nil {
		t.Fatalf("InsertDatabaseMetric (newer): %v", err)
	}

	rows, err := repository.LatestDatabaseMetrics(ctx, db)
	if err != nil {
		t.Fatalf("LatestDatabaseMetrics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("LatestDatabaseMetrics = %+v, want exactly one row", rows)
	}
	if rows[0].Active != 71 || rows[0].DatabaseName != "metrics-demo" || rows[0].OwnerEmail != "metrics-owner@example.com" {
		t.Fatalf("LatestDatabaseMetrics[0] = %+v, want the newer row joined with name/owner", rows[0])
	}
}

func TestPruneDatabaseMetricsDeletesOnlyOlderThanRetention(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "prune-owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	dbID := insertDatabase(t, db, owner.ID, "prune-demo")

	stale := metricFixture(dbID, 1, 100, time.Now().Add(-15*24*time.Hour))
	fresh := metricFixture(dbID, 2, 100, time.Now().Add(-1*time.Hour))
	if err := repository.InsertDatabaseMetric(ctx, db, stale); err != nil {
		t.Fatalf("InsertDatabaseMetric (stale): %v", err)
	}
	if err := repository.InsertDatabaseMetric(ctx, db, fresh); err != nil {
		t.Fatalf("InsertDatabaseMetric (fresh): %v", err)
	}

	n, err := repository.PruneDatabaseMetrics(ctx, db, 14)
	if err != nil {
		t.Fatalf("PruneDatabaseMetrics: %v", err)
	}
	if n != 1 {
		t.Fatalf("PruneDatabaseMetrics removed %d row(s), want exactly 1 (the 15-day-old one)", n)
	}

	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM database_metrics WHERE database_id = $1`, dbID).Scan(&remaining); err != nil {
		t.Fatalf("count remaining metrics: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining database_metrics rows = %d, want 1 (the fresh one kept)", remaining)
	}
}

func TestInsertDatabaseMetricRejectsUnknownDatabase(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	m := metricFixture("00000000-0000-0000-0000-000000000000", 1, 100, time.Now())
	if err := repository.InsertDatabaseMetric(ctx, db, m); err == nil {
		t.Fatalf("InsertDatabaseMetric for an unknown database_id = nil error, want a foreign key violation")
	}
}

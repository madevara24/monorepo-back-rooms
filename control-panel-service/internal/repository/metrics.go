package repository

import (
	"context"
	"database/sql"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

// InsertDatabaseMetric records one connection-pressure snapshot the agent
// collected for a tenant database.
func InsertDatabaseMetric(ctx context.Context, db *sql.DB, m entities.DatabaseMetric) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO database_metrics (database_id, active, idle, idle_in_transaction, max_connections, superuser_reserved_connections, memory_limit_bytes, collected_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, m.DatabaseID, m.Active, m.Idle, m.IdleInTransaction, m.MaxConnections, m.SuperuserReservedConnections, m.MemoryLimitBytes, m.CollectedAt)
	return err
}

// DatabaseMetricRow is one row of GET /api/admin/database-metrics: a
// database's latest connection snapshot plus the context an admin page
// needs to label it, without a second round trip.
type DatabaseMetricRow struct {
	entities.DatabaseMetric
	DatabaseName string
	OwnerEmail   string
}

// LatestDatabaseMetrics returns each database's most recent
// connection-pressure snapshot, newest-collected database first.
func LatestDatabaseMetrics(ctx context.Context, db *sql.DB) ([]DatabaseMetricRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT ON (dm.database_id)
			dm.id, dm.database_id, dm.active, dm.idle, dm.idle_in_transaction,
			dm.max_connections, dm.superuser_reserved_connections, dm.memory_limit_bytes, dm.collected_at,
			d.name, u.email
		FROM database_metrics dm
		JOIN databases d ON d.id = dm.database_id
		JOIN users u ON u.id = d.user_id
		ORDER BY dm.database_id, dm.collected_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []DatabaseMetricRow{}
	for rows.Next() {
		var r DatabaseMetricRow
		if err := rows.Scan(&r.ID, &r.DatabaseID, &r.Active, &r.Idle, &r.IdleInTransaction,
			&r.MaxConnections, &r.SuperuserReservedConnections, &r.MemoryLimitBytes, &r.CollectedAt,
			&r.DatabaseName, &r.OwnerEmail); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneDatabaseMetrics deletes every snapshot older than retentionDays and
// returns how many rows it removed.
func PruneDatabaseMetrics(ctx context.Context, db *sql.DB, retentionDays int) (int64, error) {
	res, err := db.ExecContext(ctx, `
		DELETE FROM database_metrics WHERE collected_at < now() - ($1 * interval '1 day')
	`, retentionDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

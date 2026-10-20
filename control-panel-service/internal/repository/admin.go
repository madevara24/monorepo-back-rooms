package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

// AdminOverview is the set of counts the admin overview page shows: how
// many users exist, how many databases are in each status, how many nodes
// are fresh vs stale, and how many jobs are in each status. "Fresh" mirrors
// the window AnyAvailableNode uses to pick a provisioning target: a
// heartbeat within the last two minutes.
type AdminOverview struct {
	UserCount         int
	DatabasesByStatus map[string]int
	NodesFresh        int
	NodesStale        int
	JobsByStatus      map[string]int
}

// AdminGetOverview loads the counts behind GET /api/admin/overview.
func AdminGetOverview(ctx context.Context, db *sql.DB) (AdminOverview, error) {
	var overview AdminOverview

	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&overview.UserCount); err != nil {
		return AdminOverview{}, err
	}

	overview.DatabasesByStatus = map[string]int{}
	dbRows, err := db.QueryContext(ctx, `SELECT status, count(*) FROM databases GROUP BY status`)
	if err != nil {
		return AdminOverview{}, err
	}
	defer func() { _ = dbRows.Close() }()
	for dbRows.Next() {
		var status string
		var count int
		if err := dbRows.Scan(&status, &count); err != nil {
			return AdminOverview{}, err
		}
		overview.DatabasesByStatus[status] = count
	}
	if err := dbRows.Err(); err != nil {
		return AdminOverview{}, err
	}

	if err := db.QueryRowContext(ctx, `
		SELECT
			count(*) FILTER (WHERE last_heartbeat IS NOT NULL AND last_heartbeat > now() - interval '2 minutes'),
			count(*) FILTER (WHERE last_heartbeat IS NULL OR last_heartbeat <= now() - interval '2 minutes')
		FROM nodes
	`).Scan(&overview.NodesFresh, &overview.NodesStale); err != nil {
		return AdminOverview{}, err
	}

	overview.JobsByStatus = map[string]int{}
	jobRows, err := db.QueryContext(ctx, `SELECT status, count(*) FROM jobs GROUP BY status`)
	if err != nil {
		return AdminOverview{}, err
	}
	defer func() { _ = jobRows.Close() }()
	for jobRows.Next() {
		var status string
		var count int
		if err := jobRows.Scan(&status, &count); err != nil {
			return AdminOverview{}, err
		}
		overview.JobsByStatus[status] = count
	}
	if err := jobRows.Err(); err != nil {
		return AdminOverview{}, err
	}

	return overview, nil
}

// AdminUserRow is one row of GET /api/admin/users. DatabaseCount counts
// every database the user ever created, including deleted ones, since
// this is an audit view rather than the tenant's own list.
type AdminUserRow struct {
	ID            string
	Email         string
	CreatedAt     time.Time
	DatabaseCount int
}

// AdminListUsers returns every user with a count of the databases they
// have ever created, newest user first.
func AdminListUsers(ctx context.Context, db *sql.DB) ([]AdminUserRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT u.id, u.email, u.created_at, count(d.id)
		FROM users u
		LEFT JOIN databases d ON d.user_id = u.id
		GROUP BY u.id, u.email, u.created_at
		ORDER BY u.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []AdminUserRow{}
	for rows.Next() {
		var r AdminUserRow
		if err := rows.Scan(&r.ID, &r.Email, &r.CreatedAt, &r.DatabaseCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdminDatabaseRow is one row of GET /api/admin/databases.
type AdminDatabaseRow struct {
	ID         string
	Name       string
	SizeGB     int
	Status     string
	OwnerEmail string
	NodeID     *string
	Host       *string
	Port       *int
	CreatedAt  time.Time
}

// AdminListDatabases returns every database row, including deleted ones,
// joined with its owner's email, newest first.
func AdminListDatabases(ctx context.Context, db *sql.DB) ([]AdminDatabaseRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT d.id, d.name, d.size_gb, d.status, u.email, d.node_id, d.host, d.port, d.created_at
		FROM databases d
		JOIN users u ON u.id = d.user_id
		ORDER BY d.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []AdminDatabaseRow{}
	for rows.Next() {
		var r AdminDatabaseRow
		if err := rows.Scan(&r.ID, &r.Name, &r.SizeGB, &r.Status, &r.OwnerEmail, &r.NodeID, &r.Host, &r.Port, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AdminNodeRow is one row of GET /api/admin/nodes, with the containers it
// last reported in a heartbeat.
type AdminNodeRow struct {
	entities.Node
	Containers []entities.NodeContainer
}

// AdminListNodes returns every node with the containers it last reported,
// as two queries assembled in Go rather than a join, since a join would
// repeat each node's columns once per container.
func AdminListNodes(ctx context.Context, db *sql.DB) ([]AdminNodeRow, error) {
	nodeRows, err := db.QueryContext(ctx, `
		SELECT `+nodeColumns+`
		FROM nodes
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = nodeRows.Close() }()

	out := []AdminNodeRow{}
	byID := map[string]int{}
	for nodeRows.Next() {
		n, err := scanNode(nodeRows)
		if err != nil {
			return nil, err
		}
		out = append(out, AdminNodeRow{Node: n, Containers: []entities.NodeContainer{}})
		byID[n.ID] = len(out) - 1
	}
	if err := nodeRows.Err(); err != nil {
		return nil, err
	}

	containerRows, err := db.QueryContext(ctx, `
		SELECT node_id, database_id, status, port, updated_at
		FROM node_containers
		ORDER BY node_id, database_id
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = containerRows.Close() }()

	for containerRows.Next() {
		var c entities.NodeContainer
		if err := containerRows.Scan(&c.NodeID, &c.DatabaseID, &c.Status, &c.Port, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if i, ok := byID[c.NodeID]; ok {
			out[i].Containers = append(out[i].Containers, c)
		}
	}
	return out, containerRows.Err()
}

// adminJobsLimit bounds how many recent jobs GET /api/admin/jobs returns.
const adminJobsLimit = 200

// AdminListJobs returns the most recently created jobs, newest first.
func AdminListJobs(ctx context.Context, db *sql.DB) ([]entities.Job, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+jobColumns+`
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1
	`, adminJobsLimit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []entities.Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// AdminForceDeleteDatabase moves any owner's database to "deleting" and
// queues the "delete" job that will remove it, regardless of who owns it.
// It mirrors SetDatabaseDeletingAndQueueDelete but without the owner
// scope, since an admin can force-delete a database it does not own. It
// returns sql.ErrNoRows if id does not exist, and ErrInvalidState if the
// row is not "running" or "failed". There is no final snapshot: this is
// the same settled test-stage decision as the tenant delete path.
func AdminForceDeleteDatabase(ctx context.Context, db *sql.DB, id string) (entities.Database, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return entities.Database{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Lock the row first so two concurrent calls serialize: the second
	// sees the "deleting" status the first wrote and is rejected instead
	// of queuing a duplicate delete job.
	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status FROM databases WHERE id = $1 FOR UPDATE
	`, id).Scan(&status); err != nil {
		return entities.Database{}, err
	}
	if status != "running" && status != "failed" {
		return entities.Database{}, ErrInvalidState
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE databases SET status = 'deleting', updated_at = now()
		WHERE id = $1 AND status IN ('running', 'failed')
		RETURNING `+databaseColumns, id)
	d, err := scanDatabase(row)
	if err != nil {
		return entities.Database{}, err
	}

	payload, err := json.Marshal(map[string]string{"database_id": d.ID})
	if err != nil {
		return entities.Database{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jobs (kind, payload) VALUES ('delete', $1)`, payload); err != nil {
		return entities.Database{}, err
	}

	if err := tx.Commit(); err != nil {
		return entities.Database{}, err
	}
	return d, nil
}

// AdminRetryDatabase requeues a "failed" database's last operation. A
// database that never finished provisioning has no host yet, so it moves
// back to "provisioning" and gets a fresh provision job; one that failed
// while being deleted still carries the host and port it had while
// running, so it moves back to "deleting" and gets a fresh delete job.
// It returns sql.ErrNoRows if id does not exist, and ErrInvalidState if
// the row is not "failed".
func AdminRetryDatabase(ctx context.Context, db *sql.DB, id string) (entities.Database, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return entities.Database{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	var host sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT status, host FROM databases WHERE id = $1 FOR UPDATE
	`, id).Scan(&status, &host); err != nil {
		return entities.Database{}, err
	}
	if status != "failed" {
		return entities.Database{}, ErrInvalidState
	}

	newStatus, jobKind := "provisioning", "provision"
	if host.Valid {
		newStatus, jobKind = "deleting", "delete"
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE databases SET status = $2, updated_at = now()
		WHERE id = $1 AND status = 'failed'
		RETURNING `+databaseColumns, id, newStatus)
	d, err := scanDatabase(row)
	if err != nil {
		return entities.Database{}, err
	}

	payload, err := json.Marshal(map[string]string{"database_id": d.ID})
	if err != nil {
		return entities.Database{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jobs (kind, payload) VALUES ($1, $2)`, jobKind, payload); err != nil {
		return entities.Database{}, err
	}

	if err := tx.Commit(); err != nil {
		return entities.Database{}, err
	}
	return d, nil
}

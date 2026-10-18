package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// ErrSlugTaken is returned by CreateDatabaseAndQueueProvision when the
// generated slug collides with an existing one; callers should generate a
// new slug and retry.
var ErrSlugTaken = errors.New("slug already taken")

const databaseColumns = `id, user_id, name, size_gb, status, slug, node_id, host, port, password_encrypted, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDatabase(row rowScanner) (Database, error) {
	var d Database
	err := row.Scan(&d.ID, &d.UserID, &d.Name, &d.SizeGB, &d.Status, &d.Slug, &d.NodeID,
		&d.Host, &d.Port, &d.PasswordEncrypted, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

// ListDatabasesByUser returns every non-deleted database row the user
// owns, newest first. Deleted rows are hidden from this list; callers that
// need them, such as DatabaseByIDForUser, look them up directly.
func ListDatabasesByUser(ctx context.Context, db *sql.DB, userID string) ([]Database, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+databaseColumns+`
		FROM databases
		WHERE user_id = $1 AND status != 'deleted'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []Database{}
	for rows.Next() {
		d, err := scanDatabase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DatabaseByIDForUser returns a database row scoped to its owner. It
// returns sql.ErrNoRows if the id does not exist or belongs to someone
// else, so callers can map both cases to 404.
func DatabaseByIDForUser(ctx context.Context, db *sql.DB, id, userID string) (Database, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+databaseColumns+`
		FROM databases
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	return scanDatabase(row)
}

// CreateDatabaseAndQueueProvision inserts a new database row in
// "provisioning" status and queues the "provision" job that will carry it
// to "running", as a single transaction so a database row is never left
// without a job to drive it. It returns ErrSlugTaken if slug collides with
// an existing row, so the caller can generate a new one and retry.
func CreateDatabaseAndQueueProvision(ctx context.Context, db *sql.DB, userID, name string, sizeGB int, slug, nodeID string, passwordEncrypted []byte) (Database, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Database{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO databases (user_id, name, size_gb, status, slug, node_id, password_encrypted)
		VALUES ($1, $2, $3, 'provisioning', $4, $5, $6)
		RETURNING `+databaseColumns, userID, name, sizeGB, slug, nodeID, passwordEncrypted)
	d, err := scanDatabase(row)
	if isUniqueViolation(err) {
		return Database{}, ErrSlugTaken
	}
	if err != nil {
		return Database{}, err
	}

	payload, err := json.Marshal(map[string]string{"database_id": d.ID})
	if err != nil {
		return Database{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jobs (kind, payload) VALUES ('provision', $1)`, payload); err != nil {
		return Database{}, err
	}

	if err := tx.Commit(); err != nil {
		return Database{}, err
	}
	return d, nil
}

// SetDatabaseDeletingAndQueueDelete moves a database row to "deleting" and
// queues the "delete" job that will carry it to "deleted", as a single
// transaction. It returns sql.ErrNoRows if the id does not exist or
// belongs to someone else.
func SetDatabaseDeletingAndQueueDelete(ctx context.Context, db *sql.DB, id, userID string) (Database, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Database{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		UPDATE databases SET status = 'deleting', updated_at = now()
		WHERE id = $1 AND user_id = $2
		RETURNING `+databaseColumns, id, userID)
	d, err := scanDatabase(row)
	if err != nil {
		return Database{}, err
	}

	payload, err := json.Marshal(map[string]string{"database_id": d.ID})
	if err != nil {
		return Database{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jobs (kind, payload) VALUES ('delete', $1)`, payload); err != nil {
		return Database{}, err
	}

	if err := tx.Commit(); err != nil {
		return Database{}, err
	}
	return d, nil
}

// DatabaseJobInfo is a database row joined with its node's agent address,
// the shape the job worker needs to call the agent.
type DatabaseJobInfo struct {
	Database
	NodeAddress string
}

// DatabaseForJob loads a database row along with its node's address. It
// returns sql.ErrNoRows if the database or its node no longer exists.
func DatabaseForJob(ctx context.Context, db *sql.DB, id string) (DatabaseJobInfo, error) {
	row := db.QueryRowContext(ctx, `
		SELECT d.id, d.user_id, d.name, d.size_gb, d.status, d.slug, d.node_id, d.host, d.port,
		       d.password_encrypted, d.created_at, d.updated_at, n.address
		FROM databases d
		JOIN nodes n ON n.id = d.node_id
		WHERE d.id = $1
	`, id)

	var info DatabaseJobInfo
	err := row.Scan(&info.ID, &info.UserID, &info.Name, &info.SizeGB, &info.Status, &info.Slug,
		&info.NodeID, &info.Host, &info.Port, &info.PasswordEncrypted, &info.CreatedAt, &info.UpdatedAt,
		&info.NodeAddress)
	return info, err
}

// MarkDatabaseRunning records that the agent confirmed a tenant container
// is up, with the host and port the client will connect to.
func MarkDatabaseRunning(ctx context.Context, db *sql.DB, id, host string, port int) error {
	_, err := db.ExecContext(ctx, `
		UPDATE databases SET status = 'running', host = $2, port = $3, updated_at = now()
		WHERE id = $1
	`, id, host, port)
	return err
}

// MarkDatabaseDeleted records that the agent removed a tenant container
// and its volume.
func MarkDatabaseDeleted(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE databases SET status = 'deleted', updated_at = now() WHERE id = $1
	`, id)
	return err
}

// MarkDatabaseFailed records that a provision or delete job exhausted its
// retries.
func MarkDatabaseFailed(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE databases SET status = 'failed', updated_at = now() WHERE id = $1
	`, id)
	return err
}

// InsertDatabaseEvent appends a row to the database's status-transition
// audit log. reason is nil for ordinary success transitions.
func InsertDatabaseEvent(ctx context.Context, db *sql.DB, databaseID, fromStatus, toStatus string, reason *string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO database_events (database_id, from_status, to_status, reason)
		VALUES ($1, $2, $3, $4)
	`, databaseID, fromStatus, toStatus, reason)
	return err
}

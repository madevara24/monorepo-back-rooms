package store

import (
	"context"
	"database/sql"
)

// ListDatabasesByUser returns every database row the user owns, newest
// first.
func ListDatabasesByUser(ctx context.Context, db *sql.DB, userID string) ([]Database, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, name, size_gb, status, created_at
		FROM databases
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []Database{}
	for rows.Next() {
		var d Database
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.SizeGB, &d.Status, &d.CreatedAt); err != nil {
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
	var d Database
	err := db.QueryRowContext(ctx, `
		SELECT id, user_id, name, size_gb, status, created_at
		FROM databases
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(&d.ID, &d.UserID, &d.Name, &d.SizeGB, &d.Status, &d.CreatedAt)
	return d, err
}

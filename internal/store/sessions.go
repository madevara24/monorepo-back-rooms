package store

import (
	"context"
	"database/sql"
	"time"
)

// CreateSession inserts a new session row.
func CreateSession(ctx context.Context, db *sql.DB, tokenHash, userID string, expiresAt time.Time) (Session, error) {
	var s Session
	err := db.QueryRowContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
		RETURNING token_hash, user_id, created_at, expires_at
	`, tokenHash, userID, expiresAt).Scan(&s.TokenHash, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	return s, err
}

// SessionByTokenHash looks up an unexpired session. It returns
// sql.ErrNoRows if none matches or the session has expired.
func SessionByTokenHash(ctx context.Context, db *sql.DB, tokenHash string) (Session, error) {
	var s Session
	err := db.QueryRowContext(ctx, `
		SELECT token_hash, user_id, created_at, expires_at
		FROM sessions
		WHERE token_hash = $1 AND expires_at > now()
	`, tokenHash).Scan(&s.TokenHash, &s.UserID, &s.CreatedAt, &s.ExpiresAt)
	return s, err
}

// DeleteSession removes a session row. It is not an error if the row does
// not exist.
func DeleteSession(ctx context.Context, db *sql.DB, tokenHash string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

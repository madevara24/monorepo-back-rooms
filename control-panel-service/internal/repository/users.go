package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

// ErrEmailTaken is returned by CreateUser when the email is already
// registered.
var ErrEmailTaken = errors.New("email already registered")

const userColumns = `id, email, password_hash, is_admin, is_seed, created_at`

func scanUser(row rowScanner) (entities.User, error) {
	var u entities.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.IsSeed, &u.CreatedAt)
	return u, err
}

// CreateUser inserts a new user and returns the row it created.
func CreateUser(ctx context.Context, db *sql.DB, email, passwordHash string) (entities.User, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING `+userColumns, email, passwordHash)
	u, err := scanUser(row)
	if isUniqueViolation(err) {
		return entities.User{}, ErrEmailTaken
	}
	return u, err
}

// UpsertSeedAdmin creates or updates the one demo admin account and
// clears the seed flag from any other row. It runs before the reset loop
// starts, so the first boot catch-up cannot delete the owner's account.
func UpsertSeedAdmin(ctx context.Context, db *sql.DB, email, passwordHash string) (entities.User, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return entities.User{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET is_seed = FALSE WHERE email <> $1 AND is_seed
	`, email); err != nil {
		return entities.User{}, err
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, is_admin, is_seed)
		VALUES ($1, $2, TRUE, TRUE)
		ON CONFLICT (email) DO UPDATE
		SET password_hash = EXCLUDED.password_hash, is_admin = TRUE, is_seed = TRUE
		RETURNING `+userColumns, email, passwordHash)
	u, err := scanUser(row)
	if err != nil {
		return entities.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return entities.User{}, err
	}
	return u, nil
}

// UserByEmail looks up a user by email. It returns sql.ErrNoRows if none
// matches.
func UserByEmail(ctx context.Context, db *sql.DB, email string) (entities.User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE email = $1
	`, email)
	return scanUser(row)
}

// UserByID looks up a user by id. It returns sql.ErrNoRows if none matches.
func UserByID(ctx context.Context, db *sql.DB, id string) (entities.User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE id = $1
	`, id)
	return scanUser(row)
}

// isUniqueViolation reports whether err is a Postgres unique_violation
// (SQLSTATE 23505), without importing the pgconn error type into every
// caller.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type sqlState interface{ SQLState() string }
	var se sqlState
	for e := err; e != nil; e = errors.Unwrap(e) {
		if s, ok := e.(sqlState); ok {
			se = s
			break
		}
	}
	return se != nil && se.SQLState() == "23505"
}

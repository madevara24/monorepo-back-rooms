package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrEmailTaken is returned by CreateUser when the email is already
// registered.
var ErrEmailTaken = errors.New("email already registered")

const userColumns = `id, email, password_hash, is_admin, created_at`

func scanUser(row rowScanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	return u, err
}

// CreateUser inserts a new user and returns the row it created.
func CreateUser(ctx context.Context, db *sql.DB, email, passwordHash string) (User, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING `+userColumns, email, passwordHash)
	u, err := scanUser(row)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return u, err
}

// UserByEmail looks up a user by email. It returns sql.ErrNoRows if none
// matches.
func UserByEmail(ctx context.Context, db *sql.DB, email string) (User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+userColumns+`
		FROM users
		WHERE email = $1
	`, email)
	return scanUser(row)
}

// UserByID looks up a user by id. It returns sql.ErrNoRows if none matches.
func UserByID(ctx context.Context, db *sql.DB, id string) (User, error) {
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

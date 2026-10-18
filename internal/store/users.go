package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrEmailTaken is returned by CreateUser when the email is already
// registered.
var ErrEmailTaken = errors.New("email already registered")

// CreateUser inserts a new user and returns the row it created.
func CreateUser(ctx context.Context, db *sql.DB, email, passwordHash string) (User, error) {
	var u User
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at
	`, email, passwordHash).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	return u, err
}

// UserByEmail looks up a user by email. It returns sql.ErrNoRows if none
// matches.
func UserByEmail(ctx context.Context, db *sql.DB, email string) (User, error) {
	var u User
	err := db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

// UserByID looks up a user by id. It returns sql.ErrNoRows if none matches.
func UserByID(ctx context.Context, db *sql.DB, id string) (User, error) {
	var u User
	err := db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
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

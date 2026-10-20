package usecase

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/password"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/session"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

// ErrInvalidEmail is returned by Signup when the email has no "@".
var ErrInvalidEmail = errors.New("email is invalid")

// ErrPasswordTooShort is returned by Signup when the password is under
// eight characters.
var ErrPasswordTooShort = errors.New("password must be at least 8 characters")

// ErrInvalidCredentials is returned by Login when the email is unknown or
// the password does not match.
var ErrInvalidCredentials = errors.New("invalid email or password")

// Auth is the control plane's account and session usecase: signup,
// login, logout, and resolving a session token back to a user.
type Auth struct {
	DB *sql.DB
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Signup validates email and password, hashes the password, and creates
// the account. It returns repository.ErrEmailTaken if the email is
// already registered.
func (a *Auth) Signup(ctx context.Context, email, pw string) (entities.User, error) {
	email = normalizeEmail(email)
	if !strings.Contains(email, "@") {
		return entities.User{}, ErrInvalidEmail
	}
	if len(pw) < 8 {
		return entities.User{}, ErrPasswordTooShort
	}

	hash, err := password.HashPassword(pw)
	if err != nil {
		return entities.User{}, err
	}
	return repository.CreateUser(ctx, a.DB, email, hash)
}

// Login verifies email and password and returns the matching user. It
// returns ErrInvalidCredentials if the email is unknown or the password
// does not match, without distinguishing the two.
func (a *Auth) Login(ctx context.Context, email, pw string) (entities.User, error) {
	user, err := repository.UserByEmail(ctx, a.DB, normalizeEmail(email))
	if err != nil {
		return entities.User{}, ErrInvalidCredentials
	}

	ok, err := password.VerifyPassword(user.PasswordHash, pw)
	if err != nil || !ok {
		return entities.User{}, ErrInvalidCredentials
	}
	return user, nil
}

// StartSession creates a session row for userID and returns the raw
// token and its expiry, for the caller to set as the session cookie.
func (a *Auth) StartSession(ctx context.Context, userID string) (token string, expiresAt time.Time, err error) {
	token, err = session.GenerateToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt = time.Now().Add(session.SessionTTL)
	if _, err := repository.CreateSession(ctx, a.DB, session.HashToken(token), userID, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Logout deletes the session a raw token resolves to. It is not an error
// if the token does not resolve to anything.
func (a *Auth) Logout(ctx context.Context, token string) {
	_ = repository.DeleteSession(ctx, a.DB, session.HashToken(token))
}

// UserFromToken resolves a raw session token to its user. It returns
// sql.ErrNoRows if the token is missing, unknown, or expired.
func (a *Auth) UserFromToken(ctx context.Context, token string) (entities.User, error) {
	s, err := repository.SessionByTokenHash(ctx, a.DB, session.HashToken(token))
	if err != nil {
		return entities.User{}, err
	}
	return repository.UserByID(ctx, a.DB, s.UserID)
}

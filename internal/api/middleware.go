package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/internal/auth"
	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

type ctxKey int

const userCtxKey ctxKey = 0

// requireAuth resolves the session cookie to a user and rejects the
// request with 401 if it is missing, unknown, or expired.
func (a *API) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := a.userFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (a *API) userFromRequest(r *http.Request) (store.User, error) {
	cookie, err := r.Cookie(auth.CookieName)
	if err != nil {
		return store.User{}, err
	}

	session, err := store.SessionByTokenHash(r.Context(), a.DB, auth.HashToken(cookie.Value))
	if err != nil {
		return store.User{}, err
	}

	return store.UserByID(r.Context(), a.DB, session.UserID)
}

func userFromContext(r *http.Request) store.User {
	user, _ := r.Context().Value(userCtxKey).(store.User)
	return user
}

// requireAdmin wraps requireAuth and additionally rejects the request with
// 403 if the signed-in user is not an admin. A missing or expired session
// still 401s via requireAuth; 403 means "signed in, but not allowed here".
func (a *API) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return a.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if !userFromContext(r).IsAdmin {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next(w, r)
	})
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

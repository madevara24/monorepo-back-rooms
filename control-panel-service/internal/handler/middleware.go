package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/session"
)

type ctxKey int

const userCtxKey ctxKey = 0

// requireAuth resolves the session cookie to a user and rejects the
// request with 401 if it is missing, unknown, or expired.
func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(session.CookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		user, err := h.Auth.UserFromToken(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next(w, r.WithContext(ctx))
	}
}

func userFromContext(r *http.Request) entities.User {
	user, _ := r.Context().Value(userCtxKey).(entities.User)
	return user
}

// requireAdmin wraps requireAuth and additionally rejects the request with
// 403 if the signed-in user is not an admin. A missing or expired session
// still 401s via requireAuth; 403 means "signed in, but not allowed here".
func (h *Handler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return h.requireAuth(func(w http.ResponseWriter, r *http.Request) {
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

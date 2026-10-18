package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/madevara24/monorepo-back-rooms/internal/auth"
	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u store.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (a *API) handleSignup(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := decodeJSON(r, &creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := normalizeEmail(creds.Email)
	if !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "email is invalid")
		return
	}
	if len(creds.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(creds.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	user, err := store.CreateUser(r.Context(), a.DB, email, hash)
	switch {
	case errors.Is(err, store.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email is already registered")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	if err := a.startSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	writeJSON(w, http.StatusCreated, toUserResponse(user))
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := decodeJSON(r, &creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := store.UserByEmail(r.Context(), a.DB, normalizeEmail(creds.Email))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	ok, err := auth.VerifyPassword(user.PasswordHash, creds.Password)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := a.startSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.CookieName); err == nil {
		_ = store.DeleteSession(r.Context(), a.DB, auth.HashToken(cookie.Value))
	}
	a.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toUserResponse(userFromContext(r)))
}

// startSession creates a session row and sets the session cookie on the
// response.
func (a *API) startSession(w http.ResponseWriter, r *http.Request, userID string) error {
	token, err := auth.GenerateToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(auth.SessionTTL)
	if _, err := store.CreateSession(r.Context(), a.DB, auth.HashToken(token), userID, expiresAt); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

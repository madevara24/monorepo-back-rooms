package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/session"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u entities.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, IsAdmin: u.IsAdmin, CreatedAt: u.CreatedAt}
}

func (h *Handler) handleSignup(w http.ResponseWriter, r *http.Request) {
	var creds credentialsRequest
	if err := decodeJSON(r, &creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.Auth.Signup(r.Context(), creds.Email, creds.Password)
	switch {
	case errors.Is(err, usecase.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "email is invalid")
		return
	case errors.Is(err, usecase.ErrPasswordTooShort):
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	case errors.Is(err, repository.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email is already registered")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not create account")
		return
	}

	if err := h.startSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	writeJSON(w, http.StatusCreated, toUserResponse(user))
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds credentialsRequest
	if err := decodeJSON(r, &creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.Auth.Login(r.Context(), creds.Email, creds.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := h.startSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(session.CookieName); err == nil {
		h.Auth.Logout(r.Context(), cookie.Value)
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toUserResponse(userFromContext(r)))
}

// startSession creates a session row and sets the session cookie on the
// response.
func (h *Handler) startSession(w http.ResponseWriter, r *http.Request, userID string) error {
	token, expiresAt, err := h.Auth.StartSession(r.Context(), userID)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     session.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     session.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

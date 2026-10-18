package api

import (
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

type databaseResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SizeGB    int       `json:"size_gb"`
	Status    string    `json:"status"`
	Host      *string   `json:"host"`
	Port      *int      `json:"port"`
	CreatedAt time.Time `json:"created_at"`
}

func toDatabaseResponse(d store.Database) databaseResponse {
	return databaseResponse{
		ID:        d.ID,
		Name:      d.Name,
		SizeGB:    d.SizeGB,
		Status:    d.Status,
		Host:      d.Host,
		Port:      d.Port,
		CreatedAt: d.CreatedAt,
	}
}

func (a *API) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	rows, err := store.ListDatabasesByUser(r.Context(), a.DB, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list databases")
		return
	}

	out := make([]databaseResponse, len(rows))
	for i, d := range rows {
		out[i] = toDatabaseResponse(d)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	d, err := store.DatabaseByIDForUser(r.Context(), a.DB, r.PathValue("id"), user.ID)
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load database")
		return
	}
	writeJSON(w, http.StatusOK, toDatabaseResponse(d))
}

type createDatabaseRequest struct {
	Name   string `json:"name"`
	SizeGB int    `json:"size_gb"`
}

type createDatabaseResponse struct {
	ID       string `json:"id"`
	Password string `json:"password"`
}

// maxSlugAttempts bounds how many times handleCreateDatabase retries a
// fresh random slug after a collision, before giving up.
const maxSlugAttempts = 5

// handleCreateDatabase provisions a new tenant database: it picks a node,
// generates a slug and a password, encrypts the password at rest, and
// queues the "provision" job that will carry the row to "running". The
// plaintext password is returned here and only here.
func (a *API) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	var req createDatabaseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.SizeGB < 1 || req.SizeGB > 20 {
		writeError(w, http.StatusBadRequest, "size_gb must be between 1 and 20")
		return
	}

	node, err := store.AnyAvailableNode(r.Context(), a.DB)
	if isNotFound(err) {
		writeError(w, http.StatusServiceUnavailable, "no node available")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not find a node")
		return
	}

	password, err := randomPassword()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create database")
		return
	}
	encrypted, err := a.Cipher.Encrypt(password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create database")
		return
	}

	var created store.Database
	for attempt := 0; attempt < maxSlugAttempts; attempt++ {
		slug, err := newSlug(req.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create database")
			return
		}

		created, err = store.CreateDatabaseAndQueueProvision(r.Context(), a.DB, user.ID, req.Name, req.SizeGB, slug, node.ID, encrypted)
		if errors.Is(err, store.ErrSlugTaken) {
			continue
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create database")
			return
		}
		break
	}
	if created.ID == "" {
		writeError(w, http.StatusInternalServerError, "could not generate a unique slug")
		return
	}

	writeJSON(w, http.StatusCreated, createDatabaseResponse{ID: created.ID, Password: password})
}

// handleDeleteDatabase moves a database to "deleting" and queues the
// "delete" job that will remove its container and volume. There is no
// final snapshot: this is a settled test-stage decision.
func (a *API) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	d, err := store.SetDatabaseDeletingAndQueueDelete(r.Context(), a.DB, r.PathValue("id"), user.ID)
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if errors.Is(err, store.ErrInvalidState) {
		writeError(w, http.StatusConflict, "database is not in a deletable state")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete database")
		return
	}
	writeJSON(w, http.StatusAccepted, toDatabaseResponse(d))
}

// handleSnapshotsGated covers every snapshot route. Snapshots need the
// data-plane agent and an S3 bucket; wiring S3 is out of scope for this
// slice.
func (a *API) handleSnapshotsGated(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "snapshots are not implemented yet")
}

const slugAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// newSlug derives a URL-safe, likely-unique slug from name: a
// lowercased, hyphenated prefix plus a short random suffix that resolves
// any collision. The caller retries on store.ErrSlugTaken.
func newSlug(name string) (string, error) {
	suffix, err := randomAlphabet(slugAlphabet, 6)
	if err != nil {
		return "", err
	}
	base := slugify(name)
	if base == "" {
		return suffix, nil
	}
	return base + "-" + suffix, nil
}

// slugify lowercases name and replaces every run of non-alphanumeric
// characters with a single hyphen, trimming hyphens from both ends and
// capping the result so the generated slug stays a reasonable length.
func slugify(name string) string {
	const maxLen = 32

	var b strings.Builder
	lastHyphen := true // trims a leading hyphen for free
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}

	out := strings.TrimSuffix(b.String(), "-")
	if len(out) > maxLen {
		out = strings.TrimSuffix(out[:maxLen], "-")
	}
	return out
}

// randomPassword returns a random 32-character alphanumeric password.
func randomPassword() (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	return randomAlphabet(alphabet, 32)
}

func randomAlphabet(alphabet string, n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i, v := range b {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b), nil
}

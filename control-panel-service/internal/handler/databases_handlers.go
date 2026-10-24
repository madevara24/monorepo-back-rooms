package handler

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

// databaseResponse is the tenant-facing shape of a database row. It
// reports the public hostname a client connects through, never the
// internal node address; admin responses keep that separately (see
// adminDatabaseResponse in admin_handlers.go). PlanID and PriceIDRMonth
// are nil for a database created without picking a plan.
type databaseResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	SizeGB        int       `json:"size_gb"`
	Status        string    `json:"status"`
	PublicHost    *string   `json:"public_host"`
	CreatedAt     time.Time `json:"created_at"`
	PlanID        *string   `json:"plan_id"`
	PriceIDRMonth *int64    `json:"price_idr_month"`
}

func toDatabaseResponse(d entities.Database) databaseResponse {
	var publicHost *string
	if d.Slug != nil {
		h := usecase.PublicHost(*d.Slug)
		publicHost = &h
	}
	return databaseResponse{
		ID:            d.ID,
		Name:          d.Name,
		SizeGB:        d.SizeGB,
		Status:        d.Status,
		PublicHost:    publicHost,
		CreatedAt:     d.CreatedAt,
		PlanID:        d.PlanID,
		PriceIDRMonth: d.PriceIDRMonth,
	}
}

func (h *Handler) handleListDatabases(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	rows, err := repository.ListDatabasesByUser(r.Context(), h.DB, user.ID)
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

func (h *Handler) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	d, err := repository.DatabaseByIDForUser(r.Context(), h.DB, r.PathValue("id"), user.ID)
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
	Name   string  `json:"name"`
	SizeGB *int    `json:"size_gb"`
	PlanID *string `json:"plan_id"`
}

// defaultPricePerGBIDRMonth is the flat per-GB price charged when a
// database is created without picking a plan, e.g. through
// POST /api/admin/databases. It mirrors the price control-web has always
// charged for a free-form size_gb.
const defaultPricePerGBIDRMonth = 10000

// createDatabaseResponse is returned exactly once, by the create
// endpoints, since it is the only time the plaintext password is
// available to build the full connection string.
type createDatabaseResponse struct {
	ID               string `json:"id"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Database         string `json:"database"`
	User             string `json:"user"`
	Password         string `json:"password"`
	ConnectionString string `json:"connection_string"`
	JDBCURL          string `json:"jdbc_url"`
}

// maxSlugAttempts bounds how many times provisionDatabase retries a fresh
// random slug after a collision, before giving up.
const maxSlugAttempts = 5

// maxDemoDatabases is the per-account product guard for the demo.
const maxDemoDatabases = 2

// errNoSlugAvailable is returned by provisionDatabase when every slug
// attempt collided.
var errNoSlugAvailable = errors.New("could not generate a unique slug")

// errDemoMaintenance is returned while a demo reset is running.
var errDemoMaintenance = errors.New("demo is resetting")

// provisionDatabase is the one provisioning path shared by the tenant's
// POST /api/databases and the admin's POST /api/admin/databases: it picks
// a node, generates a slug and a password, encrypts the password at rest,
// and queues the "provision" job that will carry the row to "running".
// The plaintext password is returned here and only here. planID is nil
// for a database created without picking a plan; priceIDRMonth is the
// price snapshotted onto the row either way. It returns whatever error
// repository.AnyAvailableNode produced (sql.ErrNoRows when no node is
// free) so the caller can map it to the right status code.
func (h *Handler) provisionDatabase(ctx context.Context, userID, name string, sizeGB int, planID *string, priceIDRMonth int64) (entities.Database, string, error) {
	if h.demoMaintenance(ctx) {
		return entities.Database{}, "", errDemoMaintenance
	}

	node, err := repository.AnyAvailableNode(ctx, h.DB)
	if err != nil {
		return entities.Database{}, "", err
	}

	password, err := randomPassword()
	if err != nil {
		return entities.Database{}, "", err
	}
	encrypted, err := h.Cipher.Encrypt(password)
	if err != nil {
		return entities.Database{}, "", err
	}

	maxDatabases := 0
	if h.Demo != nil {
		maxDatabases = maxDemoDatabases
	}

	var created entities.Database
	for attempt := 0; attempt < maxSlugAttempts; attempt++ {
		slug, err := newSlug(name)
		if err != nil {
			return entities.Database{}, "", err
		}

		created, err = repository.CreateDatabaseAndQueueProvision(ctx, h.DB, userID, name, sizeGB, slug, node.ID, encrypted, planID, priceIDRMonth, maxDatabases)
		if errors.Is(err, repository.ErrSlugTaken) {
			continue
		}
		if err != nil {
			return entities.Database{}, "", err
		}
		break
	}
	if created.ID == "" {
		return entities.Database{}, "", errNoSlugAvailable
	}

	return created, password, nil
}

func validateCreateDatabaseRequest(name string, sizeGB int) string {
	if strings.TrimSpace(name) == "" {
		return "name is required"
	}
	if sizeGB < 1 || sizeGB > 20 {
		return "size_gb must be between 1 and 20"
	}
	return ""
}

// resolveDatabasePlan works out the size and price a create request
// should use. With no plan_id, it is today's free-form path: size_gb must
// be given and in range, and the price is the flat per-GB rate. With a
// plan_id, the plan must exist and be active, size_gb comes from the
// plan, and a size_gb the caller also sent must agree with it — this
// never silently picks one over the other.
func (h *Handler) resolveDatabasePlan(ctx context.Context, req createDatabaseRequest) (sizeGB int, planID *string, priceIDRMonth int64, errMsg string, errStatus int) {
	if req.PlanID == nil {
		if req.SizeGB == nil || *req.SizeGB < 1 || *req.SizeGB > 20 {
			return 0, nil, 0, "size_gb must be between 1 and 20", http.StatusBadRequest
		}
		return *req.SizeGB, nil, int64(*req.SizeGB) * defaultPricePerGBIDRMonth, "", 0
	}

	plan, err := repository.GetPlanByID(ctx, h.DB, *req.PlanID)
	if err != nil || !plan.Active {
		return 0, nil, 0, "plan is not available", http.StatusBadRequest
	}
	if !plan.Purchasable {
		return 0, nil, 0, "plan is sold out", http.StatusConflict
	}
	if req.SizeGB != nil && *req.SizeGB != plan.SizeGB {
		return 0, nil, 0, "size_gb does not match plan", http.StatusBadRequest
	}
	return plan.SizeGB, &plan.ID, plan.PriceIDRMonth, "", 0
}

// handleCreateDatabase provisions a new tenant database through
// provisionDatabase, scoped to the signed-in user.
func (h *Handler) handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
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

	sizeGB, planID, priceIDRMonth, msg, errStatus := h.resolveDatabasePlan(r.Context(), req)
	if msg != "" {
		if errStatus == 0 {
			errStatus = http.StatusBadRequest
		}
		writeError(w, errStatus, msg)
		return
	}

	created, password, err := h.provisionDatabase(r.Context(), user.ID, req.Name, sizeGB, planID, priceIDRMonth)
	if errors.Is(err, errDemoMaintenance) {
		writeMaintenance(w)
		return
	}
	if errors.Is(err, repository.ErrDatabaseLimit) {
		writeError(w, http.StatusConflict, "you can hold at most two databases")
		return
	}
	if isNotFound(err) {
		writeError(w, http.StatusServiceUnavailable, "no node available")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create database")
		return
	}

	writeJSON(w, http.StatusCreated, createDatabaseResponse{
		ID:               created.ID,
		Host:             usecase.PublicHost(*created.Slug),
		Port:             usecase.PublicPort,
		Database:         "postgres",
		User:             "postgres",
		Password:         password,
		ConnectionString: usecase.ConnectionString(*created.Slug, password),
		JDBCURL:          usecase.JDBCConnectionString(*created.Slug),
	})
}

// handleDeleteDatabase moves a database to "deleting" and queues the
// "delete" job that will remove its container and volume. There is no
// final snapshot: this is a settled test-stage decision.
func (h *Handler) handleDeleteDatabase(w http.ResponseWriter, r *http.Request) {
	if h.demoMaintenance(r.Context()) {
		writeMaintenance(w)
		return
	}
	user := userFromContext(r)

	d, err := repository.SetDatabaseDeletingAndQueueDelete(r.Context(), h.DB, r.PathValue("id"), user.ID)
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "database not found")
		return
	}
	if errors.Is(err, repository.ErrInvalidState) {
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
func (h *Handler) handleSnapshotsGated(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "snapshots are not implemented yet")
}

const slugAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// newSlug derives a URL-safe, likely-unique slug from name: a
// lowercased, hyphenated prefix plus a short random suffix that resolves
// any collision. The caller retries on repository.ErrSlugTaken.
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

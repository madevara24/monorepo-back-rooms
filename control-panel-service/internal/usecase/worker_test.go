package usecase_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/credentials"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

func testCipher(t *testing.T) *credentials.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	c, err := credentials.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	return c
}

// fakeAgentServer mimics just enough of mbr-node's HTTP API for the job
// worker to drive: POST /v1/databases and DELETE /v1/databases/{id}. Tests
// toggle failCreate/failDelete to exercise the worker's retry path.
type fakeAgentServer struct {
	failCreate  atomic.Bool
	failDelete  atomic.Bool
	createCalls atomic.Int32
	deleteCalls atomic.Int32
}

func newFakeAgentServer(t *testing.T) (*httptest.Server, *fakeAgentServer) {
	t.Helper()
	f := &fakeAgentServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/databases", func(w http.ResponseWriter, r *http.Request) {
		f.createCalls.Add(1)
		if f.failCreate.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": req.ID, "host": "10.0.0.9", "port": 21000, "status": "running",
		})
	})
	mux.HandleFunc("DELETE /v1/databases/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.deleteCalls.Add(1)
		if f.failDelete.Load() {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, f
}

func insertNode(t *testing.T, db *sql.DB, address string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO nodes (id, name, address, token_hash, last_heartbeat)
		VALUES ('node-1', 'node-1', $1, 'tokenhash', now())
		RETURNING id
	`, address).Scan(&id)
	if err != nil {
		t.Fatalf("insert node fixture: %v", err)
	}
	return id
}

// drainJobs runs the worker until no due job remains.
func drainJobs(t *testing.T, ctx context.Context, w *usecase.Worker) {
	t.Helper()
	for {
		found, err := w.ProcessOne(ctx)
		if err != nil {
			t.Fatalf("ProcessOne: %v", err)
		}
		if !found {
			return
		}
	}
}

func jobStatusFor(t *testing.T, db *sql.DB, databaseID string) (status string, attempts int) {
	t.Helper()
	if err := db.QueryRow(`
		SELECT status, attempts FROM jobs WHERE payload->>'database_id' = $1
	`, databaseID).Scan(&status, &attempts); err != nil {
		t.Fatalf("look up job for %s: %v", databaseID, err)
	}
	return status, attempts
}

func TestProvisionJobSucceeds(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()
	server, fake := newFakeAgentServer(t)
	nodeID := insertNode(t, db, server.URL)
	cipher := testCipher(t)

	user, err := repository.CreateUser(ctx, db, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	encrypted, err := cipher.Encrypt("s3cr3t-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	created, err := repository.CreateDatabaseAndQueueProvision(ctx, db, user.ID, "demo", 5, "demo-ab12", nodeID, encrypted, nil, 50000)
	if err != nil {
		t.Fatalf("CreateDatabaseAndQueueProvision: %v", err)
	}

	w := &usecase.Worker{DB: db, Agent: integration.NewWithHTTPClient(server.Client()), Cipher: cipher, PollInterval: time.Second}

	found, err := w.ProcessOne(ctx)
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !found {
		t.Fatalf("ProcessOne found no job, want the queued provision job")
	}

	got, err := repository.DatabaseByIDForUser(ctx, db, created.ID, user.ID)
	if err != nil {
		t.Fatalf("DatabaseByIDForUser: %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("status = %q, want running", got.Status)
	}
	if got.Host == nil || *got.Host != "10.0.0.9" || got.Port == nil || *got.Port != 21000 {
		t.Fatalf("host/port = %v/%v, want 10.0.0.9/21000", got.Host, got.Port)
	}
	if fake.createCalls.Load() != 1 {
		t.Fatalf("agent create calls = %d, want 1", fake.createCalls.Load())
	}

	status, _ := jobStatusFor(t, db, created.ID)
	if status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", status)
	}

	var eventCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM database_events WHERE database_id = $1 AND to_status = 'running'
	`, created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count running events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("running events = %d, want 1", eventCount)
	}
}

func TestDeleteJobSucceeds(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()
	server, fake := newFakeAgentServer(t)
	nodeID := insertNode(t, db, server.URL)
	cipher := testCipher(t)

	user, err := repository.CreateUser(ctx, db, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	encrypted, err := cipher.Encrypt("s3cr3t-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	created, err := repository.CreateDatabaseAndQueueProvision(ctx, db, user.ID, "demo", 5, "demo-cd34", nodeID, encrypted, nil, 50000)
	if err != nil {
		t.Fatalf("CreateDatabaseAndQueueProvision: %v", err)
	}
	if _, err := repository.MarkDatabaseRunning(ctx, db, created.ID, "10.0.0.9", 21000); err != nil {
		t.Fatalf("MarkDatabaseRunning: %v", err)
	}
	if _, err := repository.SetDatabaseDeletingAndQueueDelete(ctx, db, created.ID, user.ID); err != nil {
		t.Fatalf("SetDatabaseDeletingAndQueueDelete: %v", err)
	}

	w := &usecase.Worker{DB: db, Agent: integration.NewWithHTTPClient(server.Client()), Cipher: cipher, PollInterval: time.Second}

	// Drain both the provision job CreateDatabaseAndQueueProvision queued
	// (never processed above; the running status was set directly) and
	// the delete job queued after it.
	drainJobs(t, ctx, w)

	var dbStatus string
	if err := db.QueryRow(`SELECT status FROM databases WHERE id = $1`, created.ID).Scan(&dbStatus); err != nil {
		t.Fatalf("look up database status: %v", err)
	}
	if dbStatus != "deleted" {
		t.Fatalf("status = %q, want deleted", dbStatus)
	}
	if fake.deleteCalls.Load() != 1 {
		t.Fatalf("agent delete calls = %d, want 1", fake.deleteCalls.Load())
	}

	var eventCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM database_events WHERE database_id = $1 AND to_status = 'deleted'
	`, created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count deleted events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("deleted events = %d, want 1", eventCount)
	}
}

func TestProvisionJobRetriesOnFailure(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()
	server, fake := newFakeAgentServer(t)
	fake.failCreate.Store(true)
	nodeID := insertNode(t, db, server.URL)
	cipher := testCipher(t)

	user, err := repository.CreateUser(ctx, db, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	encrypted, err := cipher.Encrypt("s3cr3t-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	created, err := repository.CreateDatabaseAndQueueProvision(ctx, db, user.ID, "demo", 5, "demo-ef56", nodeID, encrypted, nil, 50000)
	if err != nil {
		t.Fatalf("CreateDatabaseAndQueueProvision: %v", err)
	}

	w := &usecase.Worker{DB: db, Agent: integration.NewWithHTTPClient(server.Client()), Cipher: cipher, PollInterval: time.Second}

	found, err := w.ProcessOne(ctx)
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !found {
		t.Fatalf("ProcessOne found no job, want the queued provision job")
	}

	status, attempts := jobStatusFor(t, db, created.ID)
	if status != "pending" || attempts != 1 {
		t.Fatalf("job status/attempts = %q/%d, want pending/1 (scheduled for retry)", status, attempts)
	}

	got, err := repository.DatabaseByIDForUser(ctx, db, created.ID, user.ID)
	if err != nil {
		t.Fatalf("DatabaseByIDForUser: %v", err)
	}
	if got.Status != "provisioning" {
		t.Fatalf("status = %q, want provisioning (not yet failed)", got.Status)
	}
}

func TestProvisionJobFailsPermanentlyAfterMaxAttempts(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()
	server, fake := newFakeAgentServer(t)
	fake.failCreate.Store(true)
	nodeID := insertNode(t, db, server.URL)
	cipher := testCipher(t)

	user, err := repository.CreateUser(ctx, db, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	encrypted, err := cipher.Encrypt("s3cr3t-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	created, err := repository.CreateDatabaseAndQueueProvision(ctx, db, user.ID, "demo", 5, "demo-gh78", nodeID, encrypted, nil, 50000)
	if err != nil {
		t.Fatalf("CreateDatabaseAndQueueProvision: %v", err)
	}

	// Fast-forward this job to one attempt short of the limit, so the
	// next failure trips the permanent-failure path instead of another
	// retry.
	if _, err := db.Exec(`
		UPDATE jobs SET attempts = $2 WHERE payload->>'database_id' = $1
	`, created.ID, usecase.MaxAttempts-1); err != nil {
		t.Fatalf("fast-forward job attempts: %v", err)
	}

	w := &usecase.Worker{DB: db, Agent: integration.NewWithHTTPClient(server.Client()), Cipher: cipher, PollInterval: time.Second}

	found, err := w.ProcessOne(ctx)
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !found {
		t.Fatalf("ProcessOne found no job, want the queued provision job")
	}

	status, attempts := jobStatusFor(t, db, created.ID)
	if status != "failed" || attempts != usecase.MaxAttempts {
		t.Fatalf("job status/attempts = %q/%d, want failed/%d", status, attempts, usecase.MaxAttempts)
	}

	got, err := repository.DatabaseByIDForUser(ctx, db, created.ID, user.ID)
	if err != nil {
		t.Fatalf("DatabaseByIDForUser: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}

	var eventCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM database_events WHERE database_id = $1 AND to_status = 'failed'
	`, created.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count failed events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("failed events = %d, want 1", eventCount)
	}
}

func TestProvisionJobSkipsWhenStatusMovedOn(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()
	server, fake := newFakeAgentServer(t)
	nodeID := insertNode(t, db, server.URL)
	cipher := testCipher(t)

	user, err := repository.CreateUser(ctx, db, "superseded@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	encrypted, err := cipher.Encrypt("s3cr3t-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	created, err := repository.CreateDatabaseAndQueueProvision(ctx, db, user.ID, "demo", 5, "demo-super", nodeID, encrypted, nil, 50000)
	if err != nil {
		t.Fatalf("CreateDatabaseAndQueueProvision: %v", err)
	}
	// A delete moved the row on before the provision job ran.
	if _, err := db.Exec(`UPDATE databases SET status = 'deleting' WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("move database on: %v", err)
	}

	w := &usecase.Worker{DB: db, Agent: integration.NewWithHTTPClient(server.Client()), Cipher: cipher, PollInterval: time.Second}
	found, err := w.ProcessOne(ctx)
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !found {
		t.Fatalf("ProcessOne found no job, want the queued provision job")
	}
	if fake.createCalls.Load() != 0 {
		t.Fatalf("agent create calls = %d, want 0 for a superseded provision", fake.createCalls.Load())
	}
	status, _ := jobStatusFor(t, db, created.ID)
	if status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", status)
	}
}

func TestProvisionAndDeleteReconcileRoutes(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()
	server, _ := newFakeAgentServer(t)
	nodeID := insertNode(t, db, server.URL)
	cipher := testCipher(t)

	user, err := repository.CreateUser(ctx, db, "routes@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	encrypted, err := cipher.Encrypt("s3cr3t-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	created, err := repository.CreateDatabaseAndQueueProvision(ctx, db, user.ID, "demo", 5, "demo-route1", nodeID, encrypted, nil, 50000)
	if err != nil {
		t.Fatalf("CreateDatabaseAndQueueProvision: %v", err)
	}

	routesPath := filepath.Join(t.TempDir(), "mbr-routes.map")
	w := &usecase.Worker{
		DB: db, Agent: integration.NewWithHTTPClient(server.Client()), Cipher: cipher, PollInterval: time.Second,
		Routes: &usecase.Reconciler{DB: db, Path: routesPath},
	}

	found, err := w.ProcessOne(ctx)
	if err != nil {
		t.Fatalf("ProcessOne (provision): %v", err)
	}
	if !found {
		t.Fatalf("ProcessOne found no job, want the queued provision job")
	}

	got, err := os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("read routes file after provision: %v", err)
	}
	want := "demo-route1.db.mbr.madevara.com 10.0.0.9:21000;\n"
	if string(got) != want {
		t.Fatalf("routes file after provision = %q, want %q", got, want)
	}

	if _, err := repository.SetDatabaseDeletingAndQueueDelete(ctx, db, created.ID, user.ID); err != nil {
		t.Fatalf("SetDatabaseDeletingAndQueueDelete: %v", err)
	}
	drainJobs(t, ctx, w)

	got, err = os.ReadFile(routesPath)
	if err != nil {
		t.Fatalf("read routes file after delete: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("routes file after delete = %q, want empty", got)
	}
}

func TestReclaimStaleRunningJob(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	var jobID string
	if err := db.QueryRow(`
		INSERT INTO jobs (kind, payload, status, locked_at)
		VALUES ('provision', '{"database_id":"00000000-0000-0000-0000-000000000000"}', 'running', now() - interval '11 minutes')
		RETURNING id
	`).Scan(&jobID); err != nil {
		t.Fatalf("insert running job: %v", err)
	}

	n, err := repository.ReclaimStaleJobs(ctx, db, 10*time.Minute)
	if err != nil {
		t.Fatalf("ReclaimStaleJobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("reclaimed = %d, want 1", n)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM jobs WHERE id = $1`, jobID).Scan(&status); err != nil {
		t.Fatalf("look up job: %v", err)
	}
	if status != "pending" {
		t.Fatalf("job status = %q, want pending", status)
	}
}

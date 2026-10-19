// Package jobs runs the control plane's job queue: the "provision" and
// "delete" work POST /api/databases and DELETE /api/databases/{id} queue,
// claimed from the jobs table with FOR UPDATE SKIP LOCKED.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/madevara24/monorepo-back-rooms/internal/agentclient"
	"github.com/madevara24/monorepo-back-rooms/internal/credentials"
	"github.com/madevara24/monorepo-back-rooms/internal/routes"
	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

// MaxAttempts is how many times a job may be attempted before it, and the
// database it drives, are marked failed for good.
const MaxAttempts = 5

// Worker claims and runs provisioning and deletion jobs.
type Worker struct {
	DB     *sql.DB
	Agent  *agentclient.Client
	Cipher *credentials.Cipher
	// PollInterval is how often Run checks for due jobs when the queue is
	// empty.
	PollInterval time.Duration
	// LeaseTimeout is how long a job may sit in "running" before it is
	// reclaimed. Zero means 10 minutes.
	LeaseTimeout time.Duration
	// Routes reconciles the SNI proxy's host map after a provision or
	// delete job changes which databases are running. Nil disables this
	// (tests that don't care about the routes file leave it unset).
	Routes *routes.Reconciler
}

// reconcileRoutes reconciles the routes file if Routes is configured,
// logging rather than failing the job on error: a stale route map is a
// proxy-side problem the next tick or job will correct, not a reason to
// retry work that already succeeded.
func (w *Worker) reconcileRoutes(ctx context.Context) {
	if w.Routes == nil {
		return
	}
	if _, err := w.Routes.Reconcile(ctx); err != nil {
		log.Printf("jobs: reconcile routes: %v", err)
	}
}

// Run claims and processes due jobs until ctx is canceled, draining the
// queue between ticks of PollInterval. It reclaims jobs abandoned by a
// previous process before each drain.
func (w *Worker) Run(ctx context.Context) {
	lease := w.LeaseTimeout
	if lease <= 0 {
		lease = 10 * time.Minute
	}
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
		w.reclaim(ctx, lease)
		for {
			found, err := w.ProcessOne(ctx)
			if err != nil {
				log.Printf("jobs: %v", err)
			}
			if !found {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) reclaim(ctx context.Context, lease time.Duration) {
	n, err := store.ReclaimStaleJobs(ctx, w.DB, lease)
	if err != nil {
		log.Printf("jobs: reclaim stale jobs: %v", err)
		return
	}
	if n > 0 {
		log.Printf("jobs: reclaimed %d stale job(s)", n)
	}
}

// ProcessOne claims and runs a single due job. The bool reports whether a
// job was found, so Run can drain the queue before waiting on its ticker.
func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	job, err := store.ClaimNextJob(ctx, w.DB)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim job: %w", err)
	}

	return true, w.process(ctx, job)
}

type jobPayload struct {
	DatabaseID string `json:"database_id"`
}

func (w *Worker) process(ctx context.Context, job store.Job) error {
	var payload jobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return store.FailJob(ctx, w.DB, job.ID, job.Attempts+1, err.Error())
	}

	var runErr error
	switch job.Kind {
	case "provision":
		runErr = w.provision(ctx, payload.DatabaseID)
	case "delete":
		runErr = w.delete(ctx, payload.DatabaseID)
	default:
		runErr = fmt.Errorf("unknown job kind %q", job.Kind)
	}

	if runErr == nil {
		return store.CompleteJob(ctx, w.DB, job.ID)
	}
	return w.retryOrFail(ctx, job, payload.DatabaseID, runErr)
}

func (w *Worker) retryOrFail(ctx context.Context, job store.Job, databaseID string, runErr error) error {
	attempts := job.Attempts + 1
	if attempts < MaxAttempts {
		backoff := time.Duration(1<<attempts) * time.Second
		return store.RetryJob(ctx, w.DB, job.ID, attempts, time.Now().Add(backoff), runErr.Error())
	}

	if info, err := store.DatabaseForJob(ctx, w.DB, databaseID); err == nil {
		reason := runErr.Error()
		applied, merr := store.MarkDatabaseFailed(ctx, w.DB, databaseID, info.Status)
		if merr != nil {
			log.Printf("jobs: mark database %s failed: %v", databaseID, merr)
		}
		if applied {
			if err := store.InsertDatabaseEvent(ctx, w.DB, databaseID, info.Status, "failed", &reason); err != nil {
				log.Printf("jobs: record failed event for %s: %v", databaseID, err)
			}
		}
	}
	return store.FailJob(ctx, w.DB, job.ID, attempts, runErr.Error())
}

func (w *Worker) provision(ctx context.Context, databaseID string) error {
	info, err := store.DatabaseForJob(ctx, w.DB, databaseID)
	if err != nil {
		return fmt.Errorf("load database: %w", err)
	}
	if info.Status != "provisioning" {
		// A delete moved the row on before this job ran; there is
		// nothing to provision and the delete job owns the cleanup.
		return nil
	}

	password, err := w.Cipher.Decrypt(info.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	result, err := w.Agent.CreateDatabase(ctx, info.NodeAddress, databaseID, info.SizeGB, password)
	if err != nil {
		return fmt.Errorf("create database on agent: %w", err)
	}

	applied, err := store.MarkDatabaseRunning(ctx, w.DB, databaseID, result.Host, result.Port)
	if err != nil {
		return fmt.Errorf("mark database running: %w", err)
	}
	if !applied {
		// The row left provisioning while the agent call was in flight.
		// Remove the container we just created so it cannot linger, then
		// let the job finish; the state transition is owned elsewhere.
		if derr := w.Agent.DeleteDatabase(ctx, info.NodeAddress, databaseID); derr != nil {
			log.Printf("jobs: clean up superseded provision %s: %v", databaseID, derr)
		}
		return nil
	}
	if err := store.InsertDatabaseEvent(ctx, w.DB, databaseID, info.Status, "running", nil); err != nil {
		log.Printf("jobs: record running event for %s: %v", databaseID, err)
	}
	w.reconcileRoutes(ctx)
	return nil
}

func (w *Worker) delete(ctx context.Context, databaseID string) error {
	info, err := store.DatabaseForJob(ctx, w.DB, databaseID)
	if err != nil {
		return fmt.Errorf("load database: %w", err)
	}
	if info.Status != "deleting" {
		// Already deleted or superseded; a duplicate delete job is a
		// no-op rather than a failure.
		return nil
	}

	if err := w.Agent.DeleteDatabase(ctx, info.NodeAddress, databaseID); err != nil {
		return fmt.Errorf("delete database on agent: %w", err)
	}

	applied, err := store.MarkDatabaseDeleted(ctx, w.DB, databaseID)
	if err != nil {
		return fmt.Errorf("mark database deleted: %w", err)
	}
	if !applied {
		return nil
	}
	if err := store.InsertDatabaseEvent(ctx, w.DB, databaseID, info.Status, "deleted", nil); err != nil {
		log.Printf("jobs: record deleted event for %s: %v", databaseID, err)
	}
	w.reconcileRoutes(ctx)
	return nil
}

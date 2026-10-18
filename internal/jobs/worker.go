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
}

// Run claims and processes due jobs until ctx is canceled, draining the
// queue between ticks of PollInterval.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
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
		if err := store.InsertDatabaseEvent(ctx, w.DB, databaseID, info.Status, "failed", &reason); err != nil {
			log.Printf("jobs: record failed event for %s: %v", databaseID, err)
		}
	}
	if err := store.MarkDatabaseFailed(ctx, w.DB, databaseID); err != nil {
		log.Printf("jobs: mark database %s failed: %v", databaseID, err)
	}
	return store.FailJob(ctx, w.DB, job.ID, attempts, runErr.Error())
}

func (w *Worker) provision(ctx context.Context, databaseID string) error {
	info, err := store.DatabaseForJob(ctx, w.DB, databaseID)
	if err != nil {
		return fmt.Errorf("load database: %w", err)
	}

	password, err := w.Cipher.Decrypt(info.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	result, err := w.Agent.CreateDatabase(ctx, info.NodeAddress, databaseID, info.SizeGB, password)
	if err != nil {
		return fmt.Errorf("create database on agent: %w", err)
	}

	if err := store.MarkDatabaseRunning(ctx, w.DB, databaseID, result.Host, result.Port); err != nil {
		return fmt.Errorf("mark database running: %w", err)
	}
	if err := store.InsertDatabaseEvent(ctx, w.DB, databaseID, info.Status, "running", nil); err != nil {
		log.Printf("jobs: record running event for %s: %v", databaseID, err)
	}
	return nil
}

func (w *Worker) delete(ctx context.Context, databaseID string) error {
	info, err := store.DatabaseForJob(ctx, w.DB, databaseID)
	if err != nil {
		return fmt.Errorf("load database: %w", err)
	}

	if err := w.Agent.DeleteDatabase(ctx, info.NodeAddress, databaseID); err != nil {
		return fmt.Errorf("delete database on agent: %w", err)
	}

	if err := store.MarkDatabaseDeleted(ctx, w.DB, databaseID); err != nil {
		return fmt.Errorf("mark database deleted: %w", err)
	}
	if err := store.InsertDatabaseEvent(ctx, w.DB, databaseID, info.Status, "deleted", nil); err != nil {
		log.Printf("jobs: record deleted event for %s: %v", databaseID, err)
	}
	return nil
}

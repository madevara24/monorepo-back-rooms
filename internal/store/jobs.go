package store

import (
	"context"
	"database/sql"
	"time"
)

const jobColumns = `id, kind, payload, status, attempts, run_at, locked_at, last_error, created_at, updated_at`

func scanJob(row rowScanner) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.Kind, &j.Payload, &j.Status, &j.Attempts, &j.RunAt, &j.LockedAt,
		&j.LastError, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

// ClaimNextJob locks and claims the oldest due pending job, skipping any
// row a concurrent worker already has locked. It returns sql.ErrNoRows if
// nothing is due.
func ClaimNextJob(ctx context.Context, db *sql.DB) (Job, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT `+jobColumns+`
		FROM jobs
		WHERE status = 'pending' AND run_at <= now()
		ORDER BY run_at
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`)
	j, err := scanJob(row)
	if err != nil {
		return Job{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE jobs SET status = 'running', locked_at = now(), updated_at = now() WHERE id = $1
	`, j.ID); err != nil {
		return Job{}, err
	}

	if err := tx.Commit(); err != nil {
		return Job{}, err
	}
	j.Status = "running"
	return j, nil
}

// CompleteJob marks a job succeeded.
func CompleteJob(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE jobs SET status = 'succeeded', updated_at = now() WHERE id = $1
	`, id)
	return err
}

// RetryJob schedules a job for another attempt after a backoff, recording
// the error that caused the retry.
func RetryJob(ctx context.Context, db *sql.DB, id string, attempts int, runAt time.Time, lastErr string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'pending', attempts = $2, run_at = $3, last_error = $4, locked_at = NULL, updated_at = now()
		WHERE id = $1
	`, id, attempts, runAt, lastErr)
	return err
}

// FailJob marks a job permanently failed after it has exhausted its
// retries.
func FailJob(ctx context.Context, db *sql.DB, id string, attempts int, lastErr string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE jobs SET status = 'failed', attempts = $2, last_error = $3, updated_at = now()
		WHERE id = $1
	`, id, attempts, lastErr)
	return err
}

// ReclaimStaleJobs resets jobs stuck in "running" past their lease back to
// pending. A process restart or a killed worker would otherwise strand the
// database the job was driving forever. The agent's create and delete are
// idempotent, so replaying the job is safe.
func ReclaimStaleJobs(ctx context.Context, db *sql.DB, lease time.Duration) (int64, error) {
	seconds := int(lease.Seconds())
	res, err := db.ExecContext(ctx, `
		UPDATE jobs
		SET status = 'pending', run_at = now(), locked_at = NULL,
		    last_error = COALESCE(NULLIF(last_error, '') || '; ', '') || 'reclaimed after lease timeout',
		    updated_at = now()
		WHERE status = 'running' AND locked_at < now() - make_interval(secs => $1)
	`, seconds)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

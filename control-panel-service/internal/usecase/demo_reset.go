package usecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/integration"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/pkg/password"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

// DemoResetInterval is the wall-clock reset period. The boundary is
// computed from the clock, not from process start, so every client sees
// the same countdown.
const DemoResetInterval = 30 * time.Minute

// demoResetLockKey is the Postgres advisory lock that makes the reset
// single-flight across a manual trigger and the scheduler.
const demoResetLockKey = 847002616

// ErrResetInProgress is returned when another reset holds the lock.
var ErrResetInProgress = errors.New("demo reset already running")

// ResetResult is the state after a run plus what the agent removed.
type ResetResult struct {
	State  entities.DemoState
	Counts integration.ResetCounts
}

// DemoAgent is the subset of the agent client the reset needs.
type DemoAgent interface {
	ResetDemo(ctx context.Context, address string) (integration.ResetCounts, error)
	ListDatabases(ctx context.Context, address string) ([]integration.Database, error)
}

// DemoReset owns the 30-minute demo lifecycle: it cleans the data plane,
// resets the control rows, and exposes the countdown state.
type DemoReset struct {
	DB         *sql.DB
	Agent      DemoAgent
	Reconciler *Reconciler
	// Gate is shared with the worker. The reset takes the write lock so
	// an in-flight provision finishes before the cleanup starts.
	Gate *sync.RWMutex
	// Paused keeps the worker from claiming new jobs while the reset is
	// running or after a failed cleanup.
	Paused atomic.Bool

	now func() time.Time
}

func (d *DemoReset) clock() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

// NextBoundary returns the next wall-clock reset boundary after now.
func NextBoundary(now time.Time) time.Time {
	return now.UTC().Truncate(DemoResetInterval).Add(DemoResetInterval)
}

// Status reads the public demo state.
func (d *DemoReset) Status(ctx context.Context) (entities.DemoState, error) {
	return repository.GetDemoState(ctx, d.DB)
}

// IsMaintenance reports whether creates and deletes should be refused.
func (d *DemoReset) IsMaintenance(ctx context.Context) (bool, error) {
	state, err := d.Status(ctx)
	if err != nil {
		return false, err
	}
	return state.Maintenance, nil
}

// Reset runs one reset. It holds the advisory lock for the whole run, so
// a manual trigger and the scheduler cannot overlap. On a data-plane
// failure it records the error and leaves maintenance on; the control
// rows are not truncated.
func (d *DemoReset) Reset(ctx context.Context, trigger string) (ResetResult, error) {
	conn, err := d.DB.Conn(ctx)
	if err != nil {
		return ResetResult{}, err
	}
	defer func() { _ = conn.Close() }()

	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, demoResetLockKey).Scan(&locked); err != nil {
		return ResetResult{}, err
	}
	if !locked {
		return ResetResult{}, ErrResetInProgress
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1::bigint)`, demoResetLockKey)
	}()

	runID, err := repository.BeginDemoRun(ctx, d.DB, trigger)
	if err != nil {
		return ResetResult{}, err
	}

	d.Paused.Store(true)
	if d.Gate != nil {
		d.Gate.Lock()
		defer d.Gate.Unlock()
	}

	var counts integration.ResetCounts
	fail := func(cause error) (ResetResult, error) {
		recordCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if ferr := repository.FailDemoRun(recordCtx, d.DB, runID, cause.Error(), counts.Containers, counts.Volumes, counts.Orphans); ferr != nil {
			log.Printf("demo reset: record failure: %v", ferr)
		}
		return ResetResult{Counts: counts}, cause
	}

	nodes, err := repository.ListNodes(ctx, d.DB)
	if err != nil {
		return fail(fmt.Errorf("list nodes: %w", err))
	}
	if len(nodes) == 0 {
		// Without a registered agent there is no way to prove the data
		// plane is empty. Truncating anyway would orphan containers.
		return fail(errors.New("no data-plane node registered"))
	}
	for _, node := range nodes {
		c, err := d.Agent.ResetDemo(ctx, node.Address)
		counts.Containers += c.Containers
		counts.Volumes += c.Volumes
		counts.Orphans += c.Orphans
		if err != nil {
			return fail(fmt.Errorf("reset agent %s: %w", node.ID, err))
		}
	}
	for _, node := range nodes {
		dbs, err := d.Agent.ListDatabases(ctx, node.Address)
		if err != nil {
			return fail(fmt.Errorf("verify agent %s: %w", node.ID, err))
		}
		if len(dbs) != 0 {
			return fail(fmt.Errorf("agent %s still reports %d tenant container(s)", node.ID, len(dbs)))
		}
	}

	if err := repository.ResetVisitorData(ctx, d.DB); err != nil {
		return fail(fmt.Errorf("reset control rows: %w", err))
	}
	if d.Reconciler != nil {
		if _, err := d.Reconciler.Reconcile(ctx); err != nil {
			return fail(fmt.Errorf("reconcile routes: %w", err))
		}
	}
	if err := repository.FinishDemoRun(ctx, d.DB, runID, counts.Containers, counts.Volumes, counts.Orphans, NextBoundary(d.clock())); err != nil {
		return fail(fmt.Errorf("finish demo reset: %w", err))
	}

	d.Paused.Store(false)
	state, err := d.Status(ctx)
	if err != nil {
		return ResetResult{Counts: counts}, err
	}
	return ResetResult{State: state, Counts: counts}, nil
}

// Run catches up on a missed boundary, recovers a crashed reset, and
// then resets on every wall-clock boundary until ctx is canceled.
func (d *DemoReset) Run(ctx context.Context) {
	if state, err := d.Status(ctx); err == nil && needsBootReset(state, d.clock()) {
		if _, err := d.Reset(ctx, "boot"); err != nil {
			log.Printf("demo reset: boot catch-up: %v", err)
		}
	}

	for {
		next := NextBoundary(d.clock())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if _, err := d.Reset(ctx, "scheduled"); err != nil {
			log.Printf("demo reset: scheduled: %v", err)
		}
	}
}

// needsBootReset reports a missed boundary or a reset that crashed while
// holding the maintenance flag. A failed reset is retried at the next
// boundary, not on every boot.
func needsBootReset(state entities.DemoState, now time.Time) bool {
	const staleRunning = 10 * time.Minute
	if state.Status == "running" && state.StartedAt != nil && now.Sub(*state.StartedAt) > staleRunning {
		return true
	}
	return !state.NextResetAt.After(now)
}

// SeedAdmin creates or updates the one admin account the reset keeps.
// It must run before the reset loop's boot catch-up.
func SeedAdmin(ctx context.Context, db *sql.DB, email, plaintext string) (entities.User, error) {
	if email == "" || plaintext == "" {
		return entities.User{}, errors.New("DEMO_ADMIN_EMAIL and DEMO_ADMIN_PASSWORD are required in demo mode")
	}
	hash, err := password.HashPassword(plaintext)
	if err != nil {
		return entities.User{}, err
	}
	return repository.UpsertSeedAdmin(ctx, db, normalizeEmail(email), hash)
}

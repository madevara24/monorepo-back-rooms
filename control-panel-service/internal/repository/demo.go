package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

// SeedPlan is one row of the demo package seed. The reset owns these
// rows: an admin edit is reverted at the next boundary.
type SeedPlan struct {
	Slug          string
	Name          string
	SizeGB        int
	PriceIDRMonth int64
	Tagline       string
	Badge         string
	SortOrder     int
	Purchasable   bool
}

// DemoPlans is the shop the demo returns to. The 1 GB and 5 GB packages
// are buyable; the 10 GB and 20 GB packages stay visible as sold out.
var DemoPlans = []SeedPlan{
	{Slug: "sandbox", Name: "Sandbox", SizeGB: 1, PriceIDRMonth: 10000, Tagline: "For the project you swear you'll finish.", SortOrder: 1, Purchasable: true},
	{Slug: "side-project", Name: "Side project", SizeGB: 5, PriceIDRMonth: 50000, Tagline: "Real data, no real users yet.", SortOrder: 2, Purchasable: true},
	{Slug: "production", Name: "Production", SizeGB: 10, PriceIDRMonth: 100000, Tagline: "The one you actually put behind a DNS record.", Badge: "Most picked", SortOrder: 3, Purchasable: false},
	{Slug: "heavy", Name: "Heavy", SizeGB: 20, PriceIDRMonth: 200000, Tagline: "For when the indexes got out of hand.", SortOrder: 4, Purchasable: false},
}

// GetDemoState reads the singleton demo state.
func GetDemoState(ctx context.Context, db *sql.DB) (entities.DemoState, error) {
	row := db.QueryRowContext(ctx, `
		SELECT generation, status, maintenance, started_at, finished_at, next_reset_at, last_error
		FROM demo_state
		WHERE id
	`)

	var s entities.DemoState
	var started, finished sql.NullTime
	var lastErr sql.NullString
	if err := row.Scan(&s.Generation, &s.Status, &s.Maintenance, &started, &finished, &s.NextResetAt, &lastErr); err != nil {
		return entities.DemoState{}, err
	}
	if started.Valid {
		t := started.Time
		s.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time
		s.FinishedAt = &t
	}
	if lastErr.Valid {
		v := lastErr.String
		s.LastError = &v
	}
	return s, nil
}

// BeginDemoRun marks the demo as resetting and opens a run row.
func BeginDemoRun(ctx context.Context, db *sql.DB, trigger string) (string, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE demo_state
		SET status = 'running', maintenance = TRUE, started_at = now(), finished_at = NULL, last_error = NULL
		WHERE id
	`); err != nil {
		return "", err
	}

	var id string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO reset_runs (trigger_kind) VALUES ($1) RETURNING id
	`, trigger).Scan(&id); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// FinishDemoRun records a successful reset and schedules the next one.
func FinishDemoRun(ctx context.Context, db *sql.DB, runID string, containers, volumes, orphans int, next time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE demo_state
		SET status = 'idle', maintenance = FALSE, generation = generation + 1,
		    finished_at = now(), next_reset_at = $1, last_error = NULL
		WHERE id
	`, next); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE reset_runs
		SET finished_at = now(), containers_deleted = $2, volumes_deleted = $3, orphans_deleted = $4
		WHERE id = $1
	`, runID, containers, volumes, orphans); err != nil {
		return err
	}
	return tx.Commit()
}

// FailDemoRun records a failed reset and leaves maintenance on.
func FailDemoRun(ctx context.Context, db *sql.DB, runID, message string, containers, volumes, orphans int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE demo_state
		SET status = 'failed', maintenance = TRUE, finished_at = now(), last_error = $1
		WHERE id
	`, message); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE reset_runs
		SET finished_at = now(), error = $2, containers_deleted = $3, volumes_deleted = $4, orphans_deleted = $5
		WHERE id = $1
	`, runID, message, containers, volumes, orphans); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetVisitorData deletes every visitor-owned row and restores the demo
// plan seed in one transaction. It keeps nodes, seed users, and schema.
func ResetVisitorData(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{
		`DELETE FROM database_metrics`,
		`DELETE FROM database_events`,
		`DELETE FROM node_containers`,
		`DELETE FROM invoice_lines`,
		`DELETE FROM invoices`,
		`DELETE FROM jobs`,
		`DELETE FROM databases`,
		`DELETE FROM sessions`,
		`DELETE FROM users WHERE NOT is_seed`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM plans WHERE slug NOT IN ($1, $2, $3, $4)
	`, DemoPlans[0].Slug, DemoPlans[1].Slug, DemoPlans[2].Slug, DemoPlans[3].Slug); err != nil {
		return err
	}

	for _, p := range DemoPlans {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO plans (slug, name, size_gb, price_idr_month, tagline, badge, sort_order, active, purchasable)
			VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, $8)
			ON CONFLICT (slug) DO UPDATE
			SET name = EXCLUDED.name, size_gb = EXCLUDED.size_gb, price_idr_month = EXCLUDED.price_idr_month,
			    tagline = EXCLUDED.tagline, badge = EXCLUDED.badge, sort_order = EXCLUDED.sort_order,
			    active = TRUE, purchasable = EXCLUDED.purchasable, updated_at = now()
		`, p.Slug, p.Name, p.SizeGB, p.PriceIDRMonth, p.Tagline, p.Badge, p.SortOrder, p.Purchasable); err != nil {
			return err
		}
	}

	return tx.Commit()
}

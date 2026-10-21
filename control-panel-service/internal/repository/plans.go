package repository

import (
	"context"
	"database/sql"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

const planColumns = `id, slug, name, size_gb, price_idr_month, tagline, badge, sort_order, active, created_at, updated_at`

func scanPlan(row rowScanner) (entities.Plan, error) {
	var p entities.Plan
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.SizeGB, &p.PriceIDRMonth, &p.Tagline,
		&p.Badge, &p.SortOrder, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// ListActivePlans returns every active plan, ordered the way the shop
// displays them: by sort_order, then size_gb as a tiebreaker.
func ListActivePlans(ctx context.Context, db *sql.DB) ([]entities.Plan, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+planColumns+`
		FROM plans
		WHERE active
		ORDER BY sort_order, size_gb
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []entities.Plan{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlanByID returns a plan by id regardless of its active state, so
// callers can distinguish "not found" from "inactive". It returns
// sql.ErrNoRows if the id does not exist.
func GetPlanByID(ctx context.Context, db *sql.DB, id string) (entities.Plan, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+planColumns+`
		FROM plans
		WHERE id = $1
	`, id)
	return scanPlan(row)
}

// GetPlanBySlug returns a plan by slug regardless of its active state. It
// returns sql.ErrNoRows if the slug does not exist.
func GetPlanBySlug(ctx context.Context, db *sql.DB, slug string) (entities.Plan, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+planColumns+`
		FROM plans
		WHERE slug = $1
	`, slug)
	return scanPlan(row)
}

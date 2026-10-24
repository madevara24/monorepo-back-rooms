package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

// ErrPlanSlugTaken is returned by CreatePlan when the slug already
// belongs to another plan.
var ErrPlanSlugTaken = errors.New("plan slug already exists")

// ErrLastActivePlan is returned by UpdatePlan when a request would
// deactivate the only active plan left, which would empty the shop and
// block every customer from creating a database.
var ErrLastActivePlan = errors.New("at least one active plan is required")

// planActiveGuardLockKey is an arbitrary Postgres advisory lock key that
// serializes every "deactivate a plan" transaction against every other
// one. A FOR UPDATE on the row being deactivated is not enough on its
// own: it never locks the *other* plan rows the "at least one active
// plan" invariant depends on, so under READ COMMITTED two concurrent
// requests deactivating the last two active plans could each see the
// other as still active and both commit, leaving zero active plans.
// Holding this lock for the transaction's duration forces the second
// deactivation to wait for the first to commit, so its count query sees
// the first one's effect.
const planActiveGuardLockKey = 847002615

const planColumns = `id, slug, name, size_gb, price_idr_month, tagline, badge, sort_order, active, purchasable, created_at, updated_at`

func scanPlan(row rowScanner) (entities.Plan, error) {
	var p entities.Plan
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.SizeGB, &p.PriceIDRMonth, &p.Tagline,
		&p.Badge, &p.SortOrder, &p.Active, &p.Purchasable, &p.CreatedAt, &p.UpdatedAt)
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

// ListAllPlans returns every plan, active or not, ordered the way the
// shop displays active ones: by sort_order, then size_gb as a tiebreaker.
// Each row's DatabaseCount is how many databases currently reference it,
// regardless of their status.
func ListAllPlans(ctx context.Context, db *sql.DB) ([]entities.Plan, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+planColumns+`, (SELECT count(*) FROM databases WHERE databases.plan_id = plans.id)
		FROM plans
		ORDER BY sort_order, size_gb
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []entities.Plan{}
	for rows.Next() {
		var p entities.Plan
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.SizeGB, &p.PriceIDRMonth, &p.Tagline,
			&p.Badge, &p.SortOrder, &p.Active, &p.Purchasable, &p.CreatedAt, &p.UpdatedAt, &p.DatabaseCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreatePlan inserts a new plan and returns the row it created. It
// returns ErrPlanSlugTaken if the slug already belongs to another plan.
func CreatePlan(ctx context.Context, db *sql.DB, p entities.Plan) (entities.Plan, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO plans (slug, name, size_gb, price_idr_month, tagline, badge, sort_order, active, purchasable)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+planColumns, p.Slug, p.Name, p.SizeGB, p.PriceIDRMonth, p.Tagline, p.Badge, p.SortOrder, p.Active, p.Purchasable)
	created, err := scanPlan(row)
	if isUniqueViolation(err) {
		return entities.Plan{}, ErrPlanSlugTaken
	}
	if err != nil {
		return entities.Plan{}, err
	}
	return created, nil
}

// PlanUpdate carries PATCH /api/admin/plans/{id}'s optional fields: a nil
// field leaves the column unchanged. There is no Slug field because the
// slug is immutable after creation; the handler rejects a body carrying
// one before it ever reaches here.
type PlanUpdate struct {
	Name          *string
	SizeGB        *int
	PriceIDRMonth *int64
	Tagline       *string
	Badge         *string
	SortOrder     *int
	Active        *bool
	Purchasable   *bool
}

// UpdatePlan applies upd's non-nil fields to the plan identified by id and
// returns the updated row. It returns sql.ErrNoRows if id does not exist,
// and ErrLastActivePlan if the update would deactivate the only active
// plan left.
func UpdatePlan(ctx context.Context, db *sql.DB, id string, upd PlanUpdate) (entities.Plan, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return entities.Plan{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Deactivating is the only change that can violate "at least one
	// active plan": take the global guard lock before reading anything,
	// so a concurrent deactivation is forced to wait behind this one
	// instead of racing it.
	if upd.Active != nil && !*upd.Active {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, planActiveGuardLockKey); err != nil {
			return entities.Plan{}, err
		}
	}

	row := tx.QueryRowContext(ctx, `
		SELECT `+planColumns+`
		FROM plans
		WHERE id = $1
		FOR UPDATE
	`, id)
	current, err := scanPlan(row)
	if err != nil {
		return entities.Plan{}, err
	}

	if upd.Active != nil && current.Active && !*upd.Active {
		var otherActive int
		if err := tx.QueryRowContext(ctx, `
			SELECT count(*) FROM plans WHERE active AND id != $1
		`, id).Scan(&otherActive); err != nil {
			return entities.Plan{}, err
		}
		if otherActive == 0 {
			return entities.Plan{}, ErrLastActivePlan
		}
	}

	if upd.Name != nil {
		current.Name = *upd.Name
	}
	if upd.SizeGB != nil {
		current.SizeGB = *upd.SizeGB
	}
	if upd.PriceIDRMonth != nil {
		current.PriceIDRMonth = *upd.PriceIDRMonth
	}
	if upd.Tagline != nil {
		current.Tagline = *upd.Tagline
	}
	if upd.Badge != nil {
		current.Badge = *upd.Badge
	}
	if upd.SortOrder != nil {
		current.SortOrder = *upd.SortOrder
	}
	if upd.Active != nil {
		current.Active = *upd.Active
	}
	if upd.Purchasable != nil {
		current.Purchasable = *upd.Purchasable
	}

	row = tx.QueryRowContext(ctx, `
		UPDATE plans
		SET name = $2, size_gb = $3, price_idr_month = $4, tagline = $5, badge = $6, sort_order = $7, active = $8, purchasable = $9, updated_at = now()
		WHERE id = $1
		RETURNING `+planColumns, id, current.Name, current.SizeGB, current.PriceIDRMonth,
		current.Tagline, current.Badge, current.SortOrder, current.Active, current.Purchasable)
	updated, err := scanPlan(row)
	if err != nil {
		return entities.Plan{}, err
	}

	if err := tx.QueryRowContext(ctx, `
		SELECT count(*) FROM databases WHERE plan_id = $1
	`, id).Scan(&updated.DatabaseCount); err != nil {
		return entities.Plan{}, err
	}

	if err := tx.Commit(); err != nil {
		return entities.Plan{}, err
	}
	return updated, nil
}

package usecase

import (
	"context"
	"database/sql"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

// ActivePlans returns every active database package the shop offers, in
// display order.
func ActivePlans(ctx context.Context, db *sql.DB) ([]entities.Plan, error) {
	return repository.ListActivePlans(ctx, db)
}

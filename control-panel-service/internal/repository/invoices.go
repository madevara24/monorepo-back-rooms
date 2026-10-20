package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
)

// ErrInvoiceNotOpen is returned by PayInvoice when the invoice exists but
// is not in the "open" status.
var ErrInvoiceNotOpen = errors.New("invoice is not open")

const invoiceColumns = `id, user_id, status, total_amount_idr, period_start, period_end, created_at, paid_at`

func scanInvoice(row *sql.Row) (entities.Invoice, error) {
	var inv entities.Invoice
	err := row.Scan(&inv.ID, &inv.UserID, &inv.Status, &inv.TotalAmountIDR,
		&inv.PeriodStart, &inv.PeriodEnd, &inv.CreatedAt, &inv.PaidAt)
	return inv, err
}

// ListInvoicesByUser returns every invoice for a user, newest first.
func ListInvoicesByUser(ctx context.Context, db *sql.DB, userID string) ([]entities.Invoice, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+invoiceColumns+`
		FROM invoices
		WHERE user_id = $1
		ORDER BY period_start DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []entities.Invoice{}
	for rows.Next() {
		var inv entities.Invoice
		if err := rows.Scan(&inv.ID, &inv.UserID, &inv.Status, &inv.TotalAmountIDR,
			&inv.PeriodStart, &inv.PeriodEnd, &inv.CreatedAt, &inv.PaidAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// InvoiceByIDForUser returns an invoice scoped to its owner. It returns
// sql.ErrNoRows if the id does not exist or belongs to someone else.
func InvoiceByIDForUser(ctx context.Context, db *sql.DB, id, userID string) (entities.Invoice, error) {
	row := db.QueryRowContext(ctx, `
		SELECT `+invoiceColumns+`
		FROM invoices
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	return scanInvoice(row)
}

// PayInvoice moves an open invoice to paid. No money moves; this only
// flips the status and timestamp.
//
// It returns sql.ErrNoRows if the invoice does not exist or is not the
// caller's, and ErrInvoiceNotOpen if it exists but is already paid (or in
// any other non-open status).
func PayInvoice(ctx context.Context, db *sql.DB, id, userID string) (entities.Invoice, error) {
	row := db.QueryRowContext(ctx, `
		UPDATE invoices
		SET status = 'paid', paid_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'open'
		RETURNING `+invoiceColumns, id, userID)
	inv, err := scanInvoice(row)
	if errors.Is(err, sql.ErrNoRows) {
		if _, lookErr := InvoiceByIDForUser(ctx, db, id, userID); lookErr != nil {
			return entities.Invoice{}, lookErr
		}
		return entities.Invoice{}, ErrInvoiceNotOpen
	}
	return inv, err
}

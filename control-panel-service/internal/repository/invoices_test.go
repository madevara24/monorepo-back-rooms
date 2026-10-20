package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository/storetest"
)

func insertInvoice(t *testing.T, db *sql.DB, userID, status string) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO invoices (user_id, status, total_amount_idr, period_start, period_end)
		VALUES ($1, $2, 100000, '2026-10-01', '2026-10-31')
		RETURNING id
	`, userID, status).Scan(&id)
	if err != nil {
		t.Fatalf("insert invoice fixture: %v", err)
	}
	return id
}

func TestListInvoicesByUser(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "billed@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	other, err := repository.CreateUser(ctx, db, "notbilled@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	insertInvoice(t, db, owner.ID, "open")
	insertInvoice(t, db, other.ID, "open")

	rows, err := repository.ListInvoicesByUser(ctx, db, owner.ID)
	if err != nil {
		t.Fatalf("ListInvoicesByUser: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListInvoicesByUser returned %d rows, want 1", len(rows))
	}
}

func TestPayInvoiceMovesOpenToPaid(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "payer@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	invoiceID := insertInvoice(t, db, owner.ID, "open")

	paid, err := repository.PayInvoice(ctx, db, invoiceID, owner.ID)
	if err != nil {
		t.Fatalf("PayInvoice: %v", err)
	}
	if paid.Status != "paid" {
		t.Fatalf("PayInvoice status = %q, want %q", paid.Status, "paid")
	}
	if paid.PaidAt == nil {
		t.Fatal("PayInvoice did not set paid_at")
	}
}

func TestPayInvoiceAlreadyPaid(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "twice@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	invoiceID := insertInvoice(t, db, owner.ID, "paid")

	_, err = repository.PayInvoice(ctx, db, invoiceID, owner.ID)
	if !errors.Is(err, repository.ErrInvoiceNotOpen) {
		t.Fatalf("PayInvoice on an already-paid invoice error = %v, want ErrInvoiceNotOpen", err)
	}
}

func TestPayInvoiceWrongOwner(t *testing.T) {
	db := storetest.Open(t)
	ctx := context.Background()

	owner, err := repository.CreateUser(ctx, db, "actual-owner@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	intruder, err := repository.CreateUser(ctx, db, "intruder@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	invoiceID := insertInvoice(t, db, owner.ID, "open")

	_, err = repository.PayInvoice(ctx, db, invoiceID, intruder.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("PayInvoice by non-owner error = %v, want sql.ErrNoRows", err)
	}
}

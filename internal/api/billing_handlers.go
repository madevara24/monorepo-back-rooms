package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/madevara24/monorepo-back-rooms/internal/store"
)

type invoiceResponse struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	TotalAmountIDR int64      `json:"total_amount_idr"`
	PeriodStart    time.Time  `json:"period_start"`
	PeriodEnd      time.Time  `json:"period_end"`
	CreatedAt      time.Time  `json:"created_at"`
	PaidAt         *time.Time `json:"paid_at"`
}

func toInvoiceResponse(inv store.Invoice) invoiceResponse {
	return invoiceResponse{
		ID:             inv.ID,
		Status:         inv.Status,
		TotalAmountIDR: inv.TotalAmountIDR,
		PeriodStart:    inv.PeriodStart,
		PeriodEnd:      inv.PeriodEnd,
		CreatedAt:      inv.CreatedAt,
		PaidAt:         inv.PaidAt,
	}
}

func (a *API) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	rows, err := store.ListInvoicesByUser(r.Context(), a.DB, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list invoices")
		return
	}

	out := make([]invoiceResponse, len(rows))
	for i, inv := range rows {
		out[i] = toInvoiceResponse(inv)
	}
	writeJSON(w, http.StatusOK, out)
}

// handlePayInvoice moves an open invoice to paid. No money moves; this is
// a status flip only, since payments are mocked for this project.
func (a *API) handlePayInvoice(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	inv, err := store.PayInvoice(r.Context(), a.DB, r.PathValue("id"), user.ID)
	switch {
	case isNotFound(err):
		writeError(w, http.StatusNotFound, "invoice not found")
		return
	case errors.Is(err, store.ErrInvoiceNotOpen):
		writeError(w, http.StatusConflict, "invoice is not open")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not pay invoice")
		return
	}
	writeJSON(w, http.StatusOK, toInvoiceResponse(inv))
}

package handler

import (
	"net/http"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/usecase"
)

// planResponse is the public shape of a plan: the shop's price list.
type planResponse struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	SizeGB        int    `json:"size_gb"`
	PriceIDRMonth int64  `json:"price_idr_month"`
	Tagline       string `json:"tagline"`
	Badge         string `json:"badge"`
	SortOrder     int    `json:"sort_order"`
	Purchasable   bool   `json:"purchasable"`
}

func toPlanResponse(p entities.Plan) planResponse {
	return planResponse{
		ID:            p.ID,
		Slug:          p.Slug,
		Name:          p.Name,
		SizeGB:        p.SizeGB,
		PriceIDRMonth: p.PriceIDRMonth,
		Tagline:       p.Tagline,
		Badge:         p.Badge,
		SortOrder:     p.SortOrder,
		Purchasable:   p.Purchasable,
	}
}

// handleListPlans serves the shop's price list. It is public: anyone can
// see what is for sale without signing in.
func (h *Handler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := usecase.ActivePlans(r.Context(), h.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list plans")
		return
	}

	out := make([]planResponse, len(rows))
	for i, p := range rows {
		out[i] = toPlanResponse(p)
	}
	writeJSON(w, http.StatusOK, out)
}

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/entities"
	"github.com/madevara24/monorepo-back-rooms/control-panel-service/internal/repository"
)

// adminPlanResponse is the admin-facing shape of a plan: it adds the
// fields the shop's GET /api/plans deliberately omits, so the admin can
// see a plan it has hidden and how many databases still reference it.
type adminPlanResponse struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	SizeGB        int    `json:"size_gb"`
	PriceIDRMonth int64  `json:"price_idr_month"`
	Tagline       string `json:"tagline"`
	Badge         string `json:"badge"`
	SortOrder     int    `json:"sort_order"`
	Active        bool   `json:"active"`
	DatabaseCount int    `json:"database_count"`
}

func toAdminPlanResponse(p entities.Plan) adminPlanResponse {
	return adminPlanResponse{
		ID:            p.ID,
		Slug:          p.Slug,
		Name:          p.Name,
		SizeGB:        p.SizeGB,
		PriceIDRMonth: p.PriceIDRMonth,
		Tagline:       p.Tagline,
		Badge:         p.Badge,
		SortOrder:     p.SortOrder,
		Active:        p.Active,
		DatabaseCount: p.DatabaseCount,
	}
}

// handleAdminListPlans serves every plan, including inactive ones, for
// the admin Packages panel.
func (h *Handler) handleAdminListPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := repository.ListAllPlans(r.Context(), h.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list plans")
		return
	}

	out := make([]adminPlanResponse, len(rows))
	for i, p := range rows {
		out[i] = toAdminPlanResponse(p)
	}
	writeJSON(w, http.StatusOK, out)
}

// planSlugPattern matches the slug rule: lowercase letters and digits,
// hyphen-separated, no leading, trailing, or doubled hyphen.
var planSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const (
	planTaglineMaxLen = 120
	planBadgeMaxLen   = 24
)

// validatePlanSizeGB, validatePlanPrice, validatePlanName,
// validatePlanTagline, and validatePlanBadge each return the contract's
// validation message, or "" if the value is acceptable.

func validatePlanSizeGB(sizeGB int) string {
	if sizeGB < 1 || sizeGB > 20 {
		return "size_gb must be between 1 and 20"
	}
	return ""
}

func validatePlanPrice(priceIDRMonth int64) string {
	if priceIDRMonth < 1 {
		return "price_idr_month must be a whole number of at least 1"
	}
	return ""
}

func validatePlanName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "name is required"
	}
	return ""
}

func validatePlanTagline(tagline string) string {
	if len(tagline) > planTaglineMaxLen {
		return fmt.Sprintf("tagline must be at most %d characters", planTaglineMaxLen)
	}
	return ""
}

func validatePlanBadge(badge string) string {
	if len(badge) > planBadgeMaxLen {
		return fmt.Sprintf("badge must be at most %d characters", planBadgeMaxLen)
	}
	return ""
}

type createPlanRequest struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	SizeGB        int    `json:"size_gb"`
	PriceIDRMonth int64  `json:"price_idr_month"`
	Tagline       string `json:"tagline"`
	Badge         string `json:"badge"`
	SortOrder     int    `json:"sort_order"`
	Active        *bool  `json:"active"`
}

// handleAdminCreatePlan adds a new package to the shop. active defaults
// to true: a newly created package is for sale unless told otherwise.
func (h *Handler) handleAdminCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req createPlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !planSlugPattern.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "slug must be lowercase letters, numbers, and hyphens")
		return
	}
	for _, msg := range []string{
		validatePlanName(req.Name),
		validatePlanSizeGB(req.SizeGB),
		validatePlanPrice(req.PriceIDRMonth),
		validatePlanTagline(req.Tagline),
		validatePlanBadge(req.Badge),
	} {
		if msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	created, err := repository.CreatePlan(r.Context(), h.DB, entities.Plan{
		Slug:          req.Slug,
		Name:          req.Name,
		SizeGB:        req.SizeGB,
		PriceIDRMonth: req.PriceIDRMonth,
		Tagline:       req.Tagline,
		Badge:         req.Badge,
		SortOrder:     req.SortOrder,
		Active:        active,
	})
	if errors.Is(err, repository.ErrPlanSlugTaken) {
		writeError(w, http.StatusConflict, "plan slug already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create plan")
		return
	}
	writeJSON(w, http.StatusCreated, toAdminPlanResponse(created))
}

type updatePlanRequest struct {
	Slug          *string `json:"slug"`
	Name          *string `json:"name"`
	SizeGB        *int    `json:"size_gb"`
	PriceIDRMonth *int64  `json:"price_idr_month"`
	Tagline       *string `json:"tagline"`
	Badge         *string `json:"badge"`
	SortOrder     *int    `json:"sort_order"`
	Active        *bool   `json:"active"`
}

// handleAdminUpdatePlan edits any subset of a plan's fields except its
// slug, which is the buy parameter live links depend on. Deactivating the
// last active plan is rejected: it would empty the shop.
func (h *Handler) handleAdminUpdatePlan(w http.ResponseWriter, r *http.Request) {
	var req updatePlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Slug != nil {
		writeError(w, http.StatusBadRequest, "slug is immutable")
		return
	}
	if req.Name != nil {
		if msg := validatePlanName(*req.Name); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if req.SizeGB != nil {
		if msg := validatePlanSizeGB(*req.SizeGB); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if req.PriceIDRMonth != nil {
		if msg := validatePlanPrice(*req.PriceIDRMonth); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if req.Tagline != nil {
		if msg := validatePlanTagline(*req.Tagline); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if req.Badge != nil {
		if msg := validatePlanBadge(*req.Badge); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}

	updated, err := repository.UpdatePlan(r.Context(), h.DB, r.PathValue("id"), repository.PlanUpdate{
		Name:          req.Name,
		SizeGB:        req.SizeGB,
		PriceIDRMonth: req.PriceIDRMonth,
		Tagline:       req.Tagline,
		Badge:         req.Badge,
		SortOrder:     req.SortOrder,
		Active:        req.Active,
	})
	if isNotFound(err) {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}
	if errors.Is(err, repository.ErrLastActivePlan) {
		writeError(w, http.StatusConflict, "at least one active plan is required")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update plan")
		return
	}
	writeJSON(w, http.StatusOK, toAdminPlanResponse(updated))
}

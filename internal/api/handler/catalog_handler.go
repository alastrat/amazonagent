package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type CatalogHandler struct {
	products        port.DiscoveredProductRepo
	brandIntel      port.BrandIntelligenceRepo
}

func NewCatalogHandler(products port.DiscoveredProductRepo, brandIntel port.BrandIntelligenceRepo) *CatalogHandler {
	return &CatalogHandler{products: products, brandIntel: brandIntel}
}

func (h *CatalogHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	filter := port.DiscoveredProductFilter{
		Limit:  50,
		Offset: 0,
		SortBy: "last_seen_at",
		SortDir: "desc",
	}

	if v := r.URL.Query().Get("category"); v != "" {
		filter.Category = &v
	}
	if v := r.URL.Query().Get("brand_id"); v != "" {
		filter.BrandID = &v
	}
	if v := r.URL.Query().Get("min_margin"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			filter.MinMargin = &f
		}
	}
	if v := r.URL.Query().Get("min_sellers"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinSellers = &n
		}
	}
	if v := r.URL.Query().Get("eligibility"); v != "" {
		filter.EligibilityStatus = &v
	}
	if v := r.URL.Query().Get("source"); v != "" {
		st := domain.ScanType(v)
		filter.Source = &st
	}
	if v := r.URL.Query().Get("search"); v != "" {
		filter.Search = &v
	}
	if v := r.URL.Query().Get("sort_by"); v != "" {
		filter.SortBy = v
	}
	if v := r.URL.Query().Get("sort_dir"); v != "" {
		filter.SortDir = v
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	products, total, err := h.products.List(r.Context(), ac.TenantID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list products")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"products": products,
		"total":    total,
	})
}

func (h *CatalogHandler) ListBrands(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	filter := port.BrandIntelligenceFilter{
		Limit:   50,
		SortBy:  "avg_margin",
		SortDir: "desc",
	}

	if v := r.URL.Query().Get("category"); v != "" {
		filter.Category = &v
	}
	if v := r.URL.Query().Get("min_margin"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			filter.MinMargin = &f
		}
	}
	if v := r.URL.Query().Get("min_products"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.MinProducts = &n
		}
	}
	if v := r.URL.Query().Get("search"); v != "" {
		filter.Search = &v
	}
	if v := r.URL.Query().Get("sort_by"); v != "" {
		filter.SortBy = v
	}
	if v := r.URL.Query().Get("sort_dir"); v != "" {
		filter.SortDir = v
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	brands, err := h.brandIntel.List(r.Context(), ac.TenantID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list brands")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"brands": brands,
	})
}

func (h *CatalogHandler) ListBrandProducts(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	brandID := chi.URLParam(r, "id")

	filter := port.DiscoveredProductFilter{
		BrandID: &brandID,
		Limit:   50,
		SortBy:  "estimated_margin_pct",
		SortDir: "desc",
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	products, total, err := h.products.List(r.Context(), ac.TenantID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list brand products")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"products": products,
		"total":    total,
	})
}

func (h *CatalogHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	// Quick stats from the catalog
	allFilter := port.DiscoveredProductFilter{Limit: 1}
	_, total, _ := h.products.List(r.Context(), ac.TenantID, allFilter)

	eligible := "eligible"
	eligibleFilter := port.DiscoveredProductFilter{EligibilityStatus: &eligible, Limit: 1}
	_, eligibleCount, _ := h.products.List(r.Context(), ac.TenantID, eligibleFilter)

	response.JSON(w, http.StatusOK, map[string]any{
		"total_products":  total,
		"eligible_count":  eligibleCount,
	})
}

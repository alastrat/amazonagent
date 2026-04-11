package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/inngest"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type AssessmentHandler struct {
	assessment     *service.AssessmentService
	durableRuntime *inngest.DurableRuntime
}

func NewAssessmentHandler(assessment *service.AssessmentService, durableRuntime *inngest.DurableRuntime) *AssessmentHandler {
	return &AssessmentHandler{assessment: assessment, durableRuntime: durableRuntime}
}

func (h *AssessmentHandler) Start(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	// No longer collecting account age / listings / capital — inferred post-assessment
	if r.Body != nil {
		// Drain body for compatibility with old clients sending JSON
		r.Body.Close()
	}

	// Create profile synchronously
	profile, err := h.assessment.StartAssessment(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to start assessment: "+err.Error())
		return
	}

	// Trigger Inngest workflow for the async discovery assessment
	if h.durableRuntime != nil {
		slog.Info("assessment: triggering inngest workflow", "tenant_id", ac.TenantID)
		if err := h.durableRuntime.TriggerAssessment(r.Context(), ac.TenantID); err != nil {
			slog.Error("assessment: failed to trigger inngest", "tenant_id", ac.TenantID, "error", err)
			response.Error(w, http.StatusInternalServerError, "failed to trigger assessment workflow: "+err.Error())
			return
		}
		slog.Info("assessment: inngest event sent", "tenant_id", ac.TenantID)
	} else {
		slog.Warn("assessment: no durable runtime available, scan will not run")
	}

	response.JSON(w, http.StatusOK, profile)
}

func (h *AssessmentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	profile, err := h.assessment.GetProfile(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no assessment found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"status":    profile.AssessmentStatus,
		"archetype": profile.Archetype,
	})
}

func (h *AssessmentHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	profile, err := h.assessment.GetProfile(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no profile found")
		return
	}

	fingerprint, _ := h.assessment.GetFingerprint(r.Context(), ac.TenantID)

	response.JSON(w, http.StatusOK, map[string]any{
		"profile":     profile,
		"fingerprint": fingerprint,
	})
}

func (h *AssessmentHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	profile, _ := h.assessment.GetProfile(r.Context(), ac.TenantID)
	fingerprint, _ := h.assessment.GetFingerprint(r.Context(), ac.TenantID)

	status := "pending"
	if profile != nil {
		status = string(profile.AssessmentStatus)
	}

	// Build a lookup from DiscoveryCategories for canonical category names.
	discoveryCatNames := make(map[string]string, len(service.DiscoveryCategories))
	for _, dc := range service.DiscoveryCategories {
		discoveryCatNames[strings.ToLower(dc.Name)] = dc.Name
	}

	resolveCategoryName := func(raw string) string {
		if raw == "" {
			return "Uncategorized"
		}
		if canonical, ok := discoveryCatNames[strings.ToLower(raw)]; ok {
			return canonical
		}
		return raw
	}

	slugify := func(name string) string {
		s := strings.ToLower(name)
		s = strings.ReplaceAll(s, " & ", "-")
		s = strings.ReplaceAll(s, ", ", "-")
		s = strings.ReplaceAll(s, " ", "-")
		return s
	}

	truncate := func(s string, maxLen int) string {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen-1] + "…"
	}

	// ── Build tree: Root → Categories → Brands → Products ────────

	var categoryChildren []map[string]any
	var allProducts []map[string]any
	openBrandsSet := make(map[string]bool)

	if fingerprint != nil {
		// Deduplicate categories from fingerprint.
		type catInfo struct {
			name          string
			openRate      float64
			eligibleCount int
			totalCount    int
		}
		uniqueCats := make(map[string]*catInfo)
		for _, cat := range fingerprint.Categories {
			resolved := resolveCategoryName(cat.Category)
			key := strings.ToLower(resolved)
			if existing, ok := uniqueCats[key]; ok {
				existing.eligibleCount += cat.OpenCount
				existing.totalCount += cat.ProbeCount
				if existing.totalCount > 0 {
					existing.openRate = float64(existing.eligibleCount) / float64(existing.totalCount) * 100
				}
			} else {
				uniqueCats[key] = &catInfo{
					name:          resolved,
					openRate:      cat.OpenRate,
					eligibleCount: cat.OpenCount,
					totalCount:    cat.ProbeCount,
				}
			}
		}

		// Group brand results by category → brand, collecting product nodes.
		type brandAgg struct {
			brand           string
			eligible        bool
			productCount    int
			eligibleCount   int
			productChildren []map[string]any
		}
		catBrands := make(map[string]map[string]*brandAgg) // catKey -> brandKey -> agg

		for _, br := range fingerprint.BrandResults {
			resolvedCat := resolveCategoryName(br.Category)
			catKey := strings.ToLower(resolvedCat)
			brandName := br.Brand
			if brandName == "" {
				brandName = "Other"
			}
			brandKey := strings.ToLower(brandName)

			if catBrands[catKey] == nil {
				catBrands[catKey] = make(map[string]*brandAgg)
			}

			productNode := map[string]any{
				"id":             "product-" + br.ASIN,
				"name":           truncate(br.Title, 40),
				"type":           "product",
				"asin":           br.ASIN,
				"price":          br.Price,
				"est_margin_pct": br.EstMarginPct,
				"seller_count":   br.SellerCount,
				"eligible":       br.Eligible,
				"value":          1,
			}

			// Flat products array for click-to-table
			allProducts = append(allProducts, map[string]any{
				"asin":           br.ASIN,
				"title":          br.Title,
				"brand":          brandName,
				"category":       resolvedCat,
				"price":          br.Price,
				"est_margin_pct": br.EstMarginPct,
				"seller_count":   br.SellerCount,
				"eligible":       br.Eligible,
			})

			if existing, ok := catBrands[catKey][brandKey]; ok {
				existing.productCount++
				if br.Eligible {
					existing.eligible = true
					existing.eligibleCount++
				}
				existing.productChildren = append(existing.productChildren, productNode)
			} else {
				eligCount := 0
				if br.Eligible {
					eligCount = 1
				}
				catBrands[catKey][brandKey] = &brandAgg{
					brand:           brandName,
					eligible:        br.Eligible,
					productCount:    1,
					eligibleCount:   eligCount,
					productChildren: []map[string]any{productNode},
				}
			}

			if br.Eligible && brandName != "Unknown Brand" {
				openBrandsSet[brandName] = true
			}
		}

		// Build category nodes with brand children containing product children.
		for catKey, ci := range uniqueCats {
			catSlug := slugify(ci.name)

			var brandChildren []map[string]any
			if brands, ok := catBrands[catKey]; ok {
				for _, ba := range brands {
					brandValue := ba.eligibleCount
					if brandValue < 1 {
						brandValue = 1
					}
					bn := map[string]any{
						"id":            fmt.Sprintf("brand-%s", slugify(ba.brand)),
						"name":          ba.brand,
						"type":          "brand",
						"eligible":      ba.eligible,
						"product_count": ba.productCount,
						"value":         brandValue,
					}
					brandChildren = append(brandChildren, bn)
				}
			}

			catValue := ci.eligibleCount
			if catValue < 1 {
				catValue = 1
			}
			catNode := map[string]any{
				"id":             "cat-" + catSlug,
				"name":           ci.name,
				"type":           "category",
				"open_rate":      ci.openRate,
				"eligible_count": ci.eligibleCount,
				"total_count":    ci.totalCount,
				"value":          catValue,
				"children":       brandChildren,
			}
			categoryChildren = append(categoryChildren, catNode)
		}
	}

	// ── Build tree root ───────────────────────────────────────────

	rootValue := len(categoryChildren)
	if rootValue < 1 {
		rootValue = 1
	}
	tree := map[string]any{
		"id":       "root",
		"name":     "Amazon US",
		"value":    rootValue,
		"children": categoryChildren,
	}

	// ── Stats with deduplicated category count ────────────────────

	categoriesScanned := 0
	eligibleProducts := 0
	restrictedProducts := 0

	if fingerprint != nil {
		seen := make(map[string]bool)
		for _, cat := range fingerprint.Categories {
			resolved := resolveCategoryName(cat.Category)
			key := strings.ToLower(resolved)
			if !seen[key] {
				seen[key] = true
				categoriesScanned++
			}
		}
		eligibleProducts = fingerprint.TotalEligible
		restrictedProducts = fingerprint.TotalRestricted
	}

	stats := map[string]any{
		"categories_scanned":  categoriesScanned,
		"categories_total":    len(service.DiscoveryCategories),
		"eligible_products":   eligibleProducts,
		"restricted_products": restrictedProducts,
		"open_brands":         len(openBrandsSet),
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"tree":     tree,
		"products": allProducts,
		"stats":    stats,
	})
}

func (h *AssessmentHandler) Reset(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	if err := h.assessment.ResetAssessment(r.Context(), ac.TenantID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to reset assessment: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

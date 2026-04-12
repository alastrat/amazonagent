package handler

import (
	"encoding/json"
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
	hub            *service.AssessmentHub
}

func NewAssessmentHandler(assessment *service.AssessmentService, durableRuntime *inngest.DurableRuntime, hub *service.AssessmentHub) *AssessmentHandler {
	return &AssessmentHandler{assessment: assessment, durableRuntime: durableRuntime, hub: hub}
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

	// ── Build tree: Root → Categories → Subcategories → Brands ──

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

		// Group brand results by category → subcategory → brand.
		type brandAgg struct {
			brand           string
			eligible        bool
			productCount    int
			eligibleCount   int
			ungatableCount  int
			restrictedCount int
			bestStatus      string // best status among products: eligible > ungatable > restricted
		}
		type subcatAgg struct {
			name           string
			eligibleCount  int
			ungatableCount int
			totalCount     int
			brands         map[string]*brandAgg // brandKey -> agg
		}
		// catKey -> subcatKey -> subcatAgg
		catSubcats := make(map[string]map[string]*subcatAgg)

		for _, br := range fingerprint.BrandResults {
			resolvedCat := resolveCategoryName(br.Category)
			catKey := strings.ToLower(resolvedCat)
			brandName := br.Brand
			if brandName == "" {
				brandName = "Generic"
			}
			brandKey := strings.ToLower(brandName)

			subcatName := br.Subcategory
			if subcatName == "" {
				subcatName = resolvedCat // fall back to category name
			}
			subcatKey := strings.ToLower(subcatName)

			eligStatus := br.EligibilityStatus
			if eligStatus == "" {
				if br.Eligible {
					eligStatus = "eligible"
				} else {
					eligStatus = "restricted"
				}
			}

			if catSubcats[catKey] == nil {
				catSubcats[catKey] = make(map[string]*subcatAgg)
			}
			sa, ok := catSubcats[catKey][subcatKey]
			if !ok {
				sa = &subcatAgg{
					name:   subcatName,
					brands: make(map[string]*brandAgg),
				}
				catSubcats[catKey][subcatKey] = sa
			}
			sa.totalCount++
			switch eligStatus {
			case "eligible":
				sa.eligibleCount++
			case "ungatable":
				sa.ungatableCount++
			}

			if existing, ok := sa.brands[brandKey]; ok {
				existing.productCount++
				switch eligStatus {
				case "eligible":
					existing.eligible = true
					existing.eligibleCount++
					existing.bestStatus = "eligible"
				case "ungatable":
					existing.ungatableCount++
					if existing.bestStatus != "eligible" {
						existing.bestStatus = "ungatable"
					}
				default:
					existing.restrictedCount++
				}
			} else {
				bs := eligStatus
				elig := eligStatus == "eligible"
				eligCount, ungCount, resCount := 0, 0, 0
				switch eligStatus {
				case "eligible":
					eligCount = 1
				case "ungatable":
					ungCount = 1
				default:
					resCount = 1
					bs = "restricted"
				}
				sa.brands[brandKey] = &brandAgg{
					brand:           brandName,
					eligible:        elig,
					productCount:    1,
					eligibleCount:   eligCount,
					ungatableCount:  ungCount,
					restrictedCount: resCount,
					bestStatus:      bs,
				}
			}

			// Flat products array for click-to-table
			allProducts = append(allProducts, map[string]any{
				"asin":               br.ASIN,
				"title":              br.Title,
				"brand":              brandName,
				"category":           resolvedCat,
				"subcategory":        subcatName,
				"price":              br.Price,
				"est_margin_pct":     br.EstMarginPct,
				"seller_count":       br.SellerCount,
				"eligible":           br.Eligible,
				"eligibility_status": eligStatus,
				"approval_url":       br.ApprovalURL,
			})

			if (br.Eligible || eligStatus == "ungatable") && brandName != "Unknown Brand" {
				openBrandsSet[brandName] = true
			}
		}

		// Build category nodes: category → subcategory → brand.
		for catKey, ci := range uniqueCats {
			catSlug := slugify(ci.name)

			var subcatChildren []map[string]any
			if subcats, ok := catSubcats[catKey]; ok {
				for _, sa := range subcats {
					var brandChildren []map[string]any
					for _, ba := range sa.brands {
						brandValue := ba.eligibleCount
						if brandValue < 1 {
							brandValue = 1
						}
						bn := map[string]any{
							"id":                 fmt.Sprintf("brand-%s", slugify(ba.brand)),
							"name":               ba.brand,
							"type":               "brand",
							"eligible":            ba.eligible,
							"eligibility_status":  ba.bestStatus,
							"product_count":       ba.productCount,
							"eligible_count":      ba.eligibleCount,
							"ungatable_count":     ba.ungatableCount,
							"restricted_count":    ba.restrictedCount,
							"value":               brandValue,
						}
						brandChildren = append(brandChildren, bn)
					}

					subcatValue := sa.eligibleCount
					if subcatValue < 1 {
						subcatValue = 1
					}
					subcatNode := map[string]any{
						"id":               "subcat-" + slugify(sa.name),
						"name":             sa.name,
						"type":             "subcategory",
						"eligible_count":   sa.eligibleCount,
						"ungatable_count":  sa.ungatableCount,
						"total_count":      sa.totalCount,
						"value":            subcatValue,
						"children":         brandChildren,
					}
					subcatChildren = append(subcatChildren, subcatNode)
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
				"children":       subcatChildren,
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

	// ── Stats — computed from deduplicated products to match the frontend table ──

	categoriesScanned := 0
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
	}

	// Deduplicate products by ASIN and count by status
	eligibleProducts := 0
	ungatableProducts := 0
	restrictedProducts := 0
	seenASINs := make(map[string]bool, len(allProducts))
	var dedupedProducts []map[string]any
	for _, p := range allProducts {
		asin, _ := p["asin"].(string)
		if asin == "" || seenASINs[asin] {
			continue
		}
		seenASINs[asin] = true
		dedupedProducts = append(dedupedProducts, p)

		status, _ := p["eligibility_status"].(string)
		switch status {
		case "eligible":
			eligibleProducts++
		case "ungatable":
			ungatableProducts++
		default:
			restrictedProducts++
		}
	}

	stats := map[string]any{
		"categories_scanned":  categoriesScanned,
		"categories_total":    len(service.DiscoveryCategories),
		"eligible_products":   eligibleProducts,
		"ungatable_products":  ungatableProducts,
		"restricted_products": restrictedProducts,
		"open_brands":         len(openBrandsSet),
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"tree":     tree,
		"products": dedupedProducts,
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

// StreamEvents serves an SSE stream of real-time assessment progress.
func (h *AssessmentHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, history, unsub := h.hub.Subscribe(ac.TenantID)
	defer unsub()

	// Send catch-up history
	if len(history) > 0 {
		data, _ := json.Marshal(history)
		fmt.Fprintf(w, "event: catchup\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// Stream live events
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				// Channel closed — assessment done
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

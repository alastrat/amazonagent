package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ---------------------------------------------------------------------------
// Discovery Assessment categories — 20 high-value wholesale categories
// ---------------------------------------------------------------------------

// AssessmentCategory defines one browse-node category to scan.
type AssessmentCategory struct {
	Name           string
	BrowseNodeID   string
	ExpectedOpen   float64 // expected open rate for sorting/priority
}

// DiscoveryCategories are the 20 categories ordered by expected open rate (highest first).
var DiscoveryCategories = []AssessmentCategory{
	{"Home & Kitchen", "1055398", 0.85},
	{"Tools & Home Improvement", "228013", 0.80},
	{"Office Products", "1064954", 0.80},
	{"Sports & Outdoors", "3375251", 0.75},
	{"Patio, Lawn & Garden", "2972638", 0.70},
	{"Automotive", "15684181", 0.70},
	{"Arts, Crafts & Sewing", "2617941011", 0.70},
	{"Industrial & Scientific", "16310091", 0.65},
	{"Pet Supplies", "2619533011", 0.65},
	{"Musical Instruments", "11091801", 0.65},
	{"Toys & Games", "165793011", 0.60},
	{"Baby Products", "165796011", 0.55},
	{"Kitchen & Dining", "284507", 0.55},
	{"Clothing, Shoes & Jewelry", "7141123011", 0.50},
	{"Electronics", "172282", 0.45},
	{"Cell Phones & Accessories", "2335752011", 0.45},
	{"Video Games", "468642", 0.40},
	{"Grocery & Gourmet Food", "16310101", 0.30},
	{"Beauty & Personal Care", "3760911", 0.25},
	{"Health & Household", "3760901", 0.20},
}

// ---------------------------------------------------------------------------
// Circuit breaker constants
// ---------------------------------------------------------------------------

const (
	cbPerCategoryLimit       = 5   // consecutive restricted → skip rest of category
	cbEarlySuccessThreshold  = 50  // eligible products → stop scanning
	cbAPIBudget              = 600 // hard cap on total SP-API calls
	cbTimeBudgetSeconds      = 300 // 5 minutes wall-clock
	cbEmptyCategoryThreshold = 3   // consecutive 0-eligible categories → jump strategy
	cbProductsPerCategory    = 20  // products to search per category
)

// ---------------------------------------------------------------------------
// circuitState tracks breaker state across the scan
// ---------------------------------------------------------------------------

type circuitState struct {
	apiCalls              int
	totalEligible         int
	consecutiveEmptyCats  int
	startTime             time.Time
	firedBreakers         []string
}

func newCircuitState() *circuitState {
	return &circuitState{startTime: time.Now()}
}

func (cs *circuitState) addAPICalls(n int) { cs.apiCalls += n }

func (cs *circuitState) budgetExhausted() bool { return cs.apiCalls >= cbAPIBudget }

func (cs *circuitState) timeExceeded() bool {
	return time.Since(cs.startTime).Seconds() >= cbTimeBudgetSeconds
}

func (cs *circuitState) earlySuccess() bool { return cs.totalEligible >= cbEarlySuccessThreshold }

func (cs *circuitState) shouldStop() bool {
	return cs.budgetExhausted() || cs.timeExceeded() || cs.earlySuccess()
}

func (cs *circuitState) fireBreaker(name string) {
	cs.firedBreakers = append(cs.firedBreakers, name)
	slog.Info("assessment: circuit breaker fired", "breaker", name,
		"api_calls", cs.apiCalls, "eligible", cs.totalEligible,
		"elapsed_s", time.Since(cs.startTime).Seconds())
}

// ---------------------------------------------------------------------------
// AssessmentService
// ---------------------------------------------------------------------------

// AssessmentService orchestrates the seller account assessment flow.
type AssessmentService struct {
	profiles      port.SellerProfileRepo
	fingerprints  port.EligibilityFingerprintRepo
	sharedCatalog *SharedCatalogService
	funnelSvc     *FunnelService
	idGen         port.IDGenerator
}

func NewAssessmentService(
	profiles port.SellerProfileRepo,
	fingerprints port.EligibilityFingerprintRepo,
	sharedCatalog *SharedCatalogService,
	funnelSvc *FunnelService,
	idGen port.IDGenerator,
) *AssessmentService {
	return &AssessmentService{
		profiles:      profiles,
		fingerprints:  fingerprints,
		sharedCatalog: sharedCatalog,
		funnelSvc:     funnelSvc,
		idGen:         idGen,
	}
}

// StartAssessment creates a seller profile and marks assessment as running.
func (s *AssessmentService) StartAssessment(ctx context.Context, tenantID domain.TenantID) (*domain.SellerProfile, error) {
	now := time.Now()
	profile := &domain.SellerProfile{
		ID:               s.idGen.New(),
		TenantID:         tenantID,
		Archetype:        domain.SellerArchetypeGreenhorn, // default; reclassified post-assessment
		AssessmentStatus: domain.AssessmentStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.profiles.Create(ctx, profile); err != nil {
		return nil, err
	}

	slog.Info("assessment: started", "tenant_id", tenantID)
	return profile, nil
}

// RunDiscoveryAssessment executes the three-phase discovery assessment.
// It accepts a per-tenant SP-API client — never uses a global client.
func (s *AssessmentService) RunDiscoveryAssessment(
	ctx context.Context,
	tenantID domain.TenantID,
	spapi port.ProductSearcher,
) (*domain.AssessmentOutcome, error) {
	if spapi == nil {
		return nil, fmt.Errorf("assessment requires a per-tenant SP-API client")
	}

	cs := newCircuitState()

	slog.Info("assessment: beginning discovery", "tenant_id", tenantID, "categories", len(DiscoveryCategories))

	// ── Phase 1: Broad Category Search ─────────────────────────
	allResults, categoryStats := s.phase1SearchCategories(ctx, tenantID, spapi, cs)

	// ── Phase 2: Evaluate eligible products via funnel T1-T3 ───
	var survivors []FunnelSurvivor
	var funnelStats FunnelStats
	eligibleResults := filterEligible(allResults)

	if len(eligibleResults) > 0 {
		var err error
		survivors, funnelStats, err = s.phase2EvaluateProducts(ctx, tenantID, eligibleResults, spapi)
		if err != nil {
			slog.Warn("assessment: funnel evaluation failed", "error", err)
			// Continue — we still have eligibility data
		}
		// Count funnel API calls (GetProductDetails batches)
		funnelAPICalls := (len(eligibleResults) + 19) / 20
		cs.addAPICalls(funnelAPICalls)
	}

	// ── Phase 3: Build outcome ─────────────────────────────────
	outcome := s.phase3BuildOutcome(tenantID, allResults, eligibleResults, survivors, funnelStats, categoryStats, cs)

	// ── Persist fingerprint ────────────────────────────────────
	if err := s.persistFingerprint(ctx, tenantID, allResults, categoryStats); err != nil {
		slog.Warn("assessment: failed to persist fingerprint", "error", err)
	}

	slog.Info("assessment: discovery complete",
		"tenant_id", tenantID,
		"searched", outcome.TotalSearched,
		"eligible", outcome.TotalEligible,
		"qualified", outcome.TotalQualified,
		"api_calls", outcome.APICallsUsed,
		"duration_s", outcome.DurationSeconds,
		"has_opportunities", outcome.HasOpportunities,
		"breakers", outcome.CircuitBreakers)

	return outcome, nil
}

// ---------------------------------------------------------------------------
// Phase 1 — Broad Category Search with circuit breakers
// ---------------------------------------------------------------------------

type categoryStat struct {
	category     string
	browseNodeID string
	searched     int
	eligible     int
}

func (s *AssessmentService) phase1SearchCategories(
	ctx context.Context,
	tenantID domain.TenantID,
	spapi port.ProductSearcher,
	cs *circuitState,
) ([]domain.AssessmentSearchResult, []categoryStat) {
	var allResults []domain.AssessmentSearchResult
	var catStats []categoryStat

	// Build scan order: start with highest expected open rate (already sorted).
	// The repeated-failure breaker will re-prioritize if needed.
	scanOrder := make([]int, len(DiscoveryCategories))
	for i := range scanOrder {
		scanOrder[i] = i
	}

	scanned := make(map[int]bool)

	for _, idx := range scanOrder {
		if cs.shouldStop() {
			if cs.budgetExhausted() {
				cs.fireBreaker(fmt.Sprintf("api_budget_exhausted:%d_calls", cs.apiCalls))
			}
			if cs.timeExceeded() {
				cs.fireBreaker(fmt.Sprintf("time_budget_exceeded:%.0fs", time.Since(cs.startTime).Seconds()))
			}
			if cs.earlySuccess() {
				cs.fireBreaker(fmt.Sprintf("early_success:%d_eligible", cs.totalEligible))
			}
			break
		}

		if scanned[idx] {
			continue
		}
		scanned[idx] = true

		cat := DiscoveryCategories[idx]
		catResult := s.scanOneCategory(ctx, tenantID, spapi, cat, cs)
		allResults = append(allResults, catResult.results...)
		catStats = append(catStats, catResult.stat)

		// ── Repeated failure detection ──
		if catResult.stat.eligible == 0 {
			cs.consecutiveEmptyCats++
		} else {
			cs.consecutiveEmptyCats = 0
		}

		if cs.consecutiveEmptyCats >= cbEmptyCategoryThreshold {
			cs.fireBreaker(fmt.Sprintf("repeated_failure:%d_empty_categories", cs.consecutiveEmptyCats))
			cs.consecutiveEmptyCats = 0

			// Jump to highest expected open-rate category not yet scanned
			jumped := false
			for _, jumpIdx := range scanOrder {
				if !scanned[jumpIdx] && DiscoveryCategories[jumpIdx].ExpectedOpen > cat.ExpectedOpen {
					// Reorder: put this one next
					// We just continue the loop — the check at the top will skip scanned ones
					// Actually we need to scan it now
					if cs.shouldStop() {
						break
					}
					scanned[jumpIdx] = true
					jumpCat := DiscoveryCategories[jumpIdx]
					jumpResult := s.scanOneCategory(ctx, tenantID, spapi, jumpCat, cs)
					allResults = append(allResults, jumpResult.results...)
					catStats = append(catStats, jumpResult.stat)
					if jumpResult.stat.eligible > 0 {
						cs.consecutiveEmptyCats = 0
					}
					jumped = true
					break
				}
			}
			if !jumped {
				// All high-rate categories scanned — nothing left to jump to
				slog.Info("assessment: no higher open-rate categories to jump to")
			}
		}
	}

	return allResults, catStats
}

type categorySearchResult struct {
	results []domain.AssessmentSearchResult
	stat    categoryStat
}

func (s *AssessmentService) scanOneCategory(
	ctx context.Context,
	tenantID domain.TenantID,
	spapi port.ProductSearcher,
	cat AssessmentCategory,
	cs *circuitState,
) categorySearchResult {
	stat := categoryStat{
		category:     cat.Name,
		browseNodeID: cat.BrowseNodeID,
	}
	var results []domain.AssessmentSearchResult

	slog.Info("assessment: scanning category", "category", cat.Name, "node", cat.BrowseNodeID)

	// Search for products in this category using keyword search
	// (browse node search requires keywords param — use category name as keyword)
	products, err := spapi.SearchProducts(ctx, []string{cat.Name}, "US")
	cs.addAPICalls(1) // 1 catalog search call

	if err != nil {
		slog.Warn("assessment: category search failed", "category", cat.Name, "error", err)
		return categorySearchResult{results: results, stat: stat}
	}

	if len(products) == 0 {
		slog.Info("assessment: category returned 0 products", "category", cat.Name)
		return categorySearchResult{results: results, stat: stat}
	}

	// Limit to cbProductsPerCategory
	if len(products) > cbProductsPerCategory {
		products = products[:cbProductsPerCategory]
	}

	consecutiveRestricted := 0

	for _, p := range products {
		if cs.shouldStop() {
			break
		}

		// Check eligibility
		restrictions, err := spapi.CheckListingEligibility(ctx, []string{p.ASIN}, "US")
		cs.addAPICalls(1) // 1 eligibility check call

		if err != nil {
			slog.Warn("assessment: eligibility check failed", "asin", p.ASIN, "error", err)
			continue
		}

		eligible := true
		reason := ""
		if len(restrictions) > 0 && !restrictions[0].Allowed {
			eligible = false
			reason = restrictions[0].Reason
		}

		result := domain.AssessmentSearchResult{
			ASIN:              p.ASIN,
			Title:             p.Title,
			Brand:             p.Brand,
			Category:          cat.Name,
			Subcategory:       p.BSRCategory,
			AmazonPrice:       p.AmazonPrice,
			BSRRank:           p.BSRRank,
			SellerCount:       p.SellerCount,
			Eligible:          eligible,
			RestrictionReason: reason,
		}
		results = append(results, result)
		stat.searched++

		if eligible {
			stat.eligible++
			cs.totalEligible++
			consecutiveRestricted = 0
		} else {
			consecutiveRestricted++
		}

		// Record in shared catalog
		if s.sharedCatalog != nil {
			te := &domain.TenantEligibility{
				TenantID:  tenantID,
				ASIN:      p.ASIN,
				Eligible:  eligible,
				Reason:    reason,
				CheckedAt: time.Now(),
			}
			s.sharedCatalog.RecordEligibility(ctx, te)
		}

		// ── Per-category circuit breaker ──
		if consecutiveRestricted >= cbPerCategoryLimit {
			cs.fireBreaker(fmt.Sprintf("per_category_skip:%s_after_%d_restricted", cat.Name, consecutiveRestricted))
			break
		}
	}

	// ── Enrich eligible products with GetProductDetails (real price + seller count) ──
	var eligibleASINs []string
	for _, r := range results {
		if r.Eligible {
			eligibleASINs = append(eligibleASINs, r.ASIN)
		}
	}
	if len(eligibleASINs) > 0 {
		enriched, err := spapi.GetProductDetails(ctx, eligibleASINs, "US")
		cs.addAPICalls((len(eligibleASINs) + 19) / 20) // batch of 20
		if err != nil {
			slog.Warn("assessment: enrichment GetProductDetails failed", "category", cat.Name, "error", err)
		} else {
			enrichMap := make(map[string]port.ProductSearchResult, len(enriched))
			for _, e := range enriched {
				enrichMap[e.ASIN] = e
			}
			for i := range results {
				if e, ok := enrichMap[results[i].ASIN]; ok {
					if e.AmazonPrice > 0 {
						results[i].AmazonPrice = e.AmazonPrice
					}
					if e.SellerCount > 0 {
						results[i].SellerCount = e.SellerCount
					}
				}
			}
		}
	}

	slog.Info("assessment: category done",
		"category", cat.Name,
		"searched", stat.searched,
		"eligible", stat.eligible)

	return categorySearchResult{results: results, stat: stat}
}

// ---------------------------------------------------------------------------
// Phase 2 — Evaluate eligible products through funnel T1-T3
// ---------------------------------------------------------------------------

func (s *AssessmentService) phase2EvaluateProducts(
	ctx context.Context,
	tenantID domain.TenantID,
	eligible []domain.AssessmentSearchResult,
	spapi port.ProductSearcher,
) ([]FunnelSurvivor, FunnelStats, error) {
	// Convert to FunnelInput
	var inputs []FunnelInput
	for _, r := range eligible {
		inputs = append(inputs, FunnelInput{
			ASIN:           r.ASIN,
			Title:          r.Title,
			Brand:          r.Brand,
			Category:       r.Category,
			EstimatedPrice: r.AmazonPrice,
			WholesaleCost:  0, // estimate at 40% in funnel
			BSRRank:        r.BSRRank,
			SellerCount:    r.SellerCount,
			Source:         domain.ScanTypeAssessment,
		})
	}

	// Assessment thresholds
	thresholds := domain.PipelineThresholds{
		MinMarginPct:   15.0,
		MinSellerCount: 2,
	}

	if s.funnelSvc != nil {
		return s.funnelSvc.ProcessBatch(ctx, tenantID, inputs, thresholds)
	}

	// No funnel service — manual T1 filter only
	var survivors []FunnelSurvivor
	stats := FunnelStats{InputCount: len(inputs)}
	for _, inp := range inputs {
		if inp.EstimatedPrice < 10.0 || inp.EstimatedPrice > 200.0 {
			stats.T1MarginKilled++
			continue
		}
		wc := inp.EstimatedPrice * 0.4
		fbaCalc := domain.CalculateFBAFees(inp.EstimatedPrice, wc, 1.0, false)
		if fbaCalc.NetMarginPct < 15.0 {
			stats.T1MarginKilled++
			continue
		}
		if inp.SellerCount > 0 && inp.SellerCount < 2 {
			stats.T3EnrichKilled++
			continue
		}
		now := time.Now()
		survivors = append(survivors, FunnelSurvivor{
			DiscoveredProduct: domain.DiscoveredProduct{
				TenantID:           tenantID,
				ASIN:               inp.ASIN,
				Title:              inp.Title,
				Category:           inp.Category,
				EstimatedPrice:     inp.EstimatedPrice,
				BSRRank:            inp.BSRRank,
				SellerCount:        inp.SellerCount,
				EstimatedMarginPct: fbaCalc.NetMarginPct,
				Source:             domain.ScanTypeAssessment,
				FirstSeenAt:        now,
				LastSeenAt:         now,
			},
		})
	}
	stats.SurvivorCount = len(survivors)
	return survivors, stats, nil
}

// ---------------------------------------------------------------------------
// Phase 3 — Build outcome
// ---------------------------------------------------------------------------

func (s *AssessmentService) phase3BuildOutcome(
	tenantID domain.TenantID,
	allResults []domain.AssessmentSearchResult,
	eligible []domain.AssessmentSearchResult,
	survivors []FunnelSurvivor,
	funnelStats FunnelStats,
	catStats []categoryStat,
	cs *circuitState,
) *domain.AssessmentOutcome {
	outcome := &domain.AssessmentOutcome{
		TotalSearched:   len(allResults),
		TotalEligible:   len(eligible),
		TotalQualified:  len(survivors),
		APICallsUsed:    cs.apiCalls,
		DurationSeconds: time.Since(cs.startTime).Seconds(),
		CircuitBreakers: cs.firedBreakers,
	}

	if len(survivors) > 0 {
		outcome.HasOpportunities = true
		outcome.Opportunity = s.buildOpportunityResult(survivors, eligible, catStats)
	} else {
		outcome.HasOpportunities = false
		outcome.Ungating = s.buildUngatingResult(allResults, catStats)

		// Zero results safety — log it
		if len(eligible) == 0 {
			cs.fireBreaker("zero_results:0_eligible_after_full_scan")
			outcome.CircuitBreakers = cs.firedBreakers
			slog.Info("assessment: zero eligible products, building ungating roadmap", "tenant_id", tenantID)
		}
	}

	return outcome
}

func (s *AssessmentService) buildOpportunityResult(
	survivors []FunnelSurvivor,
	eligible []domain.AssessmentSearchResult,
	catStats []categoryStat,
) *domain.OpportunityResult {
	opp := &domain.OpportunityResult{}

	// Build category summaries
	catMap := make(map[string]*domain.CategorySummary)
	for _, cs := range catStats {
		if cs.eligible > 0 {
			catMap[cs.category] = &domain.CategorySummary{
				Category:      cs.category,
				BrowseNodeID:  cs.browseNodeID,
				EligibleCount: cs.eligible,
				OpenRate:      float64(cs.eligible) / float64(max(cs.searched, 1)) * 100,
			}
		}
	}

	// Count qualified per category and compute avg margin
	catMargins := make(map[string][]float64)
	for _, surv := range survivors {
		cat := surv.DiscoveredProduct.Category
		if cs, ok := catMap[cat]; ok {
			cs.QualifiedCount++
		}
		catMargins[cat] = append(catMargins[cat], surv.DiscoveredProduct.EstimatedMarginPct)
	}
	for cat, margins := range catMargins {
		if cs, ok := catMap[cat]; ok {
			sum := 0.0
			for _, m := range margins {
				sum += m
			}
			cs.AvgMarginPct = sum / float64(len(margins))
		}
	}

	for _, cs := range catMap {
		opp.EligibleCategories = append(opp.EligibleCategories, *cs)
	}
	sort.Slice(opp.EligibleCategories, func(i, j int) bool {
		return opp.EligibleCategories[i].QualifiedCount > opp.EligibleCategories[j].QualifiedCount
	})

	// Collect open brands
	brandSet := make(map[string]bool)
	for _, r := range eligible {
		if r.Brand != "" {
			brandSet[r.Brand] = true
		}
	}
	for b := range brandSet {
		opp.OpenBrands = append(opp.OpenBrands, b)
	}
	sort.Strings(opp.OpenBrands)

	// Build product recommendations, sorted by margin desc
	var recs []domain.ProductRecommendation
	for _, surv := range survivors {
		dp := surv.DiscoveredProduct
		recs = append(recs, domain.ProductRecommendation{
			ASIN:         dp.ASIN,
			Title:        dp.Title,
			Brand:        "",
			Category:     dp.Category,
			BuyBoxPrice:  dp.BuyBoxPrice,
			EstMarginPct: dp.EstimatedMarginPct,
			SellerCount:  dp.SellerCount,
			BSRRank:      dp.BSRRank,
		})
	}
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].EstMarginPct > recs[j].EstMarginPct
	})

	opp.QualifiedProducts = recs

	// Top 10 recommendations
	top := 10
	if len(recs) < top {
		top = len(recs)
	}
	opp.TopRecommendations = recs[:top]

	// Rough revenue estimate: qualified products * avg margin * avg price * 30 units/mo
	if len(survivors) > 0 {
		avgPrice := 0.0
		avgMargin := 0.0
		for _, surv := range survivors {
			p := surv.DiscoveredProduct.EstimatedPrice
			if surv.DiscoveredProduct.BuyBoxPrice > 0 {
				p = surv.DiscoveredProduct.BuyBoxPrice
			}
			avgPrice += p
			avgMargin += surv.DiscoveredProduct.EstimatedMarginPct
		}
		avgPrice /= float64(len(survivors))
		avgMargin /= float64(len(survivors))
		// Conservative: assume listing 5 products, 30 units/mo each
		listed := 5
		if len(survivors) < listed {
			listed = len(survivors)
		}
		opp.EstimatedMonthlyRev = float64(listed) * 30 * avgPrice * (avgMargin / 100)
	}

	return opp
}

func (s *AssessmentService) buildUngatingResult(
	allResults []domain.AssessmentSearchResult,
	catStats []categoryStat,
) *domain.UngatingResult {
	ur := &domain.UngatingResult{
		EstimatedTimeline: "30-60 days to first eligible products",
	}

	// Build restricted categories
	for _, cs := range catStats {
		openRate := 0.0
		if cs.searched > 0 {
			openRate = float64(cs.eligible) / float64(cs.searched) * 100
		}
		difficulty := "hard"
		if openRate > 50 {
			difficulty = "easy"
		} else if openRate > 20 {
			difficulty = "medium"
		}
		ur.RestrictedCategories = append(ur.RestrictedCategories, domain.RestrictedCategory{
			Category:   cs.category,
			OpenRate:   openRate,
			Difficulty: difficulty,
		})
	}

	// Build ungating roadmap — prioritize by known ease of ungating
	ungatingLadder := []struct {
		category   string
		action     string
		difficulty string
		estDays    int
		impact     string
	}{
		{"Grocery & Gourmet Food", "Apply for Grocery ungating via KeHE or UNFI invoice", "easy", 10, "Unlocks ~30-50 profitable replenishment ASINs"},
		{"Health & Household", "Apply with distributor invoice (overlaps with Grocery distributors)", "easy", 15, "Unlocks ~20-40 health/wellness ASINs with 20-30% margins"},
		{"Beauty & Personal Care", "Apply per-brand with authorized distributor invoices", "medium", 30, "Unlocks 30-40% margin beauty products (brand-gated is the real barrier)"},
		{"Toys & Games", "Apply with distributor invoice; easier before Q4", "medium", 20, "Seasonal Q4 opportunity with 25-35% margins"},
	}

	order := 1
	for _, step := range ungatingLadder {
		ur.RecommendedPath = append(ur.RecommendedPath, domain.UngatingStep{
			Order:      order,
			Category:   step.category,
			Action:     step.action,
			Difficulty: step.difficulty,
			EstDays:    step.estDays,
			Impact:     step.impact,
		})
		order++
	}

	return ur
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

func (s *AssessmentService) persistFingerprint(
	ctx context.Context,
	tenantID domain.TenantID,
	allResults []domain.AssessmentSearchResult,
	catStats []categoryStat,
) error {
	fingerprintID := s.idGen.New()

	// Build brand results for backward compat
	var brandResults []domain.BrandProbeResult
	for _, r := range allResults {
		estMargin := 0.0
		if r.AmazonPrice > 0 {
			estMargin = domain.EstimateMarginPct(r.AmazonPrice)
		}
		brandResults = append(brandResults, domain.BrandProbeResult{
			ASIN:         r.ASIN,
			Brand:        r.Brand,
			Category:     r.Category,
			Subcategory:  r.Subcategory,
			Tier:         "discovery",
			Eligible:     r.Eligible,
			Reason:       r.RestrictionReason,
			Title:        r.Title,
			Price:        r.AmazonPrice,
			EstMarginPct: estMargin,
			SellerCount:  r.SellerCount,
		})
	}

	// Build category eligibilities
	var categories []domain.CategoryEligibility
	for _, cs := range catStats {
		openRate := 0.0
		if cs.searched > 0 {
			openRate = float64(cs.eligible) / float64(cs.searched) * 100
		}
		categories = append(categories, domain.CategoryEligibility{
			Category:   cs.category,
			ProbeCount: cs.searched,
			OpenCount:  cs.eligible,
			GatedCount: cs.searched - cs.eligible,
			OpenRate:   openRate,
		})
	}

	totalEligible := 0
	totalRestricted := 0
	for _, r := range allResults {
		if r.Eligible {
			totalEligible++
		} else {
			totalRestricted++
		}
	}

	overallOpenRate := 0.0
	if len(allResults) > 0 {
		overallOpenRate = float64(totalEligible) / float64(len(allResults)) * 100
	}

	confidence := float64(len(allResults)) / 400.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	fp := &domain.EligibilityFingerprint{
		ID:              fingerprintID,
		TenantID:        tenantID,
		Categories:      categories,
		BrandResults:    brandResults,
		TotalProbes:     len(allResults),
		TotalEligible:   totalEligible,
		TotalRestricted: totalRestricted,
		OverallOpenRate: overallOpenRate,
		Confidence:      confidence,
		AssessedAt:      time.Now(),
	}

	if err := s.fingerprints.Create(ctx, fp); err != nil {
		return err
	}
	if err := s.fingerprints.SaveProbeResults(ctx, fingerprintID, tenantID, brandResults); err != nil {
		slog.Warn("assessment: failed to save probe results", "error", err)
	}
	if err := s.fingerprints.SaveCategoryEligibilities(ctx, fingerprintID, tenantID, categories); err != nil {
		slog.Warn("assessment: failed to save category eligibilities", "error", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func filterEligible(results []domain.AssessmentSearchResult) []domain.AssessmentSearchResult {
	var eligible []domain.AssessmentSearchResult
	for _, r := range results {
		if r.Eligible {
			eligible = append(eligible, r)
		}
	}
	return eligible
}

// CompleteAssessment marks the assessment as complete and updates the profile.
func (s *AssessmentService) CompleteAssessment(ctx context.Context, tenantID domain.TenantID) error {
	profile, err := s.profiles.Get(ctx, tenantID)
	if err != nil {
		return err
	}
	now := time.Now()
	profile.AssessmentStatus = domain.AssessmentStatusCompleted
	profile.AssessedAt = &now
	profile.UpdatedAt = now
	return s.profiles.Update(ctx, profile)
}

// FailAssessment marks the assessment as failed.
func (s *AssessmentService) FailAssessment(ctx context.Context, tenantID domain.TenantID) error {
	profile, err := s.profiles.Get(ctx, tenantID)
	if err != nil {
		return err
	}
	profile.AssessmentStatus = domain.AssessmentStatusFailed
	profile.UpdatedAt = time.Now()
	return s.profiles.Update(ctx, profile)
}

// GetProfile returns the seller profile for a tenant.
func (s *AssessmentService) GetProfile(ctx context.Context, tenantID domain.TenantID) (*domain.SellerProfile, error) {
	return s.profiles.Get(ctx, tenantID)
}

// GetFingerprint returns the eligibility fingerprint for a tenant.
func (s *AssessmentService) GetFingerprint(ctx context.Context, tenantID domain.TenantID) (*domain.EligibilityFingerprint, error) {
	return s.fingerprints.Get(ctx, tenantID)
}

// ResetAssessment deletes the seller profile and fingerprint so the assessment can be re-run.
func (s *AssessmentService) ResetAssessment(ctx context.Context, tenantID domain.TenantID) error {
	slog.Info("assessment: resetting", "tenant_id", tenantID)
	if err := s.fingerprints.Delete(ctx, tenantID); err != nil {
		slog.Warn("assessment: failed to delete fingerprint", "tenant_id", tenantID, "error", err)
	}
	return s.profiles.Delete(ctx, tenantID)
}

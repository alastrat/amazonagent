package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ---------------------------------------------------------------------------
// In-memory mock: SuggestionRepo
// ---------------------------------------------------------------------------

type mockSuggestionRepo struct {
	mu          sync.Mutex
	suggestions []domain.DiscoverySuggestion

	// error injection
	createErr     error
	createBatchErr error
	getByIDErr    error
	listPendingErr error
	listAllErr    error
	acceptErr     error
	dismissErr    error
	countTodayErr error

	// overrides
	countTodayOverride *int
}

func newMockSuggestionRepo() *mockSuggestionRepo {
	return &mockSuggestionRepo{}
}

func (r *mockSuggestionRepo) Create(_ context.Context, s *domain.DiscoverySuggestion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	r.suggestions = append(r.suggestions, *s)
	return nil
}

func (r *mockSuggestionRepo) CreateBatch(_ context.Context, suggestions []domain.DiscoverySuggestion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createBatchErr != nil {
		return r.createBatchErr
	}
	r.suggestions = append(r.suggestions, suggestions...)
	return nil
}

func (r *mockSuggestionRepo) GetByID(_ context.Context, tenantID domain.TenantID, id domain.SuggestionID) (*domain.DiscoverySuggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	for _, s := range r.suggestions {
		if s.ID == id && s.TenantID == tenantID {
			cp := s
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("suggestion %s not found", id)
}

func (r *mockSuggestionRepo) ListPending(_ context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listPendingErr != nil {
		return nil, r.listPendingErr
	}
	var result []domain.DiscoverySuggestion
	for _, s := range r.suggestions {
		if s.TenantID == tenantID && s.Status == domain.SuggestionStatusPending {
			result = append(result, s)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *mockSuggestionRepo) ListAll(_ context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listAllErr != nil {
		return nil, r.listAllErr
	}
	var result []domain.DiscoverySuggestion
	for _, s := range r.suggestions {
		if s.TenantID == tenantID {
			result = append(result, s)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *mockSuggestionRepo) Accept(_ context.Context, _ domain.TenantID, id domain.SuggestionID, dealID domain.DealID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.acceptErr != nil {
		return r.acceptErr
	}
	for i := range r.suggestions {
		if r.suggestions[i].ID == id {
			r.suggestions[i].Status = domain.SuggestionStatusAccepted
			r.suggestions[i].DealID = &dealID
			now := time.Now()
			r.suggestions[i].ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("suggestion %s not found", id)
}

func (r *mockSuggestionRepo) Dismiss(_ context.Context, _ domain.TenantID, id domain.SuggestionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dismissErr != nil {
		return r.dismissErr
	}
	for i := range r.suggestions {
		if r.suggestions[i].ID == id {
			r.suggestions[i].Status = domain.SuggestionStatusDismissed
			now := time.Now()
			r.suggestions[i].ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("suggestion %s not found", id)
}

func (r *mockSuggestionRepo) CountToday(_ context.Context, tenantID domain.TenantID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.countTodayErr != nil {
		return 0, r.countTodayErr
	}
	if r.countTodayOverride != nil {
		return *r.countTodayOverride, nil
	}
	count := 0
	today := time.Now().Truncate(24 * time.Hour)
	for _, s := range r.suggestions {
		if s.TenantID == tenantID && !s.CreatedAt.Before(today) {
			count++
		}
	}
	return count, nil
}

// helper to get a suggestion by ID without locking (use in tests only)
func (r *mockSuggestionRepo) get(id domain.SuggestionID) *domain.DiscoverySuggestion {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.suggestions {
		if s.ID == id {
			cp := s
			return &cp
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mock: ProductSearcher
// ---------------------------------------------------------------------------

type discoveryProductSearcher struct {
	// SearchProducts returns these results keyed by the first keyword
	searchResults map[string][]port.ProductSearchResult
	searchErr     error

	// GetProductDetails returns these results
	detailResults []port.ProductSearchResult
	detailErr     error
}

func newDiscoveryProductSearcher() *discoveryProductSearcher {
	return &discoveryProductSearcher{
		searchResults: make(map[string][]port.ProductSearchResult),
	}
}

func (m *discoveryProductSearcher) SearchProducts(_ context.Context, keywords []string, _ string) ([]port.ProductSearchResult, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	if len(keywords) == 0 {
		return nil, nil
	}
	return m.searchResults[keywords[0]], nil
}

func (m *discoveryProductSearcher) SearchByBrowseNode(_ context.Context, _ string, _ string, _ string) ([]port.ProductSearchResult, string, error) {
	return nil, "", nil
}

func (m *discoveryProductSearcher) GetProductDetails(_ context.Context, asins []string, _ string) ([]port.ProductSearchResult, error) {
	if m.detailErr != nil {
		return nil, m.detailErr
	}
	if m.detailResults != nil {
		return m.detailResults, nil
	}
	// Return enriched data for requested ASINs (pass-through with buy box prices)
	var results []port.ProductSearchResult
	for _, asin := range asins {
		results = append(results, port.ProductSearchResult{
			ASIN:        asin,
			AmazonPrice: 50.0,
			SellerCount: 5,
			BSRRank:     1000,
		})
	}
	return results, nil
}

func (m *discoveryProductSearcher) EstimateFees(_ context.Context, _ string, _ float64, _ string) (*port.ProductFeeEstimate, error) {
	return nil, nil
}

func (m *discoveryProductSearcher) CheckListingEligibility(_ context.Context, _ []string, _ string) ([]port.ListingRestriction, error) {
	return nil, nil
}

func (m *discoveryProductSearcher) LookupByIdentifier(_ context.Context, _ []string, _ string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Sequential ID generator for tests
// ---------------------------------------------------------------------------

type discoveryIDGen struct {
	mu      sync.Mutex
	counter int
}

func (g *discoveryIDGen) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("disc-id-%d", g.counter)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

const discoveryTenantID = domain.TenantID("tenant-disc-001")

func newDiscoveryTestService() (*DiscoveryQueueService, *strategyVersionRepo, *mockSuggestionRepo, *discoveryProductSearcher, *discoveryIDGen) {
	stratRepo := newStrategyVersionRepo()
	idGen := &discoveryIDGen{}
	strategySvc := NewStrategyService(stratRepo, idGen)

	suggRepo := newMockSuggestionRepo()
	spapi := newDiscoveryProductSearcher()

	// FunnelService with nil dependencies — passes all products through T0-T3 with no filtering
	funnel := NewFunnelService(nil, nil, nil, spapi)

	svc := NewDiscoveryQueueService(strategySvc, funnel, suggRepo, spapi, idGen)
	return svc, stratRepo, suggRepo, spapi, idGen
}

// seedActiveStrategy inserts an active strategy with the given search params.
func seedActiveStrategy(repo *strategyVersionRepo, tenantID domain.TenantID, params domain.StrategySearchParams) *domain.StrategyVersion {
	now := time.Now()
	sv := &domain.StrategyVersion{
		ID:            "strat-active-1",
		TenantID:      tenantID,
		VersionNumber: 1,
		SearchParams:  params,
		Status:        domain.StrategyStatusActive,
		ChangeReason:  "test seed",
		CreatedBy:     domain.StrategyCreatedBySystem,
		CreatedAt:     now,
		ActivatedAt:   &now,
	}
	repo.mu.Lock()
	repo.versions = append(repo.versions, sv)
	repo.mu.Unlock()
	return sv
}

// ---------------------------------------------------------------------------
// Tests: RunDailyDiscovery
// ---------------------------------------------------------------------------

func TestRunDailyDiscovery_LoadsActiveStrategyAndCreatesSuggestions(t *testing.T) {
	svc, stratRepo, suggRepo, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0, // disable margin filter for this test
		MinSellerCount:     0,
		EligibleCategories: []string{"Electronics"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	spapi.searchResults["Electronics"] = []port.ProductSearchResult{
		{ASIN: "B001", Title: "Widget A", Brand: "BrandX", Category: "Electronics", AmazonPrice: 50.0, SellerCount: 5, BSRRank: 100},
		{ASIN: "B002", Title: "Widget B", Brand: "BrandY", Category: "Electronics", AmazonPrice: 60.0, SellerCount: 3, BSRRank: 200},
	}

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) == 0 {
		t.Fatal("expected suggestions to be created, got 0")
	}

	// Verify suggestions were persisted
	stored, err := suggRepo.ListAll(ctx, discoveryTenantID, 100)
	if err != nil {
		t.Fatalf("ListAll error: %v", err)
	}
	if len(stored) != len(suggestions) {
		t.Errorf("expected %d stored suggestions, got %d", len(suggestions), len(stored))
	}

	// Verify all suggestions are pending
	for _, s := range suggestions {
		if s.Status != domain.SuggestionStatusPending {
			t.Errorf("expected status pending, got %s for suggestion %s", s.Status, s.ID)
		}
	}
}

func TestRunDailyDiscovery_RespectsDailyCapOf20(t *testing.T) {
	svc, stratRepo, suggRepo, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0,
		MinSellerCount:     0,
		EligibleCategories: []string{"Electronics"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	// Produce 30 products — more than the daily cap
	var products []port.ProductSearchResult
	for i := 0; i < 30; i++ {
		products = append(products, port.ProductSearchResult{
			ASIN:        fmt.Sprintf("B%03d", i),
			Title:       fmt.Sprintf("Product %d", i),
			Brand:       "TestBrand",
			Category:    "Electronics",
			AmazonPrice: 50.0,
			SellerCount: 5,
			BSRRank:     100 + i,
		})
	}
	spapi.searchResults["Electronics"] = products

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) > MaxDailySuggestions {
		t.Errorf("expected at most %d suggestions, got %d", MaxDailySuggestions, len(suggestions))
	}

	// Verify exactly MaxDailySuggestions were persisted
	stored, _ := suggRepo.ListAll(ctx, discoveryTenantID, 100)
	if len(stored) > MaxDailySuggestions {
		t.Errorf("expected at most %d stored suggestions, got %d", MaxDailySuggestions, len(stored))
	}
}

func TestRunDailyDiscovery_DailyCapAlreadyReached(t *testing.T) {
	svc, stratRepo, suggRepo, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0,
		MinSellerCount:     0,
		EligibleCategories: []string{"Electronics"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	spapi.searchResults["Electronics"] = []port.ProductSearchResult{
		{ASIN: "B001", Title: "Widget", AmazonPrice: 50.0, SellerCount: 5},
	}

	// Pre-set count to MaxDailySuggestions
	cap := MaxDailySuggestions
	suggRepo.countTodayOverride = &cap

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suggestions != nil {
		t.Errorf("expected nil suggestions when daily cap is reached, got %d", len(suggestions))
	}
}

func TestRunDailyDiscovery_PartialCapRemaining(t *testing.T) {
	svc, stratRepo, suggRepo, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0,
		MinSellerCount:     0,
		EligibleCategories: []string{"Electronics"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	// Produce 10 products
	var products []port.ProductSearchResult
	for i := 0; i < 10; i++ {
		products = append(products, port.ProductSearchResult{
			ASIN:        fmt.Sprintf("B%03d", i),
			Title:       fmt.Sprintf("Product %d", i),
			Brand:       "TestBrand",
			Category:    "Electronics",
			AmazonPrice: 50.0,
			SellerCount: 5,
			BSRRank:     100 + i,
		})
	}
	spapi.searchResults["Electronics"] = products

	// Already used 15 of 20
	used := 15
	suggRepo.countTodayOverride = &used

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	remaining := MaxDailySuggestions - used // 5
	if len(suggestions) > remaining {
		t.Errorf("expected at most %d suggestions (remaining cap), got %d", remaining, len(suggestions))
	}
}

func TestRunDailyDiscovery_NoActiveStrategyReturnsError(t *testing.T) {
	svc, _, _, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	// No strategy seeded — GetActive will fail
	_, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err == nil {
		t.Fatal("expected error when no active strategy, got nil")
	}
}

func TestRunDailyDiscovery_NilSPAPIReturnsEmpty(t *testing.T) {
	stratRepo := newStrategyVersionRepo()
	idGen := &discoveryIDGen{}
	strategySvc := NewStrategyService(stratRepo, idGen)
	suggRepo := newMockSuggestionRepo()
	funnel := NewFunnelService(nil, nil, nil, nil) // nil SP-API in funnel too

	// nil SP-API in DiscoveryQueueService
	svc := NewDiscoveryQueueService(strategySvc, funnel, suggRepo, nil, idGen)

	params := domain.StrategySearchParams{
		EligibleCategories: []string{"Electronics"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	ctx := context.Background()
	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suggestions != nil {
		t.Errorf("expected nil suggestions when no SP-API, got %d", len(suggestions))
	}
}

func TestRunDailyDiscovery_SuggestionsHaveCorrectStrategyVersionIDAndReason(t *testing.T) {
	svc, stratRepo, _, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0,
		MinSellerCount:     0,
		EligibleCategories: []string{"Toys"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	strategy := seedActiveStrategy(stratRepo, discoveryTenantID, params)

	spapi.searchResults["Toys"] = []port.ProductSearchResult{
		{ASIN: "B100", Title: "Action Figure", Brand: "ToyBrand", Category: "Toys", AmazonPrice: 25.0, SellerCount: 4, BSRRank: 500},
	}

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion")
	}

	for _, s := range suggestions {
		if s.StrategyVersionID != strategy.ID {
			t.Errorf("expected strategy_version_id %s, got %s", strategy.ID, s.StrategyVersionID)
		}
		if s.Reason == "" {
			t.Error("expected non-empty reason")
		}
		// Reason should mention the strategy version number
		expected := fmt.Sprintf("v%d", strategy.VersionNumber)
		if !containsSubstring(s.Reason, expected) {
			t.Errorf("expected reason to contain %q, got %q", expected, s.Reason)
		}
	}
}

func TestRunDailyDiscovery_UsesStrategyMinMarginAndMinSellerCount(t *testing.T) {
	svc, stratRepo, _, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	// Set high thresholds that should filter products
	params := domain.StrategySearchParams{
		MinMarginPct:       25.0, // high margin requirement
		MinSellerCount:     3,    // need at least 3 sellers
		EligibleCategories: []string{"Kitchen"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	spapi.searchResults["Kitchen"] = []port.ProductSearchResult{
		// This product should survive: decent price and enough sellers
		{ASIN: "B200", Title: "Good Kitchen Item", Brand: "KitchenBrand", Category: "Kitchen", AmazonPrice: 50.0, SellerCount: 5, BSRRank: 100},
		// This product has only 1 seller — below MinSellerCount
		{ASIN: "B201", Title: "Solo Seller Item", Brand: "SoloBrand", Category: "Kitchen", AmazonPrice: 50.0, SellerCount: 1, BSRRank: 200},
	}

	// Provide enrichment data with matching seller counts
	spapi.detailResults = []port.ProductSearchResult{
		{ASIN: "B200", AmazonPrice: 50.0, SellerCount: 5, BSRRank: 100},
		{ASIN: "B201", AmazonPrice: 50.0, SellerCount: 1, BSRRank: 200},
	}

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// B201 should have been filtered by MinSellerCount in the funnel's T3
	for _, s := range suggestions {
		if s.ASIN == "B201" {
			t.Error("expected B201 to be filtered out by MinSellerCount, but it survived")
		}
	}
}

func TestRunDailyDiscovery_MultipleCategories(t *testing.T) {
	svc, stratRepo, _, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0,
		MinSellerCount:     0,
		EligibleCategories: []string{"Electronics", "Toys"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	spapi.searchResults["Electronics"] = []port.ProductSearchResult{
		{ASIN: "E001", Title: "Gadget", Category: "Electronics", AmazonPrice: 40.0, SellerCount: 3},
	}
	spapi.searchResults["Toys"] = []port.ProductSearchResult{
		{ASIN: "T001", Title: "Toy", Category: "Toys", AmazonPrice: 30.0, SellerCount: 4},
	}

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) < 2 {
		t.Errorf("expected at least 2 suggestions from 2 categories, got %d", len(suggestions))
	}

	asinSet := make(map[string]bool)
	for _, s := range suggestions {
		asinSet[s.ASIN] = true
	}
	if !asinSet["E001"] {
		t.Error("expected suggestion for E001 from Electronics category")
	}
	if !asinSet["T001"] {
		t.Error("expected suggestion for T001 from Toys category")
	}
}

func TestRunDailyDiscovery_SearchErrorContinuesToNextCategory(t *testing.T) {
	svc, stratRepo, _, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0,
		MinSellerCount:     0,
		EligibleCategories: []string{"FailCategory", "GoodCategory"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	// FailCategory has no results (search returns nil for unknown keys)
	spapi.searchResults["GoodCategory"] = []port.ProductSearchResult{
		{ASIN: "G001", Title: "Good Product", Category: "GoodCategory", AmazonPrice: 50.0, SellerCount: 3},
	}

	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("expected at least 1 suggestion from GoodCategory after FailCategory returned nothing")
	}
}

func TestRunDailyDiscovery_NoProductsFoundReturnsNil(t *testing.T) {
	svc, stratRepo, _, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		EligibleCategories: []string{"EmptyCategory"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	// No products configured for EmptyCategory
	suggestions, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suggestions != nil {
		t.Errorf("expected nil suggestions for empty search results, got %d", len(suggestions))
	}
}

func TestRunDailyDiscovery_CreateBatchErrorReturnsError(t *testing.T) {
	svc, stratRepo, suggRepo, spapi, _ := newDiscoveryTestService()
	ctx := context.Background()

	params := domain.StrategySearchParams{
		MinMarginPct:       0,
		MinSellerCount:     0,
		EligibleCategories: []string{"Electronics"},
		ScoringWeights:     domain.DefaultScoringWeights(),
	}
	seedActiveStrategy(stratRepo, discoveryTenantID, params)

	spapi.searchResults["Electronics"] = []port.ProductSearchResult{
		{ASIN: "B001", Title: "Widget", AmazonPrice: 50.0, SellerCount: 5},
	}

	suggRepo.createBatchErr = fmt.Errorf("database write failed")

	_, err := svc.RunDailyDiscovery(ctx, discoveryTenantID)
	if err == nil {
		t.Fatal("expected error when CreateBatch fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: AcceptSuggestion
// ---------------------------------------------------------------------------

func TestAcceptSuggestion_MarksAsAcceptedWithDealID(t *testing.T) {
	svc, _, suggRepo, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	// Seed a pending suggestion
	suggestion := domain.DiscoverySuggestion{
		ID:       "sugg-1",
		TenantID: discoveryTenantID,
		ASIN:     "B001",
		Title:    "Widget",
		Status:   domain.SuggestionStatusPending,
	}
	suggRepo.suggestions = append(suggRepo.suggestions, suggestion)

	dealID := domain.DealID("deal-001")
	err := svc.AcceptSuggestion(ctx, discoveryTenantID, "sugg-1", dealID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := suggRepo.get("sugg-1")
	if updated == nil {
		t.Fatal("suggestion not found after accept")
	}
	if updated.Status != domain.SuggestionStatusAccepted {
		t.Errorf("expected status accepted, got %s", updated.Status)
	}
	if updated.DealID == nil || *updated.DealID != dealID {
		t.Errorf("expected deal_id %s, got %v", dealID, updated.DealID)
	}
	if updated.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
}

func TestAcceptSuggestion_NotFoundReturnsError(t *testing.T) {
	svc, _, _, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	err := svc.AcceptSuggestion(ctx, discoveryTenantID, "nonexistent", "deal-001")
	if err == nil {
		t.Fatal("expected error for nonexistent suggestion")
	}
}

// ---------------------------------------------------------------------------
// Tests: DismissSuggestion
// ---------------------------------------------------------------------------

func TestDismissSuggestion_MarksAsDismissed(t *testing.T) {
	svc, _, suggRepo, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	suggestion := domain.DiscoverySuggestion{
		ID:       "sugg-2",
		TenantID: discoveryTenantID,
		ASIN:     "B002",
		Title:    "Gadget",
		Status:   domain.SuggestionStatusPending,
	}
	suggRepo.suggestions = append(suggRepo.suggestions, suggestion)

	err := svc.DismissSuggestion(ctx, discoveryTenantID, "sugg-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := suggRepo.get("sugg-2")
	if updated == nil {
		t.Fatal("suggestion not found after dismiss")
	}
	if updated.Status != domain.SuggestionStatusDismissed {
		t.Errorf("expected status dismissed, got %s", updated.Status)
	}
	if updated.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
	// Dismiss does NOT set DealID
	if updated.DealID != nil {
		t.Errorf("expected nil deal_id after dismiss, got %v", updated.DealID)
	}
}

func TestDismissSuggestion_NotFoundReturnsError(t *testing.T) {
	svc, _, _, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	err := svc.DismissSuggestion(ctx, discoveryTenantID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent suggestion")
	}
}

// ---------------------------------------------------------------------------
// Tests: ListPending
// ---------------------------------------------------------------------------

func TestListPending_ReturnsOnlyPendingSuggestions(t *testing.T) {
	svc, _, suggRepo, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	now := time.Now()
	suggRepo.suggestions = []domain.DiscoverySuggestion{
		{ID: "s1", TenantID: discoveryTenantID, Status: domain.SuggestionStatusPending, CreatedAt: now},
		{ID: "s2", TenantID: discoveryTenantID, Status: domain.SuggestionStatusAccepted, CreatedAt: now},
		{ID: "s3", TenantID: discoveryTenantID, Status: domain.SuggestionStatusPending, CreatedAt: now},
		{ID: "s4", TenantID: discoveryTenantID, Status: domain.SuggestionStatusDismissed, CreatedAt: now},
		{ID: "s5", TenantID: domain.TenantID("other-tenant"), Status: domain.SuggestionStatusPending, CreatedAt: now},
	}

	pending, err := svc.ListPending(ctx, discoveryTenantID, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("expected 2 pending suggestions, got %d", len(pending))
	}

	for _, s := range pending {
		if s.Status != domain.SuggestionStatusPending {
			t.Errorf("expected status pending, got %s", s.Status)
		}
		if s.TenantID != discoveryTenantID {
			t.Errorf("expected tenant %s, got %s", discoveryTenantID, s.TenantID)
		}
	}
}

func TestListPending_RespectsLimit(t *testing.T) {
	svc, _, suggRepo, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 10; i++ {
		suggRepo.suggestions = append(suggRepo.suggestions, domain.DiscoverySuggestion{
			ID:        domain.SuggestionID(fmt.Sprintf("s%d", i)),
			TenantID:  discoveryTenantID,
			Status:    domain.SuggestionStatusPending,
			CreatedAt: now,
		})
	}

	pending, err := svc.ListPending(ctx, discoveryTenantID, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("expected 3 suggestions with limit=3, got %d", len(pending))
	}
}

// ---------------------------------------------------------------------------
// Tests: ListAll
// ---------------------------------------------------------------------------

func TestListAll_ReturnsAllSuggestionsForTenant(t *testing.T) {
	svc, _, suggRepo, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	now := time.Now()
	suggRepo.suggestions = []domain.DiscoverySuggestion{
		{ID: "s1", TenantID: discoveryTenantID, Status: domain.SuggestionStatusPending, CreatedAt: now},
		{ID: "s2", TenantID: discoveryTenantID, Status: domain.SuggestionStatusAccepted, CreatedAt: now},
		{ID: "s3", TenantID: discoveryTenantID, Status: domain.SuggestionStatusDismissed, CreatedAt: now},
		{ID: "s4", TenantID: domain.TenantID("other-tenant"), Status: domain.SuggestionStatusPending, CreatedAt: now},
	}

	all, err := svc.ListAll(ctx, discoveryTenantID, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("expected 3 suggestions for tenant, got %d", len(all))
	}

	for _, s := range all {
		if s.TenantID != discoveryTenantID {
			t.Errorf("expected tenant %s, got %s", discoveryTenantID, s.TenantID)
		}
	}
}

func TestListAll_RespectsLimit(t *testing.T) {
	svc, _, suggRepo, _, _ := newDiscoveryTestService()
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 10; i++ {
		suggRepo.suggestions = append(suggRepo.suggestions, domain.DiscoverySuggestion{
			ID:        domain.SuggestionID(fmt.Sprintf("s%d", i)),
			TenantID:  discoveryTenantID,
			Status:    domain.SuggestionStatusPending,
			CreatedAt: now,
		})
	}

	all, err := svc.ListAll(ctx, discoveryTenantID, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 suggestions with limit=5, got %d", len(all))
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

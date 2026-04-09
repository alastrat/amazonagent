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
// In-memory mocks (specific to shared catalog; credit mocks reused from credit_service_test.go)
// ---------------------------------------------------------------------------

// mockSharedCatalogRepo is a thread-safe in-memory SharedCatalogRepo.
type mockSharedCatalogRepo struct {
	mu       sync.Mutex
	products map[string]*domain.SharedProduct
	batches  [][]domain.SharedProduct // tracks UpsertProductBatch calls
	enriched map[string]int           // asin -> enrichment increment count
}

func newMockSharedCatalogRepo() *mockSharedCatalogRepo {
	return &mockSharedCatalogRepo{
		products: make(map[string]*domain.SharedProduct),
		enriched: make(map[string]int),
	}
}

func (m *mockSharedCatalogRepo) UpsertProduct(_ context.Context, p *domain.SharedProduct) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *p
	m.products[p.ASIN] = &cp
	return nil
}

func (m *mockSharedCatalogRepo) UpsertProductBatch(_ context.Context, products []domain.SharedProduct) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batches = append(m.batches, products)
	for i := range products {
		cp := products[i]
		m.products[cp.ASIN] = &cp
	}
	return nil
}

func (m *mockSharedCatalogRepo) GetByASIN(_ context.Context, asin string) (*domain.SharedProduct, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.products[asin]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *p
	return &cp, nil
}

func (m *mockSharedCatalogRepo) GetByASINs(_ context.Context, asins []string) ([]domain.SharedProduct, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []domain.SharedProduct
	for _, asin := range asins {
		if p, ok := m.products[asin]; ok {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *mockSharedCatalogRepo) GetStale(_ context.Context, _ time.Time, _ int) ([]domain.SharedProduct, error) {
	return nil, nil
}

func (m *mockSharedCatalogRepo) SearchByCategory(_ context.Context, _ string, _ int) ([]domain.SharedProduct, error) {
	return nil, nil
}

func (m *mockSharedCatalogRepo) IncrementEnrichment(_ context.Context, asin string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enriched[asin]++
	return nil
}

// mockBrandCatalogRepo is a thread-safe in-memory BrandCatalogRepo.
type mockBrandCatalogRepo struct {
	mu             sync.Mutex
	brands         map[string]*domain.SharedBrand
	productCounts  map[string]int
	gatingUpdates  map[string]string // normalizedName -> gating
}

func newMockBrandCatalogRepo() *mockBrandCatalogRepo {
	return &mockBrandCatalogRepo{
		brands:        make(map[string]*domain.SharedBrand),
		productCounts: make(map[string]int),
		gatingUpdates: make(map[string]string),
	}
}

func (m *mockBrandCatalogRepo) Upsert(_ context.Context, b *domain.SharedBrand) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *b
	m.brands[b.NormalizedName] = &cp
	return nil
}

func (m *mockBrandCatalogRepo) GetByName(_ context.Context, name string) (*domain.SharedBrand, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.brands[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return b, nil
}

func (m *mockBrandCatalogRepo) ListByCategory(_ context.Context, _ string) ([]domain.SharedBrand, error) {
	return nil, nil
}

func (m *mockBrandCatalogRepo) UpdateGating(_ context.Context, normalizedName string, gating string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gatingUpdates[normalizedName] = gating
	return nil
}

func (m *mockBrandCatalogRepo) IncrementProductCount(_ context.Context, normalizedName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.productCounts[normalizedName]++
	return nil
}

// mockTenantEligibilityRepo is a thread-safe in-memory TenantEligibilityRepo.
type mockTenantEligibilityRepo struct {
	mu   sync.Mutex
	data map[string]*domain.TenantEligibility // key: tenantID + ":" + asin
}

func newMockTenantEligibilityRepo() *mockTenantEligibilityRepo {
	return &mockTenantEligibilityRepo{
		data: make(map[string]*domain.TenantEligibility),
	}
}

func eligKey(tenantID domain.TenantID, asin string) string {
	return string(tenantID) + ":" + asin
}

func (m *mockTenantEligibilityRepo) Set(_ context.Context, e *domain.TenantEligibility) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *e
	m.data[eligKey(e.TenantID, e.ASIN)] = &cp
	return nil
}

func (m *mockTenantEligibilityRepo) SetBatch(_ context.Context, eligibilities []domain.TenantEligibility) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range eligibilities {
		cp := eligibilities[i]
		m.data[eligKey(cp.TenantID, cp.ASIN)] = &cp
	}
	return nil
}

func (m *mockTenantEligibilityRepo) Get(_ context.Context, tenantID domain.TenantID, asin string) (*domain.TenantEligibility, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[eligKey(tenantID, asin)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *e
	return &cp, nil
}

func (m *mockTenantEligibilityRepo) GetByASINs(_ context.Context, tenantID domain.TenantID, asins []string) ([]domain.TenantEligibility, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []domain.TenantEligibility
	for _, asin := range asins {
		if e, ok := m.data[eligKey(tenantID, asin)]; ok {
			result = append(result, *e)
		}
	}
	return result, nil
}

func (m *mockTenantEligibilityRepo) ListEligible(_ context.Context, _ domain.TenantID, _ string, _ int) ([]domain.TenantEligibility, error) {
	return nil, nil
}

// mockTenantMarginRepo is a minimal TenantMarginRepo.
type mockTenantMarginRepo struct{}

func (m *mockTenantMarginRepo) Set(_ context.Context, _ *domain.TenantMargin) error { return nil }
func (m *mockTenantMarginRepo) GetByASIN(_ context.Context, _ domain.TenantID, _ string) (*domain.TenantMargin, error) {
	return nil, fmt.Errorf("not found")
}

// mockProductSearcher controls SP-API responses for tests.
type mockProductSearcher struct {
	detailsResult     []port.ProductSearchResult
	detailsErr        error
	eligibilityResult []port.ListingRestriction
	eligibilityErr    error
	detailsCalled     int
	eligibilityCalled int
}

func (m *mockProductSearcher) SearchProducts(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

func (m *mockProductSearcher) SearchByBrowseNode(_ context.Context, _ string, _ string, _ string) ([]port.ProductSearchResult, string, error) {
	return nil, "", nil
}

func (m *mockProductSearcher) GetProductDetails(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	m.detailsCalled++
	return m.detailsResult, m.detailsErr
}

func (m *mockProductSearcher) EstimateFees(_ context.Context, _ string, _ float64, _ string) (*port.ProductFeeEstimate, error) {
	return nil, nil
}

func (m *mockProductSearcher) CheckListingEligibility(_ context.Context, _ []string, _ string) ([]port.ListingRestriction, error) {
	m.eligibilityCalled++
	return m.eligibilityResult, m.eligibilityErr
}

func (m *mockProductSearcher) LookupByIdentifier(_ context.Context, _ []string, _ string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

// failingSharedCatalogRepo wraps mockSharedCatalogRepo but fails on UpsertProductBatch.
type failingSharedCatalogRepo struct {
	*mockSharedCatalogRepo
}

func (f *failingSharedCatalogRepo) UpsertProductBatch(_ context.Context, _ []domain.SharedProduct) error {
	return fmt.Errorf("batch upsert failed")
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

const scTestTenantID domain.TenantID = "tenant-sc-test-1"

// buildSCCreditService creates a CreditService backed by in-memory repos with the given account.
// Reuses creditAcctRepo, creditTxnRepo, and creditIDGen from credit_service_test.go.
func buildSCCreditService(account *domain.CreditAccount) *CreditService {
	accounts := newCreditAcctRepo()
	if account != nil {
		accounts.seed(account)
	}
	return NewCreditService(accounts, newCreditTxnRepo(), &creditIDGen{})
}

// buildSCService creates a SharedCatalogService with all mocks wired.
// Pass nil for spapi to test the nil-SP-API path (uses explicit nil interface).
func buildSCService(
	catalog *mockSharedCatalogRepo,
	brands *mockBrandCatalogRepo,
	eligibility *mockTenantEligibilityRepo,
	spapi *mockProductSearcher,
	credits *CreditService,
) *SharedCatalogService {
	if catalog == nil {
		catalog = newMockSharedCatalogRepo()
	}
	if brands == nil {
		brands = newMockBrandCatalogRepo()
	}
	if eligibility == nil {
		eligibility = newMockTenantEligibilityRepo()
	}
	// A typed nil (*mockProductSearcher)(nil) is NOT a nil interface.
	// We must explicitly pass a nil port.ProductSearcher when spapi is nil.
	var searcher port.ProductSearcher
	if spapi != nil {
		searcher = spapi
	}
	return NewSharedCatalogService(catalog, brands, eligibility, &mockTenantMarginRepo{}, searcher, credits)
}

// freshProduct returns a SharedProduct enriched recently.
func freshProduct(asin string) *domain.SharedProduct {
	now := time.Now()
	return &domain.SharedProduct{
		ASIN:           asin,
		Title:          "Fresh Product " + asin,
		Brand:          "TestBrand",
		Category:       "Electronics",
		BuyBoxPrice:    29.99,
		BSRRank:        1000,
		SellerCount:    5,
		LastEnrichedAt: &now,
		CreatedAt:      now,
	}
}

// staleProduct returns a SharedProduct enriched 48 hours ago.
func staleProduct(asin string) *domain.SharedProduct {
	stale := time.Now().Add(-48 * time.Hour)
	return &domain.SharedProduct{
		ASIN:           asin,
		Title:          "Stale Product " + asin,
		Brand:          "OldBrand",
		Category:       "Toys",
		BuyBoxPrice:    19.99,
		BSRRank:        5000,
		SellerCount:    3,
		LastEnrichedAt: &stale,
		CreatedAt:      stale,
	}
}

// scAccountWithCredits creates a CreditAccount for scTestTenantID with credits remaining.
func scAccountWithCredits(remaining int) *domain.CreditAccount {
	return &domain.CreditAccount{
		TenantID:      scTestTenantID,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  remaining,
		UsedThisMonth: 0,
		ResetAt:       time.Now().AddDate(0, 1, 0),
	}
}

// scAccountWithNoCredits creates a CreditAccount that is fully used.
func scAccountWithNoCredits() *domain.CreditAccount {
	return &domain.CreditAccount{
		TenantID:      scTestTenantID,
		Tier:          domain.CreditTierFree,
		MonthlyLimit:  500,
		UsedThisMonth: 500,
		ResetAt:       time.Now().AddDate(0, 1, 0),
	}
}

// ---------------------------------------------------------------------------
// EnrichProduct tests
// ---------------------------------------------------------------------------

func TestEnrichProduct_ReturnsCachedWhenFresh(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	fp := freshProduct("B0001")
	catalog.products["B0001"] = fp

	spapi := &mockProductSearcher{}
	svc := buildSCService(catalog, nil, nil, spapi, nil)

	product, spent, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected credits not spent for cached fresh product")
	}
	if product == nil {
		t.Fatal("expected product, got nil")
	}
	if product.ASIN != "B0001" {
		t.Errorf("expected ASIN B0001, got %s", product.ASIN)
	}
	if spapi.detailsCalled != 0 {
		t.Errorf("expected 0 SP-API calls, got %d", spapi.detailsCalled)
	}
}

func TestEnrichProduct_CallsSPAPIWhenStale(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	sp := staleProduct("B0002")
	catalog.products["B0002"] = sp

	spapi := &mockProductSearcher{
		detailsResult: []port.ProductSearchResult{
			{
				ASIN:        "B0002",
				Title:       "Updated Product",
				Brand:       "NewBrand",
				Category:    "Electronics",
				AmazonPrice: 39.99,
				BSRRank:     800,
				SellerCount: 7,
			},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(100))

	svc := buildSCService(catalog, nil, nil, spapi, credits)

	product, spent, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spent {
		t.Error("expected credits to be spent for stale product enrichment")
	}
	if product == nil {
		t.Fatal("expected product, got nil")
	}
	if product.Title != "Updated Product" {
		t.Errorf("expected title 'Updated Product', got %q", product.Title)
	}
	if product.BuyBoxPrice != 39.99 {
		t.Errorf("expected buy box price 39.99, got %f", product.BuyBoxPrice)
	}
	if spapi.detailsCalled != 1 {
		t.Errorf("expected 1 SP-API call, got %d", spapi.detailsCalled)
	}

	// Verify catalog was updated
	catalog.mu.Lock()
	stored := catalog.products["B0002"]
	catalog.mu.Unlock()
	if stored == nil || stored.Title != "Updated Product" {
		t.Error("expected catalog to be updated with new product data")
	}
	if catalog.enriched["B0002"] != 1 {
		t.Errorf("expected enrichment incremented once, got %d", catalog.enriched["B0002"])
	}
}

func TestEnrichProduct_ReturnsStaleWhenNoCredits(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	sp := staleProduct("B0003")
	catalog.products["B0003"] = sp

	spapi := &mockProductSearcher{}
	credits := buildSCCreditService(scAccountWithNoCredits())

	svc := buildSCService(catalog, nil, nil, spapi, credits)

	product, spent, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected no credits spent when balance is zero")
	}
	if product == nil {
		t.Fatal("expected stale product returned, got nil")
	}
	if product.ASIN != "B0003" {
		t.Errorf("expected ASIN B0003, got %s", product.ASIN)
	}
	if spapi.detailsCalled != 0 {
		t.Errorf("expected 0 SP-API calls when no credits, got %d", spapi.detailsCalled)
	}
}

func TestEnrichProduct_ReturnsNilWhenNoCacheAndNoCredits(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	// No product in cache

	spapi := &mockProductSearcher{}
	credits := buildSCCreditService(scAccountWithNoCredits())

	svc := buildSCService(catalog, nil, nil, spapi, credits)

	product, spent, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0099")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected no credits spent")
	}
	if product != nil {
		t.Errorf("expected nil product when no cache and no credits, got %+v", product)
	}
}

func TestEnrichProduct_CalculatesEstimatedMargin(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	spapi := &mockProductSearcher{
		detailsResult: []port.ProductSearchResult{
			{
				ASIN:        "B0010",
				Title:       "Margin Product",
				Brand:       "MBrand",
				Category:    "Books",
				AmazonPrice: 50.0,
				BSRRank:     200,
				SellerCount: 3,
			},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(catalog, nil, nil, spapi, credits)

	product, spent, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0010")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spent {
		t.Error("expected credits spent for new product enrichment")
	}
	if product == nil {
		t.Fatal("expected product, got nil")
	}
	// CalculateFBAFees(50.0, 50.0*0.4=20.0, 1.0, false) should produce a nonzero margin
	if product.EstimatedMargin == 0 {
		t.Error("expected non-zero estimated margin for product with price > 0")
	}
}

func TestEnrichProduct_UpdatesBrandCatalog(t *testing.T) {
	brands := newMockBrandCatalogRepo()
	spapi := &mockProductSearcher{
		detailsResult: []port.ProductSearchResult{
			{
				ASIN:        "B0011",
				Title:       "Brand Test",
				Brand:       "SuperBrand",
				Category:    "Kitchen",
				AmazonPrice: 25.0,
			},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(nil, brands, nil, spapi, credits)

	_, _, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0011")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brands.mu.Lock()
	defer brands.mu.Unlock()
	if _, ok := brands.brands["superbrand"]; !ok {
		t.Error("expected brand 'superbrand' to be upserted in brand catalog")
	}
	if brands.productCounts["superbrand"] != 1 {
		t.Errorf("expected product count 1 for 'superbrand', got %d", brands.productCounts["superbrand"])
	}
}

func TestEnrichProduct_NilCreditsService(t *testing.T) {
	// When credits service is nil, enrichment should proceed without credit checks.
	catalog := newMockSharedCatalogRepo()
	spapi := &mockProductSearcher{
		detailsResult: []port.ProductSearchResult{
			{ASIN: "B0012", Title: "No Credits Check", AmazonPrice: 30.0},
		},
	}
	svc := buildSCService(catalog, nil, nil, spapi, nil)

	product, _, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0012")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product == nil {
		t.Fatal("expected product when credits is nil, got nil")
	}
	if spapi.detailsCalled != 1 {
		t.Errorf("expected SP-API called, got %d calls", spapi.detailsCalled)
	}
}

func TestEnrichProduct_SPAPIReturnsEmpty(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	sp := staleProduct("B0013")
	catalog.products["B0013"] = sp

	spapi := &mockProductSearcher{
		detailsResult: []port.ProductSearchResult{}, // empty
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(catalog, nil, nil, spapi, credits)

	product, spent, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0013")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spent {
		t.Error("expected credits spent (credit deducted before SP-API call)")
	}
	// Should return stale cached product when SP-API returns empty
	if product == nil {
		t.Fatal("expected stale cached product returned, got nil")
	}
	if product.Title != sp.Title {
		t.Errorf("expected stale product title %q, got %q", sp.Title, product.Title)
	}
}

func TestEnrichProduct_SPAPIReturnsError(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	sp := staleProduct("B0014")
	catalog.products["B0014"] = sp

	spapi := &mockProductSearcher{
		detailsErr: fmt.Errorf("SP-API throttled"),
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(catalog, nil, nil, spapi, credits)

	product, _, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0014")
	if err == nil {
		t.Fatal("expected error from SP-API failure")
	}
	// Should still return the stale cached product
	if product == nil {
		t.Fatal("expected stale cached product on error, got nil")
	}
}

func TestEnrichProduct_NilSPAPI(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	sp := staleProduct("B0015")
	catalog.products["B0015"] = sp

	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(catalog, nil, nil, nil, credits)

	product, spent, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0015")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected no credits marked as spent when spapi is nil")
	}
	// Returns cached (stale) data
	if product == nil {
		t.Fatal("expected cached product returned when spapi is nil")
	}
}

// ---------------------------------------------------------------------------
// CheckEligibility tests
// ---------------------------------------------------------------------------

func TestCheckEligibility_ReturnsCachedWhenFresh(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	eligRepo.data[eligKey(scTestTenantID, "B0020")] = &domain.TenantEligibility{
		TenantID:  scTestTenantID,
		ASIN:      "B0020",
		Eligible:  true,
		CheckedAt: time.Now().Add(-3 * 24 * time.Hour), // 3 days ago — within 14-day window
	}

	spapi := &mockProductSearcher{}
	svc := buildSCService(nil, nil, eligRepo, spapi, nil)

	result, spent, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0020")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected no credits spent for cached eligibility")
	}
	if result == nil || !result.Eligible {
		t.Error("expected cached eligible result")
	}
	if spapi.eligibilityCalled != 0 {
		t.Errorf("expected 0 SP-API eligibility calls, got %d", spapi.eligibilityCalled)
	}
}

func TestCheckEligibility_CallsSPAPIWhenNotCached(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	spapi := &mockProductSearcher{
		eligibilityResult: []port.ListingRestriction{
			{ASIN: "B0021", Allowed: true},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(nil, nil, eligRepo, spapi, credits)

	result, spent, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0021")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spent {
		t.Error("expected credits spent for fresh eligibility check")
	}
	if result == nil || !result.Eligible {
		t.Error("expected eligible result from SP-API")
	}
	if spapi.eligibilityCalled != 1 {
		t.Errorf("expected 1 SP-API eligibility call, got %d", spapi.eligibilityCalled)
	}
}

func TestCheckEligibility_StoresResultInRepo(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	spapi := &mockProductSearcher{
		eligibilityResult: []port.ListingRestriction{
			{ASIN: "B0022", Allowed: false, Reason: "Brand gated"},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	catalog := newMockSharedCatalogRepo()
	svc := buildSCService(catalog, nil, eligRepo, spapi, credits)

	result, _, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0022")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Eligible {
		t.Error("expected ineligible result")
	}
	if result.Reason != "Brand gated" {
		t.Errorf("expected reason 'Brand gated', got %q", result.Reason)
	}

	// Verify stored in repo
	eligRepo.mu.Lock()
	stored := eligRepo.data[eligKey(scTestTenantID, "B0022")]
	eligRepo.mu.Unlock()
	if stored == nil {
		t.Fatal("expected eligibility stored in repo")
	}
	if stored.Eligible {
		t.Error("expected stored result to be ineligible")
	}
}

func TestCheckEligibility_ReturnsStaleDataWhenNoCredits(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	// Stale eligibility — 20 days ago, beyond 14-day window
	eligRepo.data[eligKey(scTestTenantID, "B0023")] = &domain.TenantEligibility{
		TenantID:  scTestTenantID,
		ASIN:      "B0023",
		Eligible:  false,
		Reason:    "was gated",
		CheckedAt: time.Now().Add(-20 * 24 * time.Hour),
	}

	spapi := &mockProductSearcher{}
	credits := buildSCCreditService(scAccountWithNoCredits())
	svc := buildSCService(nil, nil, eligRepo, spapi, credits)

	result, spent, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0023")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected no credits spent when none available")
	}
	if result == nil {
		t.Fatal("expected stale result returned")
	}
	if result.Reason != "was gated" {
		t.Errorf("expected stale reason 'was gated', got %q", result.Reason)
	}
	if spapi.eligibilityCalled != 0 {
		t.Errorf("expected 0 SP-API calls, got %d", spapi.eligibilityCalled)
	}
}

func TestCheckEligibility_ReturnsNilWhenNoCacheAndNoCredits(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	spapi := &mockProductSearcher{}
	credits := buildSCCreditService(scAccountWithNoCredits())
	svc := buildSCService(nil, nil, eligRepo, spapi, credits)

	result, spent, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0099")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected no credits spent")
	}
	if result != nil {
		t.Errorf("expected nil when no cache and no credits, got %+v", result)
	}
}

func TestCheckEligibility_UpdatesBrandGatingWhenIneligible(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	catalog.products["B0024"] = &domain.SharedProduct{
		ASIN:  "B0024",
		Brand: "GatedBrand",
	}

	brands := newMockBrandCatalogRepo()
	eligRepo := newMockTenantEligibilityRepo()
	spapi := &mockProductSearcher{
		eligibilityResult: []port.ListingRestriction{
			{ASIN: "B0024", Allowed: false, Reason: "Brand requires approval"},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(catalog, brands, eligRepo, spapi, credits)

	_, _, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0024")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brands.mu.Lock()
	gating := brands.gatingUpdates["gatedbrand"]
	brands.mu.Unlock()
	if gating != "brand_gated" {
		t.Errorf("expected gating update 'brand_gated' for ineligible brand, got %q", gating)
	}
}

func TestCheckEligibility_StaleCache(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	// Beyond the 14-day boundary — should be considered stale
	eligRepo.data[eligKey(scTestTenantID, "B0025")] = &domain.TenantEligibility{
		TenantID:  scTestTenantID,
		ASIN:      "B0025",
		Eligible:  true,
		CheckedAt: time.Now().Add(-15 * 24 * time.Hour),
	}

	spapi := &mockProductSearcher{
		eligibilityResult: []port.ListingRestriction{
			{ASIN: "B0025", Allowed: false, Reason: "Now restricted"},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(nil, nil, eligRepo, spapi, credits)

	result, spent, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0025")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spent {
		t.Error("expected credits spent for stale eligibility check")
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Eligible {
		t.Error("expected ineligible after fresh check")
	}
}

func TestCheckEligibility_SPAPIError(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	spapi := &mockProductSearcher{
		eligibilityErr: fmt.Errorf("SP-API unavailable"),
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(nil, nil, eligRepo, spapi, credits)

	_, _, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0026")
	if err == nil {
		t.Fatal("expected error from SP-API failure")
	}
}

func TestCheckEligibility_EmptyRestrictions(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	spapi := &mockProductSearcher{
		eligibilityResult: []port.ListingRestriction{}, // empty = eligible
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(nil, nil, eligRepo, spapi, credits)

	result, spent, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0027")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !spent {
		t.Error("expected credits spent")
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.Eligible {
		t.Error("expected eligible when no restrictions returned")
	}
}

func TestCheckEligibility_NilSPAPI(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(nil, nil, eligRepo, nil, credits)

	result, spent, err := svc.CheckEligibility(context.Background(), scTestTenantID, "B0028")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spent {
		t.Error("expected no credits marked as spent when spapi is nil")
	}
	if result == nil {
		t.Fatal("expected default eligible result when spapi is nil")
	}
	if !result.Eligible {
		t.Error("expected eligible by default when spapi is nil")
	}
}

// ---------------------------------------------------------------------------
// RecordFromScan tests
// ---------------------------------------------------------------------------

func TestRecordFromScan_UpsertsProducts(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	svc := buildSCService(catalog, nil, nil, nil, nil)

	products := []port.ProductSearchResult{
		{ASIN: "B0030", Title: "Scan Product 1", Brand: "Brand1", Category: "Cat1", AmazonPrice: 25.0, BSRRank: 100},
		{ASIN: "B0031", Title: "Scan Product 2", Brand: "Brand2", Category: "Cat2", AmazonPrice: 35.0, BSRRank: 200},
		{ASIN: "", Title: "Empty ASIN"}, // should be skipped
	}

	err := svc.RecordFromScan(context.Background(), products)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if len(catalog.batches) != 1 {
		t.Fatalf("expected 1 batch upsert call, got %d", len(catalog.batches))
	}
	if len(catalog.batches[0]) != 2 {
		t.Errorf("expected 2 products in batch (empty ASIN skipped), got %d", len(catalog.batches[0]))
	}
	if _, ok := catalog.products["B0030"]; !ok {
		t.Error("expected B0030 in catalog")
	}
	if _, ok := catalog.products["B0031"]; !ok {
		t.Error("expected B0031 in catalog")
	}
}

func TestRecordFromScan_UpdatesBrandCatalog(t *testing.T) {
	brands := newMockBrandCatalogRepo()
	svc := buildSCService(nil, brands, nil, nil, nil)

	products := []port.ProductSearchResult{
		{ASIN: "B0040", Title: "Test", Brand: "  BrandOne  ", Category: "Cat1", AmazonPrice: 10.0},
		{ASIN: "B0041", Title: "Test2", Brand: "BrandTwo", Category: "Cat2", AmazonPrice: 20.0},
		{ASIN: "B0042", Title: "No brand", Brand: "", Category: "Cat3", AmazonPrice: 15.0},
	}

	err := svc.RecordFromScan(context.Background(), products)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brands.mu.Lock()
	defer brands.mu.Unlock()
	if _, ok := brands.brands["brandone"]; !ok {
		t.Error("expected 'brandone' (normalized) in brand catalog")
	}
	if _, ok := brands.brands["brandtwo"]; !ok {
		t.Error("expected 'brandtwo' in brand catalog")
	}
	if brands.productCounts["brandone"] != 1 {
		t.Errorf("expected product count 1 for 'brandone', got %d", brands.productCounts["brandone"])
	}
}

func TestRecordFromScan_SetsLastEnrichedAtWhenPriceOrBSR(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	svc := buildSCService(catalog, nil, nil, nil, nil)

	products := []port.ProductSearchResult{
		{ASIN: "B0050", Title: "Has price", AmazonPrice: 20.0},
		{ASIN: "B0051", Title: "Has BSR", BSRRank: 500},
		{ASIN: "B0052", Title: "No price or BSR"},
	}

	err := svc.RecordFromScan(context.Background(), products)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if catalog.products["B0050"].LastEnrichedAt == nil {
		t.Error("expected LastEnrichedAt set for product with price")
	}
	if catalog.products["B0051"].LastEnrichedAt == nil {
		t.Error("expected LastEnrichedAt set for product with BSR rank")
	}
	if catalog.products["B0052"].LastEnrichedAt != nil {
		t.Error("expected LastEnrichedAt nil for product with no price or BSR")
	}
}

func TestRecordFromScan_CalculatesMargin(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	svc := buildSCService(catalog, nil, nil, nil, nil)

	products := []port.ProductSearchResult{
		{ASIN: "B0053", Title: "Margin calc", AmazonPrice: 40.0},
		{ASIN: "B0054", Title: "Zero price", AmazonPrice: 0},
	}

	err := svc.RecordFromScan(context.Background(), products)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if catalog.products["B0053"].EstimatedMargin == 0 {
		t.Error("expected non-zero estimated margin for product with price")
	}
	if catalog.products["B0054"].EstimatedMargin != 0 {
		t.Error("expected zero estimated margin for product with zero price")
	}
}

func TestRecordFromScan_EmptyInput(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	svc := buildSCService(catalog, nil, nil, nil, nil)

	err := svc.RecordFromScan(context.Background(), []port.ProductSearchResult{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if len(catalog.batches) != 0 {
		t.Errorf("expected no batch upserts for empty input, got %d", len(catalog.batches))
	}
}

func TestRecordFromScan_AllEmptyASINs(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	svc := buildSCService(catalog, nil, nil, nil, nil)

	products := []port.ProductSearchResult{
		{ASIN: "", Title: "No ASIN 1"},
		{ASIN: "", Title: "No ASIN 2"},
	}

	err := svc.RecordFromScan(context.Background(), products)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if len(catalog.batches) != 0 {
		t.Errorf("expected no batch upserts when all ASINs empty, got %d", len(catalog.batches))
	}
}

// ---------------------------------------------------------------------------
// GetCachedProducts tests
// ---------------------------------------------------------------------------

func TestGetCachedProducts_ReturnsWithoutSpendingCredits(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	catalog.products["B0060"] = freshProduct("B0060")
	catalog.products["B0061"] = staleProduct("B0061")

	spapi := &mockProductSearcher{}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(catalog, nil, nil, spapi, credits)

	results, err := svc.GetCachedProducts(context.Background(), []string{"B0060", "B0061", "B9999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 cached products, got %d", len(results))
	}
	if spapi.detailsCalled != 0 {
		t.Errorf("expected 0 SP-API calls for cached lookup, got %d", spapi.detailsCalled)
	}
}

// ---------------------------------------------------------------------------
// GetTenantEligibility tests
// ---------------------------------------------------------------------------

func TestGetTenantEligibility_ReturnsCached(t *testing.T) {
	eligRepo := newMockTenantEligibilityRepo()
	eligRepo.data[eligKey(scTestTenantID, "B0070")] = &domain.TenantEligibility{
		TenantID:  scTestTenantID,
		ASIN:      "B0070",
		Eligible:  true,
		CheckedAt: time.Now(),
	}

	svc := buildSCService(nil, nil, eligRepo, nil, nil)

	results, err := svc.GetTenantEligibility(context.Background(), scTestTenantID, []string{"B0070", "B9999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 eligibility result, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Domain type tests
// ---------------------------------------------------------------------------

func TestSharedProduct_IsFresh_NilLastEnrichedAt(t *testing.T) {
	p := &domain.SharedProduct{
		ASIN:           "B0080",
		LastEnrichedAt: nil,
	}
	if p.IsFresh(24 * time.Hour) {
		t.Error("expected IsFresh to return false when LastEnrichedAt is nil")
	}
}

func TestSharedProduct_IsFresh_WithinWindow(t *testing.T) {
	recent := time.Now().Add(-1 * time.Hour)
	p := &domain.SharedProduct{
		ASIN:           "B0081",
		LastEnrichedAt: &recent,
	}
	if !p.IsFresh(24 * time.Hour) {
		t.Error("expected IsFresh true when enriched 1 hour ago with 24h window")
	}
}

func TestSharedProduct_IsFresh_OutsideWindow(t *testing.T) {
	old := time.Now().Add(-48 * time.Hour)
	p := &domain.SharedProduct{
		ASIN:           "B0082",
		LastEnrichedAt: &old,
	}
	if p.IsFresh(24 * time.Hour) {
		t.Error("expected IsFresh false when enriched 48 hours ago with 24h window")
	}
}

func TestSharedProduct_IsFresh_ExactBoundary(t *testing.T) {
	// Enriched exactly at the boundary — time.Since >= maxAge should be false
	boundary := time.Now().Add(-24 * time.Hour)
	p := &domain.SharedProduct{
		ASIN:           "B0083",
		LastEnrichedAt: &boundary,
	}
	// At or beyond the boundary, IsFresh should return false
	if p.IsFresh(24 * time.Hour) {
		t.Error("expected IsFresh false at exact boundary")
	}
}

func TestCreditAccount_Remaining_VariousStates(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		used     int
		expected int
	}{
		{"fully available", 500, 0, 500},
		{"partially used", 500, 200, 300},
		{"fully used", 500, 500, 0},
		{"overdrawn", 500, 600, 0}, // negative clamped to 0
		{"zero limit", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &domain.CreditAccount{
				MonthlyLimit:  tt.limit,
				UsedThisMonth: tt.used,
			}
			got := a.Remaining()
			if got != tt.expected {
				t.Errorf("Remaining() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// upsertBrand edge cases (tested indirectly through EnrichProduct and RecordFromScan)
// ---------------------------------------------------------------------------

func TestRecordFromScan_UpsertBatchError(t *testing.T) {
	catalog := newMockSharedCatalogRepo()
	// Override UpsertProductBatch to return an error by using a failing catalog
	failCatalog := &failingSharedCatalogRepo{mockSharedCatalogRepo: catalog}
	svc := NewSharedCatalogService(failCatalog, newMockBrandCatalogRepo(), newMockTenantEligibilityRepo(), &mockTenantMarginRepo{}, nil, nil)

	products := []port.ProductSearchResult{
		{ASIN: "B0055", Title: "Will fail", AmazonPrice: 10.0},
	}

	err := svc.RecordFromScan(context.Background(), products)
	if err == nil {
		t.Fatal("expected error from UpsertProductBatch failure")
	}
}

func TestUpsertBrand_WhitespaceOnlyBrandSkipped(t *testing.T) {
	brands := newMockBrandCatalogRepo()
	svc := buildSCService(nil, brands, nil, nil, nil)

	// Brands with only whitespace should be normalized to empty and skipped
	products := []port.ProductSearchResult{
		{ASIN: "B0091", Title: "Whitespace Brand", Brand: "   ", AmazonPrice: 10.0},
	}

	err := svc.RecordFromScan(context.Background(), products)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brands.mu.Lock()
	defer brands.mu.Unlock()
	if len(brands.brands) != 0 {
		t.Errorf("expected no brands upserted for whitespace-only brand, got %d", len(brands.brands))
	}
}

func TestEnrichProduct_EmptyBrandNotUpserted(t *testing.T) {
	brands := newMockBrandCatalogRepo()
	spapi := &mockProductSearcher{
		detailsResult: []port.ProductSearchResult{
			{ASIN: "B0090", Title: "No Brand", Brand: "", AmazonPrice: 20.0},
		},
	}
	credits := buildSCCreditService(scAccountWithCredits(10))
	svc := buildSCService(nil, brands, nil, spapi, credits)

	_, _, err := svc.EnrichProduct(context.Background(), scTestTenantID, "B0090")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brands.mu.Lock()
	defer brands.mu.Unlock()
	if len(brands.brands) != 0 {
		t.Errorf("expected no brands upserted for empty brand, got %d", len(brands.brands))
	}
}

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ---------------------------------------------------------------------------
// In-memory mock: SellerProfileRepo
// ---------------------------------------------------------------------------

type memSellerProfileRepo struct {
	mu       sync.Mutex
	profiles map[domain.TenantID]*domain.SellerProfile
	createErr error
	getErr    error
	updateErr error
}

func newMemSellerProfileRepo() *memSellerProfileRepo {
	return &memSellerProfileRepo{
		profiles: make(map[domain.TenantID]*domain.SellerProfile),
	}
}

func (m *memSellerProfileRepo) Create(_ context.Context, profile *domain.SellerProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	cp := *profile
	m.profiles[profile.TenantID] = &cp
	return nil
}

func (m *memSellerProfileRepo) Get(_ context.Context, tenantID domain.TenantID) (*domain.SellerProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	p, ok := m.profiles[tenantID]
	if !ok {
		return nil, fmt.Errorf("profile not found for tenant %s", tenantID)
	}
	cp := *p
	return &cp, nil
}

func (m *memSellerProfileRepo) Update(_ context.Context, profile *domain.SellerProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	cp := *profile
	m.profiles[profile.TenantID] = &cp
	return nil
}

func (m *memSellerProfileRepo) Delete(_ context.Context, tenantID domain.TenantID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.profiles, tenantID)
	return nil
}

// ---------------------------------------------------------------------------
// In-memory mock: EligibilityFingerprintRepo
// ---------------------------------------------------------------------------

type memFingerprintRepo struct {
	mu           sync.Mutex
	fingerprints map[domain.TenantID]*domain.EligibilityFingerprint
	probeResults map[string][]domain.BrandProbeResult   // keyed by fingerprintID
	categories   map[string][]domain.CategoryEligibility // keyed by fingerprintID
	createErr    error
	getErr       error
	saveProbeErr error
	saveCatErr   error
}

func newMemFingerprintRepo() *memFingerprintRepo {
	return &memFingerprintRepo{
		fingerprints: make(map[domain.TenantID]*domain.EligibilityFingerprint),
		probeResults: make(map[string][]domain.BrandProbeResult),
		categories:   make(map[string][]domain.CategoryEligibility),
	}
}

func (m *memFingerprintRepo) Create(_ context.Context, fp *domain.EligibilityFingerprint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	cp := *fp
	m.fingerprints[fp.TenantID] = &cp
	return nil
}

func (m *memFingerprintRepo) Get(_ context.Context, tenantID domain.TenantID) (*domain.EligibilityFingerprint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	fp, ok := m.fingerprints[tenantID]
	if !ok {
		return nil, fmt.Errorf("fingerprint not found for tenant %s", tenantID)
	}
	cp := *fp
	return &cp, nil
}

func (m *memFingerprintRepo) SaveProbeResults(_ context.Context, fingerprintID string, _ domain.TenantID, results []domain.BrandProbeResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveProbeErr != nil {
		return m.saveProbeErr
	}
	m.probeResults[fingerprintID] = results
	return nil
}

func (m *memFingerprintRepo) SaveCategoryEligibilities(_ context.Context, fingerprintID string, _ domain.TenantID, cats []domain.CategoryEligibility) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveCatErr != nil {
		return m.saveCatErr
	}
	m.categories[fingerprintID] = cats
	return nil
}

func (m *memFingerprintRepo) Delete(_ context.Context, tenantID domain.TenantID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.fingerprints, tenantID)
	return nil
}

// ---------------------------------------------------------------------------
// In-memory mock: ProductSearcher (SP-API)
// ---------------------------------------------------------------------------

type assessMockSPAPI struct {
	eligibilityResponses map[string]port.ListingRestriction // keyed by ASIN
	eligibilityErr       error
	errASINs             map[string]bool // ASINs that should return errors
}

func newAssessMockSPAPI() *assessMockSPAPI {
	return &assessMockSPAPI{
		eligibilityResponses: make(map[string]port.ListingRestriction),
		errASINs:             make(map[string]bool),
	}
}

func (m *assessMockSPAPI) CheckListingEligibility(_ context.Context, asins []string, _ string) ([]port.ListingRestriction, error) {
	if m.eligibilityErr != nil {
		return nil, m.eligibilityErr
	}
	if len(asins) > 0 {
		if m.errASINs[asins[0]] {
			return nil, fmt.Errorf("SP-API error for ASIN %s", asins[0])
		}
		if r, ok := m.eligibilityResponses[asins[0]]; ok {
			return []port.ListingRestriction{r}, nil
		}
	}
	// Default: allowed
	return []port.ListingRestriction{{ASIN: asins[0], Allowed: true}}, nil
}

func (m *assessMockSPAPI) SearchProducts(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

func (m *assessMockSPAPI) SearchByBrowseNode(_ context.Context, _ string, _ string, _ string) ([]port.ProductSearchResult, string, error) {
	return nil, "", nil
}

func (m *assessMockSPAPI) GetProductDetails(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

func (m *assessMockSPAPI) EstimateFees(_ context.Context, _ string, _ float64, _ string) (*port.ProductFeeEstimate, error) {
	return nil, nil
}

func (m *assessMockSPAPI) LookupByIdentifier(_ context.Context, _ []string, _ string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// In-memory mock: TenantEligibilityRepo (for SharedCatalogService)
// ---------------------------------------------------------------------------

type memTenantEligibilityRepo struct {
	mu      sync.Mutex
	records map[string]*domain.TenantEligibility // keyed by tenantID+ASIN
}

func newMemTenantEligibilityRepo() *memTenantEligibilityRepo {
	return &memTenantEligibilityRepo{
		records: make(map[string]*domain.TenantEligibility),
	}
}

func (m *memTenantEligibilityRepo) Set(_ context.Context, e *domain.TenantEligibility) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(e.TenantID) + ":" + e.ASIN
	cp := *e
	m.records[key] = &cp
	return nil
}

func (m *memTenantEligibilityRepo) SetBatch(_ context.Context, eligibilities []domain.TenantEligibility) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range eligibilities {
		e := eligibilities[i]
		key := string(e.TenantID) + ":" + e.ASIN
		m.records[key] = &e
	}
	return nil
}

func (m *memTenantEligibilityRepo) Get(_ context.Context, tenantID domain.TenantID, asin string) (*domain.TenantEligibility, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(tenantID) + ":" + asin
	e, ok := m.records[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *e
	return &cp, nil
}

func (m *memTenantEligibilityRepo) GetByASINs(_ context.Context, _ domain.TenantID, _ []string) ([]domain.TenantEligibility, error) {
	return nil, nil
}

func (m *memTenantEligibilityRepo) ListEligible(_ context.Context, _ domain.TenantID, _ string, _ int) ([]domain.TenantEligibility, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Sequential ID generator
// ---------------------------------------------------------------------------

type assessIDGen struct{ counter int }

func (g *assessIDGen) New() string {
	g.counter++
	return fmt.Sprintf("assess-id-%d", g.counter)
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

const assessTestTenant = domain.TenantID("tenant-assess-test")

type assessTestHarness struct {
	svc          *AssessmentService
	profiles     *memSellerProfileRepo
	fingerprints *memFingerprintRepo
	spapi        *assessMockSPAPI
	eligibility  *memTenantEligibilityRepo
	idGen        *assessIDGen
}

func newAssessTestHarness() *assessTestHarness {
	profiles := newMemSellerProfileRepo()
	fingerprints := newMemFingerprintRepo()
	spapi := newAssessMockSPAPI()
	eligibility := newMemTenantEligibilityRepo()
	idGen := &assessIDGen{}

	// Build a minimal SharedCatalogService with only the eligibility repo populated.
	sharedCatalog := &SharedCatalogService{
		eligibility: eligibility,
	}

	svc := NewAssessmentService(profiles, fingerprints, spapi, sharedCatalog, idGen)
	return &assessTestHarness{
		svc:          svc,
		profiles:     profiles,
		fingerprints: fingerprints,
		spapi:        spapi,
		eligibility:  eligibility,
		idGen:        idGen,
	}
}

// ---------------------------------------------------------------------------
// Tests: StartAssessment
// ---------------------------------------------------------------------------

func TestStartAssessment_CreatesProfileWithCorrectArchetype(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	profile, err := h.svc.StartAssessment(ctx, assessTestTenant, 400, 25, 30000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Archetype != domain.SellerArchetypeExpandingPro {
		t.Errorf("archetype = %q, want %q", profile.Archetype, domain.SellerArchetypeExpandingPro)
	}
	if profile.AssessmentStatus != domain.AssessmentStatusRunning {
		t.Errorf("status = %q, want %q", profile.AssessmentStatus, domain.AssessmentStatusRunning)
	}
	if profile.TenantID != assessTestTenant {
		t.Errorf("tenant_id = %q, want %q", profile.TenantID, assessTestTenant)
	}
	if profile.ID == "" {
		t.Error("profile ID should not be empty")
	}

	// Verify persisted
	stored, err := h.profiles.Get(ctx, assessTestTenant)
	if err != nil {
		t.Fatalf("profile not persisted: %v", err)
	}
	if stored.Archetype != domain.SellerArchetypeExpandingPro {
		t.Errorf("stored archetype = %q, want %q", stored.Archetype, domain.SellerArchetypeExpandingPro)
	}
}

func TestStartAssessment_GreenhornClassification(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	profile, err := h.svc.StartAssessment(ctx, assessTestTenant, 30, 3, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Archetype != domain.SellerArchetypeGreenhorn {
		t.Errorf("archetype = %q, want %q", profile.Archetype, domain.SellerArchetypeGreenhorn)
	}
	if profile.AccountAgeDays != 30 {
		t.Errorf("account_age_days = %d, want 30", profile.AccountAgeDays)
	}
	if profile.ActiveListings != 3 {
		t.Errorf("active_listings = %d, want 3", profile.ActiveListings)
	}
}

func TestStartAssessment_CapitalRichClassification(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	profile, err := h.svc.StartAssessment(ctx, assessTestTenant, 60, 2, 75000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Archetype != domain.SellerArchetypeCapitalRich {
		t.Errorf("archetype = %q, want %q", profile.Archetype, domain.SellerArchetypeCapitalRich)
	}
}

func TestStartAssessment_RAToWholesaleClassification(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	profile, err := h.svc.StartAssessment(ctx, assessTestTenant, 180, 15, 10000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Archetype != domain.SellerArchetypeRAToWholesale {
		t.Errorf("archetype = %q, want %q", profile.Archetype, domain.SellerArchetypeRAToWholesale)
	}
}

func TestStartAssessment_ExpandingProClassification(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	profile, err := h.svc.StartAssessment(ctx, assessTestTenant, 500, 30, 20000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Archetype != domain.SellerArchetypeExpandingPro {
		t.Errorf("archetype = %q, want %q", profile.Archetype, domain.SellerArchetypeExpandingPro)
	}
}

func TestStartAssessment_RepoCreateError(t *testing.T) {
	h := newAssessTestHarness()
	h.profiles.createErr = errors.New("db connection lost")

	_, err := h.svc.StartAssessment(context.Background(), assessTestTenant, 30, 3, 5000)
	if err == nil {
		t.Fatal("expected error from repo Create, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: RunEligibilityScan
// ---------------------------------------------------------------------------

func makeProbes(n int, category, brand string) []domain.AssessmentProbe {
	probes := make([]domain.AssessmentProbe, n)
	for i := 0; i < n; i++ {
		probes[i] = domain.AssessmentProbe{
			ASIN:     fmt.Sprintf("B%09d", i),
			Category: category,
			Brand:    brand,
			Tier:     "mid",
		}
	}
	return probes
}

func TestRunEligibilityScan_BuildsFingerprintWithCorrectCounts(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	// 5 probes: 3 allowed, 2 restricted
	probes := makeProbes(5, "Electronics", "BrandA")
	h.spapi.eligibilityResponses[probes[2].ASIN] = port.ListingRestriction{ASIN: probes[2].ASIN, Allowed: false, Reason: "brand_gated"}
	h.spapi.eligibilityResponses[probes[4].ASIN] = port.ListingRestriction{ASIN: probes[4].ASIN, Allowed: false, Reason: "category_gated"}

	fp, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, probes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fp.TotalProbes != 5 {
		t.Errorf("TotalProbes = %d, want 5", fp.TotalProbes)
	}
	if fp.TotalEligible != 3 {
		t.Errorf("TotalEligible = %d, want 3", fp.TotalEligible)
	}
	if fp.TotalRestricted != 2 {
		t.Errorf("TotalRestricted = %d, want 2", fp.TotalRestricted)
	}

	// Open rate: 3/5 * 100 = 60%
	wantRate := 60.0
	if fp.OverallOpenRate != wantRate {
		t.Errorf("OverallOpenRate = %.2f, want %.2f", fp.OverallOpenRate, wantRate)
	}
}

func TestRunEligibilityScan_CategoryOpenRatesCorrect(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	probes := []domain.AssessmentProbe{
		{ASIN: "ASIN-E1", Category: "Electronics", Brand: "B1", Tier: "mid"},
		{ASIN: "ASIN-E2", Category: "Electronics", Brand: "B2", Tier: "mid"},
		{ASIN: "ASIN-G1", Category: "Grocery", Brand: "B3", Tier: "mid"},
		{ASIN: "ASIN-G2", Category: "Grocery", Brand: "B4", Tier: "mid"},
	}

	// Electronics: 1 open, 1 gated => 50% open rate
	h.spapi.eligibilityResponses["ASIN-E2"] = port.ListingRestriction{ASIN: "ASIN-E2", Allowed: false, Reason: "gated"}
	// Grocery: both open => 100% open rate
	// (default is allowed)

	fp, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, probes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fp.Categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(fp.Categories))
	}

	catMap := make(map[string]domain.CategoryEligibility)
	for _, c := range fp.Categories {
		catMap[c.Category] = c
	}

	elec, ok := catMap["Electronics"]
	if !ok {
		t.Fatal("Electronics category not found in results")
	}
	if elec.OpenRate != 50.0 {
		t.Errorf("Electronics OpenRate = %.2f, want 50.00", elec.OpenRate)
	}
	if elec.OpenCount != 1 || elec.GatedCount != 1 {
		t.Errorf("Electronics counts: open=%d gated=%d, want open=1 gated=1", elec.OpenCount, elec.GatedCount)
	}

	groc, ok := catMap["Grocery"]
	if !ok {
		t.Fatal("Grocery category not found in results")
	}
	if groc.OpenRate != 100.0 {
		t.Errorf("Grocery OpenRate = %.2f, want 100.00", groc.OpenRate)
	}
}

func TestRunEligibilityScan_HandlesSPAPIErrorsGracefully(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	probes := makeProbes(5, "Electronics", "BrandA")
	// Make probes 1 and 3 fail with SP-API errors
	h.spapi.errASINs[probes[1].ASIN] = true
	h.spapi.errASINs[probes[3].ASIN] = true

	fp, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, probes)
	if err != nil {
		t.Fatalf("scan should not error on individual ASIN failures: %v", err)
	}

	// Only 3 probes should have results (2 skipped due to errors)
	if fp.TotalProbes != 3 {
		t.Errorf("TotalProbes = %d, want 3 (2 skipped due to errors)", fp.TotalProbes)
	}
	if fp.TotalEligible != 3 {
		t.Errorf("TotalEligible = %d, want 3", fp.TotalEligible)
	}
}

func TestRunEligibilityScan_NilSPAPIReturnsNil(t *testing.T) {
	h := newAssessTestHarness()
	// Replace service with one that has nil spapi
	h.svc = NewAssessmentService(h.profiles, h.fingerprints, nil, nil, h.idGen)

	probes := makeProbes(5, "Electronics", "BrandA")
	fp, err := h.svc.RunEligibilityScan(context.Background(), assessTestTenant, probes)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if fp != nil {
		t.Errorf("expected nil fingerprint when spapi is nil, got %+v", fp)
	}
}

func TestRunEligibilityScan_ConfidenceBasedOnProbeCount(t *testing.T) {
	tests := []struct {
		name       string
		probeCount int
		wantConf   float64
	}{
		{"300 probes = full confidence", 300, 1.0},
		{"150 probes = half confidence", 150, 0.5},
		{"600 probes = capped at 1.0", 600, 1.0},
		{"0 probes = zero confidence", 0, 0.0},
		{"1 probe = 1/300", 1, 1.0 / 300.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAssessTestHarness()
			ctx := context.Background()
			probes := makeProbes(tt.probeCount, "Electronics", "BrandA")

			fp, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, probes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			const epsilon = 0.0001
			diff := fp.Confidence - tt.wantConf
			if diff < -epsilon || diff > epsilon {
				t.Errorf("Confidence = %f, want %f", fp.Confidence, tt.wantConf)
			}
		})
	}
}

func TestRunEligibilityScan_StoresResultsInTenantEligibility(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	probes := []domain.AssessmentProbe{
		{ASIN: "ASIN-001", Category: "Electronics", Brand: "BrandA", Tier: "top"},
		{ASIN: "ASIN-002", Category: "Electronics", Brand: "BrandB", Tier: "mid"},
	}
	h.spapi.eligibilityResponses["ASIN-002"] = port.ListingRestriction{ASIN: "ASIN-002", Allowed: false, Reason: "brand_gated"}

	_, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, probes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that tenant eligibility records were stored in the shared catalog
	e1, err := h.eligibility.Get(ctx, assessTestTenant, "ASIN-001")
	if err != nil {
		t.Fatalf("expected eligibility record for ASIN-001: %v", err)
	}
	if !e1.Eligible {
		t.Error("ASIN-001 should be eligible")
	}

	e2, err := h.eligibility.Get(ctx, assessTestTenant, "ASIN-002")
	if err != nil {
		t.Fatalf("expected eligibility record for ASIN-002: %v", err)
	}
	if e2.Eligible {
		t.Error("ASIN-002 should not be eligible")
	}
	if e2.Reason != "brand_gated" {
		t.Errorf("ASIN-002 reason = %q, want %q", e2.Reason, "brand_gated")
	}
}

func TestRunEligibilityScan_PersistsFingerprintAndResults(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	probes := makeProbes(3, "Electronics", "BrandA")

	fp, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, probes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fingerprint persisted
	stored, err := h.fingerprints.Get(ctx, assessTestTenant)
	if err != nil {
		t.Fatalf("fingerprint not persisted: %v", err)
	}
	if stored.TotalProbes != fp.TotalProbes {
		t.Errorf("stored TotalProbes = %d, want %d", stored.TotalProbes, fp.TotalProbes)
	}

	// Probe results persisted
	h.fingerprints.mu.Lock()
	savedProbes := h.fingerprints.probeResults[fp.ID]
	h.fingerprints.mu.Unlock()
	if len(savedProbes) != 3 {
		t.Errorf("saved probe results count = %d, want 3", len(savedProbes))
	}

	// Category eligibilities persisted
	h.fingerprints.mu.Lock()
	savedCats := h.fingerprints.categories[fp.ID]
	h.fingerprints.mu.Unlock()
	if len(savedCats) != 1 {
		t.Errorf("saved category count = %d, want 1", len(savedCats))
	}
}

func TestRunEligibilityScan_EmptyProbes(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	fp, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, []domain.AssessmentProbe{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fp.TotalProbes != 0 {
		t.Errorf("TotalProbes = %d, want 0", fp.TotalProbes)
	}
	if fp.OverallOpenRate != 0 {
		t.Errorf("OverallOpenRate = %.2f, want 0", fp.OverallOpenRate)
	}
	if fp.Confidence != 0 {
		t.Errorf("Confidence = %f, want 0", fp.Confidence)
	}
}

func TestRunEligibilityScan_FingerprintCreateError(t *testing.T) {
	h := newAssessTestHarness()
	h.fingerprints.createErr = errors.New("db write failed")

	probes := makeProbes(2, "Electronics", "BrandA")

	_, err := h.svc.RunEligibilityScan(context.Background(), assessTestTenant, probes)
	if err == nil {
		t.Fatal("expected error from fingerprint Create, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: CompleteAssessment
// ---------------------------------------------------------------------------

func TestCompleteAssessment_SetsStatusToCompleted(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	// Create a profile first
	_, err := h.svc.StartAssessment(ctx, assessTestTenant, 30, 3, 5000)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	err = h.svc.CompleteAssessment(ctx, assessTestTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	profile, _ := h.profiles.Get(ctx, assessTestTenant)
	if profile.AssessmentStatus != domain.AssessmentStatusCompleted {
		t.Errorf("status = %q, want %q", profile.AssessmentStatus, domain.AssessmentStatusCompleted)
	}
	if profile.AssessedAt == nil {
		t.Error("AssessedAt should be set after completion")
	}
}

func TestCompleteAssessment_ProfileNotFound(t *testing.T) {
	h := newAssessTestHarness()

	err := h.svc.CompleteAssessment(context.Background(), "nonexistent-tenant")
	if err == nil {
		t.Fatal("expected error for nonexistent profile, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: FailAssessment
// ---------------------------------------------------------------------------

func TestFailAssessment_SetsStatusToFailed(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	_, err := h.svc.StartAssessment(ctx, assessTestTenant, 30, 3, 5000)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	err = h.svc.FailAssessment(ctx, assessTestTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	profile, _ := h.profiles.Get(ctx, assessTestTenant)
	if profile.AssessmentStatus != domain.AssessmentStatusFailed {
		t.Errorf("status = %q, want %q", profile.AssessmentStatus, domain.AssessmentStatusFailed)
	}
}

func TestFailAssessment_ProfileNotFound(t *testing.T) {
	h := newAssessTestHarness()

	err := h.svc.FailAssessment(context.Background(), "nonexistent-tenant")
	if err == nil {
		t.Fatal("expected error for nonexistent profile, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetProfile
// ---------------------------------------------------------------------------

func TestGetProfile_ReturnsProfile(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	created, err := h.svc.StartAssessment(ctx, assessTestTenant, 200, 15, 20000)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := h.svc.GetProfile(ctx, assessTestTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
	if got.Archetype != created.Archetype {
		t.Errorf("Archetype = %q, want %q", got.Archetype, created.Archetype)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	h := newAssessTestHarness()

	_, err := h.svc.GetProfile(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetFingerprint
// ---------------------------------------------------------------------------

func TestGetFingerprint_ReturnsFingerprint(t *testing.T) {
	h := newAssessTestHarness()
	ctx := context.Background()

	probes := makeProbes(10, "Electronics", "BrandA")
	created, err := h.svc.RunEligibilityScan(ctx, assessTestTenant, probes)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := h.svc.GetFingerprint(ctx, assessTestTenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
	if got.TotalProbes != 10 {
		t.Errorf("TotalProbes = %d, want 10", got.TotalProbes)
	}
}

func TestGetFingerprint_NotFound(t *testing.T) {
	h := newAssessTestHarness()

	_, err := h.svc.GetFingerprint(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent fingerprint, got nil")
	}
}

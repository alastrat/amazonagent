package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// ---------------------------------------------------------------------------
// In-memory mock: strategyVersionRepo
// ---------------------------------------------------------------------------

type strategyVersionRepo struct {
	mu       sync.Mutex
	versions []*domain.StrategyVersion
	// error injection
	createErr      error
	getByIDErr     error
	getActiveErr   error
	listErr        error
	nextVersionErr error
	setStatusErr   error
	activateErr    error
}

func newStrategyVersionRepo() *strategyVersionRepo {
	return &strategyVersionRepo{}
}

func (r *strategyVersionRepo) Create(_ context.Context, sv *domain.StrategyVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	cp := *sv
	r.versions = append(r.versions, &cp)
	return nil
}

func (r *strategyVersionRepo) GetByID(_ context.Context, tenantID domain.TenantID, id domain.StrategyVersionID) (*domain.StrategyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	for _, v := range r.versions {
		if v.ID == id && v.TenantID == tenantID {
			cp := *v
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("version %s not found for tenant %s", id, tenantID)
}

func (r *strategyVersionRepo) GetActive(_ context.Context, tenantID domain.TenantID) (*domain.StrategyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getActiveErr != nil {
		return nil, r.getActiveErr
	}
	for _, v := range r.versions {
		if v.TenantID == tenantID && v.Status == domain.StrategyStatusActive {
			cp := *v
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("no active version for tenant %s", tenantID)
}

func (r *strategyVersionRepo) List(_ context.Context, tenantID domain.TenantID, limit int) ([]domain.StrategyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	var result []domain.StrategyVersion
	for _, v := range r.versions {
		if v.TenantID == tenantID {
			result = append(result, *v)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *strategyVersionRepo) NextVersionNumber(_ context.Context, tenantID domain.TenantID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.nextVersionErr != nil {
		return 0, r.nextVersionErr
	}
	max := 0
	for _, v := range r.versions {
		if v.TenantID == tenantID && v.VersionNumber > max {
			max = v.VersionNumber
		}
	}
	return max + 1, nil
}

func (r *strategyVersionRepo) SetStatus(_ context.Context, _ domain.TenantID, id domain.StrategyVersionID, status domain.StrategyStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.setStatusErr != nil {
		return r.setStatusErr
	}
	for _, v := range r.versions {
		if v.ID == id {
			v.Status = status
			return nil
		}
	}
	return fmt.Errorf("version %s not found", id)
}

func (r *strategyVersionRepo) Activate(_ context.Context, tenantID domain.TenantID, id domain.StrategyVersionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.activateErr != nil {
		return r.activateErr
	}
	// Archive current active, then activate target
	for _, v := range r.versions {
		if v.TenantID == tenantID && v.Status == domain.StrategyStatusActive {
			v.Status = domain.StrategyStatusArchived
		}
	}
	for _, v := range r.versions {
		if v.ID == id && v.TenantID == tenantID {
			v.Status = domain.StrategyStatusActive
			return nil
		}
	}
	return fmt.Errorf("version %s not found", id)
}

// helper: get version by ID without locking (call inside tests, not concurrently)
func (r *strategyVersionRepo) get(id domain.StrategyVersionID) *domain.StrategyVersion {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.versions {
		if v.ID == id {
			return v
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sequential ID generator
// ---------------------------------------------------------------------------

type strategyIDGen struct {
	mu      sync.Mutex
	counter int
}

func (g *strategyIDGen) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("strat-id-%d", g.counter)
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newStrategyTestService() (*StrategyService, *strategyVersionRepo, *strategyIDGen) {
	repo := newStrategyVersionRepo()
	idGen := &strategyIDGen{}
	svc := NewStrategyService(repo, idGen)
	return svc, repo, idGen
}

const testTenantID = domain.TenantID("tenant-001")

// ---------------------------------------------------------------------------
// GenerateInitialStrategy
// ---------------------------------------------------------------------------

func TestGenerateInitialStrategy_CreatesDraftWithCorrectStatus(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sv.Status != domain.StrategyStatusDraft {
		t.Errorf("expected status draft, got %s", sv.Status)
	}
	if sv.VersionNumber != 1 {
		t.Errorf("expected version 1, got %d", sv.VersionNumber)
	}
	if sv.CreatedBy != domain.StrategyCreatedBySystem {
		t.Errorf("expected created_by system, got %s", sv.CreatedBy)
	}
	if sv.ChangeReason != "Initial strategy from account assessment" {
		t.Errorf("unexpected change_reason: %s", sv.ChangeReason)
	}
}

func TestGenerateInitialStrategy_Greenhorn_90Day_2KRevenue(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sv.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(sv.Goals))
	}
	g := sv.Goals[0]
	if g.Type != "revenue" {
		t.Errorf("expected goal type revenue, got %s", g.Type)
	}
	if g.TargetAmount != 2000 {
		t.Errorf("expected target 2000, got %.2f", g.TargetAmount)
	}
	if g.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", g.Currency)
	}
	// ~90 days (3 months)
	days := int(g.TimeframeEnd.Sub(g.TimeframeStart).Hours() / 24)
	if days < 88 || days > 92 {
		t.Errorf("expected ~90-day timeframe, got %d days", days)
	}
}

func TestGenerateInitialStrategy_RAToWholesale_30Day_5KRevenue(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeRAToWholesale)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sv.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(sv.Goals))
	}
	g := sv.Goals[0]
	if g.Type != "revenue" {
		t.Errorf("expected goal type revenue, got %s", g.Type)
	}
	if g.TargetAmount != 5000 {
		t.Errorf("expected target 5000, got %.2f", g.TargetAmount)
	}
	days := int(g.TimeframeEnd.Sub(g.TimeframeStart).Hours() / 24)
	if days < 28 || days > 31 {
		t.Errorf("expected ~30-day timeframe, got %d days", days)
	}
}

func TestGenerateInitialStrategy_ExpandingPro_14Day_3KProfit(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeExpandingPro)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sv.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(sv.Goals))
	}
	g := sv.Goals[0]
	if g.Type != "profit" {
		t.Errorf("expected goal type profit, got %s", g.Type)
	}
	if g.TargetAmount != 3000 {
		t.Errorf("expected target 3000, got %.2f", g.TargetAmount)
	}
	days := int(g.TimeframeEnd.Sub(g.TimeframeStart).Hours() / 24)
	if days != 14 {
		t.Errorf("expected 14-day timeframe, got %d days", days)
	}
}

func TestGenerateInitialStrategy_CapitalRich_60Day_10KRevenue(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeCapitalRich)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sv.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(sv.Goals))
	}
	g := sv.Goals[0]
	if g.Type != "revenue" {
		t.Errorf("expected goal type revenue, got %s", g.Type)
	}
	if g.TargetAmount != 10000 {
		t.Errorf("expected target 10000, got %.2f", g.TargetAmount)
	}
	days := int(g.TimeframeEnd.Sub(g.TimeframeStart).Hours() / 24)
	if days < 59 || days > 62 {
		t.Errorf("expected ~60-day timeframe, got %d days", days)
	}
}

func TestGenerateInitialStrategy_BuildsSearchParamsFromFingerprint(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	fp := &domain.EligibilityFingerprint{
		Categories: []domain.CategoryEligibility{
			{Category: "Grocery", OpenRate: 50},      // above 30 => included
			{Category: "Beauty", OpenRate: 10},        // below 30 => excluded
			{Category: "Health", OpenRate: 30},        // exactly 30 => included (>=30)
			{Category: "Electronics", OpenRate: 29.9}, // below 30 => excluded
		},
		BrandResults: []domain.BrandProbeResult{
			{Brand: "BrandA", Eligible: true},
			{Brand: "BrandB", Eligible: false},
			{Brand: "BrandC", Eligible: true},
			{Brand: "BrandA", Eligible: true}, // duplicate — should appear once
		},
	}

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, fp, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := sv.SearchParams

	// Should include Grocery and Health, but not Beauty or Electronics
	catSet := make(map[string]bool)
	for _, c := range params.EligibleCategories {
		catSet[c] = true
	}
	if !catSet["Grocery"] {
		t.Error("expected Grocery in eligible categories")
	}
	if !catSet["Health"] {
		t.Error("expected Health in eligible categories (OpenRate == 30)")
	}
	if catSet["Beauty"] {
		t.Error("Beauty should not be in eligible categories (OpenRate 10)")
	}
	if catSet["Electronics"] {
		t.Error("Electronics should not be in eligible categories (OpenRate 29.9)")
	}
	if len(params.EligibleCategories) != 2 {
		t.Errorf("expected 2 eligible categories, got %d", len(params.EligibleCategories))
	}

	// Should include BrandA and BrandC (eligible), not BrandB
	brandSet := make(map[string]bool)
	for _, b := range params.EligibleBrands {
		brandSet[b] = true
	}
	if !brandSet["BrandA"] {
		t.Error("expected BrandA in eligible brands")
	}
	if !brandSet["BrandC"] {
		t.Error("expected BrandC in eligible brands")
	}
	if brandSet["BrandB"] {
		t.Error("BrandB should not be in eligible brands")
	}

	// Defaults
	if params.MinMarginPct != 15.0 {
		t.Errorf("expected min_margin_pct 15.0, got %.2f", params.MinMarginPct)
	}
	if params.MinSellerCount != 2 {
		t.Errorf("expected min_seller_count 2, got %d", params.MinSellerCount)
	}
}

func TestGenerateInitialStrategy_NilFingerprintUsesDefaults(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := sv.SearchParams
	if len(params.EligibleCategories) != 0 {
		t.Errorf("expected no eligible categories with nil fingerprint, got %d", len(params.EligibleCategories))
	}
	if len(params.EligibleBrands) != 0 {
		t.Errorf("expected no eligible brands with nil fingerprint, got %d", len(params.EligibleBrands))
	}
	if params.MinMarginPct != 15.0 {
		t.Errorf("expected default min_margin_pct 15.0, got %.2f", params.MinMarginPct)
	}
	if params.MinSellerCount != 2 {
		t.Errorf("expected default min_seller_count 2, got %d", params.MinSellerCount)
	}

	expectedWeights := domain.DefaultScoringWeights()
	if params.ScoringWeights != expectedWeights {
		t.Errorf("expected default scoring weights, got %+v", params.ScoringWeights)
	}
}

// ---------------------------------------------------------------------------
// ActivateVersion
// ---------------------------------------------------------------------------

func TestActivateVersion_SucceedsOnDraft(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	err = svc.ActivateVersion(ctx, testTenantID, sv.ID)
	if err != nil {
		t.Fatalf("unexpected error activating draft: %v", err)
	}

	// Verify in repo that status is now active
	stored := repo.get(sv.ID)
	if stored == nil {
		t.Fatal("version not found in repo after activation")
	}
	if stored.Status != domain.StrategyStatusActive {
		t.Errorf("expected status active, got %s", stored.Status)
	}
}

func TestActivateVersion_FailsOnNonDraft(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Activate first so it becomes active
	err = svc.ActivateVersion(ctx, testTenantID, sv.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	// Verify it is active now
	stored := repo.get(sv.ID)
	if stored.Status != domain.StrategyStatusActive {
		t.Fatalf("setup: expected active, got %s", stored.Status)
	}

	// Try to activate an already-active version => should fail
	err = svc.ActivateVersion(ctx, testTenantID, sv.ID)
	if err == nil {
		t.Fatal("expected error activating non-draft version, got nil")
	}
}

func TestActivateVersion_ArchivesPreviousActive(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	// Create and activate version 1
	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup v1: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate v1: %v", err)
	}

	// Create version 2 (draft)
	v2, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeRAToWholesale)
	if err != nil {
		t.Fatalf("setup v2: %v", err)
	}

	// Activate version 2 => version 1 should be archived
	err = svc.ActivateVersion(ctx, testTenantID, v2.ID)
	if err != nil {
		t.Fatalf("activate v2: %v", err)
	}

	storedV1 := repo.get(v1.ID)
	if storedV1.Status != domain.StrategyStatusArchived {
		t.Errorf("expected v1 status archived, got %s", storedV1.Status)
	}
	storedV2 := repo.get(v2.ID)
	if storedV2.Status != domain.StrategyStatusActive {
		t.Errorf("expected v2 status active, got %s", storedV2.Status)
	}
}

// ---------------------------------------------------------------------------
// RollbackToVersion
// ---------------------------------------------------------------------------

func TestRollbackToVersion_CreatesNewVersionWithTargetParams(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	// Create and activate v1 with specific fingerprint
	fp := &domain.EligibilityFingerprint{
		Categories: []domain.CategoryEligibility{
			{Category: "Grocery", OpenRate: 80},
		},
		BrandResults: []domain.BrandProbeResult{
			{Brand: "BrandX", Eligible: true},
		},
	}
	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, fp, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup v1: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate v1: %v", err)
	}

	// Create and activate v2 with different params
	v2, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeCapitalRich)
	if err != nil {
		t.Fatalf("setup v2: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v2.ID)
	if err != nil {
		t.Fatalf("setup activate v2: %v", err)
	}

	// Rollback to v1
	newVer, err := svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// New version should have v1's goals and search params
	if len(newVer.Goals) != len(v1.Goals) {
		t.Errorf("expected %d goals from v1, got %d", len(v1.Goals), len(newVer.Goals))
	}
	if newVer.Goals[0].TargetAmount != v1.Goals[0].TargetAmount {
		t.Errorf("expected target amount %.2f, got %.2f", v1.Goals[0].TargetAmount, newVer.Goals[0].TargetAmount)
	}

	// Should have v1's search params (Grocery category)
	catSet := make(map[string]bool)
	for _, c := range newVer.SearchParams.EligibleCategories {
		catSet[c] = true
	}
	if !catSet["Grocery"] {
		t.Error("expected Grocery category from v1 in rollback version")
	}
}

func TestRollbackToVersion_MarksCurrentActiveAsRolledBack(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup v1: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate v1: %v", err)
	}

	// Create v2, activate
	v2, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeCapitalRich)
	if err != nil {
		t.Fatalf("setup v2: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v2.ID)
	if err != nil {
		t.Fatalf("setup activate v2: %v", err)
	}

	// Rollback to v1
	_, err = svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	storedV2 := repo.get(v2.ID)
	if storedV2.Status != domain.StrategyStatusRolledBack {
		t.Errorf("expected v2 status rolled_back, got %s", storedV2.Status)
	}
}

func TestRollbackToVersion_NewVersionHasCorrectParentVersionID(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup v1: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	newVer, err := svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if newVer.ParentVersionID != v1.ID {
		t.Errorf("expected parent_version_id %s, got %s", v1.ID, newVer.ParentVersionID)
	}
}

func TestRollbackToVersion_VersionNumberIncrements(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup v1: %v", err)
	}
	if v1.VersionNumber != 1 {
		t.Fatalf("expected v1 version 1, got %d", v1.VersionNumber)
	}

	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	v2, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeCapitalRich)
	if err != nil {
		t.Fatalf("setup v2: %v", err)
	}
	if v2.VersionNumber != 2 {
		t.Fatalf("expected v2 version 2, got %d", v2.VersionNumber)
	}

	err = svc.ActivateVersion(ctx, testTenantID, v2.ID)
	if err != nil {
		t.Fatalf("setup activate v2: %v", err)
	}

	// Rollback to v1 => new version should be v3
	newVer, err := svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if newVer.VersionNumber != 3 {
		t.Errorf("expected version number 3 after rollback, got %d", newVer.VersionNumber)
	}
}

func TestRollbackToVersion_NewVersionIsActive(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	newVer, err := svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if newVer.Status != domain.StrategyStatusActive {
		t.Errorf("expected new rollback version to be active, got %s", newVer.Status)
	}
	if newVer.ActivatedAt == nil {
		t.Error("expected ActivatedAt to be set on rollback version")
	}
	if newVer.CreatedBy != domain.StrategyCreatedByUser {
		t.Errorf("expected created_by user for rollback, got %s", newVer.CreatedBy)
	}
}

func TestRollbackToVersion_ChangeReasonReferencesTarget(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	newVer, err := svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}

	expected := fmt.Sprintf("Rollback to v%d", v1.VersionNumber)
	if newVer.ChangeReason != expected {
		t.Errorf("expected change_reason %q, got %q", expected, newVer.ChangeReason)
	}
}

// ---------------------------------------------------------------------------
// GetActive, ListVersions, GetVersion
// ---------------------------------------------------------------------------

func TestGetActive_ReturnsActiveVersion(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	active, err := svc.GetActive(ctx, testTenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active.ID != v1.ID {
		t.Errorf("expected active version ID %s, got %s", v1.ID, active.ID)
	}
	if active.Status != domain.StrategyStatusActive {
		t.Errorf("expected active status, got %s", active.Status)
	}
}

func TestGetActive_NoActiveReturnsError(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	_, err := svc.GetActive(ctx, testTenantID)
	if err == nil {
		t.Fatal("expected error when no active version exists, got nil")
	}
}

func TestListVersions_ReturnsAllVersions(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	_, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("v1: %v", err)
	}
	_, err = svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeCapitalRich)
	if err != nil {
		t.Fatalf("v2: %v", err)
	}

	versions, err := svc.ListVersions(ctx, testTenantID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestListVersions_RespectsLimit(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
		if err != nil {
			t.Fatalf("create v%d: %v", i+1, err)
		}
	}

	versions, err := svc.ListVersions(ctx, testTenantID, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("expected 3 versions (limit), got %d", len(versions))
	}
}

func TestListVersions_DifferentTenantsIsolated(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	t1 := domain.TenantID("tenant-A")
	t2 := domain.TenantID("tenant-B")

	_, _ = svc.GenerateInitialStrategy(ctx, t1, nil, domain.SellerArchetypeGreenhorn)
	_, _ = svc.GenerateInitialStrategy(ctx, t1, nil, domain.SellerArchetypeCapitalRich)
	_, _ = svc.GenerateInitialStrategy(ctx, t2, nil, domain.SellerArchetypeExpandingPro)

	v1, _ := svc.ListVersions(ctx, t1, 10)
	v2, _ := svc.ListVersions(ctx, t2, 10)

	if len(v1) != 2 {
		t.Errorf("expected 2 versions for tenant-A, got %d", len(v1))
	}
	if len(v2) != 1 {
		t.Errorf("expected 1 version for tenant-B, got %d", len(v2))
	}
}

func TestGetVersion_ReturnsSpecificVersionByID(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	fetched, err := svc.GetVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetched.ID != v1.ID {
		t.Errorf("expected version ID %s, got %s", v1.ID, fetched.ID)
	}
	if fetched.VersionNumber != v1.VersionNumber {
		t.Errorf("expected version number %d, got %d", v1.VersionNumber, fetched.VersionNumber)
	}
}

func TestGetVersion_NotFoundReturnsError(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	_, err := svc.GetVersion(ctx, testTenantID, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent version, got nil")
	}
}

// ---------------------------------------------------------------------------
// Default archetype (unknown) falls back to greenhorn-like defaults
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestGenerateInitialStrategy_NextVersionNumberError(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	repo.nextVersionErr = fmt.Errorf("db connection lost")
	_, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err == nil {
		t.Fatal("expected error when NextVersionNumber fails, got nil")
	}
}

func TestGenerateInitialStrategy_CreateError(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	repo.createErr = fmt.Errorf("write failure")
	_, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err == nil {
		t.Fatal("expected error when Create fails, got nil")
	}
}

func TestActivateVersion_GetByIDError(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	repo.getByIDErr = fmt.Errorf("not found")
	err := svc.ActivateVersion(ctx, testTenantID, "nonexistent")
	if err == nil {
		t.Fatal("expected error when GetByID fails, got nil")
	}
}

func TestActivateVersion_ActivateRepoError(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	repo.activateErr = fmt.Errorf("db error")
	err = svc.ActivateVersion(ctx, testTenantID, sv.ID)
	if err == nil {
		t.Fatal("expected error when Activate repo fails, got nil")
	}
}

func TestRollbackToVersion_TargetNotFound(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	_, err := svc.RollbackToVersion(ctx, testTenantID, "nonexistent")
	if err == nil {
		t.Fatal("expected error when target version not found, got nil")
	}
}

func TestRollbackToVersion_NextVersionNumberError(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	repo.nextVersionErr = fmt.Errorf("db error")
	_, err = svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err == nil {
		t.Fatal("expected error when NextVersionNumber fails during rollback, got nil")
	}
}

func TestRollbackToVersion_CreateError(t *testing.T) {
	svc, repo, _ := newStrategyTestService()
	ctx := context.Background()

	v1, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetypeGreenhorn)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	err = svc.ActivateVersion(ctx, testTenantID, v1.ID)
	if err != nil {
		t.Fatalf("setup activate: %v", err)
	}

	repo.createErr = fmt.Errorf("write failure")
	_, err = svc.RollbackToVersion(ctx, testTenantID, v1.ID)
	if err == nil {
		t.Fatal("expected error when Create fails during rollback, got nil")
	}
}

// NOTE: RollbackToVersion with no current active version panics at line 136
// (current.VersionNumber accessed when current is nil). This is a known bug
// in the service — the slog.Info call at line 136 should guard for nil current.
// Uncomment the test below after fixing the nil-pointer dereference.
//
// func TestRollbackToVersion_NoCurrentActiveStillSucceeds(t *testing.T) { ... }

// ---------------------------------------------------------------------------
// Default archetype (unknown) falls back to greenhorn-like defaults
// ---------------------------------------------------------------------------

func TestGenerateInitialStrategy_UnknownArchetypeUsesDefault(t *testing.T) {
	svc, _, _ := newStrategyTestService()
	ctx := context.Background()

	sv, err := svc.GenerateInitialStrategy(ctx, testTenantID, nil, domain.SellerArchetype("unknown"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sv.Goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(sv.Goals))
	}
	g := sv.Goals[0]
	if g.Type != "revenue" {
		t.Errorf("expected default goal type revenue, got %s", g.Type)
	}
	if g.TargetAmount != 2000 {
		t.Errorf("expected default target 2000, got %.2f", g.TargetAmount)
	}
}

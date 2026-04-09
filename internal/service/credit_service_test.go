package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// ---------------------------------------------------------------------------
// In-memory mock: creditAcctRepo (with error injection)
// ---------------------------------------------------------------------------

type creditAcctRepo struct {
	mu            sync.Mutex
	accounts      map[domain.TenantID]*domain.CreditAccount
	getErr        error
	ensureErr     error
	debitErr      error
	resetErr      error
	updateTierErr error
	resetCalls    int
}

func newCreditAcctRepo() *creditAcctRepo {
	return &creditAcctRepo{
		accounts: make(map[domain.TenantID]*domain.CreditAccount),
	}
}

func (m *creditAcctRepo) Get(_ context.Context, tenantID domain.TenantID) (*domain.CreditAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	acc, ok := m.accounts[tenantID]
	if !ok {
		return nil, fmt.Errorf("account not found for tenant %s", tenantID)
	}
	copy := *acc
	return &copy, nil
}

func (m *creditAcctRepo) EnsureExists(_ context.Context, tenantID domain.TenantID, tier domain.CreditTier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ensureErr != nil {
		return m.ensureErr
	}
	if _, ok := m.accounts[tenantID]; !ok {
		m.accounts[tenantID] = &domain.CreditAccount{
			TenantID:      tenantID,
			Tier:          tier,
			MonthlyLimit:  tier.MonthlyCredits(),
			UsedThisMonth: 0,
			ResetAt:       time.Now().AddDate(0, 1, 0),
		}
	}
	return nil
}

func (m *creditAcctRepo) Debit(_ context.Context, tenantID domain.TenantID, amount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.debitErr != nil {
		return m.debitErr
	}
	acc, ok := m.accounts[tenantID]
	if !ok {
		return fmt.Errorf("account not found for tenant %s", tenantID)
	}
	acc.UsedThisMonth += amount
	return nil
}

func (m *creditAcctRepo) ResetMonthly(_ context.Context, tenantID domain.TenantID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetCalls++
	if m.resetErr != nil {
		return m.resetErr
	}
	acc, ok := m.accounts[tenantID]
	if !ok {
		return fmt.Errorf("account not found for tenant %s", tenantID)
	}
	acc.UsedThisMonth = 0
	acc.ResetAt = time.Now().AddDate(0, 1, 0)
	return nil
}

func (m *creditAcctRepo) UpdateTier(_ context.Context, tenantID domain.TenantID, tier domain.CreditTier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateTierErr != nil {
		return m.updateTierErr
	}
	acc, ok := m.accounts[tenantID]
	if !ok {
		return fmt.Errorf("account not found for tenant %s", tenantID)
	}
	acc.Tier = tier
	acc.MonthlyLimit = tier.MonthlyCredits()
	return nil
}

func (m *creditAcctRepo) seed(acc *domain.CreditAccount) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[acc.TenantID] = acc
}

// ---------------------------------------------------------------------------
// In-memory mock: creditTxnRepo (with error injection)
// ---------------------------------------------------------------------------

type creditTxnRepo struct {
	mu           sync.Mutex
	transactions []domain.CreditTransaction
	recordErr    error
	listErr      error
}

func newCreditTxnRepo() *creditTxnRepo {
	return &creditTxnRepo{}
}

func (m *creditTxnRepo) Record(_ context.Context, tx *domain.CreditTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recordErr != nil {
		return m.recordErr
	}
	m.transactions = append(m.transactions, *tx)
	return nil
}

func (m *creditTxnRepo) ListByTenant(_ context.Context, tenantID domain.TenantID, limit int) ([]domain.CreditTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []domain.CreditTransaction
	for _, tx := range m.transactions {
		if tx.TenantID == tenantID {
			result = append(result, tx)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Mock: creditIDGen
// ---------------------------------------------------------------------------

type creditIDGen struct {
	counter int
}

func (g *creditIDGen) New() string {
	g.counter++
	return fmt.Sprintf("credit-id-%d", g.counter)
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

type creditTestHarness struct {
	svc   *CreditService
	accts *creditAcctRepo
	txns  *creditTxnRepo
	idGen *creditIDGen
}

func newCreditTestHarness() *creditTestHarness {
	accts := newCreditAcctRepo()
	txns := newCreditTxnRepo()
	idGen := &creditIDGen{}
	svc := NewCreditService(accts, txns, idGen)
	return &creditTestHarness{svc: svc, accts: accts, txns: txns, idGen: idGen}
}

const creditTestTenant = domain.TenantID("tenant-credit-test")

// ---------------------------------------------------------------------------
// Tests: EnsureAccount
// ---------------------------------------------------------------------------

func TestCreditService_EnsureAccount(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	err := h.svc.EnsureAccount(ctx, creditTestTenant, domain.CreditTierStarter)
	if err != nil {
		t.Fatalf("EnsureAccount returned error: %v", err)
	}

	acc, err := h.accts.Get(ctx, creditTestTenant)
	if err != nil {
		t.Fatalf("Get returned error after EnsureAccount: %v", err)
	}
	if acc.Tier != domain.CreditTierStarter {
		t.Errorf("expected tier %s, got %s", domain.CreditTierStarter, acc.Tier)
	}
	if acc.MonthlyLimit != domain.CreditTierStarter.MonthlyCredits() {
		t.Errorf("expected monthly limit %d, got %d", domain.CreditTierStarter.MonthlyCredits(), acc.MonthlyLimit)
	}
	if acc.UsedThisMonth != 0 {
		t.Errorf("expected used_this_month 0, got %d", acc.UsedThisMonth)
	}
}

func TestCreditService_EnsureAccount_Idempotent(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	_ = h.svc.EnsureAccount(ctx, creditTestTenant, domain.CreditTierStarter)
	_ = h.svc.EnsureAccount(ctx, creditTestTenant, domain.CreditTierGrowth)

	acc, _ := h.accts.Get(ctx, creditTestTenant)
	if acc.Tier != domain.CreditTierStarter {
		t.Errorf("expected tier to remain %s after idempotent call, got %s", domain.CreditTierStarter, acc.Tier)
	}
}

func TestCreditService_EnsureAccount_Error(t *testing.T) {
	h := newCreditTestHarness()
	h.accts.ensureErr = fmt.Errorf("db down")

	err := h.svc.EnsureAccount(context.Background(), creditTestTenant, domain.CreditTierFree)
	if err == nil {
		t.Fatal("expected error from EnsureAccount, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetBalance
// ---------------------------------------------------------------------------

func TestCreditService_GetBalance(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierGrowth,
		MonthlyLimit:  25000,
		UsedThisMonth: 3000,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	acc, err := h.svc.GetBalance(ctx, creditTestTenant)
	if err != nil {
		t.Fatalf("GetBalance returned error: %v", err)
	}
	if acc.MonthlyLimit != 25000 {
		t.Errorf("expected monthly_limit 25000, got %d", acc.MonthlyLimit)
	}
	if acc.UsedThisMonth != 3000 {
		t.Errorf("expected used_this_month 3000, got %d", acc.UsedThisMonth)
	}
	if acc.Remaining() != 22000 {
		t.Errorf("expected remaining 22000, got %d", acc.Remaining())
	}
}

func TestCreditService_GetBalance_TriggersMonthlyReset(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 4500,
		ResetAt:       time.Now().Add(-1 * time.Hour),
	})

	acc, err := h.svc.GetBalance(ctx, creditTestTenant)
	if err != nil {
		t.Fatalf("GetBalance returned error: %v", err)
	}
	if acc.UsedThisMonth != 0 {
		t.Errorf("expected used_this_month to be reset to 0, got %d", acc.UsedThisMonth)
	}
	if h.accts.resetCalls != 1 {
		t.Errorf("expected ResetMonthly to be called once, got %d", h.accts.resetCalls)
	}
}

func TestCreditService_GetBalance_ResetErrorDoesNotFail(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 4500,
		ResetAt:       time.Now().Add(-1 * time.Hour),
	})
	h.accts.resetErr = fmt.Errorf("reset failed")

	acc, err := h.svc.GetBalance(ctx, creditTestTenant)
	if err != nil {
		t.Fatalf("GetBalance should not fail when reset errors, got: %v", err)
	}
	if acc.UsedThisMonth != 4500 {
		t.Errorf("expected used_this_month 4500 (reset failed), got %d", acc.UsedThisMonth)
	}
}

func TestCreditService_GetBalance_NotFound(t *testing.T) {
	h := newCreditTestHarness()

	_, err := h.svc.GetBalance(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent tenant, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: HasCredits
// ---------------------------------------------------------------------------

func TestCreditService_HasCredits_True(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierGrowth,
		MonthlyLimit:  25000,
		UsedThisMonth: 10000,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	has, err := h.svc.HasCredits(ctx, creditTestTenant, 15000)
	if err != nil {
		t.Fatalf("HasCredits returned error: %v", err)
	}
	if !has {
		t.Error("expected HasCredits to return true for amount within remaining")
	}
}

func TestCreditService_HasCredits_ExactlyRemaining(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierGrowth,
		MonthlyLimit:  25000,
		UsedThisMonth: 10000,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	has, err := h.svc.HasCredits(ctx, creditTestTenant, 15000)
	if err != nil {
		t.Fatalf("HasCredits returned error: %v", err)
	}
	if !has {
		t.Error("expected HasCredits to return true when amount equals remaining")
	}
}

func TestCreditService_HasCredits_False(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierFree,
		MonthlyLimit:  500,
		UsedThisMonth: 490,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	has, err := h.svc.HasCredits(ctx, creditTestTenant, 11)
	if err != nil {
		t.Fatalf("HasCredits returned error: %v", err)
	}
	if has {
		t.Error("expected HasCredits to return false when amount exceeds remaining")
	}
}

func TestCreditService_HasCredits_ZeroRemaining(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierFree,
		MonthlyLimit:  500,
		UsedThisMonth: 500,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	has, err := h.svc.HasCredits(ctx, creditTestTenant, 1)
	if err != nil {
		t.Fatalf("HasCredits returned error: %v", err)
	}
	if has {
		t.Error("expected HasCredits to return false when zero remaining")
	}
}

func TestCreditService_HasCredits_Error(t *testing.T) {
	h := newCreditTestHarness()
	h.accts.getErr = fmt.Errorf("db error")

	_, err := h.svc.HasCredits(context.Background(), creditTestTenant, 1)
	if err == nil {
		t.Fatal("expected error from HasCredits, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: Spend
// ---------------------------------------------------------------------------

func TestCreditService_Spend_Success(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 1000,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	err := h.svc.Spend(ctx, creditTestTenant, 500, domain.CreditActionDiscoveryScan, "scan-job-123")
	if err != nil {
		t.Fatalf("Spend returned error: %v", err)
	}

	acc, _ := h.accts.Get(ctx, creditTestTenant)
	if acc.UsedThisMonth != 1500 {
		t.Errorf("expected used_this_month 1500 after spend, got %d", acc.UsedThisMonth)
	}
}

func TestCreditService_Spend_InsufficientCredits(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierFree,
		MonthlyLimit:  500,
		UsedThisMonth: 490,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	err := h.svc.Spend(ctx, creditTestTenant, 20, domain.CreditActionEligibilityCheck, "asin-B001")
	if err == nil {
		t.Fatal("expected error for insufficient credits, got nil")
	}

	acc, _ := h.accts.Get(ctx, creditTestTenant)
	if acc.UsedThisMonth != 490 {
		t.Errorf("expected used_this_month to remain 490, got %d", acc.UsedThisMonth)
	}
}

func TestCreditService_Spend_RecordsTransaction(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 0,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	_ = h.svc.Spend(ctx, creditTestTenant, 100, domain.CreditActionEnrichment, "asin-B002")

	txns, _ := h.txns.ListByTenant(ctx, creditTestTenant, 10)
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction recorded, got %d", len(txns))
	}

	tx := txns[0]
	if tx.Amount != -100 {
		t.Errorf("expected transaction amount -100, got %d", tx.Amount)
	}
	if tx.Action != domain.CreditActionEnrichment {
		t.Errorf("expected action %s, got %s", domain.CreditActionEnrichment, tx.Action)
	}
	if tx.Reference != "asin-B002" {
		t.Errorf("expected reference 'asin-B002', got %q", tx.Reference)
	}
	if tx.TenantID != creditTestTenant {
		t.Errorf("expected tenant_id %s, got %s", creditTestTenant, tx.TenantID)
	}
	if tx.ID == "" {
		t.Error("expected non-empty transaction ID")
	}
}

func TestCreditService_Spend_HasCreditsError(t *testing.T) {
	h := newCreditTestHarness()
	h.accts.getErr = fmt.Errorf("db error")

	err := h.svc.Spend(context.Background(), creditTestTenant, 100, domain.CreditActionEnrichment, "ref")
	if err == nil {
		t.Fatal("expected error from Spend when HasCredits fails, got nil")
	}
}

func TestCreditService_Spend_TransactionRecordFailureDoesNotFail(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 0,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})
	h.txns.recordErr = fmt.Errorf("ledger write failed")

	err := h.svc.Spend(ctx, creditTestTenant, 100, domain.CreditActionEnrichment, "ref")
	if err != nil {
		t.Fatalf("Spend should not fail when transaction record fails, got: %v", err)
	}

	acc, _ := h.accts.Get(ctx, creditTestTenant)
	if acc.UsedThisMonth != 100 {
		t.Errorf("expected used_this_month 100, got %d", acc.UsedThisMonth)
	}
}

func TestCreditService_Spend_DebitError(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 0,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})
	h.accts.debitErr = fmt.Errorf("debit failed")

	err := h.svc.Spend(ctx, creditTestTenant, 100, domain.CreditActionEnrichment, "ref")
	if err == nil {
		t.Fatal("expected error from Spend when Debit fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: SpendIfAvailable
// ---------------------------------------------------------------------------

func TestCreditService_SpendIfAvailable_Success(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 0,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	ok := h.svc.SpendIfAvailable(ctx, creditTestTenant, 100, domain.CreditActionAssessment, "ref-1")
	if !ok {
		t.Error("expected SpendIfAvailable to return true when credits available")
	}

	acc, _ := h.accts.Get(ctx, creditTestTenant)
	if acc.UsedThisMonth != 100 {
		t.Errorf("expected used_this_month 100, got %d", acc.UsedThisMonth)
	}
}

func TestCreditService_SpendIfAvailable_Insufficient(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierFree,
		MonthlyLimit:  500,
		UsedThisMonth: 500,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	ok := h.svc.SpendIfAvailable(ctx, creditTestTenant, 1, domain.CreditActionAssessment, "ref-2")
	if ok {
		t.Error("expected SpendIfAvailable to return false when insufficient credits")
	}
}

// ---------------------------------------------------------------------------
// Tests: GrantMonthly
// ---------------------------------------------------------------------------

func TestCreditService_GrantMonthly(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierGrowth,
		MonthlyLimit:  25000,
		UsedThisMonth: 20000,
		ResetAt:       time.Now().Add(-1 * time.Hour),
	})

	err := h.svc.GrantMonthly(ctx, creditTestTenant)
	if err != nil {
		t.Fatalf("GrantMonthly returned error: %v", err)
	}

	acc, _ := h.accts.Get(ctx, creditTestTenant)
	if acc.UsedThisMonth != 0 {
		t.Errorf("expected used_this_month 0 after grant, got %d", acc.UsedThisMonth)
	}

	txns, _ := h.txns.ListByTenant(ctx, creditTestTenant, 10)
	if len(txns) != 1 {
		t.Fatalf("expected 1 grant transaction, got %d", len(txns))
	}
	tx := txns[0]
	if tx.Amount != 25000 {
		t.Errorf("expected grant amount 25000, got %d", tx.Amount)
	}
	if tx.Action != domain.CreditActionMonthlyGrant {
		t.Errorf("expected action %s, got %s", domain.CreditActionMonthlyGrant, tx.Action)
	}
}

func TestCreditService_GrantMonthly_ResetError(t *testing.T) {
	h := newCreditTestHarness()
	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierGrowth,
		MonthlyLimit:  25000,
		UsedThisMonth: 20000,
		ResetAt:       time.Now().Add(-1 * time.Hour),
	})
	h.accts.resetErr = fmt.Errorf("reset failed")

	err := h.svc.GrantMonthly(context.Background(), creditTestTenant)
	if err == nil {
		t.Fatal("expected error from GrantMonthly when reset fails, got nil")
	}
}

func TestCreditService_GrantMonthly_GetErrorAfterReset(t *testing.T) {
	h := newCreditTestHarness()
	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierGrowth,
		MonthlyLimit:  25000,
		UsedThisMonth: 20000,
		ResetAt:       time.Now().Add(-1 * time.Hour),
	})
	// Get will fail, but ResetMonthly will succeed
	h.accts.getErr = fmt.Errorf("get after reset failed")

	err := h.svc.GrantMonthly(context.Background(), creditTestTenant)
	if err == nil {
		t.Fatal("expected error from GrantMonthly when Get fails after reset, got nil")
	}
}

func TestCreditService_GrantMonthly_TransactionRecordError(t *testing.T) {
	h := newCreditTestHarness()
	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierGrowth,
		MonthlyLimit:  25000,
		UsedThisMonth: 20000,
		ResetAt:       time.Now().Add(-1 * time.Hour),
	})
	h.txns.recordErr = fmt.Errorf("ledger write failed")

	err := h.svc.GrantMonthly(context.Background(), creditTestTenant)
	if err == nil {
		t.Fatal("expected error from GrantMonthly when transaction record fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: UpdateTier
// ---------------------------------------------------------------------------

func TestCreditService_UpdateTier(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierFree,
		MonthlyLimit:  500,
		UsedThisMonth: 100,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	err := h.svc.UpdateTier(ctx, creditTestTenant, domain.CreditTierScale)
	if err != nil {
		t.Fatalf("UpdateTier returned error: %v", err)
	}

	acc, _ := h.accts.Get(ctx, creditTestTenant)
	if acc.Tier != domain.CreditTierScale {
		t.Errorf("expected tier %s, got %s", domain.CreditTierScale, acc.Tier)
	}
	if acc.MonthlyLimit != domain.CreditTierScale.MonthlyCredits() {
		t.Errorf("expected monthly limit %d, got %d", domain.CreditTierScale.MonthlyCredits(), acc.MonthlyLimit)
	}
}

func TestCreditService_UpdateTier_Error(t *testing.T) {
	h := newCreditTestHarness()
	h.accts.updateTierErr = fmt.Errorf("update failed")

	err := h.svc.UpdateTier(context.Background(), creditTestTenant, domain.CreditTierGrowth)
	if err == nil {
		t.Fatal("expected error from UpdateTier, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetTransactions
// ---------------------------------------------------------------------------

func TestCreditService_GetTransactions(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 0,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	_ = h.svc.Spend(ctx, creditTestTenant, 10, domain.CreditActionEligibilityCheck, "ref-a")
	_ = h.svc.Spend(ctx, creditTestTenant, 20, domain.CreditActionEnrichment, "ref-b")
	_ = h.svc.Spend(ctx, creditTestTenant, 30, domain.CreditActionAssessment, "ref-c")

	txns, err := h.svc.GetTransactions(ctx, creditTestTenant, 10)
	if err != nil {
		t.Fatalf("GetTransactions returned error: %v", err)
	}
	if len(txns) != 3 {
		t.Errorf("expected 3 transactions, got %d", len(txns))
	}
}

func TestCreditService_GetTransactions_WithLimit(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	h.accts.seed(&domain.CreditAccount{
		TenantID:      creditTestTenant,
		Tier:          domain.CreditTierStarter,
		MonthlyLimit:  5000,
		UsedThisMonth: 0,
		ResetAt:       time.Now().Add(24 * time.Hour),
	})

	_ = h.svc.Spend(ctx, creditTestTenant, 10, domain.CreditActionEligibilityCheck, "ref-a")
	_ = h.svc.Spend(ctx, creditTestTenant, 20, domain.CreditActionEnrichment, "ref-b")
	_ = h.svc.Spend(ctx, creditTestTenant, 30, domain.CreditActionAssessment, "ref-c")

	txns, err := h.svc.GetTransactions(ctx, creditTestTenant, 2)
	if err != nil {
		t.Fatalf("GetTransactions returned error: %v", err)
	}
	if len(txns) != 2 {
		t.Errorf("expected 2 transactions (limited), got %d", len(txns))
	}
}

func TestCreditService_GetTransactions_Error(t *testing.T) {
	h := newCreditTestHarness()
	h.txns.listErr = fmt.Errorf("list failed")

	_, err := h.svc.GetTransactions(context.Background(), creditTestTenant, 10)
	if err == nil {
		t.Fatal("expected error from GetTransactions, got nil")
	}
}

// ---------------------------------------------------------------------------
// Domain-level tests
// ---------------------------------------------------------------------------

func TestCreditTier_MonthlyCredits(t *testing.T) {
	tests := []struct {
		tier     domain.CreditTier
		expected int
	}{
		{domain.CreditTierFree, 500},
		{domain.CreditTierStarter, 5000},
		{domain.CreditTierGrowth, 25000},
		{domain.CreditTierScale, 100000},
		{domain.CreditTier("unknown"), 500},
	}

	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			got := tt.tier.MonthlyCredits()
			if got != tt.expected {
				t.Errorf("CreditTier(%q).MonthlyCredits() = %d, want %d", tt.tier, got, tt.expected)
			}
		})
	}
}

func TestCreditAccount_Remaining(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		used     int
		expected int
	}{
		{"normal usage", 5000, 3000, 2000},
		{"zero used", 5000, 0, 5000},
		{"fully used", 5000, 5000, 0},
		{"overdrawn returns zero", 5000, 6000, 0},
		{"zero limit zero used", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := &domain.CreditAccount{
				MonthlyLimit:  tt.limit,
				UsedThisMonth: tt.used,
			}
			got := acc.Remaining()
			if got != tt.expected {
				t.Errorf("Remaining() = %d, want %d (limit=%d, used=%d)", got, tt.expected, tt.limit, tt.used)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration-like: full credit lifecycle
// ---------------------------------------------------------------------------

func TestCreditService_FullLifecycle(t *testing.T) {
	h := newCreditTestHarness()
	ctx := context.Background()

	// 1. Ensure account
	if err := h.svc.EnsureAccount(ctx, creditTestTenant, domain.CreditTierStarter); err != nil {
		t.Fatalf("EnsureAccount: %v", err)
	}

	// 2. Check balance
	acc, err := h.svc.GetBalance(ctx, creditTestTenant)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if acc.Remaining() != 5000 {
		t.Fatalf("expected 5000 remaining, got %d", acc.Remaining())
	}

	// 3. Spend some
	if err := h.svc.Spend(ctx, creditTestTenant, 2000, domain.CreditActionDiscoveryScan, "scan-1"); err != nil {
		t.Fatalf("Spend: %v", err)
	}

	// 4. Check balance again
	acc, _ = h.svc.GetBalance(ctx, creditTestTenant)
	if acc.Remaining() != 3000 {
		t.Errorf("expected 3000 remaining after spend, got %d", acc.Remaining())
	}

	// 5. Spend more than remaining fails
	err = h.svc.Spend(ctx, creditTestTenant, 3001, domain.CreditActionDiscoveryScan, "scan-2")
	if err == nil {
		t.Error("expected error spending more than remaining")
	}

	// 6. SpendIfAvailable for the rest
	ok := h.svc.SpendIfAvailable(ctx, creditTestTenant, 3000, domain.CreditActionDiscoveryScan, "scan-3")
	if !ok {
		t.Error("expected SpendIfAvailable to succeed for exact remaining")
	}

	// 7. Now zero credits
	has, _ := h.svc.HasCredits(ctx, creditTestTenant, 1)
	if has {
		t.Error("expected no credits remaining")
	}

	// 8. Grant monthly resets
	if err := h.svc.GrantMonthly(ctx, creditTestTenant); err != nil {
		t.Fatalf("GrantMonthly: %v", err)
	}
	acc, _ = h.svc.GetBalance(ctx, creditTestTenant)
	if acc.Remaining() != 5000 {
		t.Errorf("expected 5000 remaining after grant, got %d", acc.Remaining())
	}

	// 9. Upgrade tier
	if err := h.svc.UpdateTier(ctx, creditTestTenant, domain.CreditTierGrowth); err != nil {
		t.Fatalf("UpdateTier: %v", err)
	}
	acc, _ = h.svc.GetBalance(ctx, creditTestTenant)
	if acc.MonthlyLimit != 25000 {
		t.Errorf("expected monthly limit 25000 after upgrade, got %d", acc.MonthlyLimit)
	}

	// 10. Verify transaction history
	txns, _ := h.svc.GetTransactions(ctx, creditTestTenant, 100)
	// scan-1 spend + scan-3 spend + monthly grant = 3 (scan-2 failed)
	if len(txns) != 3 {
		t.Errorf("expected 3 transactions in history, got %d", len(txns))
	}
}

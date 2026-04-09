package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// CreditService manages credit balances, spending, and grants.
type CreditService struct {
	accounts     port.CreditAccountRepo
	transactions port.CreditTransactionRepo
	idGen        port.IDGenerator
}

func NewCreditService(accounts port.CreditAccountRepo, transactions port.CreditTransactionRepo, idGen port.IDGenerator) *CreditService {
	return &CreditService{accounts: accounts, transactions: transactions, idGen: idGen}
}

// EnsureAccount creates a credit account if it doesn't exist.
func (s *CreditService) EnsureAccount(ctx context.Context, tenantID domain.TenantID, tier domain.CreditTier) error {
	return s.accounts.EnsureExists(ctx, tenantID, tier)
}

// GetBalance returns the current credit account state.
func (s *CreditService) GetBalance(ctx context.Context, tenantID domain.TenantID) (*domain.CreditAccount, error) {
	account, err := s.accounts.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	// Check if monthly reset is due
	if time.Now().After(account.ResetAt) {
		if err := s.accounts.ResetMonthly(ctx, tenantID); err != nil {
			slog.Warn("credits: failed to reset monthly", "tenant_id", tenantID, "error", err)
		}
		refreshed, err := s.accounts.Get(ctx, tenantID)
		if err != nil {
			return account, nil // return pre-reset data on re-fetch failure
		}
		account = refreshed
	}
	return account, nil
}

// HasCredits checks if the tenant has enough credits.
func (s *CreditService) HasCredits(ctx context.Context, tenantID domain.TenantID, amount int) (bool, error) {
	account, err := s.GetBalance(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return account.Remaining() >= amount, nil
}

// Spend deducts credits and records a transaction. Returns error if insufficient.
// The atomic Debit SQL enforces the balance check, so no pre-check is needed.
func (s *CreditService) Spend(ctx context.Context, tenantID domain.TenantID, amount int, action domain.CreditAction, reference string) error {
	if err := s.accounts.Debit(ctx, tenantID, amount); err != nil {
		return fmt.Errorf("insufficient credits or debit failed: %w", err)
	}

	tx := &domain.CreditTransaction{
		ID:        s.idGen.New(),
		TenantID:  tenantID,
		Amount:    -amount,
		Action:    action,
		Reference: reference,
		CreatedAt: time.Now(),
	}
	if err := s.transactions.Record(ctx, tx); err != nil {
		slog.Warn("credits: failed to record transaction", "tenant_id", tenantID, "error", err)
	}

	slog.Info("credits: spent", "tenant_id", tenantID, "amount", amount, "action", action, "reference", reference)
	return nil
}

// SpendIfAvailable tries to spend credits but doesn't fail if insufficient.
// Returns true if credits were spent, false if not enough.
func (s *CreditService) SpendIfAvailable(ctx context.Context, tenantID domain.TenantID, amount int, action domain.CreditAction, reference string) bool {
	if err := s.Spend(ctx, tenantID, amount, action, reference); err != nil {
		return false
	}
	return true
}

// GrantMonthly resets credits for a tenant based on their tier.
func (s *CreditService) GrantMonthly(ctx context.Context, tenantID domain.TenantID) error {
	if err := s.accounts.ResetMonthly(ctx, tenantID); err != nil {
		return err
	}

	account, err := s.accounts.Get(ctx, tenantID)
	if err != nil {
		return err
	}

	tx := &domain.CreditTransaction{
		ID:        s.idGen.New(),
		TenantID:  tenantID,
		Amount:    account.MonthlyLimit,
		Action:    domain.CreditActionMonthlyGrant,
		Reference: fmt.Sprintf("monthly_reset_%s", time.Now().Format("2006-01")),
		CreatedAt: time.Now(),
	}
	return s.transactions.Record(ctx, tx)
}

// GetTransactions returns recent credit transactions for a tenant.
func (s *CreditService) GetTransactions(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.CreditTransaction, error) {
	return s.transactions.ListByTenant(ctx, tenantID, limit)
}

// UpdateTier changes a tenant's credit tier.
func (s *CreditService) UpdateTier(ctx context.Context, tenantID domain.TenantID, tier domain.CreditTier) error {
	return s.accounts.UpdateTier(ctx, tenantID, tier)
}

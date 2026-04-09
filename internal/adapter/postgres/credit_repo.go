package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type CreditAccountRepo struct {
	pool *pgxpool.Pool
}

func NewCreditAccountRepo(pool *pgxpool.Pool) *CreditAccountRepo {
	return &CreditAccountRepo{pool: pool}
}

func (r *CreditAccountRepo) Get(ctx context.Context, tenantID domain.TenantID) (*domain.CreditAccount, error) {
	var a domain.CreditAccount
	err := r.pool.QueryRow(ctx, `
		SELECT tenant_id, tier, monthly_limit, used_this_month, reset_at
		FROM credit_accounts WHERE tenant_id = $1
	`, tenantID).Scan(&a.TenantID, &a.Tier, &a.MonthlyLimit, &a.UsedThisMonth, &a.ResetAt)
	if err != nil {
		return nil, fmt.Errorf("get credit account: %w", err)
	}
	return &a, nil
}

func (r *CreditAccountRepo) EnsureExists(ctx context.Context, tenantID domain.TenantID, tier domain.CreditTier) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO credit_accounts (tenant_id, tier, monthly_limit, used_this_month, reset_at)
		VALUES ($1, $2, $3, 0, date_trunc('month', now()) + interval '1 month')
		ON CONFLICT (tenant_id) DO NOTHING
	`, tenantID, tier, tier.MonthlyCredits())
	if err != nil {
		return fmt.Errorf("ensure credit account: %w", err)
	}
	return nil
}

func (r *CreditAccountRepo) Debit(ctx context.Context, tenantID domain.TenantID, amount int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE credit_accounts SET used_this_month = used_this_month + $2 WHERE tenant_id = $1
	`, tenantID, amount)
	if err != nil {
		return fmt.Errorf("debit credits: %w", err)
	}
	return nil
}

func (r *CreditAccountRepo) ResetMonthly(ctx context.Context, tenantID domain.TenantID) error {
	nextReset := time.Now().AddDate(0, 1, 0)
	nextReset = time.Date(nextReset.Year(), nextReset.Month(), 1, 0, 0, 0, 0, time.UTC)
	_, err := r.pool.Exec(ctx, `
		UPDATE credit_accounts SET used_this_month = 0, reset_at = $2 WHERE tenant_id = $1
	`, tenantID, nextReset)
	if err != nil {
		return fmt.Errorf("reset monthly credits: %w", err)
	}
	return nil
}

func (r *CreditAccountRepo) UpdateTier(ctx context.Context, tenantID domain.TenantID, tier domain.CreditTier) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE credit_accounts SET tier = $2, monthly_limit = $3 WHERE tenant_id = $1
	`, tenantID, tier, tier.MonthlyCredits())
	if err != nil {
		return fmt.Errorf("update tier: %w", err)
	}
	return nil
}

// CreditTransactionRepo

type CreditTransactionRepo struct {
	pool *pgxpool.Pool
}

func NewCreditTransactionRepo(pool *pgxpool.Pool) *CreditTransactionRepo {
	return &CreditTransactionRepo{pool: pool}
}

func (r *CreditTransactionRepo) Record(ctx context.Context, tx *domain.CreditTransaction) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO credit_transactions (id, tenant_id, amount, action, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, tx.ID, tx.TenantID, tx.Amount, tx.Action, tx.Reference, tx.CreatedAt)
	if err != nil {
		return fmt.Errorf("record credit transaction: %w", err)
	}
	return nil
}

func (r *CreditTransactionRepo) ListByTenant(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.CreditTransaction, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, amount, action, reference, created_at
		FROM credit_transactions WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []domain.CreditTransaction
	for rows.Next() {
		var tx domain.CreditTransaction
		if err := rows.Scan(&tx.ID, &tx.TenantID, &tx.Amount, &tx.Action, &tx.Reference, &tx.CreatedAt); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

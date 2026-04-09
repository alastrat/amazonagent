package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type TenantEligibilityRepo struct {
	pool *pgxpool.Pool
}

func NewTenantEligibilityRepo(pool *pgxpool.Pool) *TenantEligibilityRepo {
	return &TenantEligibilityRepo{pool: pool}
}

func (r *TenantEligibilityRepo) Set(ctx context.Context, e *domain.TenantEligibility) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tenant_product_eligibility (tenant_id, asin, eligible, reason, checked_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, asin) DO UPDATE SET eligible = $3, reason = $4, checked_at = $5
	`, e.TenantID, e.ASIN, e.Eligible, e.Reason, e.CheckedAt)
	if err != nil {
		return fmt.Errorf("set tenant eligibility: %w", err)
	}
	return nil
}

func (r *TenantEligibilityRepo) SetBatch(ctx context.Context, eligibilities []domain.TenantEligibility) error {
	for i := range eligibilities {
		if err := r.Set(ctx, &eligibilities[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *TenantEligibilityRepo) Get(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.TenantEligibility, error) {
	var e domain.TenantEligibility
	err := r.pool.QueryRow(ctx, `
		SELECT tenant_id, asin, eligible, reason, checked_at
		FROM tenant_product_eligibility WHERE tenant_id = $1 AND asin = $2
	`, tenantID, asin).Scan(&e.TenantID, &e.ASIN, &e.Eligible, &e.Reason, &e.CheckedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *TenantEligibilityRepo) GetByASINs(ctx context.Context, tenantID domain.TenantID, asins []string) ([]domain.TenantEligibility, error) {
	if len(asins) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(asins))
	args := []any{tenantID}
	for i, asin := range asins {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, asin)
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT tenant_id, asin, eligible, reason, checked_at
		FROM tenant_product_eligibility WHERE tenant_id = $1 AND asin IN (%s)
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.TenantEligibility
	for rows.Next() {
		var e domain.TenantEligibility
		if err := rows.Scan(&e.TenantID, &e.ASIN, &e.Eligible, &e.Reason, &e.CheckedAt); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

func (r *TenantEligibilityRepo) ListEligible(ctx context.Context, tenantID domain.TenantID, category string, limit int) ([]domain.TenantEligibility, error) {
	query := `
		SELECT tpe.tenant_id, tpe.asin, tpe.eligible, tpe.reason, tpe.checked_at
		FROM tenant_product_eligibility tpe
		JOIN product_catalog pc ON tpe.asin = pc.asin
		WHERE tpe.tenant_id = $1 AND tpe.eligible = true
	`
	args := []any{tenantID}
	argN := 2
	if category != "" {
		query += fmt.Sprintf(" AND pc.category ILIKE $%d", argN)
		args = append(args, "%"+category+"%")
		argN++
	}
	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.TenantEligibility
	for rows.Next() {
		var e domain.TenantEligibility
		if err := rows.Scan(&e.TenantID, &e.ASIN, &e.Eligible, &e.Reason, &e.CheckedAt); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

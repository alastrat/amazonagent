package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type SellerProfileRepo struct {
	pool *pgxpool.Pool
}

func NewSellerProfileRepo(pool *pgxpool.Pool) *SellerProfileRepo {
	return &SellerProfileRepo{pool: pool}
}

func (r *SellerProfileRepo) Create(ctx context.Context, p *domain.SellerProfile) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO seller_profiles (id, tenant_id, archetype, account_age_days, active_listings, stated_capital, assessment_status, assessed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id) DO UPDATE SET
			archetype = EXCLUDED.archetype,
			account_age_days = EXCLUDED.account_age_days,
			active_listings = EXCLUDED.active_listings,
			stated_capital = EXCLUDED.stated_capital,
			assessment_status = EXCLUDED.assessment_status,
			assessed_at = EXCLUDED.assessed_at,
			updated_at = EXCLUDED.updated_at
	`, p.ID, p.TenantID, p.Archetype, p.AccountAgeDays, p.ActiveListings, p.StatedCapital, p.AssessmentStatus, p.AssessedAt, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create seller profile: %w", err)
	}
	return nil
}

func (r *SellerProfileRepo) Get(ctx context.Context, tenantID domain.TenantID) (*domain.SellerProfile, error) {
	var p domain.SellerProfile
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, archetype, account_age_days, active_listings, COALESCE(stated_capital, 0),
			assessment_status, assessed_at, created_at, updated_at
		FROM seller_profiles WHERE tenant_id = $1
	`, tenantID).Scan(&p.ID, &p.TenantID, &p.Archetype, &p.AccountAgeDays, &p.ActiveListings, &p.StatedCapital,
		&p.AssessmentStatus, &p.AssessedAt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get seller profile: %w", err)
	}
	return &p, nil
}

func (r *SellerProfileRepo) Update(ctx context.Context, p *domain.SellerProfile) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE seller_profiles SET archetype = $2, account_age_days = $3, active_listings = $4,
			stated_capital = $5, assessment_status = $6, assessed_at = $7, updated_at = $8
		WHERE tenant_id = $1
	`, p.TenantID, p.Archetype, p.AccountAgeDays, p.ActiveListings, p.StatedCapital, p.AssessmentStatus, p.AssessedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update seller profile: %w", err)
	}
	return nil
}

func (r *SellerProfileRepo) Delete(ctx context.Context, tenantID domain.TenantID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM seller_profiles WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return fmt.Errorf("delete seller profile: %w", err)
	}
	return nil
}

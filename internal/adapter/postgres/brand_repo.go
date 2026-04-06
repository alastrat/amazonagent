package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type BrandRepo struct {
	pool *pgxpool.Pool
}

func NewBrandRepo(pool *pgxpool.Pool) *BrandRepo {
	return &BrandRepo{pool: pool}
}

// GetOrCreateBrand finds a brand by normalized name or creates it.
func (r *BrandRepo) GetOrCreateBrand(ctx context.Context, brandName string) (*domain.Brand, error) {
	normalized := strings.ToLower(strings.TrimSpace(brandName))
	if normalized == "" {
		return nil, fmt.Errorf("empty brand name")
	}

	var b domain.Brand
	err := r.pool.QueryRow(ctx,
		"SELECT id, name, normalized_name, created_at FROM brands WHERE normalized_name = $1",
		normalized).Scan(&b.ID, &b.Name, &b.NormalizedName, &b.CreatedAt)
	if err == nil {
		return &b, nil
	}

	// Create new brand
	err = r.pool.QueryRow(ctx,
		`INSERT INTO brands (name, normalized_name, created_at) VALUES ($1, $2, $3)
		 ON CONFLICT (normalized_name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id, name, normalized_name, created_at`,
		brandName, normalized, time.Now()).Scan(&b.ID, &b.Name, &b.NormalizedName, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create brand: %w", err)
	}
	return &b, nil
}

// GetEligibility returns the cached eligibility for a brand+tenant.
func (r *BrandRepo) GetEligibility(ctx context.Context, tenantID domain.TenantID, brandID domain.BrandID) (*domain.BrandEligibility, error) {
	var be domain.BrandEligibility
	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, brand_id, status, reason, sample_asin, checked_at
		 FROM brand_eligibility WHERE tenant_id = $1 AND brand_id = $2`,
		tenantID, brandID).Scan(&be.ID, &be.TenantID, &be.BrandID, &be.Status, &be.Reason, &be.SampleASIN, &be.CheckedAt)
	if err != nil {
		return nil, err
	}
	return &be, nil
}

// SetEligibility creates or updates eligibility cache for a brand+tenant.
func (r *BrandRepo) SetEligibility(ctx context.Context, be *domain.BrandEligibility) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO brand_eligibility (id, tenant_id, brand_id, status, reason, sample_asin, checked_at)
		 VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
		 ON CONFLICT (tenant_id, brand_id) DO UPDATE
		 SET status = $3, reason = $4, sample_asin = $5, checked_at = $6`,
		be.TenantID, be.BrandID, be.Status, be.Reason, be.SampleASIN, be.CheckedAt)
	if err != nil {
		return fmt.Errorf("set eligibility: %w", err)
	}
	return nil
}

// GetStaleEligibilities returns brands that need re-checking (older than maxAge).
func (r *BrandRepo) GetStaleEligibilities(ctx context.Context, tenantID domain.TenantID, maxAge time.Duration) ([]domain.BrandEligibility, error) {
	cutoff := time.Now().Add(-maxAge)
	rows, err := r.pool.Query(ctx,
		`SELECT be.id, be.tenant_id, be.brand_id, be.status, be.reason, be.sample_asin, be.checked_at
		 FROM brand_eligibility be WHERE be.tenant_id = $1 AND be.checked_at < $2`,
		tenantID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.BrandEligibility
	for rows.Next() {
		var be domain.BrandEligibility
		if err := rows.Scan(&be.ID, &be.TenantID, &be.BrandID, &be.Status, &be.Reason, &be.SampleASIN, &be.CheckedAt); err != nil {
			return nil, err
		}
		results = append(results, be)
	}
	return results, nil
}

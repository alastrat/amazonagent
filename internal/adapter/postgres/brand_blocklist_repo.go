package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type BrandBlocklistRepo struct {
	pool *pgxpool.Pool
}

func NewBrandBlocklistRepo(pool *pgxpool.Pool) *BrandBlocklistRepo {
	return &BrandBlocklistRepo{pool: pool}
}

func (r *BrandBlocklistRepo) List(ctx context.Context, tenantID domain.TenantID) ([]domain.BlockedBrand, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT id, tenant_id, brand, reason, source, asin, created_at FROM brand_blocklist WHERE tenant_id = $1 ORDER BY brand",
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("list blocked brands: %w", err)
	}
	defer rows.Close()

	var brands []domain.BlockedBrand
	for rows.Next() {
		var b domain.BlockedBrand
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Brand, &b.Reason, &b.Source, &b.ASIN, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan blocked brand: %w", err)
		}
		brands = append(brands, b)
	}
	return brands, nil
}

func (r *BrandBlocklistRepo) Add(ctx context.Context, b *domain.BlockedBrand) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO brand_blocklist (id, tenant_id, brand, reason, source, asin, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (tenant_id, brand) DO UPDATE SET reason = $4, source = $5, asin = $6`,
		b.ID, b.TenantID, strings.ToLower(b.Brand), b.Reason, b.Source, b.ASIN, b.CreatedAt)
	if err != nil {
		return fmt.Errorf("add blocked brand: %w", err)
	}
	return nil
}

func (r *BrandBlocklistRepo) Remove(ctx context.Context, tenantID domain.TenantID, brand string) error {
	_, err := r.pool.Exec(ctx,
		"DELETE FROM brand_blocklist WHERE tenant_id = $1 AND brand = $2",
		tenantID, strings.ToLower(brand))
	if err != nil {
		return fmt.Errorf("remove blocked brand: %w", err)
	}
	return nil
}

func (r *BrandBlocklistRepo) Exists(ctx context.Context, tenantID domain.TenantID, brand string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM brand_blocklist WHERE tenant_id = $1 AND brand = $2)",
		tenantID, strings.ToLower(brand)).Scan(&exists)
	return exists, err
}

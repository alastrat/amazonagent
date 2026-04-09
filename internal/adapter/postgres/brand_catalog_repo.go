package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type BrandCatalogRepo struct {
	pool *pgxpool.Pool
}

func NewBrandCatalogRepo(pool *pgxpool.Pool) *BrandCatalogRepo {
	return &BrandCatalogRepo{pool: pool}
}

func (r *BrandCatalogRepo) Upsert(ctx context.Context, b *domain.SharedBrand) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO brand_catalog (name, normalized_name, typical_gating, categories, created_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (normalized_name) DO UPDATE SET
			name = COALESCE(NULLIF(EXCLUDED.name, ''), brand_catalog.name),
			categories = ARRAY(SELECT DISTINCT unnest(brand_catalog.categories || EXCLUDED.categories))
	`, b.Name, b.NormalizedName, b.TypicalGating, b.Categories)
	if err != nil {
		return fmt.Errorf("upsert brand catalog: %w", err)
	}
	return nil
}

func (r *BrandCatalogRepo) GetByName(ctx context.Context, name string) (*domain.SharedBrand, error) {
	normalized := domain.NormalizeBrandName(name)
	var b domain.SharedBrand
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, normalized_name, typical_gating, categories, product_count, created_at
		FROM brand_catalog WHERE normalized_name = $1
	`, normalized).Scan(&b.ID, &b.Name, &b.NormalizedName, &b.TypicalGating, &b.Categories, &b.ProductCount, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BrandCatalogRepo) ListByCategory(ctx context.Context, category string) ([]domain.SharedBrand, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, normalized_name, typical_gating, categories, product_count, created_at
		FROM brand_catalog WHERE $1 = ANY(categories)
		ORDER BY product_count DESC
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var brands []domain.SharedBrand
	for rows.Next() {
		var b domain.SharedBrand
		if err := rows.Scan(&b.ID, &b.Name, &b.NormalizedName, &b.TypicalGating, &b.Categories, &b.ProductCount, &b.CreatedAt); err != nil {
			return nil, err
		}
		brands = append(brands, b)
	}
	return brands, nil
}

func (r *BrandCatalogRepo) UpdateGating(ctx context.Context, normalizedName string, gating string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE brand_catalog SET typical_gating = $2 WHERE normalized_name = $1
	`, normalizedName, gating)
	return err
}

func (r *BrandCatalogRepo) IncrementProductCount(ctx context.Context, normalizedName string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE brand_catalog SET product_count = product_count + 1 WHERE normalized_name = $1
	`, normalizedName)
	return err
}

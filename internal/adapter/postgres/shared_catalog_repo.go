package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type SharedCatalogRepo struct {
	pool *pgxpool.Pool
}

func NewSharedCatalogRepo(pool *pgxpool.Pool) *SharedCatalogRepo {
	return &SharedCatalogRepo{pool: pool}
}

func (r *SharedCatalogRepo) UpsertProduct(ctx context.Context, p *domain.SharedProduct) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO product_catalog (asin, title, brand, category, bsr_rank, seller_count, buy_box_price, estimated_margin_pct, image_url, last_enriched_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (asin) DO UPDATE SET
			title = COALESCE(NULLIF(EXCLUDED.title, ''), product_catalog.title),
			brand = COALESCE(NULLIF(EXCLUDED.brand, ''), product_catalog.brand),
			category = COALESCE(NULLIF(EXCLUDED.category, ''), product_catalog.category),
			bsr_rank = CASE WHEN EXCLUDED.bsr_rank > 0 THEN EXCLUDED.bsr_rank ELSE product_catalog.bsr_rank END,
			seller_count = CASE WHEN EXCLUDED.seller_count > 0 THEN EXCLUDED.seller_count ELSE product_catalog.seller_count END,
			buy_box_price = CASE WHEN EXCLUDED.buy_box_price > 0 THEN EXCLUDED.buy_box_price ELSE product_catalog.buy_box_price END,
			estimated_margin_pct = CASE WHEN EXCLUDED.estimated_margin_pct != 0 THEN EXCLUDED.estimated_margin_pct ELSE product_catalog.estimated_margin_pct END,
			last_enriched_at = COALESCE(EXCLUDED.last_enriched_at, product_catalog.last_enriched_at)
	`, p.ASIN, p.Title, p.Brand, p.Category, p.BSRRank, p.SellerCount, p.BuyBoxPrice, p.EstimatedMargin, p.ImageURL, p.LastEnrichedAt, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert shared product: %w", err)
	}
	return nil
}

func (r *SharedCatalogRepo) UpsertProductBatch(ctx context.Context, products []domain.SharedProduct) error {
	for i := range products {
		if err := r.UpsertProduct(ctx, &products[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *SharedCatalogRepo) GetByASIN(ctx context.Context, asin string) (*domain.SharedProduct, error) {
	var p domain.SharedProduct
	err := r.pool.QueryRow(ctx, `
		SELECT asin, title, brand, category, COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(buy_box_price, 0), COALESCE(estimated_margin_pct, 0), COALESCE(image_url, ''),
			last_enriched_at, enrichment_count, created_at
		FROM product_catalog WHERE asin = $1
	`, asin).Scan(&p.ASIN, &p.Title, &p.Brand, &p.Category, &p.BSRRank, &p.SellerCount,
		&p.BuyBoxPrice, &p.EstimatedMargin, &p.ImageURL, &p.LastEnrichedAt, &p.EnrichmentCount, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *SharedCatalogRepo) GetByASINs(ctx context.Context, asins []string) ([]domain.SharedProduct, error) {
	if len(asins) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(asins))
	args := make([]any, len(asins))
	for i, asin := range asins {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = asin
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT asin, title, brand, category, COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(buy_box_price, 0), COALESCE(estimated_margin_pct, 0), COALESCE(image_url, ''),
			last_enriched_at, enrichment_count, created_at
		FROM product_catalog WHERE asin IN (%s)
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []domain.SharedProduct
	for rows.Next() {
		var p domain.SharedProduct
		if err := rows.Scan(&p.ASIN, &p.Title, &p.Brand, &p.Category, &p.BSRRank, &p.SellerCount,
			&p.BuyBoxPrice, &p.EstimatedMargin, &p.ImageURL, &p.LastEnrichedAt, &p.EnrichmentCount, &p.CreatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *SharedCatalogRepo) GetStale(ctx context.Context, olderThan time.Time, limit int) ([]domain.SharedProduct, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT asin, title, brand, category, COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(buy_box_price, 0), COALESCE(estimated_margin_pct, 0), COALESCE(image_url, ''),
			last_enriched_at, enrichment_count, created_at
		FROM product_catalog
		WHERE last_enriched_at IS NULL OR last_enriched_at < $1
		ORDER BY last_enriched_at NULLS FIRST LIMIT $2
	`, olderThan, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []domain.SharedProduct
	for rows.Next() {
		var p domain.SharedProduct
		if err := rows.Scan(&p.ASIN, &p.Title, &p.Brand, &p.Category, &p.BSRRank, &p.SellerCount,
			&p.BuyBoxPrice, &p.EstimatedMargin, &p.ImageURL, &p.LastEnrichedAt, &p.EnrichmentCount, &p.CreatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *SharedCatalogRepo) SearchByCategory(ctx context.Context, category string, limit int) ([]domain.SharedProduct, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT asin, title, brand, category, COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(buy_box_price, 0), COALESCE(estimated_margin_pct, 0), COALESCE(image_url, ''),
			last_enriched_at, enrichment_count, created_at
		FROM product_catalog WHERE category ILIKE $1
		ORDER BY enrichment_count DESC LIMIT $2
	`, "%"+category+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []domain.SharedProduct
	for rows.Next() {
		var p domain.SharedProduct
		if err := rows.Scan(&p.ASIN, &p.Title, &p.Brand, &p.Category, &p.BSRRank, &p.SellerCount,
			&p.BuyBoxPrice, &p.EstimatedMargin, &p.ImageURL, &p.LastEnrichedAt, &p.EnrichmentCount, &p.CreatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *SharedCatalogRepo) IncrementEnrichment(ctx context.Context, asin string) error {
	_, err := r.pool.Exec(ctx, `UPDATE product_catalog SET enrichment_count = enrichment_count + 1 WHERE asin = $1`, asin)
	return err
}

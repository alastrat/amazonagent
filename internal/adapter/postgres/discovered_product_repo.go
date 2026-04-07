package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type DiscoveredProductRepo struct {
	pool *pgxpool.Pool
}

func NewDiscoveredProductRepo(pool *pgxpool.Pool) *DiscoveredProductRepo {
	return &DiscoveredProductRepo{pool: pool}
}

func (r *DiscoveredProductRepo) Upsert(ctx context.Context, p *domain.DiscoveredProduct) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO discovered_products (
			id, tenant_id, asin, title, brand_id, category, browse_node_id,
			estimated_price, buy_box_price, bsr_rank, seller_count,
			estimated_margin_pct, real_margin_pct, eligibility_status,
			data_quality, refresh_priority, source, first_seen_at, last_seen_at, price_updated_at
		) VALUES (
			COALESCE(NULLIF($1, ''), gen_random_uuid()::text),
			$2, $3, $4, NULLIF($5, '')::uuid, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14,
			$15, $16, $17, $18, $19, $20
		)
		ON CONFLICT (tenant_id, asin) DO UPDATE SET
			title = COALESCE(NULLIF(EXCLUDED.title, ''), discovered_products.title),
			brand_id = COALESCE(EXCLUDED.brand_id, discovered_products.brand_id),
			category = COALESCE(NULLIF(EXCLUDED.category, ''), discovered_products.category),
			browse_node_id = COALESCE(NULLIF(EXCLUDED.browse_node_id, ''), discovered_products.browse_node_id),
			estimated_price = CASE WHEN EXCLUDED.estimated_price > 0 THEN EXCLUDED.estimated_price ELSE discovered_products.estimated_price END,
			buy_box_price = CASE WHEN EXCLUDED.buy_box_price > 0 THEN EXCLUDED.buy_box_price ELSE discovered_products.buy_box_price END,
			bsr_rank = CASE WHEN EXCLUDED.bsr_rank > 0 THEN EXCLUDED.bsr_rank ELSE discovered_products.bsr_rank END,
			seller_count = CASE WHEN EXCLUDED.seller_count > 0 THEN EXCLUDED.seller_count ELSE discovered_products.seller_count END,
			estimated_margin_pct = CASE WHEN EXCLUDED.estimated_margin_pct != 0 THEN EXCLUDED.estimated_margin_pct ELSE discovered_products.estimated_margin_pct END,
			real_margin_pct = CASE WHEN EXCLUDED.real_margin_pct != 0 THEN EXCLUDED.real_margin_pct ELSE discovered_products.real_margin_pct END,
			eligibility_status = CASE WHEN EXCLUDED.eligibility_status != 'unknown' THEN EXCLUDED.eligibility_status ELSE discovered_products.eligibility_status END,
			data_quality = discovered_products.data_quality | EXCLUDED.data_quality,
			last_seen_at = now(),
			price_updated_at = CASE WHEN EXCLUDED.buy_box_price > 0 THEN now() ELSE discovered_products.price_updated_at END
	`, p.ID, p.TenantID, p.ASIN, p.Title, p.BrandID, p.Category, p.BrowseNodeID,
		p.EstimatedPrice, p.BuyBoxPrice, p.BSRRank, p.SellerCount,
		p.EstimatedMarginPct, p.RealMarginPct, p.EligibilityStatus,
		p.DataQuality, p.RefreshPriority, p.Source, p.FirstSeenAt, p.LastSeenAt, p.PriceUpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert discovered product: %w", err)
	}
	return nil
}

func (r *DiscoveredProductRepo) UpsertBatch(ctx context.Context, products []domain.DiscoveredProduct) error {
	for i := range products {
		if err := r.Upsert(ctx, &products[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *DiscoveredProductRepo) GetByASIN(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.DiscoveredProduct, error) {
	var p domain.DiscoveredProduct
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, asin, title, COALESCE(brand_id::text, ''), category, COALESCE(browse_node_id, ''),
			COALESCE(estimated_price, 0), COALESCE(buy_box_price, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(estimated_margin_pct, 0), COALESCE(real_margin_pct, 0), eligibility_status,
			data_quality, refresh_priority, source, first_seen_at, last_seen_at, price_updated_at
		FROM discovered_products WHERE tenant_id = $1 AND asin = $2
	`, tenantID, asin).Scan(
		&p.ID, &p.TenantID, &p.ASIN, &p.Title, &p.BrandID, &p.Category, &p.BrowseNodeID,
		&p.EstimatedPrice, &p.BuyBoxPrice, &p.BSRRank, &p.SellerCount,
		&p.EstimatedMarginPct, &p.RealMarginPct, &p.EligibilityStatus,
		&p.DataQuality, &p.RefreshPriority, &p.Source, &p.FirstSeenAt, &p.LastSeenAt, &p.PriceUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get discovered product: %w", err)
	}
	return &p, nil
}

func (r *DiscoveredProductRepo) GetByASINs(ctx context.Context, tenantID domain.TenantID, asins []string) ([]domain.DiscoveredProduct, error) {
	if len(asins) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(asins))
	args := []any{tenantID}
	for i, asin := range asins {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, asin)
	}
	query := fmt.Sprintf(`
		SELECT id, tenant_id, asin, title, COALESCE(brand_id::text, ''), category, COALESCE(browse_node_id, ''),
			COALESCE(estimated_price, 0), COALESCE(buy_box_price, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(estimated_margin_pct, 0), COALESCE(real_margin_pct, 0), eligibility_status,
			data_quality, refresh_priority, source, first_seen_at, last_seen_at, price_updated_at
		FROM discovered_products WHERE tenant_id = $1 AND asin IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get discovered products by ASINs: %w", err)
	}
	defer rows.Close()

	var products []domain.DiscoveredProduct
	for rows.Next() {
		var p domain.DiscoveredProduct
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ASIN, &p.Title, &p.BrandID, &p.Category, &p.BrowseNodeID,
			&p.EstimatedPrice, &p.BuyBoxPrice, &p.BSRRank, &p.SellerCount,
			&p.EstimatedMarginPct, &p.RealMarginPct, &p.EligibilityStatus,
			&p.DataQuality, &p.RefreshPriority, &p.Source, &p.FirstSeenAt, &p.LastSeenAt, &p.PriceUpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *DiscoveredProductRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.DiscoveredProductFilter) ([]domain.DiscoveredProduct, int, error) {
	where := "WHERE tenant_id = $1"
	args := []any{tenantID}
	argN := 2

	if filter.Category != nil {
		where += fmt.Sprintf(" AND category = $%d", argN)
		args = append(args, *filter.Category)
		argN++
	}
	if filter.BrandID != nil {
		where += fmt.Sprintf(" AND brand_id = $%d::uuid", argN)
		args = append(args, *filter.BrandID)
		argN++
	}
	if filter.MinMargin != nil {
		where += fmt.Sprintf(" AND estimated_margin_pct >= $%d", argN)
		args = append(args, *filter.MinMargin)
		argN++
	}
	if filter.MinSellers != nil {
		where += fmt.Sprintf(" AND seller_count >= $%d", argN)
		args = append(args, *filter.MinSellers)
		argN++
	}
	if filter.EligibilityStatus != nil {
		where += fmt.Sprintf(" AND eligibility_status = $%d", argN)
		args = append(args, *filter.EligibilityStatus)
		argN++
	}
	if filter.Source != nil {
		where += fmt.Sprintf(" AND source = $%d", argN)
		args = append(args, *filter.Source)
		argN++
	}
	if filter.Search != nil && *filter.Search != "" {
		where += fmt.Sprintf(" AND (title ILIKE $%d OR asin ILIKE $%d)", argN, argN)
		args = append(args, "%"+*filter.Search+"%")
		argN++
	}

	// Count
	var total int
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM discovered_products "+where, args...).Scan(&total)

	// Sort
	sortBy := "last_seen_at"
	if filter.SortBy != "" {
		allowed := map[string]bool{"estimated_margin_pct": true, "real_margin_pct": true, "bsr_rank": true, "seller_count": true, "last_seen_at": true, "estimated_price": true, "buy_box_price": true}
		if allowed[filter.SortBy] {
			sortBy = filter.SortBy
		}
	}
	sortDir := "DESC"
	if filter.SortDir == "asc" {
		sortDir = "ASC"
	}

	limit := 50
	if filter.Limit > 0 && filter.Limit <= 200 {
		limit = filter.Limit
	}
	offset := filter.Offset

	query := fmt.Sprintf(`
		SELECT id, tenant_id, asin, title, COALESCE(brand_id::text, ''), category, COALESCE(browse_node_id, ''),
			COALESCE(estimated_price, 0), COALESCE(buy_box_price, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(estimated_margin_pct, 0), COALESCE(real_margin_pct, 0), eligibility_status,
			data_quality, refresh_priority, source, first_seen_at, last_seen_at, price_updated_at
		FROM discovered_products %s ORDER BY %s %s LIMIT %d OFFSET %d
	`, where, sortBy, sortDir, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list discovered products: %w", err)
	}
	defer rows.Close()

	var products []domain.DiscoveredProduct
	for rows.Next() {
		var p domain.DiscoveredProduct
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ASIN, &p.Title, &p.BrandID, &p.Category, &p.BrowseNodeID,
			&p.EstimatedPrice, &p.BuyBoxPrice, &p.BSRRank, &p.SellerCount,
			&p.EstimatedMarginPct, &p.RealMarginPct, &p.EligibilityStatus,
			&p.DataQuality, &p.RefreshPriority, &p.Source, &p.FirstSeenAt, &p.LastSeenAt, &p.PriceUpdatedAt); err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, nil
}

func (r *DiscoveredProductRepo) ListStale(ctx context.Context, tenantID domain.TenantID, olderThan time.Time, limit int) ([]domain.DiscoveredProduct, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, asin, title, COALESCE(brand_id::text, ''), category, COALESCE(browse_node_id, ''),
			COALESCE(estimated_price, 0), COALESCE(buy_box_price, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			COALESCE(estimated_margin_pct, 0), COALESCE(real_margin_pct, 0), eligibility_status,
			data_quality, refresh_priority, source, first_seen_at, last_seen_at, price_updated_at
		FROM discovered_products
		WHERE tenant_id = $1 AND (price_updated_at IS NULL OR price_updated_at < $2)
		ORDER BY refresh_priority DESC LIMIT $3
	`, tenantID, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("list stale products: %w", err)
	}
	defer rows.Close()

	var products []domain.DiscoveredProduct
	for rows.Next() {
		var p domain.DiscoveredProduct
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ASIN, &p.Title, &p.BrandID, &p.Category, &p.BrowseNodeID,
			&p.EstimatedPrice, &p.BuyBoxPrice, &p.BSRRank, &p.SellerCount,
			&p.EstimatedMarginPct, &p.RealMarginPct, &p.EligibilityStatus,
			&p.DataQuality, &p.RefreshPriority, &p.Source, &p.FirstSeenAt, &p.LastSeenAt, &p.PriceUpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *DiscoveredProductRepo) ListByRefreshPriority(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoveredProduct, error) {
	return r.ListStale(ctx, tenantID, time.Now(), limit)
}

func (r *DiscoveredProductRepo) UpdatePricing(ctx context.Context, tenantID domain.TenantID, asin string, buyBoxPrice float64, sellers int, bsr int, realMarginPct float64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE discovered_products SET
			buy_box_price = $3, seller_count = $4, bsr_rank = $5, real_margin_pct = $6,
			data_quality = data_quality | $7,
			price_updated_at = now(), last_seen_at = now()
		WHERE tenant_id = $1 AND asin = $2
	`, tenantID, asin, buyBoxPrice, sellers, bsr, realMarginPct, domain.DataQualityBuyBox)
	if err != nil {
		return fmt.Errorf("update pricing: %w", err)
	}
	return nil
}

func (r *DiscoveredProductRepo) UpdateRefreshPriority(ctx context.Context, tenantID domain.TenantID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE discovered_products SET refresh_priority =
			(COALESCE(estimated_margin_pct, 0) / 100.0) * 0.4
			+ (EXTRACT(EPOCH FROM now() - COALESCE(price_updated_at, first_seen_at)) / 86400.0) * 0.3
			+ (CASE WHEN seller_count BETWEEN 3 AND 10 THEN 0.3 ELSE 0.1 END)
		WHERE tenant_id = $1
	`, tenantID)
	if err != nil {
		return fmt.Errorf("update refresh priority: %w", err)
	}
	return nil
}

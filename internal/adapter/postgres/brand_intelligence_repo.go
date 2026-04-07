package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type BrandIntelligenceRepo struct {
	pool *pgxpool.Pool
}

func NewBrandIntelligenceRepo(pool *pgxpool.Pool) *BrandIntelligenceRepo {
	return &BrandIntelligenceRepo{pool: pool}
}

func (r *BrandIntelligenceRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.BrandIntelligenceFilter) ([]domain.BrandIntelligence, error) {
	where := "WHERE tenant_id = $1"
	args := []any{tenantID}
	argN := 2

	if filter.Category != nil {
		where += fmt.Sprintf(" AND category = $%d", argN)
		args = append(args, *filter.Category)
		argN++
	}
	if filter.MinMargin != nil {
		where += fmt.Sprintf(" AND avg_margin >= $%d", argN)
		args = append(args, *filter.MinMargin)
		argN++
	}
	if filter.MinProducts != nil {
		where += fmt.Sprintf(" AND product_count >= $%d", argN)
		args = append(args, *filter.MinProducts)
		argN++
	}
	if filter.Search != nil && *filter.Search != "" {
		where += fmt.Sprintf(" AND brand_name ILIKE $%d", argN)
		args = append(args, "%"+*filter.Search+"%")
		argN++
	}

	sortBy := "avg_margin"
	if filter.SortBy != "" {
		allowed := map[string]bool{"brand_name": true, "category": true, "product_count": true, "high_margin_count": true, "avg_margin": true, "avg_sellers": true, "avg_bsr": true}
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
		SELECT tenant_id, COALESCE(brand_id::text, ''), brand_name, category,
			product_count, high_margin_count,
			COALESCE(avg_margin, 0), COALESCE(avg_sellers, 0), COALESCE(avg_bsr, 0)
		FROM brand_intelligence %s ORDER BY %s %s LIMIT %d OFFSET %d
	`, where, sortBy, sortDir, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list brand intelligence: %w", err)
	}
	defer rows.Close()

	var brands []domain.BrandIntelligence
	for rows.Next() {
		var b domain.BrandIntelligence
		if err := rows.Scan(
			&b.TenantID, &b.BrandID, &b.BrandName, &b.Category,
			&b.ProductCount, &b.HighMarginCount,
			&b.AvgMargin, &b.AvgSellers, &b.AvgBSR); err != nil {
			return nil, err
		}
		brands = append(brands, b)
	}
	return brands, nil
}

func (r *BrandIntelligenceRepo) Refresh(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY brand_intelligence")
	if err != nil {
		return fmt.Errorf("refresh brand intelligence: %w", err)
	}
	return nil
}

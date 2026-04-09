package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type SuggestionRepo struct {
	pool *pgxpool.Pool
}

func NewSuggestionRepo(pool *pgxpool.Pool) *SuggestionRepo {
	return &SuggestionRepo{pool: pool}
}

func (r *SuggestionRepo) Create(ctx context.Context, s *domain.DiscoverySuggestion) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO discovery_suggestions (id, tenant_id, strategy_version_id, goal_id, asin, title, brand, category,
			buy_box_price, estimated_margin_pct, bsr_rank, seller_count, reason, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, s.ID, s.TenantID, s.StrategyVersionID, s.GoalID, s.ASIN, s.Title, s.Brand, s.Category,
		s.BuyBoxPrice, s.EstimatedMargin, s.BSRRank, s.SellerCount, s.Reason, s.Status, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create suggestion: %w", err)
	}
	return nil
}

func (r *SuggestionRepo) CreateBatch(ctx context.Context, suggestions []domain.DiscoverySuggestion) error {
	for i := range suggestions {
		if err := r.Create(ctx, &suggestions[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *SuggestionRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.SuggestionID) (*domain.DiscoverySuggestion, error) {
	var s domain.DiscoverySuggestion
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, strategy_version_id, COALESCE(goal_id, ''), asin, title, brand, category,
			COALESCE(buy_box_price, 0), COALESCE(estimated_margin_pct, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			reason, status, deal_id, created_at, resolved_at
		FROM discovery_suggestions WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(&s.ID, &s.TenantID, &s.StrategyVersionID, &s.GoalID, &s.ASIN, &s.Title, &s.Brand, &s.Category,
		&s.BuyBoxPrice, &s.EstimatedMargin, &s.BSRRank, &s.SellerCount,
		&s.Reason, &s.Status, &s.DealID, &s.CreatedAt, &s.ResolvedAt)
	if err != nil {
		return nil, fmt.Errorf("get suggestion: %w", err)
	}
	return &s, nil
}

func (r *SuggestionRepo) ListPending(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error) {
	return r.listByStatus(ctx, tenantID, "pending", limit)
}

func (r *SuggestionRepo) ListAll(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, strategy_version_id, COALESCE(goal_id, ''), asin, title, brand, category,
			COALESCE(buy_box_price, 0), COALESCE(estimated_margin_pct, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			reason, status, deal_id, created_at, resolved_at
		FROM discovery_suggestions WHERE tenant_id = $1
		ORDER BY created_at DESC LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *SuggestionRepo) Accept(ctx context.Context, id domain.SuggestionID, dealID domain.DealID) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE discovery_suggestions SET status = 'accepted', deal_id = $2, resolved_at = $3 WHERE id = $1
	`, id, dealID, now)
	return err
}

func (r *SuggestionRepo) Dismiss(ctx context.Context, id domain.SuggestionID) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE discovery_suggestions SET status = 'dismissed', resolved_at = $2 WHERE id = $1
	`, id, now)
	return err
}

func (r *SuggestionRepo) CountToday(ctx context.Context, tenantID domain.TenantID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM discovery_suggestions
		WHERE tenant_id = $1 AND created_at >= date_trunc('day', now())
	`, tenantID).Scan(&count)
	return count, err
}

func (r *SuggestionRepo) listByStatus(ctx context.Context, tenantID domain.TenantID, status string, limit int) ([]domain.DiscoverySuggestion, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, strategy_version_id, COALESCE(goal_id, ''), asin, title, brand, category,
			COALESCE(buy_box_price, 0), COALESCE(estimated_margin_pct, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0),
			reason, status, deal_id, created_at, resolved_at
		FROM discovery_suggestions WHERE tenant_id = $1 AND status = $2
		ORDER BY created_at DESC LIMIT $3
	`, tenantID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *SuggestionRepo) scanRows(rows interface{ Next() bool; Scan(...any) error }) ([]domain.DiscoverySuggestion, error) {
	var suggestions []domain.DiscoverySuggestion
	for rows.Next() {
		var s domain.DiscoverySuggestion
		if err := rows.Scan(&s.ID, &s.TenantID, &s.StrategyVersionID, &s.GoalID, &s.ASIN, &s.Title, &s.Brand, &s.Category,
			&s.BuyBoxPrice, &s.EstimatedMargin, &s.BSRRank, &s.SellerCount,
			&s.Reason, &s.Status, &s.DealID, &s.CreatedAt, &s.ResolvedAt); err != nil {
			return nil, err
		}
		suggestions = append(suggestions, s)
	}
	return suggestions, nil
}

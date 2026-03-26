package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type DealRepo struct {
	pool *pgxpool.Pool
}

func NewDealRepo(pool *pgxpool.Pool) *DealRepo {
	return &DealRepo{pool: pool}
}

func (r *DealRepo) Create(ctx context.Context, d *domain.Deal) error {
	scores, _ := json.Marshal(d.Scores)
	evidence, _ := json.Marshal(d.Evidence)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO deals (id, tenant_id, campaign_id, asin, title, brand, category, status, scores, evidence, reviewer_verdict, iteration_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, d.ID, d.TenantID, d.CampaignID, d.ASIN, d.Title, d.Brand, d.Category, d.Status, scores, evidence, d.ReviewerVerdict, d.IterationCount, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert deal: %w", err)
	}
	return nil
}

func (r *DealRepo) CreateBatch(ctx context.Context, deals []domain.Deal) error {
	for _, d := range deals {
		if err := r.Create(ctx, &d); err != nil {
			return err
		}
	}
	return nil
}

func (r *DealRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.DealID) (*domain.Deal, error) {
	var d domain.Deal
	var scoresJSON, evidenceJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, campaign_id, asin, title, brand, category, status, scores, evidence, reviewer_verdict, iteration_count, supplier_id, listing_id, created_at, updated_at
		FROM deals WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(
		&d.ID, &d.TenantID, &d.CampaignID, &d.ASIN, &d.Title, &d.Brand, &d.Category, &d.Status,
		&scoresJSON, &evidenceJSON, &d.ReviewerVerdict, &d.IterationCount,
		&d.SupplierID, &d.ListingID, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get deal: %w", err)
	}
	json.Unmarshal(scoresJSON, &d.Scores)
	json.Unmarshal(evidenceJSON, &d.Evidence)
	return &d, nil
}

func (r *DealRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.DealFilter) ([]domain.Deal, int, error) {
	countQuery := "SELECT COUNT(*) FROM deals WHERE tenant_id = $1"
	countArgs := []any{tenantID}
	argIdx := 2

	if filter.CampaignID != nil {
		countQuery += fmt.Sprintf(" AND campaign_id = $%d", argIdx)
		countArgs = append(countArgs, *filter.CampaignID)
		argIdx++
	}
	if filter.Status != nil {
		countQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		countArgs = append(countArgs, *filter.Status)
		argIdx++
	}
	if filter.Search != nil {
		countQuery += fmt.Sprintf(" AND (title ILIKE $%d OR brand ILIKE $%d OR asin ILIKE $%d)", argIdx, argIdx, argIdx)
		countArgs = append(countArgs, "%"+*filter.Search+"%")
		argIdx++
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count deals: %w", err)
	}

	query := `SELECT id, tenant_id, campaign_id, asin, title, brand, category, status, scores, evidence, reviewer_verdict, iteration_count, supplier_id, listing_id, created_at, updated_at
		FROM deals WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if filter.CampaignID != nil {
		query += fmt.Sprintf(" AND campaign_id = $%d", argIdx)
		args = append(args, *filter.CampaignID)
		argIdx++
	}
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Search != nil {
		query += fmt.Sprintf(" AND (title ILIKE $%d OR brand ILIKE $%d OR asin ILIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	sortCol := "created_at"
	switch filter.SortBy {
	case "overall_score":
		sortCol = "(scores->>'overall')::float"
	case "margin":
		sortCol = "(scores->>'margin')::int"
	case "created_at":
		sortCol = "created_at"
	}
	sortDir := "DESC"
	if filter.SortDir == "asc" {
		sortDir = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortCol, sortDir)

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list deals: %w", err)
	}
	defer rows.Close()

	var deals []domain.Deal
	for rows.Next() {
		var d domain.Deal
		var scoresJSON, evidenceJSON []byte
		if err := rows.Scan(&d.ID, &d.TenantID, &d.CampaignID, &d.ASIN, &d.Title, &d.Brand, &d.Category, &d.Status, &scoresJSON, &evidenceJSON, &d.ReviewerVerdict, &d.IterationCount, &d.SupplierID, &d.ListingID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan deal: %w", err)
		}
		json.Unmarshal(scoresJSON, &d.Scores)
		json.Unmarshal(evidenceJSON, &d.Evidence)
		deals = append(deals, d)
	}
	return deals, total, nil
}

func (r *DealRepo) Update(ctx context.Context, d *domain.Deal) error {
	scores, _ := json.Marshal(d.Scores)
	evidence, _ := json.Marshal(d.Evidence)

	_, err := r.pool.Exec(ctx, `
		UPDATE deals SET status = $1, scores = $2, evidence = $3, reviewer_verdict = $4, iteration_count = $5, supplier_id = $6, listing_id = $7, updated_at = $8
		WHERE id = $9 AND tenant_id = $10
	`, d.Status, scores, evidence, d.ReviewerVerdict, d.IterationCount, d.SupplierID, d.ListingID, d.UpdatedAt, d.ID, d.TenantID)
	if err != nil {
		return fmt.Errorf("update deal: %w", err)
	}
	return nil
}

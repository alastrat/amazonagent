package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type CampaignRepo struct {
	pool *pgxpool.Pool
}

func NewCampaignRepo(pool *pgxpool.Pool) *CampaignRepo {
	return &CampaignRepo{pool: pool}
}

func (r *CampaignRepo) Create(ctx context.Context, c *domain.Campaign) error {
	criteria, err := json.Marshal(c.Criteria)
	if err != nil {
		return fmt.Errorf("marshal criteria: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO campaigns (id, tenant_id, type, criteria, scoring_config_id, experiment_id, source_file, status, created_by, trigger_type, created_at, completed_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9, $10, $11, $12)
	`, c.ID, c.TenantID, c.Type, string(criteria), c.ScoringConfigID, c.ExperimentID, c.SourceFile, c.Status, c.CreatedBy, c.TriggerType, c.CreatedAt, c.CompletedAt)
	if err != nil {
		return fmt.Errorf("insert campaign: %w", err)
	}
	return nil
}

func (r *CampaignRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.CampaignID) (*domain.Campaign, error) {
	var c domain.Campaign
	var criteriaJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, type, criteria, scoring_config_id, experiment_id, source_file, status, created_by, trigger_type, created_at, completed_at
		FROM campaigns
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(
		&c.ID, &c.TenantID, &c.Type, &criteriaJSON, &c.ScoringConfigID, &c.ExperimentID, &c.SourceFile, &c.Status, &c.CreatedBy, &c.TriggerType, &c.CreatedAt, &c.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get campaign: %w", err)
	}
	if err := json.Unmarshal(criteriaJSON, &c.Criteria); err != nil {
		return nil, fmt.Errorf("unmarshal criteria: %w", err)
	}
	return &c, nil
}

func (r *CampaignRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.CampaignFilter) ([]domain.Campaign, error) {
	query := `SELECT id, tenant_id, type, criteria, scoring_config_id, experiment_id, source_file, status, created_by, trigger_type, created_at, completed_at
		FROM campaigns WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, *filter.Type)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

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
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []domain.Campaign
	for rows.Next() {
		var c domain.Campaign
		var criteriaJSON []byte
		if err := rows.Scan(&c.ID, &c.TenantID, &c.Type, &criteriaJSON, &c.ScoringConfigID, &c.ExperimentID, &c.SourceFile, &c.Status, &c.CreatedBy, &c.TriggerType, &c.CreatedAt, &c.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		if err := json.Unmarshal(criteriaJSON, &c.Criteria); err != nil {
			return nil, fmt.Errorf("unmarshal criteria: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

func (r *CampaignRepo) Update(ctx context.Context, c *domain.Campaign) error {
	criteria, err := json.Marshal(c.Criteria)
	if err != nil {
		return fmt.Errorf("marshal criteria: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE campaigns SET status = $1, completed_at = $2, criteria = $3::jsonb WHERE id = $4 AND tenant_id = $5
	`, c.Status, c.CompletedAt, string(criteria), c.ID, c.TenantID)
	if err != nil {
		return fmt.Errorf("update campaign: %w", err)
	}
	return nil
}

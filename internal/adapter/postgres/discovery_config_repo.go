package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type DiscoveryConfigRepo struct {
	pool *pgxpool.Pool
}

func NewDiscoveryConfigRepo(pool *pgxpool.Pool) *DiscoveryConfigRepo {
	return &DiscoveryConfigRepo{pool: pool}
}

func (r *DiscoveryConfigRepo) Get(ctx context.Context, tenantID domain.TenantID) (*domain.DiscoveryConfig, error) {
	var dc domain.DiscoveryConfig
	var categoriesJSON, criteriaJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, categories, baseline_criteria, scoring_config_id, cadence, enabled, last_run_at, next_run_at
		FROM discovery_configs WHERE tenant_id = $1
	`, tenantID).Scan(&dc.ID, &dc.TenantID, &categoriesJSON, &criteriaJSON, &dc.ScoringConfigID, &dc.Cadence, &dc.Enabled, &dc.LastRunAt, &dc.NextRunAt)
	if err != nil {
		return nil, fmt.Errorf("get discovery config: %w", err)
	}
	json.Unmarshal(categoriesJSON, &dc.Categories)
	json.Unmarshal(criteriaJSON, &dc.BaselineCriteria)
	return &dc, nil
}

func (r *DiscoveryConfigRepo) Upsert(ctx context.Context, dc *domain.DiscoveryConfig) error {
	categories, _ := json.Marshal(dc.Categories)
	criteria, _ := json.Marshal(dc.BaselineCriteria)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO discovery_configs (id, tenant_id, categories, baseline_criteria, scoring_config_id, cadence, enabled, last_run_at, next_run_at)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id)
		DO UPDATE SET categories = $3::jsonb, baseline_criteria = $4::jsonb, scoring_config_id = $5, cadence = $6, enabled = $7, last_run_at = $8, next_run_at = $9
	`, dc.ID, dc.TenantID, string(categories), string(criteria), dc.ScoringConfigID, dc.Cadence, dc.Enabled, dc.LastRunAt, dc.NextRunAt)
	if err != nil {
		return fmt.Errorf("upsert discovery config: %w", err)
	}
	return nil
}

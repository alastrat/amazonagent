package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type ScoringConfigRepo struct {
	pool *pgxpool.Pool
}

func NewScoringConfigRepo(pool *pgxpool.Pool) *ScoringConfigRepo {
	return &ScoringConfigRepo{pool: pool}
}

func (r *ScoringConfigRepo) Create(ctx context.Context, sc *domain.ScoringConfig) error {
	weights, _ := json.Marshal(sc.Weights)
	thresholds, _ := json.Marshal(sc.Thresholds)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO scoring_configs (id, tenant_id, version, weights, thresholds, created_by, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sc.ID, sc.TenantID, sc.Version, weights, thresholds, sc.CreatedBy, sc.Active, sc.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert scoring config: %w", err)
	}
	return nil
}

func (r *ScoringConfigRepo) GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.ScoringConfig, error) {
	var sc domain.ScoringConfig
	var weightsJSON, thresholdsJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, version, weights, thresholds, created_by, active, created_at
		FROM scoring_configs WHERE tenant_id = $1 AND active = true
	`, tenantID).Scan(&sc.ID, &sc.TenantID, &sc.Version, &weightsJSON, &thresholdsJSON, &sc.CreatedBy, &sc.Active, &sc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get active scoring config: %w", err)
	}
	json.Unmarshal(weightsJSON, &sc.Weights)
	json.Unmarshal(thresholdsJSON, &sc.Thresholds)
	return &sc, nil
}

func (r *ScoringConfigRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) (*domain.ScoringConfig, error) {
	var sc domain.ScoringConfig
	var weightsJSON, thresholdsJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, version, weights, thresholds, created_by, active, created_at
		FROM scoring_configs WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(&sc.ID, &sc.TenantID, &sc.Version, &weightsJSON, &thresholdsJSON, &sc.CreatedBy, &sc.Active, &sc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get scoring config: %w", err)
	}
	json.Unmarshal(weightsJSON, &sc.Weights)
	json.Unmarshal(thresholdsJSON, &sc.Thresholds)
	return &sc, nil
}

func (r *ScoringConfigRepo) SetActive(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "UPDATE scoring_configs SET active = false WHERE tenant_id = $1 AND active = true", tenantID)
	if err != nil {
		return fmt.Errorf("deactivate old: %w", err)
	}

	_, err = tx.Exec(ctx, "UPDATE scoring_configs SET active = true WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return fmt.Errorf("activate new: %w", err)
	}

	return tx.Commit(ctx)
}

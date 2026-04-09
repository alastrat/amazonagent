package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type StrategyVersionRepo struct {
	pool *pgxpool.Pool
}

func NewStrategyVersionRepo(pool *pgxpool.Pool) *StrategyVersionRepo {
	return &StrategyVersionRepo{pool: pool}
}

func (r *StrategyVersionRepo) Create(ctx context.Context, sv *domain.StrategyVersion) error {
	goalsJSON, _ := json.Marshal(sv.Goals)
	paramsJSON, _ := json.Marshal(sv.SearchParams)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO strategy_versions (id, tenant_id, version_number, goals, search_params, scoring_config_id,
			status, parent_version_id, promoted_from_experiment_id, change_reason, created_by, created_at, activated_at, rolled_back_at)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, sv.ID, sv.TenantID, sv.VersionNumber, string(goalsJSON), string(paramsJSON), sv.ScoringConfigID,
		sv.Status, sv.ParentVersionID, sv.PromotedFromExperimentID, sv.ChangeReason, sv.CreatedBy, sv.CreatedAt, sv.ActivatedAt, sv.RolledBackAt)
	if err != nil {
		return fmt.Errorf("create strategy version: %w", err)
	}
	return nil
}

// versionColumns is the shared SELECT column list for StrategyVersion queries.
const versionColumns = `id, tenant_id, version_number, goals, search_params, COALESCE(scoring_config_id::text, ''),
	status, COALESCE(parent_version_id::text, ''), COALESCE(promoted_from_experiment_id, ''),
	change_reason, created_by, created_at, activated_at, rolled_back_at`

func scanVersion(scanner interface{ Scan(...any) error }) (domain.StrategyVersion, error) {
	var sv domain.StrategyVersion
	var goalsJSON, paramsJSON []byte
	err := scanner.Scan(&sv.ID, &sv.TenantID, &sv.VersionNumber, &goalsJSON, &paramsJSON, &sv.ScoringConfigID,
		&sv.Status, &sv.ParentVersionID, &sv.PromotedFromExperimentID,
		&sv.ChangeReason, &sv.CreatedBy, &sv.CreatedAt, &sv.ActivatedAt, &sv.RolledBackAt)
	if err != nil {
		return sv, err
	}
	json.Unmarshal(goalsJSON, &sv.Goals)
	json.Unmarshal(paramsJSON, &sv.SearchParams)
	return sv, nil
}

func (r *StrategyVersionRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.StrategyVersionID) (*domain.StrategyVersion, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+versionColumns+` FROM strategy_versions WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	sv, err := scanVersion(row)
	if err != nil {
		return nil, fmt.Errorf("get strategy version: %w", err)
	}
	return &sv, nil
}

func (r *StrategyVersionRepo) GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.StrategyVersion, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+versionColumns+` FROM strategy_versions WHERE tenant_id = $1 AND status = `+
		`$2 ORDER BY version_number DESC LIMIT 1`, tenantID, string(domain.StrategyStatusActive))
	sv, err := scanVersion(row)
	if err != nil {
		return nil, fmt.Errorf("get active strategy: %w", err)
	}
	return &sv, nil
}

func (r *StrategyVersionRepo) List(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.StrategyVersion, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `SELECT `+versionColumns+` FROM strategy_versions WHERE tenant_id = $1
		ORDER BY version_number DESC LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []domain.StrategyVersion
	for rows.Next() {
		sv, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, sv)
	}
	return versions, nil
}

func (r *StrategyVersionRepo) NextVersionNumber(ctx context.Context, tenantID domain.TenantID) (int, error) {
	var maxVersion int
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version_number), 0) FROM strategy_versions WHERE tenant_id = $1
	`, tenantID).Scan(&maxVersion)
	if err != nil {
		return 0, err
	}
	return maxVersion + 1, nil
}

func (r *StrategyVersionRepo) SetStatus(ctx context.Context, id domain.StrategyVersionID, status domain.StrategyStatus) error {
	now := time.Now()
	var extra string
	switch status {
	case domain.StrategyStatusActive:
		extra = ", activated_at = $3"
	case domain.StrategyStatusRolledBack:
		extra = ", rolled_back_at = $3"
	default:
		_, err := r.pool.Exec(ctx, `UPDATE strategy_versions SET status = $2 WHERE id = $1`, id, status)
		return err
	}
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`UPDATE strategy_versions SET status = $2%s WHERE id = $1`, extra), id, status, now)
	return err
}

func (r *StrategyVersionRepo) Activate(ctx context.Context, tenantID domain.TenantID, id domain.StrategyVersionID) error {
	// Archive current active
	r.pool.Exec(ctx, `
		UPDATE strategy_versions SET status = $2 WHERE tenant_id = $1 AND status = $3
	`, tenantID, string(domain.StrategyStatusArchived), string(domain.StrategyStatusActive))
	// Activate new
	return r.SetStatus(ctx, id, domain.StrategyStatusActive)
}

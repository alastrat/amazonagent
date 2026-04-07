package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type ScanJobRepo struct {
	pool *pgxpool.Pool
}

func NewScanJobRepo(pool *pgxpool.Pool) *ScanJobRepo {
	return &ScanJobRepo{pool: pool}
}

func (r *ScanJobRepo) Create(ctx context.Context, job *domain.ScanJob) error {
	metadata, _ := json.Marshal(job.Metadata)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scan_jobs (id, tenant_id, type, status, total_items, processed, qualified, eliminated, started_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
	`, job.ID, job.TenantID, job.Type, job.Status, job.TotalItems, job.Processed, job.Qualified, job.Eliminated, job.StartedAt, string(metadata))
	if err != nil {
		return fmt.Errorf("create scan job: %w", err)
	}
	return nil
}

func (r *ScanJobRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.ScanJobID) (*domain.ScanJob, error) {
	var j domain.ScanJob
	var metadataJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, type, status, total_items, processed, qualified, eliminated, started_at, completed_at, metadata
		FROM scan_jobs WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(
		&j.ID, &j.TenantID, &j.Type, &j.Status, &j.TotalItems, &j.Processed, &j.Qualified, &j.Eliminated,
		&j.StartedAt, &j.CompletedAt, &metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("get scan job: %w", err)
	}
	json.Unmarshal(metadataJSON, &j.Metadata)
	return &j, nil
}

func (r *ScanJobRepo) List(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.ScanJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, type, status, total_items, processed, qualified, eliminated, started_at, completed_at, metadata
		FROM scan_jobs WHERE tenant_id = $1 ORDER BY started_at DESC LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list scan jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.ScanJob
	for rows.Next() {
		var j domain.ScanJob
		var metadataJSON []byte
		if err := rows.Scan(
			&j.ID, &j.TenantID, &j.Type, &j.Status, &j.TotalItems, &j.Processed, &j.Qualified, &j.Eliminated,
			&j.StartedAt, &j.CompletedAt, &metadataJSON); err != nil {
			return nil, err
		}
		json.Unmarshal(metadataJSON, &j.Metadata)
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (r *ScanJobRepo) UpdateProgress(ctx context.Context, id domain.ScanJobID, processed, qualified, eliminated int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE scan_jobs SET processed = $2, qualified = $3, eliminated = $4
		WHERE id = $1
	`, id, processed, qualified, eliminated)
	if err != nil {
		return fmt.Errorf("update scan progress: %w", err)
	}
	return nil
}

func (r *ScanJobRepo) Complete(ctx context.Context, id domain.ScanJobID) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE scan_jobs SET status = 'completed', completed_at = $2 WHERE id = $1
	`, id, now)
	if err != nil {
		return fmt.Errorf("complete scan job: %w", err)
	}
	return nil
}

func (r *ScanJobRepo) Fail(ctx context.Context, id domain.ScanJobID) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE scan_jobs SET status = 'failed', completed_at = $2 WHERE id = $1
	`, id, now)
	if err != nil {
		return fmt.Errorf("fail scan job: %w", err)
	}
	return nil
}

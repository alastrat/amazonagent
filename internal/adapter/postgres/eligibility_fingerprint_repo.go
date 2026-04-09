package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type EligibilityFingerprintRepo struct {
	pool *pgxpool.Pool
}

func NewEligibilityFingerprintRepo(pool *pgxpool.Pool) *EligibilityFingerprintRepo {
	return &EligibilityFingerprintRepo{pool: pool}
}

func (r *EligibilityFingerprintRepo) Create(ctx context.Context, fp *domain.EligibilityFingerprint) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO eligibility_fingerprints (id, tenant_id, total_probes, total_eligible, total_restricted, overall_open_rate, confidence, assessed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id) DO UPDATE SET
			total_probes = EXCLUDED.total_probes,
			total_eligible = EXCLUDED.total_eligible,
			total_restricted = EXCLUDED.total_restricted,
			overall_open_rate = EXCLUDED.overall_open_rate,
			confidence = EXCLUDED.confidence,
			assessed_at = EXCLUDED.assessed_at
	`, fp.ID, fp.TenantID, fp.TotalProbes, fp.TotalEligible, fp.TotalRestricted, fp.OverallOpenRate, fp.Confidence, fp.AssessedAt)
	if err != nil {
		return fmt.Errorf("create eligibility fingerprint: %w", err)
	}
	return nil
}

func (r *EligibilityFingerprintRepo) Get(ctx context.Context, tenantID domain.TenantID) (*domain.EligibilityFingerprint, error) {
	var fp domain.EligibilityFingerprint
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, total_probes, total_eligible, total_restricted, overall_open_rate, confidence, assessed_at
		FROM eligibility_fingerprints WHERE tenant_id = $1
	`, tenantID).Scan(&fp.ID, &fp.TenantID, &fp.TotalProbes, &fp.TotalEligible, &fp.TotalRestricted, &fp.OverallOpenRate, &fp.Confidence, &fp.AssessedAt)
	if err != nil {
		return nil, fmt.Errorf("get eligibility fingerprint: %w", err)
	}

	// Load categories
	catRows, err := r.pool.Query(ctx, `
		SELECT category, probe_count, open_count, gated_count, open_rate
		FROM category_eligibilities WHERE tenant_id = $1
	`, tenantID)
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var ce domain.CategoryEligibility
			if err := catRows.Scan(&ce.Category, &ce.ProbeCount, &ce.OpenCount, &ce.GatedCount, &ce.OpenRate); err != nil {
				return nil, fmt.Errorf("scan category eligibility: %w", err)
			}
			fp.Categories = append(fp.Categories, ce)
		}
	}

	// Load brand results
	brandRows, err := r.pool.Query(ctx, `
		SELECT asin, brand, category, tier, eligible, reason
		FROM assessment_probe_results WHERE tenant_id = $1
	`, tenantID)
	if err == nil {
		defer brandRows.Close()
		for brandRows.Next() {
			var br domain.BrandProbeResult
			if err := brandRows.Scan(&br.ASIN, &br.Brand, &br.Category, &br.Tier, &br.Eligible, &br.Reason); err != nil {
				return nil, fmt.Errorf("scan brand probe result: %w", err)
			}
			fp.BrandResults = append(fp.BrandResults, br)
		}
	}

	return &fp, nil
}

func (r *EligibilityFingerprintRepo) SaveProbeResults(ctx context.Context, fingerprintID string, tenantID domain.TenantID, results []domain.BrandProbeResult) error {
	rows := make([][]any, len(results))
	for i, res := range results {
		rows[i] = []any{fingerprintID, tenantID, res.ASIN, res.Brand, res.Category, res.Tier, res.Eligible, res.Reason}
	}
	return BatchInsert(ctx, r.pool, "assessment_probe_results",
		[]string{"fingerprint_id", "tenant_id", "asin", "brand", "category", "tier", "eligible", "reason"}, rows)
}

func (r *EligibilityFingerprintRepo) SaveCategoryEligibilities(ctx context.Context, fingerprintID string, tenantID domain.TenantID, categories []domain.CategoryEligibility) error {
	rows := make([][]any, len(categories))
	for i, cat := range categories {
		rows[i] = []any{fingerprintID, tenantID, cat.Category, cat.ProbeCount, cat.OpenCount, cat.GatedCount, cat.OpenRate}
	}
	return BatchInsert(ctx, r.pool, "category_eligibilities",
		[]string{"fingerprint_id", "tenant_id", "category", "probe_count", "open_count", "gated_count", "open_rate"}, rows)
}


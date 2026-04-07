package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type PriceHistoryRepo struct {
	pool *pgxpool.Pool
}

func NewPriceHistoryRepo(pool *pgxpool.Pool) *PriceHistoryRepo {
	return &PriceHistoryRepo{pool: pool}
}

func (r *PriceHistoryRepo) Record(ctx context.Context, snapshot domain.PriceSnapshot) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO price_history (asin, tenant_id, recorded_at, amazon_price, bsr_rank, seller_count)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, snapshot.ASIN, snapshot.TenantID, snapshot.RecordedAt, snapshot.AmazonPrice, snapshot.BSRRank, snapshot.SellerCount)
	if err != nil {
		return fmt.Errorf("record price snapshot: %w", err)
	}
	return nil
}

func (r *PriceHistoryRepo) RecordBatch(ctx context.Context, snapshots []domain.PriceSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	// Use a batch insert for efficiency
	batch := &pgxpool.Pool{}
	_ = batch // using simple loop for now — COPY optimization can come later
	for _, s := range snapshots {
		if err := r.Record(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (r *PriceHistoryRepo) GetHistory(ctx context.Context, tenantID domain.TenantID, asin string, since time.Time) ([]domain.PriceSnapshot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT asin, tenant_id, recorded_at, COALESCE(amazon_price, 0), COALESCE(bsr_rank, 0), COALESCE(seller_count, 0)
		FROM price_history
		WHERE tenant_id = $1 AND asin = $2 AND recorded_at >= $3
		ORDER BY recorded_at DESC
	`, tenantID, asin, since)
	if err != nil {
		return nil, fmt.Errorf("get price history: %w", err)
	}
	defer rows.Close()

	var snapshots []domain.PriceSnapshot
	for rows.Next() {
		var s domain.PriceSnapshot
		if err := rows.Scan(&s.ASIN, &s.TenantID, &s.RecordedAt, &s.AmazonPrice, &s.BSRRank, &s.SellerCount); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

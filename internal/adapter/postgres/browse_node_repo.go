package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type BrowseNodeRepo struct {
	pool *pgxpool.Pool
}

func NewBrowseNodeRepo(pool *pgxpool.Pool) *BrowseNodeRepo {
	return &BrowseNodeRepo{pool: pool}
}

func (r *BrowseNodeRepo) Upsert(ctx context.Context, node *domain.BrowseNode) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO browse_nodes (amazon_node_id, name, parent_node_id, depth, is_leaf, scan_priority)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (amazon_node_id) DO UPDATE SET
			name = EXCLUDED.name,
			parent_node_id = EXCLUDED.parent_node_id,
			depth = EXCLUDED.depth,
			is_leaf = EXCLUDED.is_leaf,
			scan_priority = EXCLUDED.scan_priority
	`, node.AmazonNodeID, node.Name, node.ParentNodeID, node.Depth, node.IsLeaf, node.ScanPriority)
	if err != nil {
		return fmt.Errorf("upsert browse node: %w", err)
	}
	return nil
}

func (r *BrowseNodeRepo) UpsertBatch(ctx context.Context, nodes []domain.BrowseNode) error {
	for i := range nodes {
		if err := r.Upsert(ctx, &nodes[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *BrowseNodeRepo) GetNextForScan(ctx context.Context, limit int) ([]domain.BrowseNode, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, amazon_node_id, name, COALESCE(parent_node_id, ''), depth, is_leaf, last_scanned_at, products_found, scan_priority
		FROM browse_nodes
		WHERE is_leaf = true
		ORDER BY last_scanned_at NULLS FIRST, scan_priority DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get next browse nodes: %w", err)
	}
	defer rows.Close()

	var nodes []domain.BrowseNode
	for rows.Next() {
		var n domain.BrowseNode
		if err := rows.Scan(&n.ID, &n.AmazonNodeID, &n.Name, &n.ParentNodeID, &n.Depth, &n.IsLeaf, &n.LastScannedAt, &n.ProductsFound, &n.ScanPriority); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (r *BrowseNodeRepo) MarkScanned(ctx context.Context, amazonNodeID string, productsFound int) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE browse_nodes SET last_scanned_at = $2, products_found = $3
		WHERE amazon_node_id = $1
	`, amazonNodeID, now, productsFound)
	if err != nil {
		return fmt.Errorf("mark scanned: %w", err)
	}
	return nil
}

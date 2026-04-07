package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// CategoryScanService orchestrates background category scanning.
// It picks browse nodes from the rotation queue, searches SP-API for products,
// and runs them through the tiered funnel.
type CategoryScanService struct {
	nodes   port.BrowseNodeRepo
	catalog *CatalogService
	funnel  *FunnelService
	spapi   port.ProductSearcher
	scans   port.ScanJobRepo
	idGen   port.IDGenerator
}

func NewCategoryScanService(
	nodes port.BrowseNodeRepo,
	catalog *CatalogService,
	funnel *FunnelService,
	spapi port.ProductSearcher,
	scans port.ScanJobRepo,
	idGen port.IDGenerator,
) *CategoryScanService {
	return &CategoryScanService{
		nodes:   nodes,
		catalog: catalog,
		funnel:  funnel,
		spapi:   spapi,
		scans:   scans,
		idGen:   idGen,
	}
}

// ScanNextNodes picks the next N browse nodes from the rotation queue and scans them.
// For each node, searches SP-API (up to 200 products via pagination), converts to
// FunnelInput, and runs through T0-T3.
func (s *CategoryScanService) ScanNextNodes(
	ctx context.Context,
	tenantID domain.TenantID,
	maxNodes int,
	thresholds domain.PipelineThresholds,
) (*domain.ScanJob, error) {
	if maxNodes <= 0 {
		maxNodes = 100
	}

	// Pick nodes
	nodes, err := s.nodes.GetNextForScan(ctx, maxNodes)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		slog.Info("category-scan: no browse nodes to scan")
		return nil, nil
	}

	slog.Info("category-scan: starting", "nodes", len(nodes), "tenant_id", tenantID)

	// Create scan job
	job := &domain.ScanJob{
		ID:       domain.ScanJobID(s.idGen.New()),
		TenantID: tenantID,
		Type:     domain.ScanTypeCategory,
		Status:   "running",
		StartedAt: time.Now(),
		Metadata: map[string]any{
			"nodes_count": len(nodes),
		},
	}
	if s.scans != nil {
		s.scans.Create(ctx, job)
	}

	var allInputs []FunnelInput
	totalProductsFound := 0

	// Scan each node
	for _, node := range nodes {
		products, err := s.scanNode(ctx, node, "US")
		if err != nil {
			slog.Warn("category-scan: node failed", "node", node.AmazonNodeID, "error", err)
			continue
		}

		// Mark node as scanned
		s.nodes.MarkScanned(ctx, node.AmazonNodeID, len(products))
		totalProductsFound += len(products)

		// Convert to funnel input
		for _, p := range products {
			allInputs = append(allInputs, FunnelInput{
				ASIN:           p.ASIN,
				Title:          p.Title,
				Brand:          p.Brand,
				Category:       p.Category,
				EstimatedPrice: p.AmazonPrice,
				BSRRank:        p.BSRRank,
				SellerCount:    p.SellerCount,
				Source:         domain.ScanTypeCategory,
			})
		}
	}

	slog.Info("category-scan: discovery complete",
		"nodes_scanned", len(nodes),
		"products_found", totalProductsFound,
		"funnel_input", len(allInputs))

	job.TotalItems = totalProductsFound

	if len(allInputs) == 0 {
		if s.scans != nil {
			s.scans.Complete(ctx, job.ID)
		}
		return job, nil
	}

	// Run through funnel (T0-T3)
	survivors, stats, err := s.funnel.ProcessBatch(ctx, tenantID, allInputs, thresholds)
	if err != nil {
		slog.Error("category-scan: funnel failed", "error", err)
		if s.scans != nil {
			s.scans.Fail(ctx, job.ID)
		}
		return job, err
	}

	job.Processed = stats.InputCount
	job.Qualified = stats.SurvivorCount
	job.Eliminated = stats.T1MarginKilled + stats.T2BrandKilled + stats.T3EnrichKilled

	if s.scans != nil {
		s.scans.UpdateProgress(ctx, job.ID, job.Processed, job.Qualified, job.Eliminated)
		s.scans.Complete(ctx, job.ID)
	}

	slog.Info("category-scan: complete",
		"nodes", len(nodes),
		"products", totalProductsFound,
		"survivors", len(survivors),
		"stats", stats)

	return job, nil
}

// scanNode searches a single browse node, paginates through results (max 10 pages = 200 products).
func (s *CategoryScanService) scanNode(ctx context.Context, node domain.BrowseNode, marketplace string) ([]port.ProductSearchResult, error) {
	var allProducts []port.ProductSearchResult
	pageToken := ""
	maxPages := 10

	for page := 0; page < maxPages; page++ {
		products, nextToken, err := s.spapi.SearchByBrowseNode(ctx, node.AmazonNodeID, marketplace, pageToken)
		if err != nil {
			return allProducts, err
		}

		allProducts = append(allProducts, products...)

		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}

	slog.Info("category-scan: node scanned",
		"node", node.AmazonNodeID,
		"name", node.Name,
		"products", len(allProducts))

	return allProducts, nil
}

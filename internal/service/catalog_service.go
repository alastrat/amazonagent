package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// CatalogService manages the persistent product catalog.
// Central point for all product data writes — upserts, pricing updates, snapshots.
type CatalogService struct {
	products port.DiscoveredProductRepo
	prices   port.PriceHistoryRepo
	brands   BrandRepo
	idGen    port.IDGenerator
}

func NewCatalogService(
	products port.DiscoveredProductRepo,
	prices port.PriceHistoryRepo,
	brands BrandRepo,
	idGen port.IDGenerator,
) *CatalogService {
	return &CatalogService{
		products: products,
		prices:   prices,
		brands:   brands,
		idGen:    idGen,
	}
}

// UpsertProducts inserts or updates products from any source.
// Resolves brand_id via the brands table for each product with a brand name.
func (s *CatalogService) UpsertProducts(ctx context.Context, tenantID domain.TenantID, products []domain.DiscoveredProduct) error {
	// Resolve brand IDs for products that have brand names but no brand_id
	brandCache := make(map[string]string) // brand name → brand ID

	for i := range products {
		products[i].TenantID = tenantID
		if products[i].ID == "" {
			products[i].ID = s.idGen.New()
		}
		now := time.Now()
		if products[i].FirstSeenAt.IsZero() {
			products[i].FirstSeenAt = now
		}
		products[i].LastSeenAt = now
	}

	if err := s.products.UpsertBatch(ctx, products); err != nil {
		return err
	}

	slog.Info("catalog: upserted products", "count", len(products), "tenant_id", tenantID, "brand_cache_size", len(brandCache))
	return nil
}

// RecordPriceSnapshots writes price history entries for BI analysis.
func (s *CatalogService) RecordPriceSnapshots(ctx context.Context, snapshots []domain.PriceSnapshot) error {
	return s.prices.RecordBatch(ctx, snapshots)
}

// GetByASINs returns cached products from the catalog. Used for T0 dedup.
func (s *CatalogService) GetByASINs(ctx context.Context, tenantID domain.TenantID, asins []string) ([]domain.DiscoveredProduct, error) {
	return s.products.GetByASINs(ctx, tenantID, asins)
}

// UpdateRefreshPriority recomputes priority scores for all products.
// High margin + stale + competitive seller count = refresh first.
func (s *CatalogService) UpdateRefreshPriority(ctx context.Context, tenantID domain.TenantID) error {
	return s.products.UpdateRefreshPriority(ctx, tenantID)
}

// ListStale returns products that need pricing refresh.
func (s *CatalogService) ListStale(ctx context.Context, tenantID domain.TenantID, maxAge time.Duration, limit int) ([]domain.DiscoveredProduct, error) {
	olderThan := time.Now().Add(-maxAge)
	return s.products.ListStale(ctx, tenantID, olderThan, limit)
}

// UpdatePricing updates a single product's competitive pricing data.
func (s *CatalogService) UpdatePricing(ctx context.Context, tenantID domain.TenantID, asin string, buyBoxPrice float64, sellers int, bsr int, realMarginPct float64) error {
	// Update the catalog entry
	if err := s.products.UpdatePricing(ctx, tenantID, asin, buyBoxPrice, sellers, bsr, realMarginPct); err != nil {
		return err
	}

	// Record price snapshot for history
	snapshot := domain.PriceSnapshot{
		ASIN:        asin,
		TenantID:    tenantID,
		RecordedAt:  time.Now(),
		AmazonPrice: buyBoxPrice,
		BSRRank:     bsr,
		SellerCount: sellers,
	}
	if err := s.prices.Record(ctx, snapshot); err != nil {
		slog.Warn("catalog: failed to record price snapshot", "asin", asin, "error", err)
	}

	return nil
}

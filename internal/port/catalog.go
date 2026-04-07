package port

import (
	"context"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type DiscoveredProductRepo interface {
	Upsert(ctx context.Context, product *domain.DiscoveredProduct) error
	UpsertBatch(ctx context.Context, products []domain.DiscoveredProduct) error
	GetByASIN(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.DiscoveredProduct, error)
	GetByASINs(ctx context.Context, tenantID domain.TenantID, asins []string) ([]domain.DiscoveredProduct, error)
	List(ctx context.Context, tenantID domain.TenantID, filter DiscoveredProductFilter) ([]domain.DiscoveredProduct, int, error)
	ListStale(ctx context.Context, tenantID domain.TenantID, olderThan time.Time, limit int) ([]domain.DiscoveredProduct, error)
	ListByRefreshPriority(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoveredProduct, error)
	UpdatePricing(ctx context.Context, tenantID domain.TenantID, asin string, buyBoxPrice float64, sellers int, bsr int, realMarginPct float64) error
	UpdateRefreshPriority(ctx context.Context, tenantID domain.TenantID) error
}

type DiscoveredProductFilter struct {
	Category          *string
	BrandID           *string
	MinMargin         *float64
	MinSellers        *int
	EligibilityStatus *string
	Source            *domain.ScanType
	Search            *string
	SortBy            string
	SortDir           string
	Limit             int
	Offset            int
}

type PriceHistoryRepo interface {
	Record(ctx context.Context, snapshot domain.PriceSnapshot) error
	RecordBatch(ctx context.Context, snapshots []domain.PriceSnapshot) error
	GetHistory(ctx context.Context, tenantID domain.TenantID, asin string, since time.Time) ([]domain.PriceSnapshot, error)
}

type BrowseNodeRepo interface {
	Upsert(ctx context.Context, node *domain.BrowseNode) error
	UpsertBatch(ctx context.Context, nodes []domain.BrowseNode) error
	GetNextForScan(ctx context.Context, limit int) ([]domain.BrowseNode, error)
	MarkScanned(ctx context.Context, amazonNodeID string, productsFound int) error
}

type ScanJobRepo interface {
	Create(ctx context.Context, job *domain.ScanJob) error
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.ScanJobID) (*domain.ScanJob, error)
	List(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.ScanJob, error)
	UpdateProgress(ctx context.Context, id domain.ScanJobID, processed, qualified, eliminated int) error
	Complete(ctx context.Context, id domain.ScanJobID) error
	Fail(ctx context.Context, id domain.ScanJobID) error
}

type BrandIntelligenceRepo interface {
	List(ctx context.Context, tenantID domain.TenantID, filter BrandIntelligenceFilter) ([]domain.BrandIntelligence, error)
	Refresh(ctx context.Context) error
}

type BrandIntelligenceFilter struct {
	Category    *string
	MinMargin   *float64
	MinProducts *int
	Search      *string
	SortBy      string
	SortDir     string
	Limit       int
	Offset      int
}

type RateLimiter interface {
	Wait(ctx context.Context, endpoint string) error
	ReportThrottle(endpoint string)
}

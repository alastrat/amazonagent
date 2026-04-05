package domain

import "time"

type BlockedBrandID string

type BlockedBrandSource string

const (
	BlockedBrandSourceManual   BlockedBrandSource = "manual"
	BlockedBrandSourcePipeline BlockedBrandSource = "pipeline"
	BlockedBrandSourceImport   BlockedBrandSource = "import"
)

type BlockedBrand struct {
	ID        BlockedBrandID     `json:"id"`
	TenantID  TenantID           `json:"tenant_id"`
	Brand     string             `json:"brand"`
	Reason    string             `json:"reason"`
	Source    BlockedBrandSource `json:"source"`
	ASIN      string             `json:"asin,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

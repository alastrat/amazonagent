package domain

import "time"

// BrowseNode represents an Amazon category node used for background scanning.
type BrowseNode struct {
	ID            string     `json:"id"`
	AmazonNodeID  string     `json:"amazon_node_id"`
	Name          string     `json:"name"`
	ParentNodeID  string     `json:"parent_node_id,omitempty"`
	Depth         int        `json:"depth"`
	IsLeaf        bool       `json:"is_leaf"`
	LastScannedAt *time.Time `json:"last_scanned_at,omitempty"`
	ProductsFound int        `json:"products_found"`
	ScanPriority  float64    `json:"scan_priority"`
}

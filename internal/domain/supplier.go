package domain

import "time"

type SupplierID string

type Supplier struct {
	ID                  SupplierID `json:"id"`
	TenantID            TenantID   `json:"tenant_id"`
	Name                string     `json:"name"`
	Website             string     `json:"website"`
	AuthorizationStatus string     `json:"authorization_status"`
	ReliabilityScore    *float64   `json:"reliability_score,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

package domain

import "time"

// SellerAccountStatus represents the health of stored SP-API credentials.
type SellerAccountStatus string

const (
	SellerAccountStatusPending SellerAccountStatus = "pending"
	SellerAccountStatusValid   SellerAccountStatus = "valid"
	SellerAccountStatusInvalid SellerAccountStatus = "invalid"
	SellerAccountStatusExpired SellerAccountStatus = "expired"
)

// AmazonSellerAccount stores per-tenant SP-API credentials.
// Secrets (SPAPIClientSecret, SPAPIRefreshToken) are encrypted at rest via AES-256-GCM.
type AmazonSellerAccount struct {
	ID                string              `json:"id"`
	TenantID          TenantID            `json:"tenant_id"`
	SPAPIClientID     string              `json:"sp_api_client_id"`
	SPAPIClientSecret string              `json:"-"` // never serialized
	SPAPIRefreshToken string              `json:"-"` // never serialized
	SellerID          string              `json:"seller_id"`
	MarketplaceID     string              `json:"marketplace_id"`
	Status            SellerAccountStatus `json:"status"`
	LastVerified      *time.Time          `json:"last_verified,omitempty"`
	ErrorMessage      string              `json:"error_message,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

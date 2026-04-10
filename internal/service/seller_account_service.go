package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/spapi"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// SellerAccountService manages per-tenant Amazon seller account connections.
type SellerAccountService struct {
	repo  port.SellerAccountRepo
	idGen port.IDGenerator
}

func NewSellerAccountService(repo port.SellerAccountRepo, idGen port.IDGenerator) *SellerAccountService {
	return &SellerAccountService{repo: repo, idGen: idGen}
}

// ConnectAccountInput holds the credentials for connecting an Amazon seller account.
type ConnectAccountInput struct {
	SPAPIClientID     string `json:"sp_api_client_id"`
	SPAPIClientSecret string `json:"sp_api_client_secret"`
	SPAPIRefreshToken string `json:"sp_api_refresh_token"`
	SellerID          string `json:"seller_id"`
	MarketplaceID     string `json:"marketplace_id"`
}

// ConnectAccount validates SP-API credentials by attempting to get an access token,
// then stores them if valid.
func (s *SellerAccountService) ConnectAccount(ctx context.Context, tenantID domain.TenantID, input ConnectAccountInput) (*domain.AmazonSellerAccount, error) {
	if input.SPAPIClientID == "" || input.SPAPIClientSecret == "" || input.SPAPIRefreshToken == "" || input.SellerID == "" {
		return nil, fmt.Errorf("all credential fields are required")
	}

	if input.MarketplaceID == "" {
		input.MarketplaceID = "ATVPDKIKX0DER" // US marketplace default
	}

	// Validate credentials by building a client and attempting a token exchange
	marketplace := marketplaceFromID(input.MarketplaceID)
	testClient := spapi.NewClientFromCredentials(
		input.SPAPIClientID, input.SPAPIClientSecret, input.SPAPIRefreshToken,
		marketplace, input.SellerID,
	)

	status := domain.SellerAccountStatusValid
	errMsg := ""

	// Try to verify by checking listing eligibility with a known ASIN
	// This also validates the seller ID. If it fails, credentials are invalid.
	_, err := testClient.CheckListingEligibility(ctx, []string{"B0CX23V5KK"}, marketplace)
	if err != nil {
		slog.Warn("seller-account: credential validation failed", "tenant_id", tenantID, "error", err)
		status = domain.SellerAccountStatusInvalid
		errMsg = err.Error()
	}

	now := time.Now()
	account := &domain.AmazonSellerAccount{
		ID:                s.idGen.New(),
		TenantID:          tenantID,
		SPAPIClientID:     input.SPAPIClientID,
		SPAPIClientSecret: input.SPAPIClientSecret,
		SPAPIRefreshToken: input.SPAPIRefreshToken,
		SellerID:          input.SellerID,
		MarketplaceID:     input.MarketplaceID,
		Status:            status,
		LastVerified:      &now,
		ErrorMessage:      errMsg,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	// Try to delete existing account first (upsert behavior)
	_ = s.repo.Delete(ctx, tenantID)

	if err := s.repo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("store seller account: %w", err)
	}

	slog.Info("seller-account: connected", "tenant_id", tenantID, "seller_id", input.SellerID, "status", status)
	return account, nil
}

// GetAccount returns the seller account for a tenant, or nil if not connected.
func (s *SellerAccountService) GetAccount(ctx context.Context, tenantID domain.TenantID) (*domain.AmazonSellerAccount, error) {
	account, err := s.repo.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return account, nil
}

// DisconnectAccount removes the seller account connection for a tenant.
func (s *SellerAccountService) DisconnectAccount(ctx context.Context, tenantID domain.TenantID) error {
	if err := s.repo.Delete(ctx, tenantID); err != nil {
		return fmt.Errorf("disconnect seller account: %w", err)
	}
	slog.Info("seller-account: disconnected", "tenant_id", tenantID)
	return nil
}

// BuildSPAPIClient constructs a per-tenant SP-API client from stored credentials.
func (s *SellerAccountService) BuildSPAPIClient(ctx context.Context, tenantID domain.TenantID) (*spapi.Client, error) {
	account, err := s.repo.Get(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load tenant credentials: %w", err)
	}

	if account.Status == domain.SellerAccountStatusInvalid || account.Status == domain.SellerAccountStatusExpired {
		return nil, fmt.Errorf("seller account credentials are %s", account.Status)
	}

	marketplace := marketplaceFromID(account.MarketplaceID)
	return spapi.NewClientFromCredentials(
		account.SPAPIClientID, account.SPAPIClientSecret, account.SPAPIRefreshToken,
		marketplace, account.SellerID,
	), nil
}

// marketplaceFromID converts a marketplace ID to a short code used by the SP-API client.
func marketplaceFromID(mpID string) string {
	switch mpID {
	case "ATVPDKIKX0DER":
		return "US"
	case "A1F83G8C2ARO7P":
		return "UK"
	case "A1PA6795UKMFR9":
		return "EU"
	default:
		return "US"
	}
}

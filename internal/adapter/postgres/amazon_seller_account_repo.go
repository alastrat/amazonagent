package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AmazonSellerAccountRepo struct {
	pool      *pgxpool.Pool
	encryptor *domain.AESEncryptor
}

func NewAmazonSellerAccountRepo(pool *pgxpool.Pool, encryptor *domain.AESEncryptor) *AmazonSellerAccountRepo {
	return &AmazonSellerAccountRepo{pool: pool, encryptor: encryptor}
}

func (r *AmazonSellerAccountRepo) Create(ctx context.Context, account *domain.AmazonSellerAccount) error {
	encSecret, err := r.encryptor.Encrypt(account.SPAPIClientSecret)
	if err != nil {
		return fmt.Errorf("encrypt client secret: %w", err)
	}
	encToken, err := r.encryptor.Encrypt(account.SPAPIRefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO amazon_seller_accounts
			(id, tenant_id, sp_api_client_id, sp_api_client_secret, sp_api_refresh_token,
			 seller_id, marketplace_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, account.ID, account.TenantID, account.SPAPIClientID, encSecret, encToken,
		account.SellerID, account.MarketplaceID, account.Status,
		account.CreatedAt, account.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create amazon seller account: %w", err)
	}
	return nil
}

func (r *AmazonSellerAccountRepo) Get(ctx context.Context, tenantID domain.TenantID) (*domain.AmazonSellerAccount, error) {
	var a domain.AmazonSellerAccount
	var encSecret, encToken string
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, sp_api_client_id, sp_api_client_secret, sp_api_refresh_token,
		       seller_id, marketplace_id, status, last_verified, COALESCE(error_message, ''), created_at, updated_at
		FROM amazon_seller_accounts WHERE tenant_id = $1
	`, tenantID).Scan(
		&a.ID, &a.TenantID, &a.SPAPIClientID, &encSecret, &encToken,
		&a.SellerID, &a.MarketplaceID, &a.Status, &a.LastVerified, &a.ErrorMessage,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get amazon seller account: %w", err)
	}

	a.SPAPIClientSecret, err = r.encryptor.Decrypt(encSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt client secret: %w", err)
	}
	a.SPAPIRefreshToken, err = r.encryptor.Decrypt(encToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh token: %w", err)
	}

	return &a, nil
}

func (r *AmazonSellerAccountRepo) Update(ctx context.Context, account *domain.AmazonSellerAccount) error {
	encSecret, err := r.encryptor.Encrypt(account.SPAPIClientSecret)
	if err != nil {
		return fmt.Errorf("encrypt client secret: %w", err)
	}
	encToken, err := r.encryptor.Encrypt(account.SPAPIRefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	account.UpdatedAt = time.Now()
	_, err = r.pool.Exec(ctx, `
		UPDATE amazon_seller_accounts SET
			sp_api_client_id = $2,
			sp_api_client_secret = $3,
			sp_api_refresh_token = $4,
			seller_id = $5,
			marketplace_id = $6,
			status = $7,
			updated_at = $8
		WHERE tenant_id = $1
	`, account.TenantID, account.SPAPIClientID, encSecret, encToken,
		account.SellerID, account.MarketplaceID, account.Status, account.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update amazon seller account: %w", err)
	}
	return nil
}

func (r *AmazonSellerAccountRepo) Delete(ctx context.Context, tenantID domain.TenantID) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM amazon_seller_accounts WHERE tenant_id = $1
	`, tenantID)
	if err != nil {
		return fmt.Errorf("delete amazon seller account: %w", err)
	}
	return nil
}

func (r *AmazonSellerAccountRepo) UpdateStatus(ctx context.Context, tenantID domain.TenantID, status domain.SellerAccountStatus, errMsg string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `
		UPDATE amazon_seller_accounts SET status = $2, error_message = $3, last_verified = $4, updated_at = $4
		WHERE tenant_id = $1
	`, tenantID, status, errMsg, now)
	if err != nil {
		return fmt.Errorf("update seller account status: %w", err)
	}
	return nil
}

-- Per-tenant Amazon seller account credentials
CREATE TABLE IF NOT EXISTS amazon_seller_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE,

    -- SP-API credentials (encrypted at rest via app-level AES-256-GCM)
    sp_api_client_id     TEXT NOT NULL,
    sp_api_client_secret TEXT NOT NULL,
    sp_api_refresh_token TEXT NOT NULL,
    seller_id            TEXT NOT NULL,
    marketplace_id       TEXT NOT NULL DEFAULT 'ATVPDKIKX0DER',

    -- Credential health
    status        TEXT NOT NULL DEFAULT 'pending',
    last_verified TIMESTAMPTZ,
    error_message TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- RLS policy: tenants can only access their own credentials
ALTER TABLE amazon_seller_accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY amazon_seller_accounts_isolation ON amazon_seller_accounts
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

COMMENT ON COLUMN amazon_seller_accounts.sp_api_client_secret IS 'Encrypted at rest via app-level AES-256-GCM';
COMMENT ON COLUMN amazon_seller_accounts.sp_api_refresh_token IS 'Encrypted at rest via app-level AES-256-GCM';

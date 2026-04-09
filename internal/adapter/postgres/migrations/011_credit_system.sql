-- Credit accounts track per-tenant balance and tier
CREATE TABLE IF NOT EXISTS credit_accounts (
    tenant_id UUID PRIMARY KEY,
    tier TEXT NOT NULL DEFAULT 'free',
    monthly_limit INT NOT NULL DEFAULT 500,
    used_this_month INT NOT NULL DEFAULT 0,
    reset_at TIMESTAMPTZ NOT NULL DEFAULT (date_trunc('month', now()) + interval '1 month')
);

-- Immutable ledger of credit transactions
CREATE TABLE IF NOT EXISTS credit_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    amount INT NOT NULL,
    action TEXT NOT NULL,
    reference TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_tenant ON credit_transactions(tenant_id, created_at DESC);

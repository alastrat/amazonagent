-- Brands table (shared across tenants, normalized)
CREATE TABLE IF NOT EXISTS brands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(normalized_name)
);

CREATE INDEX IF NOT EXISTS idx_brands_normalized ON brands(normalized_name);

-- Per-tenant eligibility cache
CREATE TABLE brand_eligibility (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    brand_id UUID NOT NULL REFERENCES brands(id),
    status TEXT NOT NULL DEFAULT 'unknown',
    reason TEXT NOT NULL DEFAULT '',
    sample_asin TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, brand_id)
);

CREATE INDEX idx_brand_eligibility_tenant ON brand_eligibility(tenant_id);
CREATE INDEX idx_brand_eligibility_status ON brand_eligibility(tenant_id, status);

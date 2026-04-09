-- Platform-wide product catalog (shared across all tenants)
-- Product data is universal. Tenant-specific eligibility/margins are separate.
CREATE TABLE IF NOT EXISTS product_catalog (
    asin TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    bsr_rank INT,
    seller_count INT,
    buy_box_price NUMERIC(10,2),
    estimated_margin_pct NUMERIC(5,2),
    image_url TEXT,
    last_enriched_at TIMESTAMPTZ,
    enrichment_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pc_brand ON product_catalog(brand);
CREATE INDEX IF NOT EXISTS idx_pc_category ON product_catalog(category);
CREATE INDEX IF NOT EXISTS idx_pc_stale ON product_catalog(last_enriched_at NULLS FIRST);

-- Platform-wide brand catalog with gating metadata
CREATE TABLE IF NOT EXISTS brand_catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL UNIQUE,
    typical_gating TEXT NOT NULL DEFAULT 'unknown',
    categories TEXT[] NOT NULL DEFAULT '{}',
    product_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-tenant eligibility (private — never shared across tenants)
CREATE TABLE IF NOT EXISTS tenant_product_eligibility (
    tenant_id UUID NOT NULL,
    asin TEXT NOT NULL,
    eligible BOOLEAN NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, asin)
);

CREATE INDEX IF NOT EXISTS idx_tpe_tenant ON tenant_product_eligibility(tenant_id, eligible);

-- Per-tenant margin data from price lists (private)
CREATE TABLE IF NOT EXISTS tenant_product_margins (
    tenant_id UUID NOT NULL,
    asin TEXT NOT NULL,
    wholesale_cost NUMERIC(10,2) NOT NULL,
    real_margin_pct NUMERIC(5,2),
    source TEXT NOT NULL DEFAULT 'pricelist',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, asin)
);

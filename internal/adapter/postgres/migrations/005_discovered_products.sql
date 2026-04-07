CREATE TABLE IF NOT EXISTS discovered_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    asin TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    brand_id UUID REFERENCES brands(id),
    category TEXT NOT NULL DEFAULT '',
    browse_node_id TEXT,
    estimated_price NUMERIC(10,2),
    buy_box_price NUMERIC(10,2),
    bsr_rank INT,
    seller_count INT,
    estimated_margin_pct NUMERIC(5,2),
    real_margin_pct NUMERIC(5,2),
    eligibility_status TEXT NOT NULL DEFAULT 'unknown',
    data_quality SMALLINT NOT NULL DEFAULT 0,
    refresh_priority REAL NOT NULL DEFAULT 0.0,
    source TEXT NOT NULL DEFAULT 'search',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    price_updated_at TIMESTAMPTZ,
    UNIQUE(tenant_id, asin)
);

CREATE INDEX IF NOT EXISTS idx_dp_tenant_brand ON discovered_products(tenant_id, brand_id);
CREATE INDEX IF NOT EXISTS idx_dp_tenant_category ON discovered_products(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_dp_tenant_refresh ON discovered_products(tenant_id, refresh_priority DESC)
    WHERE data_quality < 31;
CREATE INDEX IF NOT EXISTS idx_dp_tenant_margin ON discovered_products(tenant_id, estimated_margin_pct DESC);
CREATE INDEX IF NOT EXISTS idx_dp_browse_node ON discovered_products(browse_node_id);

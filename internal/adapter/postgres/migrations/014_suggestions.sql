-- Discovery suggestions: products found by the daily queue matching the seller's strategy
CREATE TABLE IF NOT EXISTS discovery_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    strategy_version_id UUID,
    goal_id TEXT,
    asin TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    buy_box_price NUMERIC(10,2),
    estimated_margin_pct NUMERIC(5,2),
    bsr_rank INT,
    seller_count INT,
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    deal_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ds_tenant_status ON discovery_suggestions(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_ds_tenant_created ON discovery_suggestions(tenant_id, created_at DESC);

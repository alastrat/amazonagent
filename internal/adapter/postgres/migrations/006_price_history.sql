CREATE TABLE IF NOT EXISTS price_history (
    id UUID DEFAULT gen_random_uuid(),
    asin TEXT NOT NULL,
    tenant_id UUID NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    amazon_price NUMERIC(10,2),
    bsr_rank INT,
    seller_count INT
) PARTITION BY RANGE (recorded_at);

CREATE TABLE IF NOT EXISTS price_history_2026_04 PARTITION OF price_history
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS price_history_2026_05 PARTITION OF price_history
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS price_history_2026_06 PARTITION OF price_history
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX IF NOT EXISTS idx_ph_asin_time ON price_history(tenant_id, asin, recorded_at DESC);

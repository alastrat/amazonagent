CREATE MATERIALIZED VIEW IF NOT EXISTS brand_intelligence AS
SELECT
    dp.tenant_id,
    dp.brand_id,
    b.name AS brand_name,
    dp.category,
    COUNT(*) AS product_count,
    COUNT(*) FILTER (WHERE dp.estimated_margin_pct >= 20) AS high_margin_count,
    AVG(dp.estimated_margin_pct) AS avg_margin,
    AVG(dp.seller_count) AS avg_sellers,
    AVG(dp.bsr_rank) FILTER (WHERE dp.bsr_rank > 0) AS avg_bsr
FROM discovered_products dp
JOIN brands b ON dp.brand_id = b.id
GROUP BY dp.tenant_id, dp.brand_id, b.name, dp.category;

CREATE UNIQUE INDEX IF NOT EXISTS idx_bi_tenant_brand_cat
    ON brand_intelligence(tenant_id, brand_id, category);

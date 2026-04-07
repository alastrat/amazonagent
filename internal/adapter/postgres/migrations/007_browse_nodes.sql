CREATE TABLE IF NOT EXISTS browse_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    amazon_node_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    parent_node_id TEXT,
    depth INT NOT NULL DEFAULT 0,
    is_leaf BOOLEAN NOT NULL DEFAULT false,
    last_scanned_at TIMESTAMPTZ,
    products_found INT DEFAULT 0,
    scan_priority REAL NOT NULL DEFAULT 0.0
);

CREATE INDEX IF NOT EXISTS idx_bn_scan ON browse_nodes(last_scanned_at NULLS FIRST)
    WHERE is_leaf = true;

-- Add product-level fields to assessment_probe_results for the ECharts radial tree
ALTER TABLE assessment_probe_results ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
ALTER TABLE assessment_probe_results ADD COLUMN IF NOT EXISTS price NUMERIC(10,2) NOT NULL DEFAULT 0;
ALTER TABLE assessment_probe_results ADD COLUMN IF NOT EXISTS est_margin_pct NUMERIC(5,2) NOT NULL DEFAULT 0;
ALTER TABLE assessment_probe_results ADD COLUMN IF NOT EXISTS seller_count INT NOT NULL DEFAULT 0;

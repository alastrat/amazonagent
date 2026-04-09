-- Seller profiles track each tenant's assessed situation
CREATE TABLE IF NOT EXISTS seller_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE,
    archetype TEXT NOT NULL DEFAULT 'greenhorn',
    account_age_days INT NOT NULL DEFAULT 0,
    active_listings INT NOT NULL DEFAULT 0,
    stated_capital NUMERIC(12,2),
    assessment_status TEXT NOT NULL DEFAULT 'pending',
    assessed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Eligibility fingerprints store the 300-ASIN scan results
CREATE TABLE IF NOT EXISTS eligibility_fingerprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    total_probes INT NOT NULL DEFAULT 0,
    total_eligible INT NOT NULL DEFAULT 0,
    total_restricted INT NOT NULL DEFAULT 0,
    overall_open_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
    confidence NUMERIC(3,2) NOT NULL DEFAULT 0,
    assessed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id)
);

-- Category-level eligibility scores from the assessment
CREATE TABLE IF NOT EXISTS category_eligibilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    category TEXT NOT NULL,
    probe_count INT NOT NULL DEFAULT 0,
    open_count INT NOT NULL DEFAULT 0,
    gated_count INT NOT NULL DEFAULT 0,
    open_rate NUMERIC(5,2) NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_ce_fingerprint ON category_eligibilities(fingerprint_id);
CREATE INDEX IF NOT EXISTS idx_ce_tenant ON category_eligibilities(tenant_id, category);

-- Individual probe results (raw data from the 300-ASIN scan)
CREATE TABLE IF NOT EXISTS assessment_probe_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    asin TEXT NOT NULL,
    brand TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    tier TEXT NOT NULL DEFAULT '',
    eligible BOOLEAN NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_apr_fingerprint ON assessment_probe_results(fingerprint_id);

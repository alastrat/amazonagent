-- Scoring configs
CREATE TABLE scoring_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    version INT NOT NULL DEFAULT 1,
    weights JSONB NOT NULL,
    thresholds JSONB NOT NULL,
    created_by TEXT NOT NULL DEFAULT 'user',
    active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_scoring_configs_tenant_active ON scoring_configs (tenant_id, active) WHERE active = true;

-- Discovery configs
CREATE TABLE discovery_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE,
    categories JSONB NOT NULL DEFAULT '[]',
    baseline_criteria JSONB NOT NULL DEFAULT '{}',
    scoring_config_id UUID REFERENCES scoring_configs(id),
    cadence TEXT NOT NULL DEFAULT 'nightly',
    enabled BOOLEAN NOT NULL DEFAULT false,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ
);

-- Campaigns
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    type TEXT NOT NULL,
    criteria JSONB NOT NULL,
    scoring_config_id UUID REFERENCES scoring_configs(id),
    experiment_id UUID,
    source_file TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_by TEXT NOT NULL DEFAULT 'user',
    trigger_type TEXT NOT NULL DEFAULT 'dashboard',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_campaigns_tenant ON campaigns (tenant_id);
CREATE INDEX idx_campaigns_tenant_status ON campaigns (tenant_id, status);

-- Deals
CREATE TABLE deals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id),
    asin TEXT NOT NULL,
    title TEXT NOT NULL,
    brand TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'discovered',
    scores JSONB NOT NULL DEFAULT '{}',
    evidence JSONB NOT NULL DEFAULT '{}',
    reviewer_verdict TEXT NOT NULL DEFAULT '',
    iteration_count INT NOT NULL DEFAULT 0,
    supplier_id UUID,
    listing_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deals_tenant ON deals (tenant_id);
CREATE INDEX idx_deals_tenant_status ON deals (tenant_id, status);
CREATE INDEX idx_deals_campaign ON deals (campaign_id);
CREATE INDEX idx_deals_asin ON deals (tenant_id, asin);

-- Domain events
CREATE TABLE domain_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    correlation_id TEXT NOT NULL DEFAULT '',
    actor_id TEXT NOT NULL DEFAULT '',
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_events_tenant ON domain_events (tenant_id);
CREATE INDEX idx_domain_events_entity ON domain_events (tenant_id, entity_type, entity_id);
CREATE INDEX idx_domain_events_type ON domain_events (tenant_id, event_type);

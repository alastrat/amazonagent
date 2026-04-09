-- Strategy versions: immutable snapshots of a seller's growth strategy.
-- Every change creates a new version. Full audit trail with rollback.
CREATE TABLE IF NOT EXISTS strategy_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    version_number INT NOT NULL,
    goals JSONB NOT NULL DEFAULT '[]',
    search_params JSONB NOT NULL DEFAULT '{}',
    scoring_config_id UUID,
    status TEXT NOT NULL DEFAULT 'draft',
    parent_version_id UUID,
    promoted_from_experiment_id TEXT,
    change_reason TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at TIMESTAMPTZ,
    rolled_back_at TIMESTAMPTZ,
    UNIQUE(tenant_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_sv_tenant_status ON strategy_versions(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_sv_tenant_version ON strategy_versions(tenant_id, version_number DESC);

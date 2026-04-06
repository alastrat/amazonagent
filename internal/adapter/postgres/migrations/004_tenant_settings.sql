CREATE TABLE tenant_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE,
    agent_memory_enabled BOOLEAN NOT NULL DEFAULT false,
    telegram_enabled BOOLEAN NOT NULL DEFAULT false,
    telegram_chat_id BIGINT DEFAULT 0,
    auto_advance_on_approve BOOLEAN NOT NULL DEFAULT true,
    outreach_auto_send TEXT NOT NULL DEFAULT 'never',
    notify_campaign_complete BOOLEAN NOT NULL DEFAULT true,
    notify_new_deals BOOLEAN NOT NULL DEFAULT true,
    notify_price_changes BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

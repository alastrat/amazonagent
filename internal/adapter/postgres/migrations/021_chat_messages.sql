-- Chat sessions and messages for the concierge interface.

CREATE TABLE IF NOT EXISTS chat_sessions (
    id TEXT PRIMARY KEY,
    tenant_id UUID NOT NULL,
    agent_session_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_sessions_tenant ON chat_sessions(tenant_id);

CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    tenant_id UUID NOT NULL,
    session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL, -- user, assistant, system
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_messages_tenant ON chat_messages(tenant_id);
CREATE INDEX idx_chat_messages_session ON chat_messages(session_id, created_at);

-- RLS
ALTER TABLE chat_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY chat_sessions_isolation ON chat_sessions
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY chat_messages_isolation ON chat_messages
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type ChatRepo struct {
	pool *pgxpool.Pool
}

func NewChatRepo(pool *pgxpool.Pool) *ChatRepo {
	return &ChatRepo{pool: pool}
}

func (r *ChatRepo) CreateSession(ctx context.Context, session *domain.ChatSession) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO chat_sessions (id, tenant_id, agent_session_id, status, created_at, last_message_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, session.ID, session.TenantID, session.AgentSessionID, session.Status, session.CreatedAt, session.LastMessageAt)
	if err != nil {
		return fmt.Errorf("create chat session: %w", err)
	}
	return nil
}

func (r *ChatRepo) GetSession(ctx context.Context, tenantID domain.TenantID) (*domain.ChatSession, error) {
	var s domain.ChatSession
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, agent_session_id, status, created_at, last_message_at
		FROM chat_sessions
		WHERE tenant_id = $1 AND status = 'active'
		ORDER BY created_at DESC LIMIT 1
	`, tenantID).Scan(&s.ID, &s.TenantID, &s.AgentSessionID, &s.Status, &s.CreatedAt, &s.LastMessageAt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat session: %w", err)
	}
	return &s, nil
}

func (r *ChatRepo) UpdateSession(ctx context.Context, session *domain.ChatSession) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE chat_sessions SET status = $1, last_message_at = $2, agent_session_id = $3
		WHERE id = $4
	`, session.Status, session.LastMessageAt, session.AgentSessionID, session.ID)
	if err != nil {
		return fmt.Errorf("update chat session: %w", err)
	}
	return nil
}

func (r *ChatRepo) SaveMessage(ctx context.Context, msg *domain.ChatMessage) error {
	metadata, _ := json.Marshal(msg.Metadata)
	if metadata == nil {
		metadata = []byte("{}")
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO chat_messages (id, tenant_id, session_id, role, content, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, msg.ID, msg.TenantID, msg.SessionID, msg.Role, msg.Content, metadata, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("save chat message: %w", err)
	}
	return nil
}

func (r *ChatRepo) ListMessages(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.ChatMessage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, session_id, role, content, metadata, created_at
		FROM chat_messages
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()

	var messages []domain.ChatMessage
	for rows.Next() {
		var msg domain.ChatMessage
		var metadataJSON []byte
		if err := rows.Scan(&msg.ID, &msg.TenantID, &msg.SessionID, &msg.Role, &msg.Content, &metadataJSON, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &msg.Metadata)
		}
		messages = append(messages, msg)
	}

	// Reverse to chronological order (query returns newest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

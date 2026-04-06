package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type TenantSettingsRepo struct {
	pool *pgxpool.Pool
}

func NewTenantSettingsRepo(pool *pgxpool.Pool) *TenantSettingsRepo {
	return &TenantSettingsRepo{pool: pool}
}

func (r *TenantSettingsRepo) Get(ctx context.Context, tenantID domain.TenantID) (*domain.TenantSettings, error) {
	var s domain.TenantSettings
	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, agent_memory_enabled, telegram_enabled, telegram_chat_id,
		 auto_advance_on_approve, outreach_auto_send, notify_campaign_complete,
		 notify_new_deals, notify_price_changes, created_at, updated_at
		 FROM tenant_settings WHERE tenant_id = $1`, tenantID).Scan(
		&s.ID, &s.TenantID, &s.AgentMemoryEnabled, &s.TelegramEnabled, &s.TelegramChatID,
		&s.AutoAdvanceOnApprove, &s.OutreachAutoSend, &s.NotifyOnCampaignComplete,
		&s.NotifyOnNewDeals, &s.NotifyOnPriceChanges, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get tenant settings: %w", err)
	}
	return &s, nil
}

func (r *TenantSettingsRepo) Upsert(ctx context.Context, s *domain.TenantSettings) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tenant_settings (tenant_id, agent_memory_enabled, telegram_enabled, telegram_chat_id,
		 auto_advance_on_approve, outreach_auto_send, notify_campaign_complete,
		 notify_new_deals, notify_price_changes, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
		 ON CONFLICT (tenant_id) DO UPDATE SET
		 agent_memory_enabled = $2, telegram_enabled = $3, telegram_chat_id = $4,
		 auto_advance_on_approve = $5, outreach_auto_send = $6,
		 notify_campaign_complete = $7, notify_new_deals = $8,
		 notify_price_changes = $9, updated_at = now()`,
		s.TenantID, s.AgentMemoryEnabled, s.TelegramEnabled, s.TelegramChatID,
		s.AutoAdvanceOnApprove, s.OutreachAutoSend, s.NotifyOnCampaignComplete,
		s.NotifyOnNewDeals, s.NotifyOnPriceChanges)
	if err != nil {
		return fmt.Errorf("upsert tenant settings: %w", err)
	}
	return nil
}

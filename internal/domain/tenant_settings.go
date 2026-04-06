package domain

import "time"

type TenantSettings struct {
	ID                       string   `json:"id"`
	TenantID                 TenantID `json:"tenant_id"`
	AgentMemoryEnabled       bool     `json:"agent_memory_enabled"`
	TelegramEnabled          bool     `json:"telegram_enabled"`
	TelegramChatID           int64    `json:"telegram_chat_id,omitempty"`
	AutoAdvanceOnApprove     bool     `json:"auto_advance_on_approve"`
	OutreachAutoSend         string   `json:"outreach_auto_send"`
	NotifyOnCampaignComplete bool     `json:"notify_campaign_complete"`
	NotifyOnNewDeals         bool     `json:"notify_new_deals"`
	NotifyOnPriceChanges     bool     `json:"notify_price_changes"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

func DefaultTenantSettings(tenantID TenantID) TenantSettings {
	return TenantSettings{
		TenantID:                 tenantID,
		AgentMemoryEnabled:       false,
		TelegramEnabled:          false,
		AutoAdvanceOnApprove:     true,
		OutreachAutoSend:         "never",
		NotifyOnCampaignComplete: true,
		NotifyOnNewDeals:         true,
		NotifyOnPriceChanges:     false,
	}
}

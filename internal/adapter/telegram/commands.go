package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type CommandHandler struct {
	deals     *service.DealService
	campaigns *service.CampaignService
	blocklist *service.BrandBlocklistService
	settings  *service.TenantSettingsService
	tenantID  domain.TenantID
}

func NewCommandHandler(
	deals *service.DealService,
	campaigns *service.CampaignService,
	blocklist *service.BrandBlocklistService,
	settings *service.TenantSettingsService,
	tenantID domain.TenantID,
) *CommandHandler {
	return &CommandHandler{deals: deals, campaigns: campaigns, blocklist: blocklist, settings: settings, tenantID: tenantID}
}

func (h *CommandHandler) Handle(ctx context.Context, chatID int64, text string) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}
	cmd := strings.ToLower(parts[0])
	switch cmd {
	case "/start":
		return h.handleStart(ctx, chatID)
	case "/status", "/dashboard":
		return h.handleStatus(ctx)
	case "/deals":
		return h.handleDeals(ctx)
	case "/campaigns":
		return h.handleCampaigns(ctx)
	case "/blocklist":
		return h.handleBlocklist(ctx)
	case "/connect":
		return h.handleConnect(ctx, chatID)
	case "/help":
		return h.handleHelp()
	default:
		if strings.HasPrefix(cmd, "/") {
			return "Unknown command. Type /help for available commands."
		}
		return "💡 I only respond to commands for now. Type /help to see what I can do."
	}
}

func (h *CommandHandler) handleStart(ctx context.Context, chatID int64) string {
	settings, _ := h.settings.Get(ctx, h.tenantID)
	settings.TelegramChatID = chatID
	settings.TelegramEnabled = true
	_ = h.settings.Update(ctx, settings)
	return "🤖 <b>FBA Agent Orchestrator</b>\n\nI'm your Amazon wholesale research assistant.\n\nTelegram connected! I'll notify you about:\n• Campaign completions\n• New profitable deals\n• Nightly scan summaries\n\nType /help for commands."
}

func (h *CommandHandler) handleConnect(ctx context.Context, chatID int64) string {
	settings, _ := h.settings.Get(ctx, h.tenantID)
	settings.TelegramChatID = chatID
	settings.TelegramEnabled = true
	_ = h.settings.Update(ctx, settings)
	return fmt.Sprintf("✅ Notifications connected! Chat ID: %d", chatID)
}

func (h *CommandHandler) handleStatus(ctx context.Context) string {
	needsReview := domain.DealStatusNeedsReview
	_, reviewCount, _ := h.deals.List(ctx, h.tenantID, port.DealFilter{Status: &needsReview, Limit: 0})
	approved := domain.DealStatusApproved
	_, approvedCount, _ := h.deals.List(ctx, h.tenantID, port.DealFilter{Status: &approved, Limit: 0})
	running := domain.CampaignStatusRunning
	activeCampaigns, _ := h.campaigns.List(ctx, h.tenantID, port.CampaignFilter{Status: &running})
	return fmt.Sprintf("📊 <b>Dashboard</b>\n\n📋 Pending review: <b>%d</b>\n✅ Approved: <b>%d</b>\n🔄 Active campaigns: <b>%d</b>",
		reviewCount, approvedCount, len(activeCampaigns))
}

func (h *CommandHandler) handleDeals(ctx context.Context) string {
	needsReview := domain.DealStatusNeedsReview
	deals, total, _ := h.deals.List(ctx, h.tenantID, port.DealFilter{Status: &needsReview, Limit: 5, SortBy: "created_at", SortDir: "desc"})
	if total == 0 {
		return "📦 No deals pending review."
	}
	msg := fmt.Sprintf("📦 <b>Deals Pending</b> (%d total)\n\n", total)
	for _, d := range deals {
		msg += fmt.Sprintf("• <a href=\"https://amazon.com/dp/%s\">%s</a>\n  Score: %.1f | %s\n\n", d.ASIN, d.Title, d.Scores.Overall, d.ASIN)
	}
	return msg
}

func (h *CommandHandler) handleCampaigns(ctx context.Context) string {
	campaigns, _ := h.campaigns.List(ctx, h.tenantID, port.CampaignFilter{Limit: 5})
	if len(campaigns) == 0 {
		return "🔍 No campaigns yet."
	}
	msg := "🔍 <b>Recent Campaigns</b>\n\n"
	for _, c := range campaigns {
		msg += fmt.Sprintf("• <b>%s</b> [%s]\n  %s\n\n", c.Type, c.Status, strings.Join(c.Criteria.Keywords, ", "))
	}
	return msg
}

func (h *CommandHandler) handleBlocklist(ctx context.Context) string {
	brands, _ := h.blocklist.List(ctx, h.tenantID)
	if len(brands) == 0 {
		return "🚫 Brand blocklist is empty."
	}
	msg := fmt.Sprintf("🚫 <b>Blocked Brands</b> (%d)\n\n", len(brands))
	for _, b := range brands {
		msg += fmt.Sprintf("• %s (%s)\n", b.Brand, b.Source)
	}
	return msg
}

func (h *CommandHandler) handleHelp() string {
	return "🤖 <b>Commands</b>\n\n/status — Dashboard\n/deals — Pending deals\n/campaigns — Recent campaigns\n/blocklist — Blocked brands\n/connect — Enable notifications\n/help — This message"
}

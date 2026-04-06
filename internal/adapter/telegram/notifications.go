package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type NotificationService struct {
	bot      *Bot
	settings *service.TenantSettingsService
}

func NewNotificationService(bot *Bot, settings *service.TenantSettingsService) *NotificationService {
	return &NotificationService{bot: bot, settings: settings}
}

func (n *NotificationService) NotifyCampaignComplete(ctx context.Context, tenantID domain.TenantID, dealsFound int, keywords []string) {
	settings, err := n.settings.Get(ctx, tenantID)
	if err != nil || !settings.TelegramEnabled || settings.TelegramChatID == 0 || !settings.NotifyOnCampaignComplete {
		return
	}
	msg := fmt.Sprintf("✅ <b>Campaign Complete</b>\n\n🔍 Keywords: %v\n📦 Deals found: <b>%d</b>\n\nType /deals to review.", keywords, dealsFound)
	if err := n.bot.SendMessage(settings.TelegramChatID, msg); err != nil {
		slog.Warn("telegram: notification failed", "error", err)
	}
}

func (n *NotificationService) NotifyNewDeals(ctx context.Context, tenantID domain.TenantID, deals []domain.Deal) {
	settings, err := n.settings.Get(ctx, tenantID)
	if err != nil || !settings.TelegramEnabled || settings.TelegramChatID == 0 || !settings.NotifyOnNewDeals || len(deals) == 0 {
		return
	}
	msg := fmt.Sprintf("🆕 <b>%d New Deals</b>\n\n", len(deals))
	for i, d := range deals {
		if i >= 5 {
			msg += fmt.Sprintf("\n... and %d more", len(deals)-5)
			break
		}
		msg += fmt.Sprintf("• <a href=\"https://amazon.com/dp/%s\">%s</a>\n  Score: %.1f\n\n", d.ASIN, d.Title, d.Scores.Overall)
	}
	msg += "Type /deals to review."
	if err := n.bot.SendMessage(settings.TelegramChatID, msg); err != nil {
		slog.Warn("telegram: notification failed", "error", err)
	}
}

func (n *NotificationService) NotifyNightlyScan(ctx context.Context, tenantID domain.TenantID, scanned, eligible, profitable int) {
	settings, err := n.settings.Get(ctx, tenantID)
	if err != nil || !settings.TelegramEnabled || settings.TelegramChatID == 0 {
		return
	}
	msg := fmt.Sprintf("🌙 <b>Nightly Scan Complete</b>\n\n📊 Scanned: <b>%d</b>\n✅ Eligible: <b>%d</b>\n💰 Profitable: <b>%d</b>\n\nType /deals to browse.", scanned, eligible, profitable)
	if err := n.bot.SendMessage(settings.TelegramChatID, msg); err != nil {
		slog.Warn("telegram: nightly notification failed", "error", err)
	}
}

# Telegram Bot + Agent Memory — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a hybrid Telegram bot (@amazonagent_bot) for interacting with the platform via chat — simple commands call the Go API directly (free), complex queries route through OpenFang (LLM), and nightly scan results push as notifications. Enable configurable per-tenant agent memory in OpenFang.

**Architecture:** New Go `TelegramBot` service uses the Telegram Bot API directly (not through OpenFang's channel adapter) for command handling and notifications. Complex queries spawn a dedicated OpenFang "assistant" agent with the API as tools. TenantSettings table stores per-tenant preferences for memory and notifications. OpenFang config updated to enable memory.

**Tech Stack:** Go, Telegram Bot API (HTTP), OpenFang API, Postgres, Inngest (for nightly notifications)

---

## File Structure

### New files
```
internal/domain/tenant_settings.go         -- TenantSettings domain type
internal/adapter/postgres/tenant_settings_repo.go
internal/adapter/postgres/migrations/004_tenant_settings.sql
internal/service/tenant_settings_service.go
internal/adapter/telegram/bot.go           -- Telegram bot (commands + notifications)
internal/adapter/telegram/commands.go      -- Command handlers (/status, /deals, etc.)
internal/adapter/telegram/notifications.go -- Push notification formatting
internal/api/handler/settings_handler.go   -- Tenant settings API
```

### Modified files
```
internal/adapter/openfang/agent_runtime.go -- Add memory flag to agent spawn
deploy/openfang/config.toml               -- Enable memory section
docker-compose.yml                         -- Pass TELEGRAM_BOT_TOKEN
internal/api/router.go                     -- Mount settings endpoints
apps/api/main.go                           -- Wire telegram bot + settings
```

---

## Task 1: Tenant Settings Domain + Migration

**Files:**
- Create: `internal/domain/tenant_settings.go`
- Create: `internal/adapter/postgres/migrations/004_tenant_settings.sql`
- Create: `internal/adapter/postgres/tenant_settings_repo.go`
- Create: `internal/service/tenant_settings_service.go`

- [ ] **Step 1: Create `internal/domain/tenant_settings.go`**

```go
package domain

import "time"

type TenantSettings struct {
	ID        string   `json:"id"`
	TenantID  TenantID `json:"tenant_id"`

	// Agent memory
	AgentMemoryEnabled bool `json:"agent_memory_enabled"` // default false

	// Telegram notifications
	TelegramEnabled bool   `json:"telegram_enabled"`
	TelegramChatID  int64  `json:"telegram_chat_id,omitempty"`

	// Automation levels
	AutoAdvanceOnApprove bool   `json:"auto_advance_on_approve"` // default true
	OutreachAutoSend     string `json:"outreach_auto_send"`      // "never", "first_approved", "always"

	// Notification preferences
	NotifyOnCampaignComplete bool `json:"notify_campaign_complete"` // default true
	NotifyOnNewDeals         bool `json:"notify_new_deals"`         // default true
	NotifyOnPriceChanges     bool `json:"notify_price_changes"`     // default false

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func DefaultTenantSettings(tenantID TenantID) TenantSettings {
	return TenantSettings{
		TenantID:                tenantID,
		AgentMemoryEnabled:      false,
		TelegramEnabled:         false,
		AutoAdvanceOnApprove:    true,
		OutreachAutoSend:        "never",
		NotifyOnCampaignComplete: true,
		NotifyOnNewDeals:        true,
		NotifyOnPriceChanges:    false,
	}
}
```

- [ ] **Step 2: Create migration `internal/adapter/postgres/migrations/004_tenant_settings.sql`**

```sql
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
```

- [ ] **Step 3: Create `internal/adapter/postgres/tenant_settings_repo.go`**

```go
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
```

- [ ] **Step 4: Create `internal/service/tenant_settings_service.go`**

```go
package service

import (
	"context"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type TenantSettingsRepo interface {
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.TenantSettings, error)
	Upsert(ctx context.Context, s *domain.TenantSettings) error
}

type TenantSettingsService struct {
	repo TenantSettingsRepo
}

func NewTenantSettingsService(repo TenantSettingsRepo) *TenantSettingsService {
	return &TenantSettingsService{repo: repo}
}

func (s *TenantSettingsService) Get(ctx context.Context, tenantID domain.TenantID) (*domain.TenantSettings, error) {
	settings, err := s.repo.Get(ctx, tenantID)
	if err != nil {
		// Return defaults if not found
		defaults := domain.DefaultTenantSettings(tenantID)
		return &defaults, nil
	}
	return settings, nil
}

func (s *TenantSettingsService) Update(ctx context.Context, settings *domain.TenantSettings) error {
	if err := s.repo.Upsert(ctx, settings); err != nil {
		return err
	}
	slog.Info("tenant settings updated", "tenant_id", settings.TenantID,
		"memory", settings.AgentMemoryEnabled, "telegram", settings.TelegramEnabled)
	return nil
}
```

- [ ] **Step 5: Verify build and commit**

```bash
go build ./...
git add internal/domain/tenant_settings.go internal/adapter/postgres/migrations/004_tenant_settings.sql internal/adapter/postgres/tenant_settings_repo.go internal/service/tenant_settings_service.go
git commit -m "feat: add tenant settings — memory, telegram, automation preferences"
```

---

## Task 2: Telegram Bot — Core + Commands

**Files:**
- Create: `internal/adapter/telegram/bot.go`
- Create: `internal/adapter/telegram/commands.go`

- [ ] **Step 1: Create `internal/adapter/telegram/bot.go`**

The bot uses the Telegram Bot API directly via HTTP — no third-party library needed.

```go
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Bot struct {
	token      string
	apiBase    string
	httpClient *http.Client
	commands   *CommandHandler
	stopCh     chan struct{}
}

func NewBot(token string, commands *CommandHandler) *Bot {
	return &Bot{
		token:      token,
		apiBase:    "https://api.telegram.org/bot" + token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		commands:   commands,
		stopCh:     make(chan struct{}),
	}
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	From      *User  `json:"from,omitempty"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

// StartPolling begins long-polling for Telegram updates.
func (b *Bot) StartPolling(ctx context.Context) {
	slog.Info("telegram: starting bot polling")
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		default:
		}

		updates, err := b.getUpdates(offset, 30)
		if err != nil {
			slog.Warn("telegram: poll error", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			if update.Message != nil && update.Message.Text != "" {
				go b.handleMessage(ctx, update.Message)
			}
		}
	}
}

func (b *Bot) Stop() {
	close(b.stopCh)
}

func (b *Bot) handleMessage(ctx context.Context, msg *Message) {
	text := strings.TrimSpace(msg.Text)
	chatID := msg.Chat.ID

	slog.Info("telegram: received message", "chat_id", chatID, "text", text)

	// Route to command handler
	response := b.commands.Handle(ctx, chatID, text)
	if response != "" {
		b.SendMessage(chatID, response)
	}
}

func (b *Bot) getUpdates(offset, timeout int) ([]Update, error) {
	url := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=%d", b.apiBase, offset, timeout)
	resp, err := b.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Result, nil
}

// SendMessage sends a text message to a chat.
func (b *Bot) SendMessage(chatID int64, text string) error {
	body, _ := json.Marshal(map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})

	resp, err := b.httpClient.Post(b.apiBase+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}
```

- [ ] **Step 2: Create `internal/adapter/telegram/commands.go`**

```go
package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// CommandHandler routes Telegram commands to the appropriate service.
type CommandHandler struct {
	deals     *service.DealService
	campaigns *service.CampaignService
	blocklist *service.BrandBlocklistService
	settings  *service.TenantSettingsService
	tenantID  domain.TenantID // dev: single-tenant mode
}

func NewCommandHandler(
	deals *service.DealService,
	campaigns *service.CampaignService,
	blocklist *service.BrandBlocklistService,
	settings *service.TenantSettingsService,
	tenantID domain.TenantID,
) *CommandHandler {
	return &CommandHandler{
		deals:     deals,
		campaigns: campaigns,
		blocklist: blocklist,
		settings:  settings,
		tenantID:  tenantID,
	}
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
	case "/help":
		return h.handleHelp()
	case "/connect":
		return h.handleConnect(ctx, chatID)
	default:
		if strings.HasPrefix(cmd, "/") {
			return "Unknown command. Type /help for available commands."
		}
		// Non-command text — could route to OpenFang for conversational queries
		return "💡 I only respond to commands for now. Type /help to see what I can do."
	}
}

func (h *CommandHandler) handleStart(ctx context.Context, chatID int64) string {
	// Save chat ID for notifications
	settings, _ := h.settings.Get(ctx, h.tenantID)
	settings.TelegramChatID = chatID
	settings.TelegramEnabled = true
	h.settings.Update(ctx, settings)

	return "🤖 <b>FBA Agent Orchestrator</b>\n\n" +
		"I'm your Amazon wholesale research assistant.\n\n" +
		"Your Telegram is now connected! I'll send you notifications about:\n" +
		"• Campaign completions\n" +
		"• New profitable deals discovered\n" +
		"• Nightly scan summaries\n\n" +
		"Type /help to see available commands."
}

func (h *CommandHandler) handleConnect(ctx context.Context, chatID int64) string {
	settings, _ := h.settings.Get(ctx, h.tenantID)
	settings.TelegramChatID = chatID
	settings.TelegramEnabled = true
	h.settings.Update(ctx, settings)
	return "✅ Telegram notifications connected! Chat ID: " + fmt.Sprintf("%d", chatID)
}

func (h *CommandHandler) handleStatus(ctx context.Context) string {
	needsReview := domain.DealStatusNeedsReview
	_, reviewCount, _ := h.deals.List(ctx, h.tenantID, port.DealFilter{Status: &needsReview, Limit: 0})

	approved := domain.DealStatusApproved
	_, approvedCount, _ := h.deals.List(ctx, h.tenantID, port.DealFilter{Status: &approved, Limit: 0})

	running := domain.CampaignStatusRunning
	activeCampaigns, _ := h.campaigns.List(ctx, h.tenantID, port.CampaignFilter{Status: &running})

	return fmt.Sprintf("📊 <b>Dashboard</b>\n\n"+
		"📋 Pending review: <b>%d</b>\n"+
		"✅ Approved deals: <b>%d</b>\n"+
		"🔄 Active campaigns: <b>%d</b>",
		reviewCount, approvedCount, len(activeCampaigns))
}

func (h *CommandHandler) handleDeals(ctx context.Context) string {
	needsReview := domain.DealStatusNeedsReview
	deals, total, _ := h.deals.List(ctx, h.tenantID, port.DealFilter{
		Status:  &needsReview,
		Limit:   5,
		SortBy:  "created_at",
		SortDir: "desc",
	})

	if total == 0 {
		return "📦 No deals pending review."
	}

	msg := fmt.Sprintf("📦 <b>Deals Pending Review</b> (%d total)\n\n", total)
	for _, d := range deals {
		msg += fmt.Sprintf("• <a href=\"https://amazon.com/dp/%s\">%s</a>\n"+
			"  Score: %.1f | %s\n\n",
			d.ASIN, d.Title, d.Scores.Overall, d.ASIN)
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
		keywords := strings.Join(c.Criteria.Keywords, ", ")
		msg += fmt.Sprintf("• <b>%s</b> [%s]\n  Keywords: %s\n\n",
			c.Type, c.Status, keywords)
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
	return "🤖 <b>Available Commands</b>\n\n" +
		"/status — Dashboard summary\n" +
		"/deals — Pending deals for review\n" +
		"/campaigns — Recent campaigns\n" +
		"/blocklist — Blocked brands\n" +
		"/connect — Enable notifications\n" +
		"/help — This message"
}
```

- [ ] **Step 3: Verify build and commit**

```bash
go build ./...
git add internal/adapter/telegram/
git commit -m "feat: add Telegram bot with command handlers"
```

---

## Task 3: Telegram Notifications Service

**Files:**
- Create: `internal/adapter/telegram/notifications.go`

- [ ] **Step 1: Create `internal/adapter/telegram/notifications.go`**

```go
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

// NotifyCampaignComplete sends a campaign completion summary.
func (n *NotificationService) NotifyCampaignComplete(ctx context.Context, tenantID domain.TenantID, campaignID string, dealsFound int, keywords []string) {
	settings, err := n.settings.Get(ctx, tenantID)
	if err != nil || !settings.TelegramEnabled || settings.TelegramChatID == 0 || !settings.NotifyOnCampaignComplete {
		return
	}

	msg := fmt.Sprintf("✅ <b>Campaign Complete</b>\n\n"+
		"🔍 Keywords: %s\n"+
		"📦 Deals found: <b>%d</b>\n\n"+
		"Type /deals to review.",
		fmt.Sprint(keywords), dealsFound)

	if err := n.bot.SendMessage(settings.TelegramChatID, msg); err != nil {
		slog.Warn("telegram: notification failed", "error", err)
	}
}

// NotifyNewDeals sends a notification about new deals discovered.
func (n *NotificationService) NotifyNewDeals(ctx context.Context, tenantID domain.TenantID, deals []domain.Deal) {
	settings, err := n.settings.Get(ctx, tenantID)
	if err != nil || !settings.TelegramEnabled || settings.TelegramChatID == 0 || !settings.NotifyOnNewDeals {
		return
	}

	if len(deals) == 0 {
		return
	}

	msg := fmt.Sprintf("🆕 <b>%d New Deals Discovered</b>\n\n", len(deals))
	for i, d := range deals {
		if i >= 5 {
			msg += fmt.Sprintf("\n... and %d more", len(deals)-5)
			break
		}
		msg += fmt.Sprintf("• <a href=\"https://amazon.com/dp/%s\">%s</a>\n"+
			"  Score: %.1f\n\n", d.ASIN, d.Title, d.Scores.Overall)
	}
	msg += "Type /deals to review all."

	if err := n.bot.SendMessage(settings.TelegramChatID, msg); err != nil {
		slog.Warn("telegram: notification failed", "error", err)
	}
}

// NotifyNightlyScanComplete sends a summary of the nightly category scan.
func (n *NotificationService) NotifyNightlyScanComplete(ctx context.Context, tenantID domain.TenantID, scanned int, eligible int, profitable int) {
	settings, err := n.settings.Get(ctx, tenantID)
	if err != nil || !settings.TelegramEnabled || settings.TelegramChatID == 0 {
		return
	}

	msg := fmt.Sprintf("🌙 <b>Nightly Scan Complete</b>\n\n"+
		"📊 Products scanned: <b>%d</b>\n"+
		"✅ Eligible for your account: <b>%d</b>\n"+
		"💰 Profitable (>15%% margin): <b>%d</b>\n\n"+
		"Type /deals to browse opportunities.",
		scanned, eligible, profitable)

	if err := n.bot.SendMessage(settings.TelegramChatID, msg); err != nil {
		slog.Warn("telegram: nightly notification failed", "error", err)
	}
}
```

- [ ] **Step 2: Verify build and commit**

```bash
go build ./...
git add internal/adapter/telegram/notifications.go
git commit -m "feat: add Telegram notification service for campaigns, deals, nightly scans"
```

---

## Task 4: OpenFang Memory Configuration

**Files:**
- Modify: `deploy/openfang/config.toml`
- Modify: `internal/adapter/openfang/agent_runtime.go`

- [ ] **Step 1: Update OpenFang config to enable memory**

The memory section already exists in config.toml. Memory is stored per-agent in OpenFang's SQLite database. We just need to ensure agents persist across campaigns (which they already do — we cache agent IDs).

No config change needed — memory is already enabled in OpenFang. The control is whether our Go code reuses agents (memory accumulates) or creates fresh ones (no memory).

- [ ] **Step 2: Add memory toggle to agent spawn in `internal/adapter/openfang/agent_runtime.go`**

Modify the `spawnAgent` method to include a `memory_enabled` flag in the manifest. When memory is disabled (default), reset the agent's session before each use.

In `RunAgent`, before sending the message, check if memory is disabled and if so, reset the agent's session:

```go
// Add to AgentRuntime struct:
memoryEnabled bool

// Add to NewAgentRuntime:
func NewAgentRuntime(apiURL, apiKey string, memoryEnabled bool) *AgentRuntime {
	return &AgentRuntime{
		apiURL:        apiURL,
		apiKey:        apiKey,
		memoryEnabled: memoryEnabled,
		// ...
	}
}

// In RunAgent, before sending message:
if !r.memoryEnabled {
	// Reset session to prevent memory accumulation
	r.resetSession(ctx, agentID)
}

// New method:
func (r *AgentRuntime) resetSession(ctx context.Context, agentID string) {
	req, _ := http.NewRequestWithContext(ctx, "POST", r.apiURL+"/api/agents/"+agentID+"/session/reset", nil)
	r.setHeaders(req)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		slog.Debug("openfang: session reset failed", "agent_id", agentID, "error", err)
		return
	}
	resp.Body.Close()
}
```

- [ ] **Step 3: Update `main.go` to read memory setting from tenant settings**

When creating the OpenFang runtime, read the tenant's memory preference:

```go
// For now, default to false (memory disabled)
// In production, this would be read per-campaign from tenant settings
agentRuntime = openfang.NewAgentRuntime(cfg.OpenFangAPIURL, cfg.OpenFangAPIKey, false)
```

- [ ] **Step 4: Verify build and commit**

```bash
go build ./...
git add deploy/openfang/config.toml internal/adapter/openfang/agent_runtime.go apps/api/main.go
git commit -m "feat: add configurable agent memory — disabled by default, session reset between evaluations"
```

---

## Task 5: Wire Everything + Settings API

**Files:**
- Create: `internal/api/handler/settings_handler.go`
- Modify: `internal/api/router.go`
- Modify: `apps/api/main.go`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Create `internal/api/handler/settings_handler.go`**

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type SettingsHandler struct {
	svc *service.TenantSettingsService
}

func NewSettingsHandler(svc *service.TenantSettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	settings, err := h.svc.Get(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	var settings domain.TenantSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	settings.TenantID = ac.TenantID
	if err := h.svc.Update(r.Context(), &settings); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, settings)
}
```

- [ ] **Step 2: Add routes and handlers to router**

In `internal/api/router.go`, add `Settings *handler.SettingsHandler` to Handlers struct, and routes:

```go
r.Get("/settings", h.Settings.Get)
r.Put("/settings", h.Settings.Update)
```

- [ ] **Step 3: Add TELEGRAM_BOT_TOKEN to docker-compose.yml**

```yaml
      TELEGRAM_BOT_TOKEN: ${TELEGRAM_BOT_TOKEN:-}
```

- [ ] **Step 4: Add TelegramBotToken to config.go**

```go
TelegramBotToken string
```

With: `TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),`

- [ ] **Step 5: Wire everything in `main.go`**

```go
// Tenant settings
tenantSettingsRepo := postgres.NewTenantSettingsRepo(pool)
tenantSettingsSvc := service.NewTenantSettingsService(tenantSettingsRepo)

// Telegram bot (if token configured)
if cfg.TelegramBotToken != "" {
    cmdHandler := telegram.NewCommandHandler(dealSvc, campaignSvc, brandBlocklistSvc, tenantSettingsSvc, devTenantID)
    telegramBot := telegram.NewBot(cfg.TelegramBotToken, cmdHandler)
    telegramNotifications := telegram.NewNotificationService(telegramBot, tenantSettingsSvc)
    go telegramBot.StartPolling(ctx)
    _ = telegramNotifications // available for Inngest workflows
    slog.Info("telegram bot started", "bot", "@amazonagent_bot")
}

// Add to handlers:
Settings: handler.NewSettingsHandler(tenantSettingsSvc),
```

- [ ] **Step 6: Verify build and commit**

```bash
go build ./...
go test ./... -count=1
git add -A
git commit -m "feat: wire Telegram bot + tenant settings API + Docker config"
```

---

## Task 6: Docker Rebuild + End-to-End Test

- [ ] **Step 1: Rebuild and start**

```bash
docker compose up --build -d api
```

- [ ] **Step 2: Test Telegram bot**

Open Telegram, find @amazonagent_bot, send `/start`. Should receive welcome message and register chat ID.

Then test commands:
- `/status` — dashboard summary
- `/deals` — pending deals
- `/campaigns` — recent campaigns
- `/blocklist` — blocked brands
- `/help` — command list

- [ ] **Step 3: Test settings API**

```bash
curl -s -H "Authorization: Bearer dev-token" http://localhost:8081/settings | python3 -m json.tool
```

- [ ] **Step 4: Commit**

```bash
git commit --allow-empty -m "verified: Telegram bot + settings API working end-to-end"
```

---

## Self-Review

**Spec coverage:**
- Tenant settings with memory toggle (default off): Task 1 ✓
- Telegram bot with commands (/status, /deals, /campaigns, /blocklist): Task 2 ✓
- Notifications (campaign complete, new deals, nightly scan): Task 3 ✓
- OpenFang memory configurable, session reset when disabled: Task 4 ✓
- Settings API (GET/PUT /settings): Task 5 ✓
- Docker wiring: Task 5 ✓
- E2E test: Task 6 ✓

**Not in this plan (future):**
- Complex queries through OpenFang agent (conversational)
- Nightly scan Inngest cron that triggers notifications (Phase 2)
- Multi-tenant Telegram (mapping chat IDs to tenants)
- Inline keyboard buttons for approve/reject from Telegram

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/exa"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/firecrawl"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/inngest"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/openfang"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/posthog"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/postgres"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/simulator"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/spapi"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/supabase"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/telegram"
	"github.com/pluriza/fba-agent-orchestrator/internal/api"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/config"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting", "env", cfg.Env, "port", cfg.Port)

	ctx := context.Background()

	// Database
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Repos
	campaignRepo := postgres.NewCampaignRepo(pool)
	dealRepo := postgres.NewDealRepo(pool)
	eventRepo := postgres.NewEventRepo(pool)
	scoringRepo := postgres.NewScoringConfigRepo(pool)
	discoveryRepo := postgres.NewDiscoveryConfigRepo(pool)

	// Adapters
	authProvider := supabase.NewAuthProviderWithURL(cfg.SupabaseJWTSecret, cfg.SupabaseURL, cfg.SupabaseAnonKey, cfg.IsDev())
	analyticsProvider := posthog.NewAnalyticsProvider(cfg.PostHogAPIKey, cfg.PostHogHost, cfg.IsDev())

	// Agent runtime: use simulator in dev, OpenFang in production
	var agentRuntime port.AgentRuntime
	if cfg.OpenFangAPIURL != "" {
		agentRuntime = openfang.NewAgentRuntime(cfg.OpenFangAPIURL, cfg.OpenFangAPIKey, false)
		slog.Info("using OpenFang agent runtime", "url", cfg.OpenFangAPIURL)
	} else {
		agentRuntime = simulator.NewAgentRuntime()
		slog.Info("using simulated agent runtime (set OPENFANG_API_URL to use OpenFang)")
	}

	// ID generator
	idGen := port.UUIDGenerator{}

	// Tool clients (pre-resolve external data for agents)
	spapiClient := spapi.NewClient(cfg.SPAPIClientID, cfg.SPAPIClientSecret, cfg.SPAPIRefreshToken, cfg.SPAPIMarketplace, cfg.SPAPISellerID)
	exaClient := exa.NewClient(cfg.ExaAPIKey)
	firecrawlClient := firecrawl.NewClient(cfg.FirecrawlAPIKey)
	toolResolver := service.NewToolResolver(spapiClient, exaClient, firecrawlClient)

	// Services
	eventSvc := service.NewEventService(eventRepo, analyticsProvider, idGen)
	scoringSvc := service.NewScoringService(scoringRepo, idGen)
	dealSvc := service.NewDealService(dealRepo, eventSvc, idGen)
	orchestrator := service.NewPipelineOrchestrator(agentRuntime, toolResolver)
	pipelineSvc := service.NewPipelineService(orchestrator, campaignRepo, scoringRepo, dealSvc)
	brandBlocklistRepo := postgres.NewBrandBlocklistRepo(pool)
	brandBlocklistSvc := service.NewBrandBlocklistService(brandBlocklistRepo, idGen)
	brandRepo := postgres.NewBrandRepo(pool)
	brandEligibilitySvc := service.NewBrandEligibilityService(brandRepo, spapiClient, 7*24*time.Hour)
	productDiscovery := service.NewProductDiscovery(spapiClient, brandEligibilitySvc)
	tenantSettingsRepo := postgres.NewTenantSettingsRepo(pool)
	tenantSettingsSvc := service.NewTenantSettingsService(tenantSettingsRepo)

	// Durable runtime (Inngest) — optional, falls back to goroutine if unavailable
	var durableRuntime *inngest.DurableRuntime
	inngestRuntime, err := inngest.NewDurableRuntime(
		pipelineSvc, orchestrator, toolResolver,
		productDiscovery, brandBlocklistSvc,
		campaignRepo, scoringRepo, dealSvc,
	)
	if err != nil {
		slog.Warn("inngest not available, using goroutine fallback", "error", err)
	} else {
		durableRuntime = inngestRuntime
	}

	// If Inngest is unavailable, pass pipeline for goroutine fallback
	var fallbackPipeline *service.PipelineService
	if durableRuntime == nil {
		fallbackPipeline = pipelineSvc
	}

	campaignSvc := service.NewCampaignService(campaignRepo, scoringRepo, eventSvc, durableRuntime, fallbackPipeline, idGen)
	discoverySvc := service.NewDiscoveryService(discoveryRepo)

	// Seed default scoring config for the default tenant
	defaultTenantID := domain.TenantID("00000000-0000-0000-0000-000000000010")
	if err := scoringSvc.EnsureDefault(ctx, defaultTenantID); err != nil {
		slog.Warn("failed to seed scoring config", "error", err)
	} else {
		slog.Info("scoring config ready", "tenant_id", defaultTenantID)
	}

	// Handlers
	handlers := api.Handlers{
		Health:         handler.NewHealthHandler(),
		Campaign:       handler.NewCampaignHandler(campaignSvc),
		Deal:           handler.NewDealHandler(dealSvc),
		Scoring:        handler.NewScoringHandler(scoringSvc),
		Discovery:      handler.NewDiscoveryHandler(discoverySvc),
		Event:          handler.NewEventHandler(eventSvc),
		Dashboard:      handler.NewDashboardHandler(campaignSvc, dealSvc),
		BrandBlocklist: handler.NewBrandBlocklistHandler(brandBlocklistSvc),
		PriceList:      handler.NewPriceListHandler(service.NewPriceListScanner(spapiClient)),
		Settings:       handler.NewSettingsHandler(tenantSettingsSvc),
	}

	router := api.NewRouter(handlers, authProvider, idGen)

	// Mount Inngest handler (if available)
	if durableRuntime != nil {
		router.Mount("/api/inngest", durableRuntime.Handler())
	}

	// Telegram bot (if token configured)
	if cfg.TelegramBotToken != "" {
		devTenantID := domain.TenantID("00000000-0000-0000-0000-000000000010")
		cmdHandler := telegram.NewCommandHandler(dealSvc, campaignSvc, brandBlocklistSvc, tenantSettingsSvc, devTenantID)
		telegramBot := telegram.NewBot(cfg.TelegramBotToken, cmdHandler)
		go telegramBot.StartPolling(ctx)
		slog.Info("telegram bot started", "bot", "@amazonagent_bot")
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting server", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

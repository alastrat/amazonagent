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
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/inngest"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/openfang"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/posthog"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/postgres"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/simulator"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/supabase"
	"github.com/pluriza/fba-agent-orchestrator/internal/api"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/config"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Database
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
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
	authProvider := supabase.NewAuthProvider(cfg.SupabaseJWTSecret, cfg.IsDev())
	analyticsProvider := posthog.NewAnalyticsProvider(cfg.PostHogAPIKey, cfg.PostHogHost, cfg.IsDev())

	// Agent runtime: use simulator in dev, OpenFang in production
	var agentRuntime port.AgentRuntime
	if cfg.OpenFangAPIURL != "" {
		agentRuntime = openfang.NewAgentRuntime(cfg.OpenFangAPIURL, cfg.OpenFangAPIKey)
		slog.Info("using OpenFang agent runtime", "url", cfg.OpenFangAPIURL)
	} else {
		agentRuntime = simulator.NewAgentRuntime()
		slog.Info("using simulated agent runtime (set OPENFANG_API_URL to use OpenFang)")
	}

	// ID generator
	idGen := port.UUIDGenerator{}

	// Services
	eventSvc := service.NewEventService(eventRepo, analyticsProvider, idGen)
	scoringSvc := service.NewScoringService(scoringRepo, idGen)
	dealSvc := service.NewDealService(dealRepo, eventSvc, idGen)
	pipelineSvc := service.NewPipelineService(agentRuntime, campaignRepo, scoringRepo, dealSvc)

	// Durable runtime (Inngest) — needs pipelineSvc for workflow registration
	durableRuntime, err := inngest.NewDurableRuntime(pipelineSvc)
	if err != nil {
		slog.Error("failed to create inngest runtime", "error", err)
		os.Exit(1)
	}

	campaignSvc := service.NewCampaignService(campaignRepo, scoringRepo, eventSvc, durableRuntime, idGen)
	discoverySvc := service.NewDiscoveryService(discoveryRepo)

	// Seed default scoring config for dev tenant
	if cfg.IsDev() {
		devTenantID := domain.TenantID("00000000-0000-0000-0000-000000000010")
		if err := scoringSvc.EnsureDefault(ctx, devTenantID); err != nil {
			slog.Warn("failed to seed dev scoring config", "error", err)
		} else {
			slog.Info("dev scoring config ready", "tenant_id", devTenantID)
		}
	}

	// Handlers
	handlers := api.Handlers{
		Health:    handler.NewHealthHandler(),
		Campaign:  handler.NewCampaignHandler(campaignSvc),
		Deal:      handler.NewDealHandler(dealSvc),
		Scoring:   handler.NewScoringHandler(scoringSvc),
		Discovery: handler.NewDiscoveryHandler(discoverySvc),
		Event:     handler.NewEventHandler(eventSvc),
		Dashboard: handler.NewDashboardHandler(campaignSvc, dealSvc),
	}

	router := api.NewRouter(handlers, authProvider, idGen)

	// Mount Inngest handler — Inngest dev server calls this to execute functions
	router.Mount("/api/inngest", durableRuntime.Handler())

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

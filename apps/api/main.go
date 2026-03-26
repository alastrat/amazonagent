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
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/supabase"
	"github.com/pluriza/fba-agent-orchestrator/internal/api"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/config"
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
	agentRuntime := openfang.NewAgentRuntime(cfg.OpenFangAPIURL, cfg.OpenFangAPIKey)
	durableRuntime := inngest.NewDurableRuntime(cfg.InngestEventKey, cfg.InngestDev)

	// ID generator
	idGen := port.UUIDGenerator{}

	// Services
	eventSvc := service.NewEventService(eventRepo, analyticsProvider, idGen)
	scoringSvc := service.NewScoringService(scoringRepo, idGen)
	dealSvc := service.NewDealService(dealRepo, eventSvc, idGen)
	campaignSvc := service.NewCampaignService(campaignRepo, scoringRepo, eventSvc, durableRuntime, idGen)
	discoverySvc := service.NewDiscoveryService(discoveryRepo)
	_ = service.NewPipelineService(agentRuntime, campaignRepo, scoringRepo, dealSvc)

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

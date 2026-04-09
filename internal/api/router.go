package api

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type Handlers struct {
	Health         *handler.HealthHandler
	Campaign       *handler.CampaignHandler
	Deal           *handler.DealHandler
	Scoring        *handler.ScoringHandler
	Discovery      *handler.DiscoveryHandler
	Event          *handler.EventHandler
	Dashboard      *handler.DashboardHandler
	BrandBlocklist *handler.BrandBlocklistHandler
	PriceList *handler.PriceListHandler
	Settings  *handler.SettingsHandler
	Scan      *handler.ScanHandler
	Catalog   *handler.CatalogHandler
	Credit     *handler.CreditHandler
	Assessment *handler.AssessmentHandler
	Strategy   *handler.StrategyHandler
	Suggestion *handler.SuggestionHandler
}

func NewRouter(h Handlers, auth port.AuthProvider, idGen port.IDGenerator) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "https://localhost:*", "https://*.up.railway.app", "https://*.workers.dev"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(middleware.RequestID(idGen))

	r.Get("/health", h.Health.Health)
	r.Get("/ready", h.Health.Ready)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		r.Use(middleware.RequireTenant)

		r.Post("/campaigns", h.Campaign.Create)
		r.Get("/campaigns", h.Campaign.List)
		r.Get("/campaigns/{id}", h.Campaign.GetByID)

		r.Get("/deals", h.Deal.List)
		r.Get("/deals/{id}", h.Deal.GetByID)
		r.Post("/deals/{id}/approve", h.Deal.Approve)
		r.Post("/deals/{id}/reject", h.Deal.Reject)

		r.Get("/config/scoring", h.Scoring.Get)
		r.Put("/config/scoring", h.Scoring.Update)

		r.Get("/discovery", h.Discovery.Get)
		r.Put("/discovery", h.Discovery.Update)

		r.Get("/events", h.Event.List)

		r.Get("/dashboard/summary", h.Dashboard.Summary)

		r.Post("/pricelist/upload", h.PriceList.Upload)
		r.Post("/pricelist/upload-funnel", h.PriceList.UploadWithFunnel)
		r.Get("/scans", h.PriceList.ListScanJobs)
		r.Get("/scans/{id}", h.PriceList.GetScanJob)
		r.Post("/scans/category", h.Scan.TriggerCategoryScan)

		r.Get("/catalog/products", h.Catalog.ListProducts)
		r.Get("/catalog/brands", h.Catalog.ListBrands)
		r.Get("/catalog/brands/{id}/products", h.Catalog.ListBrandProducts)
		r.Get("/catalog/stats", h.Catalog.Stats)

		r.Get("/credits", h.Credit.GetBalance)
		r.Get("/credits/transactions", h.Credit.GetTransactions)

		r.Post("/assessment/start", h.Assessment.Start)
		r.Get("/assessment/status", h.Assessment.GetStatus)
		r.Get("/assessment/profile", h.Assessment.GetProfile)

		r.Get("/strategy", h.Strategy.GetActive)
		r.Get("/strategy/versions", h.Strategy.ListVersions)
		r.Get("/strategy/versions/{id}", h.Strategy.GetVersion)
		r.Post("/strategy/versions/{id}/activate", h.Strategy.ActivateVersion)
		r.Post("/strategy/versions/{id}/rollback", h.Strategy.RollbackToVersion)

		r.Get("/suggestions", h.Suggestion.ListPending)
		r.Get("/suggestions/all", h.Suggestion.ListAll)
		r.Post("/suggestions/{id}/accept", h.Suggestion.Accept)
		r.Post("/suggestions/{id}/dismiss", h.Suggestion.Dismiss)

		r.Get("/settings", h.Settings.Get)
		r.Put("/settings", h.Settings.Update)

		r.Get("/brand-blocklist", h.BrandBlocklist.List)
		r.Post("/brand-blocklist", h.BrandBlocklist.Add)
		r.Delete("/brand-blocklist", h.BrandBlocklist.Remove)
	})

	return r
}

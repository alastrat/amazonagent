package api

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type Handlers struct {
	Health    *handler.HealthHandler
	Campaign  *handler.CampaignHandler
	Deal      *handler.DealHandler
	Scoring   *handler.ScoringHandler
	Discovery *handler.DiscoveryHandler
	Event     *handler.EventHandler
	Dashboard *handler.DashboardHandler
}

func NewRouter(h Handlers, auth port.AuthProvider, idGen port.IDGenerator) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
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
	})

	return r
}

package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type EventService struct {
	repo      port.EventRepo
	analytics port.AnalyticsProvider
	idGen     port.IDGenerator
}

func NewEventService(repo port.EventRepo, analytics port.AnalyticsProvider, idGen port.IDGenerator) *EventService {
	return &EventService{repo: repo, analytics: analytics, idGen: idGen}
}

func (s *EventService) Emit(ctx context.Context, tenantID domain.TenantID, eventType, entityType, entityID, actorID string, payload map[string]any) error {
	event := &domain.DomainEvent{
		ID:            domain.DomainEventID(s.idGen.New()),
		TenantID:      tenantID,
		EventType:     eventType,
		EntityType:    entityType,
		EntityID:      entityID,
		Payload:       payload,
		CorrelationID: s.idGen.New(),
		ActorID:       actorID,
		Timestamp:     time.Now(),
	}

	if err := s.repo.Create(ctx, event); err != nil {
		return err
	}

	if s.analytics != nil {
		if err := s.analytics.CaptureEvent(ctx, string(tenantID), eventType, payload); err != nil {
			slog.Warn("failed to capture analytics event", "event_type", eventType, "error", err)
		}
	}

	return nil
}

func (s *EventService) List(ctx context.Context, tenantID domain.TenantID, filter port.EventFilter) ([]domain.DomainEvent, error) {
	return s.repo.List(ctx, tenantID, filter)
}

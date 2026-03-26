package domain

import "time"

type DomainEventID string

type DomainEvent struct {
	ID            DomainEventID  `json:"id"`
	TenantID      TenantID       `json:"tenant_id"`
	EventType     string         `json:"event_type"`
	EntityType    string         `json:"entity_type"`
	EntityID      string         `json:"entity_id"`
	Payload       map[string]any `json:"payload"`
	CorrelationID string         `json:"correlation_id"`
	ActorID       string         `json:"actor_id"`
	Timestamp     time.Time      `json:"timestamp"`
}

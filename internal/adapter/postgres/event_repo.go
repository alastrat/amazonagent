package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type EventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepo(pool *pgxpool.Pool) *EventRepo {
	return &EventRepo{pool: pool}
}

func (r *EventRepo) Create(ctx context.Context, e *domain.DomainEvent) error {
	payload, _ := json.Marshal(e.Payload)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO domain_events (id, tenant_id, event_type, entity_type, entity_id, payload, correlation_id, actor_id, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, e.ID, e.TenantID, e.EventType, e.EntityType, e.EntityID, payload, e.CorrelationID, e.ActorID, e.Timestamp)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (r *EventRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.EventFilter) ([]domain.DomainEvent, error) {
	query := `SELECT id, tenant_id, event_type, entity_type, entity_id, payload, correlation_id, actor_id, timestamp
		FROM domain_events WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if filter.EntityType != nil {
		query += fmt.Sprintf(" AND entity_type = $%d", argIdx)
		args = append(args, *filter.EntityType)
		argIdx++
	}
	if filter.EntityID != nil {
		query += fmt.Sprintf(" AND entity_id = $%d", argIdx)
		args = append(args, *filter.EntityID)
		argIdx++
	}
	if filter.EventType != nil {
		query += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, *filter.EventType)
		argIdx++
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []domain.DomainEvent
	for rows.Next() {
		var e domain.DomainEvent
		var payloadJSON []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.EntityType, &e.EntityID, &payloadJSON, &e.CorrelationID, &e.ActorID, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		json.Unmarshal(payloadJSON, &e.Payload)
		events = append(events, e)
	}
	return events, nil
}

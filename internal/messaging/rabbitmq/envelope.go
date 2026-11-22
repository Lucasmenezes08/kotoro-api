package rabbitmq

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
)

type EventEnvelope struct {
	ID            uuid.UUID       `json:"id"`
	UserID        uuid.UUID       `json:"user_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	EventVersion  int             `json:"event_version"`
	Payload       json.RawMessage `json:"payload"`
	CorrelationID *uuid.UUID      `json:"correlation_id,omitempty"`
	CausationID   *uuid.UUID      `json:"causation_id,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
}

func BuildEnvelope(event outbox.OutboxEvent) (EventEnvelope, error) {
	if event.ID == uuid.Nil {
		return EventEnvelope{}, fmt.Errorf("event ID is required")
	}
	if event.UserID == uuid.Nil {
		return EventEnvelope{}, fmt.Errorf("user ID is required")
	}
	if event.EventType == "" {
		return EventEnvelope{}, fmt.Errorf("event type is required")
	}
	if len(event.Payload) == 0 {
		return EventEnvelope{}, fmt.Errorf("payload is required")
	}

	version := event.EventVersion
	if version <= 0 {
		version = 1
	}

	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	return EventEnvelope{
		ID:            event.ID,
		UserID:        event.UserID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		EventVersion:  version,
		Payload:       event.Payload,
		CorrelationID: event.CorrelationID,
		CausationID:   event.CausationID,
		OccurredAt:    occurredAt.UTC(),
	}, nil
}

func BuildRoutingKey(eventType string, eventVersion int) string {
	version := eventVersion
	if version <= 0 {
		version = 1
	}
	return eventType + ".v" + strconv.Itoa(version)
}

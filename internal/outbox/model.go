package outbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OutboxEventStatus string

const (
	StatusPending    OutboxEventStatus = "pending"
	StatusProcessing OutboxEventStatus = "processing"
	StatusPublished  OutboxEventStatus = "published"
	StatusFailed     OutboxEventStatus = "failed"
)

type EventStatus = OutboxEventStatus

const (
	EventStatusPending    = StatusPending
	EventStatusProcessing = StatusProcessing
	EventStatusPublished  = StatusPublished
	EventStatusFailed     = StatusFailed
)

const (
	AggregateTypeSprint      = "sprint"
	AggregateTypeStudyCycle  = "study_cycle"
	AggregateTypeSubject     = "subject"
	AggregateTypeSprintBlock = "sprint_block"
	AggregateTypeBlockTask   = "block_task"
)

const (
	EventTypeStudyCycleStarted   = "study_cycle.started"
	EventTypeStudyCyclePaused    = "study_cycle.paused"
	EventTypeStudyCycleResumed   = "study_cycle.resumed"
	EventTypeStudyCycleFinished  = "study_cycle.finished"
	EventTypeStudyCycleCancelled = "study_cycle.cancelled"

	EventTypeSprintStarted   = "sprint.started"
	EventTypeSprintFinished  = "sprint.finished"
	EventTypeSprintAbandoned = "sprint.abandoned"

	EventTypeBlockTaskCompleted = "block_task.completed"
)

var (
	ErrEventNotFound           = errors.New("outbox event not found")
	ErrInvalidEventStatus      = errors.New("invalid outbox event status")
	ErrInvalidEventPayload     = errors.New("invalid outbox event payload")
	ErrEventAlreadyPublished   = errors.New("outbox event is already published")
	ErrEventMaxAttemptsReached = errors.New("outbox event reached maximum attempts")
	ErrMissingRequiredField    = errors.New("missing required field for outbox event")
)

type OutboxEvent struct {
	ID            uuid.UUID         `db:"id" json:"id"`
	UserID        uuid.UUID         `db:"user_id" json:"user_id"`
	AggregateType string            `db:"aggregate_type" json:"aggregate_type"`
	AggregateID   uuid.UUID         `db:"aggregate_id" json:"aggregate_id"`
	EventType     string            `db:"event_type" json:"event_type"`
	EventVersion  int               `db:"event_version" json:"event_version"`
	Payload       json.RawMessage   `db:"payload" json:"payload"`
	CorrelationID *uuid.UUID        `db:"correlation_id" json:"correlation_id,omitempty"`
	CausationID   *uuid.UUID        `db:"causation_id" json:"causation_id,omitempty"`
	Status        OutboxEventStatus `db:"status" json:"status"`
	OccurredAt    time.Time         `db:"occurred_at" json:"occurred_at"`
	AvailableAt   time.Time         `db:"available_at" json:"available_at"`
	PublishedAt   *time.Time        `db:"published_at" json:"published_at,omitempty"`
	Attempts      int               `db:"attempts" json:"attempts"`
	LastError     *string           `db:"last_error" json:"last_error,omitempty"`
	LockedAt      *time.Time        `db:"locked_at" json:"locked_at,omitempty"`
	LockedBy      *string           `db:"locked_by" json:"locked_by,omitempty"`
	CreatedAt     time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time         `db:"updated_at" json:"updated_at"`
}

func (e *OutboxEvent) IsPending() bool {
	return e.Status == StatusPending
}

func (e *OutboxEvent) IsProcessing() bool {
	return e.Status == StatusProcessing
}

func (e *OutboxEvent) IsPublished() bool {
	return e.Status == StatusPublished
}

func (e *OutboxEvent) IsFailed() bool {
	return e.Status == StatusFailed
}

func (e *OutboxEvent) CanRetry(maxAttempts int) bool {
	if e.Status == StatusPublished {
		return false
	}
	if maxAttempts <= 0 {
		return true
	}
	return e.Attempts < maxAttempts
}

func (e *OutboxEvent) UnmarshalPayload(target any) error {
	if len(e.Payload) == 0 {
		return ErrInvalidEventPayload
	}
	return json.Unmarshal(e.Payload, target)
}

type CreateEventInput struct {
	UserID        uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	EventVersion  int
	Payload       any
	CorrelationID *uuid.UUID
	CausationID   *uuid.UUID
	OccurredAt    *time.Time
	AvailableAt   *time.Time
}

type CreateEventModel struct {
	ID            uuid.UUID         `db:"id"`
	UserID        uuid.UUID         `db:"user_id"`
	AggregateType string            `db:"aggregate_type"`
	AggregateID   uuid.UUID         `db:"aggregate_id"`
	EventType     string            `db:"event_type"`
	EventVersion  int               `db:"event_version"`
	Payload       json.RawMessage   `db:"payload"`
	CorrelationID *uuid.UUID        `db:"correlation_id"`
	CausationID   *uuid.UUID        `db:"causation_id"`
	Status        OutboxEventStatus `db:"status"`
	OccurredAt    time.Time         `db:"occurred_at"`
	AvailableAt   time.Time         `db:"available_at"`
	Attempts      int               `db:"attempts"`
	CreatedAt     time.Time         `db:"created_at"`
	UpdatedAt     time.Time         `db:"updated_at"`
}

func NewCreateEventModel(input CreateEventInput) (*CreateEventModel, error) {
	if input.UserID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrMissingRequiredField)
	}
	if input.AggregateType == "" {
		return nil, fmt.Errorf("%w: aggregate_type is required", ErrMissingRequiredField)
	}
	if input.AggregateID == uuid.Nil {
		return nil, fmt.Errorf("%w: aggregate_id is required", ErrMissingRequiredField)
	}
	if input.EventType == "" {
		return nil, fmt.Errorf("%w: event_type is required", ErrMissingRequiredField)
	}
	if input.Payload == nil {
		return nil, fmt.Errorf("%w: payload is required", ErrMissingRequiredField)
	}

	var rawPayload json.RawMessage
	switch p := input.Payload.(type) {
	case json.RawMessage:
		if !json.Valid(p) {
			return nil, ErrInvalidEventPayload
		}
		rawPayload = p
	case []byte:
		if !json.Valid(p) {
			return nil, ErrInvalidEventPayload
		}
		rawPayload = json.RawMessage(p)
	case string:
		b := []byte(p)
		if !json.Valid(b) {
			return nil, ErrInvalidEventPayload
		}
		rawPayload = json.RawMessage(b)
	default:
		encoded, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidEventPayload, err)
		}
		rawPayload = json.RawMessage(encoded)
	}

	version := input.EventVersion
	if version <= 0 {
		version = 1
	}

	now := time.Now().UTC()
	occurredAt := now
	if input.OccurredAt != nil {
		occurredAt = input.OccurredAt.UTC()
	}

	availableAt := occurredAt
	if input.AvailableAt != nil {
		availableAt = input.AvailableAt.UTC()
	}

	return &CreateEventModel{
		ID:            uuid.New(),
		UserID:        input.UserID,
		AggregateType: input.AggregateType,
		AggregateID:   input.AggregateID,
		EventType:     input.EventType,
		EventVersion:  version,
		Payload:       rawPayload,
		CorrelationID: input.CorrelationID,
		CausationID:   input.CausationID,
		Status:        StatusPending,
		OccurredAt:    occurredAt,
		AvailableAt:   availableAt,
		Attempts:      0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

type LockEventsFilter struct {
	Limit        int
	LockedBy     string
	LockDuration time.Duration
	Now          time.Time
}

type MarkPublishedModel struct {
	ID          uuid.UUID
	PublishedAt time.Time
}

type MarkFailedModel struct {
	ID          uuid.UUID
	LastError   string
	NextAttempt *time.Time
	MaxAttempts int
}

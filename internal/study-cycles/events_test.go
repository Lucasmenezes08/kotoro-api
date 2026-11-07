package studycycles

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
)

func TestNewStudyCycleFinishedEvent(t *testing.T) {
	userID := uuid.New()
	subjectID := uuid.New()
	cycleID := uuid.New()
	completedTaskID := uuid.New()
	startedAt := time.Date(2026, 11, 4, 8, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	endedAt := startedAt.Add(25 * time.Minute)
	occurredAt := endedAt.Add(time.Second)
	reason := CycleStopReasonCompleted
	cycle := StudyCycle{
		ID:             cycleID,
		SprintID:       uuid.New(),
		SprintBlockID:  uuid.New(),
		SequenceNumber: 3,
		ExecutionMode:  ExecutionModeSingleTask,
		TimerType:      TimerTypeTimed,
		StartedAt:      startedAt,
		EndedAt:        &endedAt,
		ElapsedSeconds: 1500,
		StopReason:     &reason,
	}

	event, err := NewStudyCycleFinishedEvent(userID, cycle, subjectID, []uuid.UUID{completedTaskID}, occurredAt)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if event.ID == uuid.Nil || event.UserID != userID || event.AggregateType != outbox.AggregateTypeStudyCycle || event.AggregateID != cycleID || event.EventType != StudyCycleFinishedEventType || event.EventVersion != StudyCycleFinishedEventVersion {
		t.Fatalf("unexpected event envelope: %#v", event)
	}
	if !event.OccurredAt.Equal(occurredAt.UTC()) {
		t.Fatalf("occurred_at = %v, want %v", event.OccurredAt, occurredAt.UTC())
	}

	var payload StudyCycleFinishedEventV1
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.SubjectID != subjectID || payload.ElapsedSeconds != 1500 || len(payload.CompletedTaskIDs) != 1 || payload.CompletedTaskIDs[0] != completedTaskID {
		t.Fatalf("unexpected event payload: %#v", payload)
	}
	if !payload.StartedAt.Equal(startedAt) || !payload.EndedAt.Equal(endedAt) {
		t.Fatalf("unexpected payload timestamps: %#v", payload)
	}
}

func TestNewStudyCycleFinishedEventValidation(t *testing.T) {
	cycle := StudyCycle{}
	occurredAt := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	if _, err := NewStudyCycleFinishedEvent(uuid.New(), cycle, uuid.New(), []uuid.UUID{}, occurredAt); !errors.Is(err, ErrFinishedCycleWithoutEndedAt) {
		t.Fatalf("expected ended_at error, got %v", err)
	}

	endedAt := occurredAt
	cycle.EndedAt = &endedAt
	if _, err := NewStudyCycleFinishedEvent(uuid.New(), cycle, uuid.New(), []uuid.UUID{}, occurredAt); !errors.Is(err, ErrFinishedCycleWithoutStopReason) {
		t.Fatalf("expected stop_reason error, got %v", err)
	}
}

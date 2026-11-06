package outbox

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOutboxEventStatusHelpers(t *testing.T) {
	tests := []struct {
		name         string
		status       OutboxEventStatus
		isPending    bool
		isProcessing bool
		isPublished  bool
		isFailed     bool
	}{
		{
			name:         "pending status",
			status:       StatusPending,
			isPending:    true,
			isProcessing: false,
			isPublished:  false,
			isFailed:     false,
		},
		{
			name:         "processing status",
			status:       StatusProcessing,
			isPending:    false,
			isProcessing: true,
			isPublished:  false,
			isFailed:     false,
		},
		{
			name:         "published status",
			status:       StatusPublished,
			isPending:    false,
			isProcessing: false,
			isPublished:  true,
			isFailed:     false,
		},
		{
			name:         "failed status",
			status:       StatusFailed,
			isPending:    false,
			isProcessing: false,
			isPublished:  false,
			isFailed:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := OutboxEvent{Status: tt.status}
			if got := event.IsPending(); got != tt.isPending {
				t.Errorf("IsPending() = %v, want %v", got, tt.isPending)
			}
			if got := event.IsProcessing(); got != tt.isProcessing {
				t.Errorf("IsProcessing() = %v, want %v", got, tt.isProcessing)
			}
			if got := event.IsPublished(); got != tt.isPublished {
				t.Errorf("IsPublished() = %v, want %v", got, tt.isPublished)
			}
			if got := event.IsFailed(); got != tt.isFailed {
				t.Errorf("IsFailed() = %v, want %v", got, tt.isFailed)
			}
		})
	}
}

func TestOutboxEventCanRetry(t *testing.T) {
	t.Run("published event cannot retry", func(t *testing.T) {
		event := OutboxEvent{Status: StatusPublished, Attempts: 1}
		if event.CanRetry(5) {
			t.Error("expected published event not to be retryable")
		}
	})

	t.Run("attempts below max can retry", func(t *testing.T) {
		event := OutboxEvent{Status: StatusPending, Attempts: 2}
		if !event.CanRetry(3) {
			t.Error("expected event with 2 attempts and max 3 to be retryable")
		}
	})

	t.Run("attempts at or above max cannot retry", func(t *testing.T) {
		event := OutboxEvent{Status: StatusFailed, Attempts: 3}
		if event.CanRetry(3) {
			t.Error("expected event at max attempts not to be retryable")
		}
	})

	t.Run("maxAttempts <= 0 means unlimited retries for non-published", func(t *testing.T) {
		event := OutboxEvent{Status: StatusPending, Attempts: 100}
		if !event.CanRetry(0) {
			t.Error("expected unlimited retries when maxAttempts is 0")
		}
	})
}

func TestOutboxEventUnmarshalPayload(t *testing.T) {
	type SamplePayload struct {
		CycleID uuid.UUID `json:"cycle_id"`
		Elapsed int       `json:"elapsed_seconds"`
	}

	cycleID := uuid.New()
	raw, err := json.Marshal(SamplePayload{CycleID: cycleID, Elapsed: 3600})
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}

	event := OutboxEvent{Payload: raw}

	var parsed SamplePayload
	if err := event.UnmarshalPayload(&parsed); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if parsed.CycleID != cycleID || parsed.Elapsed != 3600 {
		t.Fatalf("unexpected payload values: %#v", parsed)
	}

	t.Run("empty payload returns error", func(t *testing.T) {
		emptyEvent := OutboxEvent{}
		var target SamplePayload
		if err := emptyEvent.UnmarshalPayload(&target); !errors.Is(err, ErrInvalidEventPayload) {
			t.Fatalf("expected ErrInvalidEventPayload, got %v", err)
		}
	})
}

func TestNewCreateEventModel(t *testing.T) {
	userID := uuid.New()
	aggID := uuid.New()
	corrID := uuid.New()
	causID := uuid.New()
	now := time.Now().UTC()

	t.Run("valid input with struct payload", func(t *testing.T) {
		type PayloadData struct {
			Message string `json:"message"`
		}

		model, err := NewCreateEventModel(CreateEventInput{
			UserID:        userID,
			AggregateType: AggregateTypeSprint,
			AggregateID:   aggID,
			EventType:     EventTypeSprintFinished,
			EventVersion:  2,
			Payload:       PayloadData{Message: "finished"},
			CorrelationID: &corrID,
			CausationID:   &causID,
			OccurredAt:    &now,
			AvailableAt:   &now,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if model.ID == uuid.Nil {
			t.Error("expected generated ID")
		}
		if model.UserID != userID {
			t.Errorf("got UserID %v, want %v", model.UserID, userID)
		}
		if model.AggregateType != AggregateTypeSprint {
			t.Errorf("got AggregateType %v, want %v", model.AggregateType, AggregateTypeSprint)
		}
		if model.AggregateID != aggID {
			t.Errorf("got AggregateID %v, want %v", model.AggregateID, aggID)
		}
		if model.EventType != EventTypeSprintFinished {
			t.Errorf("got EventType %v, want %v", model.EventType, EventTypeSprintFinished)
		}
		if model.EventVersion != 2 {
			t.Errorf("got EventVersion %v, want 2", model.EventVersion)
		}
		if model.Status != StatusPending {
			t.Errorf("got Status %v, want %v", model.Status, StatusPending)
		}
		if model.Attempts != 0 {
			t.Errorf("got Attempts %v, want 0", model.Attempts)
		}
		if string(model.Payload) != `{"message":"finished"}` {
			t.Errorf("got Payload %s, want %s", string(model.Payload), `{"message":"finished"}`)
		}
	})

	t.Run("defaults EventVersion to 1 when <= 0", func(t *testing.T) {
		model, err := NewCreateEventModel(CreateEventInput{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   aggID,
			EventType:     EventTypeStudyCycleFinished,
			EventVersion:  0,
			Payload:       map[string]any{"ok": true},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if model.EventVersion != 1 {
			t.Errorf("got EventVersion %d, want 1", model.EventVersion)
		}
	})

	t.Run("validates required fields", func(t *testing.T) {
		base := CreateEventInput{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   aggID,
			EventType:     EventTypeStudyCycleFinished,
			Payload:       map[string]any{"ok": true},
		}

		cases := []struct {
			name   string
			mutate func(in *CreateEventInput)
		}{
			{"missing user_id", func(in *CreateEventInput) { in.UserID = uuid.Nil }},
			{"missing aggregate_type", func(in *CreateEventInput) { in.AggregateType = "" }},
			{"missing aggregate_id", func(in *CreateEventInput) { in.AggregateID = uuid.Nil }},
			{"missing event_type", func(in *CreateEventInput) { in.EventType = "" }},
			{"missing payload", func(in *CreateEventInput) { in.Payload = nil }},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				in := base
				tc.mutate(&in)
				_, err := NewCreateEventModel(in)
				if !errors.Is(err, ErrMissingRequiredField) {
					t.Fatalf("expected ErrMissingRequiredField, got %v", err)
				}
			})
		}
	})

	t.Run("rejects invalid json payloads", func(t *testing.T) {
		_, err := NewCreateEventModel(CreateEventInput{
			UserID:        userID,
			AggregateType: AggregateTypeSprint,
			AggregateID:   aggID,
			EventType:     EventTypeSprintStarted,
			Payload:       `{invalid json`,
		})
		if !errors.Is(err, ErrInvalidEventPayload) {
			t.Fatalf("expected ErrInvalidEventPayload, got %v", err)
		}
	})
}

package studycycles

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
)

const (
	StudyCycleFinishedEventType    = "study_cycle.finished"
	StudyCycleFinishedEventVersion = 1
)

var ErrFinishedCycleWithoutEndedAt = errors.New(
	"finished study cycle must have ended_at",
)

var ErrFinishedCycleWithoutStopReason = errors.New(
	"finished study cycle must have stop_reason",
)

type StudyCycleFinishedEventV1 struct {
	CycleID          uuid.UUID       `json:"cycle_id"`
	SprintID         uuid.UUID       `json:"sprint_id"`
	SprintBlockID    uuid.UUID       `json:"sprint_block_id"`
	SubjectID        uuid.UUID       `json:"subject_id"`
	SequenceNumber   int             `json:"sequence_number"`
	ExecutionMode    ExecutionMode   `json:"execution_mode"`
	TimerType        TimerType       `json:"timer_type"`
	StartedAt        time.Time       `json:"started_at"`
	EndedAt          time.Time       `json:"ended_at"`
	ElapsedSeconds   int             `json:"elapsed_seconds"`
	StopReason       CycleStopReason `json:"stop_reason"`
	CompletedTaskIDs []uuid.UUID     `json:"completed_task_ids"`
}

func NewStudyCycleFinishedEvent(
	userID uuid.UUID,
	cycle StudyCycle,
	subjectID uuid.UUID,
	completedTaskIDs []uuid.UUID,
	occurredAt time.Time,
) (outbox.Event, error) {
	if cycle.EndedAt == nil {
		return outbox.Event{},
			ErrFinishedCycleWithoutEndedAt
	}

	if cycle.StopReason == nil {
		return outbox.Event{}, ErrFinishedCycleWithoutStopReason
	}
	if completedTaskIDs == nil {
		completedTaskIDs = make([]uuid.UUID, 0)
	}

	eventPayload := StudyCycleFinishedEventV1{
		CycleID:          cycle.ID,
		SprintID:         cycle.SprintID,
		SprintBlockID:    cycle.SprintBlockID,
		SubjectID:        subjectID,
		SequenceNumber:   cycle.SequenceNumber,
		ExecutionMode:    cycle.ExecutionMode,
		TimerType:        cycle.TimerType,
		StartedAt:        cycle.StartedAt,
		EndedAt:          cycle.EndedAt.UTC(),
		ElapsedSeconds:   cycle.ElapsedSeconds,
		StopReason:       *cycle.StopReason,
		CompletedTaskIDs: completedTaskIDs,
	}

	payload, err := json.Marshal(eventPayload)
	if err != nil {
		return outbox.Event{}, err
	}

	return outbox.Event{
		ID:            uuid.New(),
		UserID:        userID,
		AggregateType: outbox.AggregateTypeStudyCycle,
		AggregateID:   cycle.ID,
		EventType:     StudyCycleFinishedEventType,
		EventVersion:  StudyCycleFinishedEventVersion,
		Payload:       payload,
		CorrelationID: nil,
		CausationID:   nil,
		OccurredAt:    occurredAt.UTC(),
	}, nil
}

package studycycles

import (
	"time"

	"github.com/google/uuid"
)

type ExecutionMode string

const (
	ExecutionModeSubjectBlock  ExecutionMode = "bloco_materia"
	ExecutionModeSingleTask    ExecutionMode = "tarefa_individual"
	ExecutionModeMultipleTasks ExecutionMode = "multiplas_tarefas"
)

type TimerType string

const (
	TimerTypeFree  TimerType = "cronometro_livre"
	TimerTypeTimed TimerType = "temporizado"
)

type StudyCycleStatus string

const (
	StudyCycleInProgress StudyCycleStatus = "em_andamento"
	StudyCyclePaused     StudyCycleStatus = "pausado"
	StudyCycleFinished   StudyCycleStatus = "finalizado"
	StudyCycleCancelled  StudyCycleStatus = "cancelado"
)

type CycleStopReason string

const (
	CycleStopReasonTimeExpired CycleStopReason = "tempo_esgotado"
	CycleStopReasonCompleted   CycleStopReason = "concluido"
	CycleStopReasonCancelled   CycleStopReason = "cancelado"
)

type StudyCycle struct {
	ID                     uuid.UUID        `db:"id"`
	SprintID               uuid.UUID        `db:"sprint_id"`
	SprintBlockID          uuid.UUID        `db:"sprint_block_id"`
	SequenceNumber         int              `db:"sequence_number"`
	ExecutionMode          ExecutionMode    `db:"execution_mode"`
	TimerType              TimerType        `db:"timer_type"`
	Status                 StudyCycleStatus `db:"status"`
	PlannedDurationSeconds *int             `db:"planned_duration_seconds"`
	StartedAt              time.Time        `db:"started_at"`
	RunningSince           *time.Time       `db:"running_since"`
	PausedAt               *time.Time       `db:"paused_at"`
	EndedAt                *time.Time       `db:"ended_at"`
	ElapsedSeconds         int              `db:"elapsed_seconds"`
	StopReason             *CycleStopReason `db:"stop_reason"`
	CreatedAt              time.Time        `db:"created_at"`
	UpdatedAt              *time.Time       `db:"updated_at"`
}

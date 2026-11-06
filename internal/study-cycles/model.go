package studycycles

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const maxPostgresInteger = 1<<31 - 1

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
	StudyCycleCanceled   StudyCycleStatus = StudyCycleCancelled
)

type CycleStopReason string

const (
	CycleStopReasonTimeExpired CycleStopReason = "tempo_esgotado"
	CycleStopReasonCompleted   CycleStopReason = "concluido"
	CycleStopReasonCancelled   CycleStopReason = "cancelado"
	CycleStopReasonCanceled    CycleStopReason = CycleStopReasonCancelled
)

type transitionError struct {
	msg     string
	targets []error
}

func (e *transitionError) Error() string {
	return e.msg
}

func (e *transitionError) Unwrap() []error {
	return e.targets
}

var (
	ErrStudyCycleNotFound          = errors.New("study cycle not found")
	ErrStudyCycleAlreadyActive     = errors.New("sprint already has an active study cycle")
	ErrStudyCycleInvalidTransition = errors.New("invalid study cycle status transition")
	ErrStudyCycleNotRunning        = &transitionError{
		msg:     "study cycle is not running",
		targets: []error{ErrStudyCycleInvalidTransition},
	}
	ErrStudyCycleNotPaused = &transitionError{
		msg:     "study cycle is not paused",
		targets: []error{ErrStudyCycleInvalidTransition},
	}
	ErrStudyCycleAlreadyStopped = &transitionError{
		msg:     "study cycle is already stopped",
		targets: []error{ErrStudyCycleInvalidTransition},
	}
	errPauseStoppedCycle = &transitionError{
		msg:     ErrStudyCycleAlreadyStopped.Error(),
		targets: []error{ErrStudyCycleAlreadyStopped, ErrStudyCycleNotRunning, ErrStudyCycleInvalidTransition},
	}
	errResumeStoppedCycle = &transitionError{
		msg:     ErrStudyCycleAlreadyStopped.Error(),
		targets: []error{ErrStudyCycleAlreadyStopped, ErrStudyCycleNotPaused, ErrStudyCycleInvalidTransition},
	}
	ErrSprintNotStarted          = errors.New("sprint must be in progress to start a study cycle")
	ErrSprintHasNoAvailableBlock = errors.New("sprint has no available block for execution")
	ErrInvalidExecutionMode      = errors.New("invalid execution mode")
	ErrInvalidTimerConfiguration = errors.New("invalid timer configuration")
	ErrInvalidSelectedTasks      = errors.New("invalid selected tasks for execution mode")
	ErrInvalidStopReason         = errors.New("invalid study cycle stop reason")
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

type StudyCycleTask struct {
	StudyCycleID uuid.UUID  `db:"study_cycle_id"`
	BlockTaskID  uuid.UUID  `db:"block_task_id"`
	CompletedAt  *time.Time `db:"completed_at"`
	CreatedAt    time.Time  `db:"created_at"`
}

type PlayStudyCycleInput struct {
	ExecutionMode          ExecutionMode
	TimerType              TimerType
	PlannedDurationSeconds *int
	BlockTaskIDs           []uuid.UUID
}

type StopStudyCycleInput struct {
	Reason CycleStopReason
}

type CreateStudyCycleModel struct {
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
}

type PauseStudyCycleModel struct {
	ID             uuid.UUID        `db:"id"`
	ExpectedStatus StudyCycleStatus `db:"expected_status"`
	Status         StudyCycleStatus `db:"status"`
	PausedAt       time.Time        `db:"paused_at"`
	ElapsedSeconds int              `db:"elapsed_seconds"`
}

type ResumeStudyCycleModel struct {
	ID             uuid.UUID        `db:"id"`
	ExpectedStatus StudyCycleStatus `db:"expected_status"`
	Status         StudyCycleStatus `db:"status"`
	RunningSince   time.Time        `db:"running_since"`
	StartedAt      time.Time        `db:"started_at"`
	ElapsedSeconds int              `db:"elapsed_seconds"`
}

type StopStudyCycleModel struct {
	ID             uuid.UUID        `db:"id"`
	ExpectedStatus StudyCycleStatus `db:"expected_status"`
	Status         StudyCycleStatus `db:"status"`
	EndedAt        time.Time        `db:"ended_at"`
	ElapsedSeconds int              `db:"elapsed_seconds"`
	StopReason     CycleStopReason  `db:"stop_reason"`
}

type StudyCycleResponse struct {
	ID                     uuid.UUID        `json:"id"`
	SprintID               uuid.UUID        `json:"sprint_id"`
	SprintBlockID          uuid.UUID        `json:"sprint_block_id"`
	SequenceNumber         int              `json:"sequence_number"`
	ExecutionMode          ExecutionMode    `json:"execution_mode"`
	TimerType              TimerType        `json:"timer_type"`
	Status                 StudyCycleStatus `json:"status"`
	PlannedDurationSeconds *int             `json:"planned_duration_seconds"`
	StartedAt              time.Time        `json:"started_at"`
	RunningSince           *time.Time       `json:"running_since"`
	PausedAt               *time.Time       `json:"paused_at"`
	EndedAt                *time.Time       `json:"ended_at"`
	ElapsedSeconds         int              `json:"elapsed_seconds"`
	StopReason             *CycleStopReason `json:"stop_reason"`
	ServerNow              time.Time        `json:"server_now"`
}

func responseFromModel(cycle *StudyCycle, serverNow time.Time) StudyCycleResponse {
	return StudyCycleResponse{
		ID:                     cycle.ID,
		SprintID:               cycle.SprintID,
		SprintBlockID:          cycle.SprintBlockID,
		SequenceNumber:         cycle.SequenceNumber,
		ExecutionMode:          cycle.ExecutionMode,
		TimerType:              cycle.TimerType,
		Status:                 cycle.Status,
		PlannedDurationSeconds: cycle.PlannedDurationSeconds,
		StartedAt:              cycle.StartedAt,
		RunningSince:           cycle.RunningSince,
		PausedAt:               cycle.PausedAt,
		EndedAt:                cycle.EndedAt,
		ElapsedSeconds:         cycle.ElapsedSeconds,
		StopReason:             cycle.StopReason,
		ServerNow:              serverNow.UTC(),
	}
}

func validExecutionMode(mode ExecutionMode) bool {
	switch mode {
	case ExecutionModeSubjectBlock, ExecutionModeSingleTask, ExecutionModeMultipleTasks:
		return true
	default:
		return false
	}
}

func validTimerType(timerType TimerType) bool {
	switch timerType {
	case TimerTypeFree, TimerTypeTimed:
		return true
	default:
		return false
	}
}

func validStopReason(reason CycleStopReason) bool {
	switch reason {
	case CycleStopReasonCompleted, CycleStopReasonTimeExpired, CycleStopReasonCancelled:
		return true
	default:
		return false
	}
}

func terminalStatusForReason(reason CycleStopReason) (StudyCycleStatus, error) {
	switch reason {
	case CycleStopReasonCompleted, CycleStopReasonTimeExpired:
		return StudyCycleFinished, nil
	case CycleStopReasonCancelled:
		return StudyCycleCancelled, nil
	default:
		return "", ErrInvalidStopReason
	}
}

func validatePlayInput(input PlayStudyCycleInput) error {
	if !validExecutionMode(input.ExecutionMode) {
		return ErrInvalidExecutionMode
	}
	if !validTimerType(input.TimerType) {
		return ErrInvalidTimerConfiguration
	}

	switch input.TimerType {
	case TimerTypeTimed:
		if input.PlannedDurationSeconds == nil || *input.PlannedDurationSeconds <= 0 || *input.PlannedDurationSeconds > maxPostgresInteger {
			return ErrInvalidTimerConfiguration
		}
	case TimerTypeFree:
		if input.PlannedDurationSeconds != nil {
			return ErrInvalidTimerConfiguration
		}
	}

	switch input.ExecutionMode {
	case ExecutionModeSubjectBlock:
		if len(input.BlockTaskIDs) > 0 {
			return ErrInvalidSelectedTasks
		}
	case ExecutionModeSingleTask:
		if len(input.BlockTaskIDs) != 1 || input.BlockTaskIDs[0] == uuid.Nil {
			return ErrInvalidSelectedTasks
		}
	case ExecutionModeMultipleTasks:
		if len(input.BlockTaskIDs) == 0 {
			return ErrInvalidSelectedTasks
		}
		seen := make(map[uuid.UUID]struct{}, len(input.BlockTaskIDs))
		for _, id := range input.BlockTaskIDs {
			if id == uuid.Nil {
				return ErrInvalidSelectedTasks
			}
			if _, exists := seen[id]; exists {
				return ErrInvalidSelectedTasks
			}
			seen[id] = struct{}{}
		}
	}

	return nil
}

func (c *StudyCycle) IsOpen() bool {
	return c.Status == StudyCycleInProgress || c.Status == StudyCyclePaused
}

func (c *StudyCycle) IsTerminal() bool {
	return c.Status == StudyCycleFinished || c.Status == StudyCycleCancelled
}

func (c *StudyCycle) ApplyPause(now time.Time) (StudyCycle, error) {
	switch c.Status {
	case StudyCycleInProgress:
		pausedAt := now.UTC()
		updated := *c
		updated.ElapsedSeconds = c.ElapsedSeconds + calculateAdditionalElapsedSeconds(c.RunningSince, pausedAt)
		updated.Status = StudyCyclePaused
		updated.PausedAt = &pausedAt
		updated.RunningSince = nil
		updated.UpdatedAt = &pausedAt
		return updated, nil
	case StudyCyclePaused:
		return StudyCycle{}, ErrStudyCycleNotRunning
	case StudyCycleFinished, StudyCycleCancelled:
		return StudyCycle{}, errPauseStoppedCycle
	default:
		return StudyCycle{}, ErrStudyCycleInvalidTransition
	}
}

func (c *StudyCycle) ApplyResume(now time.Time) (StudyCycle, error) {
	switch c.Status {
	case StudyCyclePaused:
		resumedAt := now.UTC()
		updated := *c
		updated.Status = StudyCycleInProgress
		updated.RunningSince = &resumedAt
		updated.PausedAt = nil
		updated.UpdatedAt = &resumedAt
		return updated, nil
	case StudyCycleInProgress:
		return StudyCycle{}, ErrStudyCycleNotPaused
	case StudyCycleFinished, StudyCycleCancelled:
		return StudyCycle{}, errResumeStoppedCycle
	default:
		return StudyCycle{}, ErrStudyCycleInvalidTransition
	}
}

func (c *StudyCycle) ApplyStop(now time.Time, reason CycleStopReason) (StudyCycle, error) {
	terminalStatus, err := terminalStatusForReason(reason)
	if err != nil {
		return StudyCycle{}, err
	}

	switch c.Status {
	case StudyCycleInProgress:
		endedAt := now.UTC()
		stopReason := reason
		updated := *c
		if terminalStatus == StudyCycleCancelled {
			updated.ElapsedSeconds = 0
		} else {
			updated.ElapsedSeconds = c.ElapsedSeconds + calculateAdditionalElapsedSeconds(c.RunningSince, endedAt)
		}
		updated.Status = terminalStatus
		updated.RunningSince = nil
		updated.EndedAt = &endedAt
		updated.StopReason = &stopReason
		updated.UpdatedAt = &endedAt
		return updated, nil
	case StudyCyclePaused:
		endedAt := now.UTC()
		stopReason := reason
		updated := *c
		if terminalStatus == StudyCycleCancelled {
			updated.ElapsedSeconds = 0
		}
		updated.Status = terminalStatus
		updated.RunningSince = nil
		updated.EndedAt = &endedAt
		updated.StopReason = &stopReason
		updated.UpdatedAt = &endedAt
		return updated, nil
	case StudyCycleFinished, StudyCycleCancelled:
		return StudyCycle{}, ErrStudyCycleAlreadyStopped
	default:
		return StudyCycle{}, ErrStudyCycleInvalidTransition
	}
}

func calculateAdditionalElapsedSeconds(runningSince *time.Time, now time.Time) int {
	if runningSince == nil {
		return 0
	}
	delta := int(now.UTC().Sub(runningSince.UTC()).Seconds())
	if delta < 0 {
		return 0
	}
	return delta
}

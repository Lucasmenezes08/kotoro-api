package studycycles

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidatePlayInput(t *testing.T) {
	validDuration := 1500
	zeroDuration := 0
	negativeDuration := -30
	task1 := uuid.New()
	task2 := uuid.New()

	tests := []struct {
		name  string
		input PlayStudyCycleInput
		want  error
	}{
		{
			name: "valid subject block with free timer",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSubjectBlock,
				TimerType:     TimerTypeFree,
			},
			want: nil,
		},
		{
			name: "valid subject block with timed timer",
			input: PlayStudyCycleInput{
				ExecutionMode:          ExecutionModeSubjectBlock,
				TimerType:              TimerTypeTimed,
				PlannedDurationSeconds: &validDuration,
			},
			want: nil,
		},
		{
			name: "valid single task mode",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSingleTask,
				TimerType:     TimerTypeFree,
				BlockTaskIDs:  []uuid.UUID{task1},
			},
			want: nil,
		},
		{
			name: "valid multiple tasks mode",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeMultipleTasks,
				TimerType:     TimerTypeFree,
				BlockTaskIDs:  []uuid.UUID{task1, task2},
			},
			want: nil,
		},
		{
			name: "invalid execution mode",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionMode("invalido"),
				TimerType:     TimerTypeFree,
			},
			want: ErrInvalidExecutionMode,
		},
		{
			name: "invalid timer type",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSubjectBlock,
				TimerType:     TimerType("invalido"),
			},
			want: ErrInvalidTimerConfiguration,
		},
		{
			name: "timed without duration",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSubjectBlock,
				TimerType:     TimerTypeTimed,
			},
			want: ErrInvalidTimerConfiguration,
		},
		{
			name: "timed with zero duration",
			input: PlayStudyCycleInput{
				ExecutionMode:          ExecutionModeSubjectBlock,
				TimerType:              TimerTypeTimed,
				PlannedDurationSeconds: &zeroDuration,
			},
			want: ErrInvalidTimerConfiguration,
		},
		{
			name: "timed with negative duration",
			input: PlayStudyCycleInput{
				ExecutionMode:          ExecutionModeSubjectBlock,
				TimerType:              TimerTypeTimed,
				PlannedDurationSeconds: &negativeDuration,
			},
			want: ErrInvalidTimerConfiguration,
		},
		{
			name: "free timer with duration",
			input: PlayStudyCycleInput{
				ExecutionMode:          ExecutionModeSubjectBlock,
				TimerType:              TimerTypeFree,
				PlannedDurationSeconds: &validDuration,
			},
			want: ErrInvalidTimerConfiguration,
		},
		{
			name: "subject block with tasks",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSubjectBlock,
				TimerType:     TimerTypeFree,
				BlockTaskIDs:  []uuid.UUID{task1},
			},
			want: ErrInvalidSelectedTasks,
		},
		{
			name: "single task with no tasks",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSingleTask,
				TimerType:     TimerTypeFree,
			},
			want: ErrInvalidSelectedTasks,
		},
		{
			name: "single task with multiple tasks",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSingleTask,
				TimerType:     TimerTypeFree,
				BlockTaskIDs:  []uuid.UUID{task1, task2},
			},
			want: ErrInvalidSelectedTasks,
		},
		{
			name: "multiple tasks with empty slice",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeMultipleTasks,
				TimerType:     TimerTypeFree,
			},
			want: ErrInvalidSelectedTasks,
		},
		{
			name: "multiple tasks with duplicates",
			input: PlayStudyCycleInput{
				ExecutionMode: ExecutionModeMultipleTasks,
				TimerType:     TimerTypeFree,
				BlockTaskIDs:  []uuid.UUID{task1, task1},
			},
			want: ErrInvalidSelectedTasks,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePlayInput(tc.input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected error %v, got %v", tc.want, err)
			}
		})
	}
}

func TestStudyCycleStateTransitions(t *testing.T) {
	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(90 * time.Second)
	t2 := t1.Add(300 * time.Second)
	t3 := t2.Add(60 * time.Second)

	cycle := StudyCycle{
		ID:             uuid.New(),
		SprintID:       uuid.New(),
		SprintBlockID:  uuid.New(),
		SequenceNumber: 1,
		ExecutionMode:  ExecutionModeSubjectBlock,
		TimerType:      TimerTypeFree,
		Status:         StudyCycleInProgress,
		StartedAt:      t0,
		RunningSince:   &t0,
		ElapsedSeconds: 0,
	}

	paused, err := cycle.ApplyPause(t1)
	if err != nil {
		t.Fatalf("expected pause to succeed, got %v", err)
	}
	if paused.Status != StudyCyclePaused || paused.ElapsedSeconds != 90 || paused.RunningSince != nil || paused.PausedAt == nil || !paused.PausedAt.Equal(t1) {
		t.Fatalf("unexpected paused state: %#v", paused)
	}

	if _, err := paused.ApplyPause(t2); !errors.Is(err, ErrStudyCycleNotRunning) || !errors.Is(err, ErrStudyCycleInvalidTransition) {
		t.Fatalf("expected ErrStudyCycleNotRunning on second pause, got %v", err)
	}

	resumed, err := paused.ApplyResume(t2)
	if err != nil {
		t.Fatalf("expected resume to succeed, got %v", err)
	}
	if resumed.Status != StudyCycleInProgress || resumed.ElapsedSeconds != 90 || !resumed.StartedAt.Equal(t0) || resumed.PausedAt != nil || resumed.RunningSince == nil || !resumed.RunningSince.Equal(t2) {
		t.Fatalf("unexpected resumed state: %#v", resumed)
	}

	if _, err := resumed.ApplyResume(t3); !errors.Is(err, ErrStudyCycleNotPaused) || !errors.Is(err, ErrStudyCycleInvalidTransition) {
		t.Fatalf("expected ErrStudyCycleNotPaused on in-progress resume, got %v", err)
	}

	stopped, err := resumed.ApplyStop(t3, CycleStopReasonCompleted)
	if err != nil {
		t.Fatalf("expected stop to succeed, got %v", err)
	}
	if stopped.Status != StudyCycleFinished || stopped.ElapsedSeconds != 150 || stopped.RunningSince != nil || stopped.EndedAt == nil || !stopped.EndedAt.Equal(t3) || stopped.StopReason == nil || *stopped.StopReason != CycleStopReasonCompleted {
		t.Fatalf("unexpected stopped state: %#v", stopped)
	}

	if _, err := stopped.ApplyPause(t3); !errors.Is(err, ErrStudyCycleAlreadyStopped) || !errors.Is(err, ErrStudyCycleNotRunning) {
		t.Fatalf("expected ErrStudyCycleAlreadyStopped on terminal pause, got %v", err)
	}
	if _, err := stopped.ApplyResume(t3); !errors.Is(err, ErrStudyCycleAlreadyStopped) || !errors.Is(err, ErrStudyCycleNotPaused) {
		t.Fatalf("expected ErrStudyCycleAlreadyStopped on terminal resume, got %v", err)
	}
	if _, err := stopped.ApplyStop(t3, CycleStopReasonCancelled); !errors.Is(err, ErrStudyCycleAlreadyStopped) {
		t.Fatalf("expected ErrStudyCycleAlreadyStopped on terminal stop, got %v", err)
	}
}

func TestStudyCycle_ApplyStop_CancelledZeroesElapsedSeconds(t *testing.T) {
	t0 := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Minute)

	// Stopping in-progress cycle as cancelled
	cycle := StudyCycle{
		ID:             uuid.New(),
		Status:         StudyCycleInProgress,
		StartedAt:      t0,
		RunningSince:   &t0,
		ElapsedSeconds: 300,
	}

	cancelled, err := cycle.ApplyStop(t1, CycleStopReasonCancelled)
	if err != nil {
		t.Fatalf("expected stop to succeed, got %v", err)
	}
	if cancelled.Status != StudyCycleCancelled {
		t.Fatalf("expected status cancelado, got %s", cancelled.Status)
	}
	if cancelled.ElapsedSeconds != 0 {
		t.Fatalf("expected elapsed seconds to be 0 for cancelled cycle, got %d", cancelled.ElapsedSeconds)
	}

	// Stopping paused cycle as cancelled
	pausedCycle := StudyCycle{
		ID:             uuid.New(),
		Status:         StudyCyclePaused,
		StartedAt:      t0,
		PausedAt:       &t1,
		ElapsedSeconds: 600,
	}

	cancelledPaused, err := pausedCycle.ApplyStop(t1, CycleStopReasonCancelled)
	if err != nil {
		t.Fatalf("expected stop to succeed, got %v", err)
	}
	if cancelledPaused.Status != StudyCycleCancelled {
		t.Fatalf("expected status cancelado, got %s", cancelledPaused.Status)
	}
	if cancelledPaused.ElapsedSeconds != 0 {
		t.Fatalf("expected elapsed seconds to be 0 for cancelled paused cycle, got %d", cancelledPaused.ElapsedSeconds)
	}
}

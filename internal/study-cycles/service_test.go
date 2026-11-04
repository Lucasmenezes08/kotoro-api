package studycycles

import (
	"context"
	"errors"
	"testing"
	"time"

	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
)

type fakeStudyCycleTxRepository struct {
	lockSprintByIDFn          func(ctx context.Context, sprintID uuid.UUID) (*sprint.Sprint, error)
	hasActiveCycleBySprintFn  func(ctx context.Context, sprintID uuid.UUID) (bool, error)
	lockBlocksBySprintIDFn    func(ctx context.Context, sprintID uuid.UUID) ([]sprintblocks.SprintBlock, error)
	getActiveTasksByBlockIDFn func(ctx context.Context, blockID uuid.UUID) ([]blocktasks.BlockTask, error)
	getNextSequenceNumberFn   func(ctx context.Context, sprintID uuid.UUID) (int, error)
	createCycleFn             func(ctx context.Context, input CreateStudyCycleModel) (*StudyCycle, error)
	attachCycleTasksFn        func(ctx context.Context, cycleID uuid.UUID, taskIDs []uuid.UUID) error
	updateBlockStatusFn       func(ctx context.Context, blockID uuid.UUID, status sprintblocks.SprintBlocksStatus, startedAt, completedAt *time.Time) error
	updateTasksStatusFn       func(ctx context.Context, blockID uuid.UUID, taskIDs []uuid.UUID, status blocktasks.BlockTaskStatus, completedAt *time.Time) error
	lockCycleByIDFn           func(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error)
	getCycleTaskIDsFn         func(ctx context.Context, cycleID uuid.UUID) ([]uuid.UUID, error)
	completeCycleTasksFn      func(ctx context.Context, cycleID uuid.UUID, completedAt time.Time) error
	stopCycleConditionalFn    func(ctx context.Context, input StopStudyCycleModel) (*StudyCycle, error)
}

func (f *fakeStudyCycleTxRepository) LockSprintByID(ctx context.Context, sprintID uuid.UUID) (*sprint.Sprint, error) {
	if f.lockSprintByIDFn == nil {
		panic("unexpected call to LockSprintByID")
	}
	return f.lockSprintByIDFn(ctx, sprintID)
}

func (f *fakeStudyCycleTxRepository) HasActiveCycleBySprintID(ctx context.Context, sprintID uuid.UUID) (bool, error) {
	if f.hasActiveCycleBySprintFn == nil {
		panic("unexpected call to HasActiveCycleBySprintID")
	}
	return f.hasActiveCycleBySprintFn(ctx, sprintID)
}

func (f *fakeStudyCycleTxRepository) LockBlocksBySprintID(ctx context.Context, sprintID uuid.UUID) ([]sprintblocks.SprintBlock, error) {
	if f.lockBlocksBySprintIDFn == nil {
		panic("unexpected call to LockBlocksBySprintID")
	}
	return f.lockBlocksBySprintIDFn(ctx, sprintID)
}

func (f *fakeStudyCycleTxRepository) GetActiveTasksByBlockID(ctx context.Context, blockID uuid.UUID) ([]blocktasks.BlockTask, error) {
	if f.getActiveTasksByBlockIDFn == nil {
		panic("unexpected call to GetActiveTasksByBlockID")
	}
	return f.getActiveTasksByBlockIDFn(ctx, blockID)
}

func (f *fakeStudyCycleTxRepository) GetNextSequenceNumber(ctx context.Context, sprintID uuid.UUID) (int, error) {
	if f.getNextSequenceNumberFn == nil {
		panic("unexpected call to GetNextSequenceNumber")
	}
	return f.getNextSequenceNumberFn(ctx, sprintID)
}

func (f *fakeStudyCycleTxRepository) CreateCycle(ctx context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
	if f.createCycleFn == nil {
		panic("unexpected call to CreateCycle")
	}
	return f.createCycleFn(ctx, input)
}

func (f *fakeStudyCycleTxRepository) AttachCycleTasks(ctx context.Context, cycleID uuid.UUID, taskIDs []uuid.UUID) error {
	if f.attachCycleTasksFn == nil {
		panic("unexpected call to AttachCycleTasks")
	}
	return f.attachCycleTasksFn(ctx, cycleID, taskIDs)
}

func (f *fakeStudyCycleTxRepository) UpdateBlockStatus(ctx context.Context, blockID uuid.UUID, status sprintblocks.SprintBlocksStatus, startedAt, completedAt *time.Time) error {
	if f.updateBlockStatusFn == nil {
		panic("unexpected call to UpdateBlockStatus")
	}
	return f.updateBlockStatusFn(ctx, blockID, status, startedAt, completedAt)
}

func (f *fakeStudyCycleTxRepository) UpdateTasksStatus(ctx context.Context, blockID uuid.UUID, taskIDs []uuid.UUID, status blocktasks.BlockTaskStatus, completedAt *time.Time) error {
	if f.updateTasksStatusFn == nil {
		panic("unexpected call to UpdateTasksStatus")
	}
	return f.updateTasksStatusFn(ctx, blockID, taskIDs, status, completedAt)
}

func (f *fakeStudyCycleTxRepository) LockCycleByID(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error) {
	if f.lockCycleByIDFn == nil {
		panic("unexpected call to LockCycleByID")
	}
	return f.lockCycleByIDFn(ctx, cycleID)
}

func (f *fakeStudyCycleTxRepository) GetCycleTaskIDs(ctx context.Context, cycleID uuid.UUID) ([]uuid.UUID, error) {
	if f.getCycleTaskIDsFn == nil {
		panic("unexpected call to GetCycleTaskIDs")
	}
	return f.getCycleTaskIDsFn(ctx, cycleID)
}

func (f *fakeStudyCycleTxRepository) CompleteCycleTasks(ctx context.Context, cycleID uuid.UUID, completedAt time.Time) error {
	if f.completeCycleTasksFn == nil {
		panic("unexpected call to CompleteCycleTasks")
	}
	return f.completeCycleTasksFn(ctx, cycleID, completedAt)
}

func (f *fakeStudyCycleTxRepository) StopCycleConditional(ctx context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
	if f.stopCycleConditionalFn == nil {
		panic("unexpected call to StopCycleConditional")
	}
	return f.stopCycleConditionalFn(ctx, input)
}

type fakeStudyCycleRepository struct {
	tx                  *fakeStudyCycleTxRepository
	withinTransactionFn func(ctx context.Context, fn func(ctx context.Context, tx StudyCycleTxRepositoryContract) error) error
	getByIDFn           func(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error)
	pauseConditionalFn  func(ctx context.Context, input PauseStudyCycleModel) (*StudyCycle, error)
	resumeConditionalFn func(ctx context.Context, input ResumeStudyCycleModel) (*StudyCycle, error)
	committed           bool
	rolledBack          bool
}

func (f *fakeStudyCycleRepository) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx StudyCycleTxRepositoryContract) error) error {
	if f.withinTransactionFn != nil {
		return f.withinTransactionFn(ctx, fn)
	}
	if f.tx == nil {
		panic("unexpected call to WithinTransaction without fake tx")
	}
	if err := fn(ctx, f.tx); err != nil {
		f.rolledBack = true
		return err
	}
	f.committed = true
	return nil
}

func (f *fakeStudyCycleRepository) GetByID(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error) {
	if f.getByIDFn == nil {
		panic("unexpected call to GetByID")
	}
	return f.getByIDFn(ctx, cycleID)
}

func (f *fakeStudyCycleRepository) PauseConditional(ctx context.Context, input PauseStudyCycleModel) (*StudyCycle, error) {
	if f.pauseConditionalFn == nil {
		panic("unexpected call to PauseConditional")
	}
	return f.pauseConditionalFn(ctx, input)
}

func (f *fakeStudyCycleRepository) ResumeConditional(ctx context.Context, input ResumeStudyCycleModel) (*StudyCycle, error) {
	if f.resumeConditionalFn == nil {
		panic("unexpected call to ResumeConditional")
	}
	return f.resumeConditionalFn(ctx, input)
}

var _ StudyCycleRepositoryContract = (*fakeStudyCycleRepository)(nil)
var _ StudyCycleTxRepositoryContract = (*fakeStudyCycleTxRepository)(nil)

func TestStudyCycleServicePlay(t *testing.T) {
	fixedNow := time.Date(2026, 11, 4, 14, 0, 0, 0, time.UTC)

	t.Run("starts a cycle for a started sprint and updates pending block to in progress", func(t *testing.T) {
		sprintID := uuid.New()
		blockID := uuid.New()
		cycleID := uuid.New()
		blockStatusUpdated := false

		tx := &fakeStudyCycleTxRepository{
			lockSprintByIDFn: func(_ context.Context, id uuid.UUID) (*sprint.Sprint, error) {
				if id != sprintID {
					t.Fatalf("expected sprint %v, got %v", sprintID, id)
				}
				return &sprint.Sprint{ID: sprintID, Status: sprint.InProgress}, nil
			},
			hasActiveCycleBySprintFn: func(context.Context, uuid.UUID) (bool, error) {
				return false, nil
			},
			lockBlocksBySprintIDFn: func(context.Context, uuid.UUID) ([]sprintblocks.SprintBlock, error) {
				return []sprintblocks.SprintBlock{
					{ID: blockID, SprintID: sprintID, Position: 1, Status: sprintblocks.Pending},
				}, nil
			},
			getNextSequenceNumberFn: func(context.Context, uuid.UUID) (int, error) {
				return 1, nil
			},
			createCycleFn: func(_ context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
				if input.SprintID != sprintID || input.SprintBlockID != blockID || input.SequenceNumber != 1 {
					t.Fatalf("unexpected create input: %#v", input)
				}
				if input.Status != StudyCycleInProgress || input.ElapsedSeconds != 0 || !input.StartedAt.Equal(fixedNow) {
					t.Fatalf("unexpected initial cycle timestamps/status: %#v", input)
				}
				if input.RunningSince == nil || !input.RunningSince.Equal(fixedNow) || input.PausedAt != nil || input.EndedAt != nil {
					t.Fatalf("unexpected running_since/paused_at/ended_at: %#v", input)
				}
				return &StudyCycle{
					ID:             cycleID,
					SprintID:       input.SprintID,
					SprintBlockID:  input.SprintBlockID,
					SequenceNumber: input.SequenceNumber,
					ExecutionMode:  input.ExecutionMode,
					TimerType:      input.TimerType,
					Status:         input.Status,
					StartedAt:      input.StartedAt,
					RunningSince:   input.RunningSince,
					ElapsedSeconds: input.ElapsedSeconds,
				}, nil
			},
			updateBlockStatusFn: func(_ context.Context, gotBlockID uuid.UUID, status sprintblocks.SprintBlocksStatus, startedAt, completedAt *time.Time) error {
				if gotBlockID != blockID || status != sprintblocks.InProgress || startedAt == nil || !startedAt.Equal(fixedNow) || completedAt != nil {
					t.Fatalf("unexpected block status update: id=%v status=%v startedAt=%v completedAt=%v", gotBlockID, status, startedAt, completedAt)
				}
				blockStatusUpdated = true
				return nil
			},
		}
		repo := &fakeStudyCycleRepository{tx: tx}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

		cycle, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSubjectBlock,
			TimerType:     TimerTypeFree,
		})
		if err != nil {
			t.Fatalf("expected play to succeed, got %v", err)
		}
		if cycle.ID != cycleID || cycle.Status != StudyCycleInProgress || cycle.SequenceNumber != 1 {
			t.Fatalf("unexpected created cycle: %#v", cycle)
		}
		if !blockStatusUpdated || !repo.committed {
			t.Fatal("expected block status update and transaction commit")
		}
	})

	t.Run("rejects a sprint that is not started", func(t *testing.T) {
		for _, status := range []sprint.SprintStatus{sprint.Creating, sprint.Finished, sprint.Abandoned} {
			tx := &fakeStudyCycleTxRepository{
				lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
					return &sprint.Sprint{Status: status}, nil
				},
			}
			repo := &fakeStudyCycleRepository{tx: tx}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

			_, err := service.Play(context.Background(), uuid.New(), PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSubjectBlock,
				TimerType:     TimerTypeFree,
			})
			if !errors.Is(err, ErrSprintNotStarted) {
				t.Fatalf("for sprint status %q expected ErrSprintNotStarted, got %v", status, err)
			}
			if !repo.rolledBack {
				t.Fatal("expected rollback when sprint is not started")
			}
		}
	})

	t.Run("rejects a sprint with no available block", func(t *testing.T) {
		sprintID := uuid.New()
		tx := &fakeStudyCycleTxRepository{
			lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
				return &sprint.Sprint{ID: sprintID, Status: sprint.InProgress}, nil
			},
			hasActiveCycleBySprintFn: func(context.Context, uuid.UUID) (bool, error) {
				return false, nil
			},
			lockBlocksBySprintIDFn: func(context.Context, uuid.UUID) ([]sprintblocks.SprintBlock, error) {
				return []sprintblocks.SprintBlock{
					{ID: uuid.New(), SprintID: sprintID, Position: 1, Status: sprintblocks.Finished},
				}, nil
			},
		}
		repo := &fakeStudyCycleRepository{tx: tx}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

		_, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSubjectBlock,
			TimerType:     TimerTypeFree,
		})
		if !errors.Is(err, ErrSprintHasNoAvailableBlock) {
			t.Fatalf("expected ErrSprintHasNoAvailableBlock, got %v", err)
		}
	})

	t.Run("rejects play when an active cycle exists", func(t *testing.T) {
		sprintID := uuid.New()
		tx := &fakeStudyCycleTxRepository{
			lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
				return &sprint.Sprint{ID: sprintID, Status: sprint.InProgress}, nil
			},
			hasActiveCycleBySprintFn: func(context.Context, uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		repo := &fakeStudyCycleRepository{tx: tx}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

		_, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSubjectBlock,
			TimerType:     TimerTypeFree,
		})
		if !errors.Is(err, ErrStudyCycleAlreadyActive) {
			t.Fatalf("expected ErrStudyCycleAlreadyActive, got %v", err)
		}
	})

	t.Run("chooses the correct block by position and prioritizes in-progress over lower pending", func(t *testing.T) {
		sprintID := uuid.New()
		finishedBlock := uuid.New()
		pendingLower := uuid.New()
		inProgressHigher := uuid.New()

		t.Run("prioritizes in-progress block", func(t *testing.T) {
			tx := &fakeStudyCycleTxRepository{
				lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
					return &sprint.Sprint{ID: sprintID, Status: sprint.InProgress}, nil
				},
				hasActiveCycleBySprintFn: func(context.Context, uuid.UUID) (bool, error) { return false, nil },
				lockBlocksBySprintIDFn: func(context.Context, uuid.UUID) ([]sprintblocks.SprintBlock, error) {
					return []sprintblocks.SprintBlock{
						{ID: finishedBlock, SprintID: sprintID, Position: 1, Status: sprintblocks.Finished},
						{ID: pendingLower, SprintID: sprintID, Position: 2, Status: sprintblocks.Pending},
						{ID: inProgressHigher, SprintID: sprintID, Position: 3, Status: sprintblocks.InProgress},
					}, nil
				},
				getNextSequenceNumberFn: func(context.Context, uuid.UUID) (int, error) { return 3, nil },
				createCycleFn: func(_ context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
					if input.SprintBlockID != inProgressHigher {
						t.Fatalf("expected in-progress block %v, got %v", inProgressHigher, input.SprintBlockID)
					}
					if input.SequenceNumber != 3 {
						t.Fatalf("expected sequence 3, got %d", input.SequenceNumber)
					}
					return &StudyCycle{ID: uuid.New(), SprintID: sprintID, SprintBlockID: input.SprintBlockID, SequenceNumber: input.SequenceNumber, Status: StudyCycleInProgress}, nil
				},
			}
			repo := &fakeStudyCycleRepository{tx: tx}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

			cycle, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSubjectBlock,
				TimerType:     TimerTypeFree,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cycle.SprintBlockID != inProgressHigher || cycle.SequenceNumber != 3 {
				t.Fatalf("unexpected block/sequence: %#v", cycle)
			}
		})

		t.Run("selects lowest pending block by position when none in progress", func(t *testing.T) {
			pendingPos3 := uuid.New()
			tx := &fakeStudyCycleTxRepository{
				lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
					return &sprint.Sprint{ID: sprintID, Status: sprint.InProgress}, nil
				},
				hasActiveCycleBySprintFn: func(context.Context, uuid.UUID) (bool, error) { return false, nil },
				lockBlocksBySprintIDFn: func(context.Context, uuid.UUID) ([]sprintblocks.SprintBlock, error) {
					return []sprintblocks.SprintBlock{
						{ID: finishedBlock, SprintID: sprintID, Position: 1, Status: sprintblocks.Finished},
						{ID: pendingPos3, SprintID: sprintID, Position: 3, Status: sprintblocks.Pending},
						{ID: pendingLower, SprintID: sprintID, Position: 2, Status: sprintblocks.Pending},
					}, nil
				},
				getNextSequenceNumberFn: func(context.Context, uuid.UUID) (int, error) { return 2, nil },
				createCycleFn: func(_ context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
					if input.SprintBlockID != pendingLower {
						t.Fatalf("expected lowest pending block %v, got %v", pendingLower, input.SprintBlockID)
					}
					return &StudyCycle{ID: uuid.New(), SprintID: sprintID, SprintBlockID: input.SprintBlockID, SequenceNumber: input.SequenceNumber, Status: StudyCycleInProgress}, nil
				},
				updateBlockStatusFn: func(context.Context, uuid.UUID, sprintblocks.SprintBlocksStatus, *time.Time, *time.Time) error {
					return nil
				},
			}
			repo := &fakeStudyCycleRepository{tx: tx}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

			cycle, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
				ExecutionMode: ExecutionModeSubjectBlock,
				TimerType:     TimerTypeFree,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cycle.SprintBlockID != pendingLower || cycle.SequenceNumber != 2 {
				t.Fatalf("unexpected cycle: %#v", cycle)
			}
		})
	})

	t.Run("validates free and timed timer configurations", func(t *testing.T) {
		sprintID := uuid.New()
		blockID := uuid.New()
		duration := 1800
		invalidDuration := 0

		service := NewStudyCycleService(&fakeStudyCycleRepository{})
		if _, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSubjectBlock,
			TimerType:     TimerTypeTimed,
		}); !errors.Is(err, ErrInvalidTimerConfiguration) {
			t.Fatalf("expected ErrInvalidTimerConfiguration for timed without duration, got %v", err)
		}

		if _, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode:          ExecutionModeSubjectBlock,
			TimerType:              TimerTypeTimed,
			PlannedDurationSeconds: &invalidDuration,
		}); !errors.Is(err, ErrInvalidTimerConfiguration) {
			t.Fatalf("expected ErrInvalidTimerConfiguration for timed with 0 duration, got %v", err)
		}

		if _, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode:          ExecutionModeSubjectBlock,
			TimerType:              TimerTypeFree,
			PlannedDurationSeconds: &duration,
		}); !errors.Is(err, ErrInvalidTimerConfiguration) {
			t.Fatalf("expected ErrInvalidTimerConfiguration for free timer with duration, got %v", err)
		}

		tx := &fakeStudyCycleTxRepository{
			lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
				return &sprint.Sprint{ID: sprintID, Status: sprint.InProgress}, nil
			},
			hasActiveCycleBySprintFn: func(context.Context, uuid.UUID) (bool, error) { return false, nil },
			lockBlocksBySprintIDFn: func(context.Context, uuid.UUID) ([]sprintblocks.SprintBlock, error) {
				return []sprintblocks.SprintBlock{{ID: blockID, SprintID: sprintID, Position: 1, Status: sprintblocks.InProgress}}, nil
			},
			getNextSequenceNumberFn: func(context.Context, uuid.UUID) (int, error) { return 1, nil },
			createCycleFn: func(_ context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
				if input.PlannedDurationSeconds == nil || *input.PlannedDurationSeconds != duration {
					t.Fatalf("expected duration %d, got %v", duration, input.PlannedDurationSeconds)
				}
				return &StudyCycle{ID: uuid.New(), SprintID: sprintID, SprintBlockID: blockID, TimerType: TimerTypeTimed, PlannedDurationSeconds: input.PlannedDurationSeconds, Status: StudyCycleInProgress}, nil
			},
		}
		timedService := NewStudyCycleServiceWithClock(&fakeStudyCycleRepository{tx: tx}, func() time.Time { return fixedNow })
		cycle, err := timedService.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode:          ExecutionModeSubjectBlock,
			TimerType:              TimerTypeTimed,
			PlannedDurationSeconds: &duration,
		})
		if err != nil || cycle.PlannedDurationSeconds == nil || *cycle.PlannedDurationSeconds != duration {
			t.Fatalf("expected timed cycle to succeed, got %#v %v", cycle, err)
		}
	})

	t.Run("validates individual-task and multiple-task modes and attaches tasks", func(t *testing.T) {
		sprintID := uuid.New()
		blockID := uuid.New()
		otherBlockID := uuid.New()
		task1 := uuid.New()
		task2 := uuid.New()
		doneTask := uuid.New()
		foreignTask := uuid.New()
		var attachedTasks []uuid.UUID

		makeTx := func() *fakeStudyCycleTxRepository {
			return &fakeStudyCycleTxRepository{
				lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
					return &sprint.Sprint{ID: sprintID, Status: sprint.InProgress}, nil
				},
				hasActiveCycleBySprintFn: func(context.Context, uuid.UUID) (bool, error) { return false, nil },
				lockBlocksBySprintIDFn: func(context.Context, uuid.UUID) ([]sprintblocks.SprintBlock, error) {
					return []sprintblocks.SprintBlock{{ID: blockID, SprintID: sprintID, Position: 1, Status: sprintblocks.InProgress}}, nil
				},
				getActiveTasksByBlockIDFn: func(context.Context, uuid.UUID) ([]blocktasks.BlockTask, error) {
					return []blocktasks.BlockTask{
						{ID: task1, SprintBlockID: blockID, Status: blocktasks.TaskPending},
						{ID: task2, SprintBlockID: blockID, Status: blocktasks.TaskInProgress},
						{ID: doneTask, SprintBlockID: blockID, Status: blocktasks.TaskDone},
						{ID: foreignTask, SprintBlockID: otherBlockID, Status: blocktasks.TaskPending},
					}, nil
				},
				getNextSequenceNumberFn: func(context.Context, uuid.UUID) (int, error) { return 4, nil },
				createCycleFn: func(_ context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
					return &StudyCycle{ID: uuid.New(), SprintID: sprintID, SprintBlockID: blockID, SequenceNumber: input.SequenceNumber, ExecutionMode: input.ExecutionMode, Status: StudyCycleInProgress}, nil
				},
				attachCycleTasksFn: func(_ context.Context, _ uuid.UUID, taskIDs []uuid.UUID) error {
					attachedTasks = append([]uuid.UUID(nil), taskIDs...)
					return nil
				},
			}
		}

		// Valid individual task
		service := NewStudyCycleServiceWithClock(&fakeStudyCycleRepository{tx: makeTx()}, func() time.Time { return fixedNow })
		if _, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSingleTask,
			TimerType:     TimerTypeFree,
			BlockTaskIDs:  []uuid.UUID{task1},
		}); err != nil {
			t.Fatalf("expected individual task play to succeed, got %v", err)
		}
		if len(attachedTasks) != 1 || attachedTasks[0] != task1 {
			t.Fatalf("expected attached task %v, got %v", task1, attachedTasks)
		}

		// Valid multiple tasks
		service = NewStudyCycleServiceWithClock(&fakeStudyCycleRepository{tx: makeTx()}, func() time.Time { return fixedNow })
		if _, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeMultipleTasks,
			TimerType:     TimerTypeFree,
			BlockTaskIDs:  []uuid.UUID{task1, task2},
		}); err != nil {
			t.Fatalf("expected multiple tasks play to succeed, got %v", err)
		}
		if len(attachedTasks) != 2 || attachedTasks[0] != task1 || attachedTasks[1] != task2 {
			t.Fatalf("expected attached tasks [%v, %v], got %v", task1, task2, attachedTasks)
		}

		// Rejects missing task, completed task, or task from another block
		for _, invalidIDs := range [][]uuid.UUID{
			{uuid.New()},
			{doneTask},
			{foreignTask},
			{task1, foreignTask},
		} {
			service = NewStudyCycleServiceWithClock(&fakeStudyCycleRepository{tx: makeTx()}, func() time.Time { return fixedNow })
			mode := ExecutionModeSingleTask
			if len(invalidIDs) > 1 {
				mode = ExecutionModeMultipleTasks
			}
			if _, err := service.Play(context.Background(), sprintID, PlayStudyCycleInput{
				ExecutionMode: mode,
				TimerType:     TimerTypeFree,
				BlockTaskIDs:  invalidIDs,
			}); !errors.Is(err, ErrInvalidSelectedTasks) {
				t.Fatalf("expected ErrInvalidSelectedTasks for %v, got %v", invalidIDs, err)
			}
		}
	})

	t.Run("propagates repository errors", func(t *testing.T) {
		repoErr := errors.New("db connection lost")
		tx := &fakeStudyCycleTxRepository{
			lockSprintByIDFn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
				return nil, repoErr
			},
		}
		repo := &fakeStudyCycleRepository{tx: tx}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

		_, err := service.Play(context.Background(), uuid.New(), PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSubjectBlock,
			TimerType:     TimerTypeFree,
		})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if !repo.rolledBack {
			t.Fatal("expected transaction rollback on error")
		}
	})
}

func TestStudyCycleServicePause(t *testing.T) {
	startedAt := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	runningSince := startedAt.Add(10 * time.Minute)
	pauseAt := runningSince.Add(75 * time.Second)

	t.Run("pauses an in-progress cycle and consolidates the running interval", func(t *testing.T) {
		cycleID := uuid.New()
		sprintID := uuid.New()
		blockID := uuid.New()

		repo := &fakeStudyCycleRepository{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*StudyCycle, error) {
				if id != cycleID {
					t.Fatalf("expected cycle id %v, got %v", cycleID, id)
				}
				return &StudyCycle{
					ID:             cycleID,
					SprintID:       sprintID,
					SprintBlockID:  blockID,
					SequenceNumber: 2,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCycleInProgress,
					StartedAt:      startedAt,
					RunningSince:   &runningSince,
					ElapsedSeconds: 120,
				}, nil
			},
			pauseConditionalFn: func(_ context.Context, input PauseStudyCycleModel) (*StudyCycle, error) {
				if input.ID != cycleID || input.ExpectedStatus != StudyCycleInProgress || input.Status != StudyCyclePaused {
					t.Fatalf("unexpected pause model: %#v", input)
				}
				if input.ElapsedSeconds != 195 {
					t.Fatalf("expected consolidated elapsed_seconds 195, got %d", input.ElapsedSeconds)
				}
				if !input.PausedAt.Equal(pauseAt) {
					t.Fatalf("expected paused_at %v, got %v", pauseAt, input.PausedAt)
				}
				pausedCopy := input.PausedAt
				return &StudyCycle{
					ID:             cycleID,
					SprintID:       sprintID,
					SprintBlockID:  blockID,
					SequenceNumber: 2,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCyclePaused,
					StartedAt:      startedAt,
					RunningSince:   nil,
					PausedAt:       &pausedCopy,
					ElapsedSeconds: input.ElapsedSeconds,
				}, nil
			},
		}

		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return pauseAt })
		cycle, err := service.Pause(context.Background(), cycleID)
		if err != nil {
			t.Fatalf("expected pause to succeed, got %v", err)
		}
		if cycle.Status != StudyCyclePaused || cycle.ElapsedSeconds != 195 || cycle.RunningSince != nil || cycle.PausedAt == nil || !cycle.PausedAt.Equal(pauseAt) {
			t.Fatalf("unexpected paused cycle: %#v", cycle)
		}
	})

	t.Run("rejects an already paused cycle", func(t *testing.T) {
		cycleID := uuid.New()
		repo := &fakeStudyCycleRepository{
			getByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
				return &StudyCycle{ID: cycleID, Status: StudyCyclePaused, PausedAt: &pauseAt}, nil
			},
		}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return pauseAt })
		_, err := service.Pause(context.Background(), cycleID)
		if !errors.Is(err, ErrStudyCycleNotRunning) || !errors.Is(err, ErrStudyCycleInvalidTransition) {
			t.Fatalf("expected ErrStudyCycleNotRunning, got %v", err)
		}
	})

	t.Run("rejects a finished or canceled cycle", func(t *testing.T) {
		for _, status := range []StudyCycleStatus{StudyCycleFinished, StudyCycleCancelled} {
			cycleID := uuid.New()
			repo := &fakeStudyCycleRepository{
				getByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
					return &StudyCycle{ID: cycleID, Status: status, EndedAt: &pauseAt}, nil
				},
			}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return pauseAt })
			_, err := service.Pause(context.Background(), cycleID)
			if !errors.Is(err, ErrStudyCycleAlreadyStopped) || !errors.Is(err, ErrStudyCycleNotRunning) {
				t.Fatalf("for status %q expected ErrStudyCycleAlreadyStopped/ErrStudyCycleNotRunning, got %v", status, err)
			}
		}
	})

	t.Run("returns not found correctly", func(t *testing.T) {
		repo := &fakeStudyCycleRepository{
			getByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
				return nil, ErrStudyCycleNotFound
			},
		}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return pauseAt })
		_, err := service.Pause(context.Background(), uuid.New())
		if !errors.Is(err, ErrStudyCycleNotFound) {
			t.Fatalf("expected ErrStudyCycleNotFound, got %v", err)
		}
	})
}

func TestStudyCycleServiceResume(t *testing.T) {
	startedAt := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	pausedAt := startedAt.Add(15 * time.Minute)
	resumeAt := pausedAt.Add(5 * time.Minute)

	t.Run("resumes a paused cycle preserving started_at and accumulated elapsed time", func(t *testing.T) {
		cycleID := uuid.New()
		sprintID := uuid.New()
		blockID := uuid.New()

		repo := &fakeStudyCycleRepository{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*StudyCycle, error) {
				if id != cycleID {
					t.Fatalf("expected cycle id %v, got %v", cycleID, id)
				}
				return &StudyCycle{
					ID:             cycleID,
					SprintID:       sprintID,
					SprintBlockID:  blockID,
					SequenceNumber: 1,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCyclePaused,
					StartedAt:      startedAt,
					PausedAt:       &pausedAt,
					ElapsedSeconds: 900,
				}, nil
			},
			resumeConditionalFn: func(_ context.Context, input ResumeStudyCycleModel) (*StudyCycle, error) {
				if input.ID != cycleID || input.ExpectedStatus != StudyCyclePaused || input.Status != StudyCycleInProgress {
					t.Fatalf("unexpected resume model: %#v", input)
				}
				if !input.StartedAt.Equal(startedAt) || input.ElapsedSeconds != 900 || !input.RunningSince.Equal(resumeAt) {
					t.Fatalf("expected preserved started_at and elapsed_seconds, got %#v", input)
				}
				runningCopy := input.RunningSince
				return &StudyCycle{
					ID:             cycleID,
					SprintID:       sprintID,
					SprintBlockID:  blockID,
					SequenceNumber: 1,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCycleInProgress,
					StartedAt:      input.StartedAt,
					RunningSince:   &runningCopy,
					PausedAt:       nil,
					ElapsedSeconds: input.ElapsedSeconds,
				}, nil
			},
		}

		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return resumeAt })
		cycle, err := service.Resume(context.Background(), cycleID)
		if err != nil {
			t.Fatalf("expected resume to succeed, got %v", err)
		}
		if cycle.Status != StudyCycleInProgress || !cycle.StartedAt.Equal(startedAt) || cycle.ElapsedSeconds != 900 || cycle.PausedAt != nil || cycle.RunningSince == nil || !cycle.RunningSince.Equal(resumeAt) {
			t.Fatalf("unexpected resumed cycle: %#v", cycle)
		}
	})

	t.Run("rejects an in-progress cycle", func(t *testing.T) {
		cycleID := uuid.New()
		repo := &fakeStudyCycleRepository{
			getByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
				return &StudyCycle{ID: cycleID, Status: StudyCycleInProgress, StartedAt: startedAt, RunningSince: &startedAt}, nil
			},
		}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return resumeAt })
		_, err := service.Resume(context.Background(), cycleID)
		if !errors.Is(err, ErrStudyCycleNotPaused) || !errors.Is(err, ErrStudyCycleInvalidTransition) {
			t.Fatalf("expected ErrStudyCycleNotPaused, got %v", err)
		}
	})

	t.Run("rejects terminal cycles", func(t *testing.T) {
		for _, status := range []StudyCycleStatus{StudyCycleFinished, StudyCycleCancelled} {
			cycleID := uuid.New()
			repo := &fakeStudyCycleRepository{
				getByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
					return &StudyCycle{ID: cycleID, Status: status, EndedAt: &pausedAt}, nil
				},
			}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return resumeAt })
			_, err := service.Resume(context.Background(), cycleID)
			if !errors.Is(err, ErrStudyCycleAlreadyStopped) || !errors.Is(err, ErrStudyCycleNotPaused) {
				t.Fatalf("for status %q expected ErrStudyCycleAlreadyStopped/ErrStudyCycleNotPaused, got %v", status, err)
			}
		}
	})

	t.Run("returns not found correctly", func(t *testing.T) {
		repo := &fakeStudyCycleRepository{
			getByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
				return nil, ErrStudyCycleNotFound
			},
		}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return resumeAt })
		_, err := service.Resume(context.Background(), uuid.New())
		if !errors.Is(err, ErrStudyCycleNotFound) {
			t.Fatalf("expected ErrStudyCycleNotFound, got %v", err)
		}
	})
}

func TestStudyCycleServiceStop(t *testing.T) {
	startedAt := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	runningSince := startedAt.Add(20 * time.Minute)
	pausedAt := startedAt.Add(15 * time.Minute)
	stopAt := runningSince.Add(40 * time.Second)

	t.Run("stops an in-progress cycle and adds its last running interval", func(t *testing.T) {
		cycleID := uuid.New()
		blockID := uuid.New()
		blockFinished := false

		tx := &fakeStudyCycleTxRepository{
			lockCycleByIDFn: func(_ context.Context, id uuid.UUID) (*StudyCycle, error) {
				if id != cycleID {
					t.Fatalf("expected cycle %v, got %v", cycleID, id)
				}
				return &StudyCycle{
					ID:             cycleID,
					SprintID:       uuid.New(),
					SprintBlockID:  blockID,
					SequenceNumber: 1,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCycleInProgress,
					StartedAt:      startedAt,
					RunningSince:   &runningSince,
					ElapsedSeconds: 600,
				}, nil
			},
			stopCycleConditionalFn: func(_ context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
				if input.ExpectedStatus != StudyCycleInProgress || input.Status != StudyCycleFinished || input.ElapsedSeconds != 640 || input.StopReason != CycleStopReasonCompleted || !input.EndedAt.Equal(stopAt) {
					t.Fatalf("unexpected stop model: %#v", input)
				}
				reason := input.StopReason
				endedCopy := input.EndedAt
				return &StudyCycle{
					ID:             cycleID,
					SprintBlockID:  blockID,
					Status:         input.Status,
					StartedAt:      startedAt,
					RunningSince:   nil,
					EndedAt:        &endedCopy,
					ElapsedSeconds: input.ElapsedSeconds,
					StopReason:     &reason,
				}, nil
			},
			getActiveTasksByBlockIDFn: func(context.Context, uuid.UUID) ([]blocktasks.BlockTask, error) {
				return []blocktasks.BlockTask{}, nil
			},
			updateBlockStatusFn: func(_ context.Context, gotBlockID uuid.UUID, status sprintblocks.SprintBlocksStatus, _, completedAt *time.Time) error {
				if gotBlockID != blockID || status != sprintblocks.Finished || completedAt == nil || !completedAt.Equal(stopAt) {
					t.Fatalf("unexpected block finish update: id=%v status=%v completedAt=%v", gotBlockID, status, completedAt)
				}
				blockFinished = true
				return nil
			},
		}
		repo := &fakeStudyCycleRepository{tx: tx}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return stopAt })

		cycle, err := service.Stop(context.Background(), cycleID, StopStudyCycleInput{Reason: CycleStopReasonCompleted})
		if err != nil {
			t.Fatalf("expected stop to succeed, got %v", err)
		}
		if cycle.Status != StudyCycleFinished || cycle.ElapsedSeconds != 640 || cycle.RunningSince != nil || cycle.EndedAt == nil || !cycle.EndedAt.Equal(stopAt) {
			t.Fatalf("unexpected stopped cycle: %#v", cycle)
		}
		if !blockFinished || !repo.committed {
			t.Fatal("expected block finished and transaction committed")
		}
	})

	t.Run("stops a paused cycle without adding paused time", func(t *testing.T) {
		cycleID := uuid.New()
		blockID := uuid.New()

		tx := &fakeStudyCycleTxRepository{
			lockCycleByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
				return &StudyCycle{
					ID:             cycleID,
					SprintBlockID:  blockID,
					ExecutionMode:  ExecutionModeSubjectBlock,
					Status:         StudyCyclePaused,
					StartedAt:      startedAt,
					PausedAt:       &pausedAt,
					ElapsedSeconds: 900,
				}, nil
			},
			stopCycleConditionalFn: func(_ context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
				if input.ExpectedStatus != StudyCyclePaused || input.ElapsedSeconds != 900 {
					t.Fatalf("expected elapsed_seconds to remain 900, got %#v", input)
				}
				reason := input.StopReason
				endedCopy := input.EndedAt
				return &StudyCycle{
					ID:             cycleID,
					SprintBlockID:  blockID,
					Status:         input.Status,
					StartedAt:      startedAt,
					PausedAt:       &pausedAt,
					EndedAt:        &endedCopy,
					ElapsedSeconds: input.ElapsedSeconds,
					StopReason:     &reason,
				}, nil
			},
			getActiveTasksByBlockIDFn: func(context.Context, uuid.UUID) ([]blocktasks.BlockTask, error) {
				return []blocktasks.BlockTask{}, nil
			},
			updateBlockStatusFn: func(context.Context, uuid.UUID, sprintblocks.SprintBlocksStatus, *time.Time, *time.Time) error {
				return nil
			},
		}
		repo := &fakeStudyCycleRepository{tx: tx}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return stopAt })

		cycle, err := service.Stop(context.Background(), cycleID, StopStudyCycleInput{Reason: CycleStopReasonCompleted})
		if err != nil {
			t.Fatalf("expected stop to succeed, got %v", err)
		}
		if cycle.ElapsedSeconds != 900 {
			t.Fatalf("expected elapsed 900, got %d", cycle.ElapsedSeconds)
		}
	})

	t.Run("maps completed and time expired to finished and canceled to canceled", func(t *testing.T) {
		cases := []struct {
			reason     CycleStopReason
			wantStatus StudyCycleStatus
		}{
			{CycleStopReasonCompleted, StudyCycleFinished},
			{CycleStopReasonTimeExpired, StudyCycleFinished},
			{CycleStopReasonCancelled, StudyCycleCancelled},
		}
		for _, tc := range cases {
			cycleID := uuid.New()
			blockID := uuid.New()
			tx := &fakeStudyCycleTxRepository{
				lockCycleByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
					return &StudyCycle{
						ID:             cycleID,
						SprintBlockID:  blockID,
						ExecutionMode:  ExecutionModeSubjectBlock,
						Status:         StudyCyclePaused,
						StartedAt:      startedAt,
						PausedAt:       &pausedAt,
						ElapsedSeconds: 300,
					}, nil
				},
				stopCycleConditionalFn: func(_ context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
					reason := input.StopReason
					endedCopy := input.EndedAt
					return &StudyCycle{
						ID:             cycleID,
						SprintBlockID:  blockID,
						Status:         input.Status,
						EndedAt:        &endedCopy,
						ElapsedSeconds: input.ElapsedSeconds,
						StopReason:     &reason,
					}, nil
				},
				getActiveTasksByBlockIDFn: func(context.Context, uuid.UUID) ([]blocktasks.BlockTask, error) {
					return []blocktasks.BlockTask{{ID: uuid.New(), SprintBlockID: blockID, Status: blocktasks.TaskPending}}, nil
				},
			}
			repo := &fakeStudyCycleRepository{tx: tx}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return stopAt })

			cycle, err := service.Stop(context.Background(), cycleID, StopStudyCycleInput{Reason: tc.reason})
			if err != nil {
				t.Fatalf("for reason %q unexpected error: %v", tc.reason, err)
			}
			if cycle.Status != tc.wantStatus || cycle.StopReason == nil || *cycle.StopReason != tc.reason {
				t.Fatalf("for reason %q expected status %q, got %#v", tc.reason, tc.wantStatus, cycle)
			}
		}
	})

	t.Run("rejects an already terminal cycle and invalid stop reasons", func(t *testing.T) {
		service := NewStudyCycleService(&fakeStudyCycleRepository{})
		if _, err := service.Stop(context.Background(), uuid.New(), StopStudyCycleInput{Reason: CycleStopReason("pausado")}); !errors.Is(err, ErrInvalidStopReason) {
			t.Fatalf("expected ErrInvalidStopReason, got %v", err)
		}

		for _, status := range []StudyCycleStatus{StudyCycleFinished, StudyCycleCancelled} {
			tx := &fakeStudyCycleTxRepository{
				lockCycleByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
					return &StudyCycle{ID: uuid.New(), Status: status, EndedAt: &stopAt}, nil
				},
			}
			repo := &fakeStudyCycleRepository{tx: tx}
			svc := NewStudyCycleServiceWithClock(repo, func() time.Time { return stopAt })

			_, err := svc.Stop(context.Background(), uuid.New(), StopStudyCycleInput{Reason: CycleStopReasonCompleted})
			if !errors.Is(err, ErrStudyCycleAlreadyStopped) || !errors.Is(err, ErrStudyCycleInvalidTransition) {
				t.Fatalf("for status %q expected ErrStudyCycleAlreadyStopped, got %v", status, err)
			}
			if !repo.rolledBack {
				t.Fatal("expected rollback on terminal cycle stop")
			}
		}
	})

	t.Run("updates selected tasks correctly and updates block state only when all active tasks are terminal", func(t *testing.T) {
		cycleID := uuid.New()
		blockID := uuid.New()
		task1 := uuid.New()
		task2 := uuid.New()

		t.Run("keeps block in progress when remaining active task is still pending", func(t *testing.T) {
			cycleTasksCompleted := false
			tasksStatusUpdated := false
			blockFinished := false

			tx := &fakeStudyCycleTxRepository{
				lockCycleByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
					return &StudyCycle{
						ID:            cycleID,
						SprintBlockID: blockID,
						ExecutionMode: ExecutionModeSingleTask,
						Status:        StudyCycleInProgress,
						StartedAt:     startedAt,
						RunningSince:  &runningSince,
					}, nil
				},
				stopCycleConditionalFn: func(_ context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
					return &StudyCycle{ID: cycleID, SprintBlockID: blockID, Status: input.Status, ElapsedSeconds: input.ElapsedSeconds}, nil
				},
				getCycleTaskIDsFn: func(context.Context, uuid.UUID) ([]uuid.UUID, error) {
					return []uuid.UUID{task1}, nil
				},
				completeCycleTasksFn: func(_ context.Context, gotCycleID uuid.UUID, completedAt time.Time) error {
					if gotCycleID != cycleID || !completedAt.Equal(stopAt) {
						t.Fatal("unexpected CompleteCycleTasks args")
					}
					cycleTasksCompleted = true
					return nil
				},
				updateTasksStatusFn: func(_ context.Context, gotBlockID uuid.UUID, ids []uuid.UUID, status blocktasks.BlockTaskStatus, completedAt *time.Time) error {
					if gotBlockID != blockID || len(ids) != 1 || ids[0] != task1 || status != blocktasks.TaskDone || completedAt == nil || !completedAt.Equal(stopAt) {
						t.Fatalf("unexpected UpdateTasksStatus args: %v %v %v %v", gotBlockID, ids, status, completedAt)
					}
					tasksStatusUpdated = true
					return nil
				},
				getActiveTasksByBlockIDFn: func(context.Context, uuid.UUID) ([]blocktasks.BlockTask, error) {
					return []blocktasks.BlockTask{
						{ID: task1, SprintBlockID: blockID, Status: blocktasks.TaskDone},
						{ID: task2, SprintBlockID: blockID, Status: blocktasks.TaskPending},
					}, nil
				},
				updateBlockStatusFn: func(context.Context, uuid.UUID, sprintblocks.SprintBlocksStatus, *time.Time, *time.Time) error {
					blockFinished = true
					return nil
				},
			}
			repo := &fakeStudyCycleRepository{tx: tx}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return stopAt })

			if _, err := service.Stop(context.Background(), cycleID, StopStudyCycleInput{Reason: CycleStopReasonCompleted}); err != nil {
				t.Fatalf("expected stop to succeed, got %v", err)
			}
			if !cycleTasksCompleted || !tasksStatusUpdated {
				t.Fatal("expected cycle tasks and block tasks to be marked completed")
			}
			if blockFinished {
				t.Fatal("did not expect block to be finished while task2 is still pending")
			}
		})

		t.Run("marks block as finished when all active tasks are terminal", func(t *testing.T) {
			blockFinished := false

			tx := &fakeStudyCycleTxRepository{
				lockCycleByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
					return &StudyCycle{
						ID:            cycleID,
						SprintBlockID: blockID,
						ExecutionMode: ExecutionModeMultipleTasks,
						Status:        StudyCycleInProgress,
						StartedAt:     startedAt,
						RunningSince:  &runningSince,
					}, nil
				},
				stopCycleConditionalFn: func(_ context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
					return &StudyCycle{ID: cycleID, SprintBlockID: blockID, Status: input.Status, ElapsedSeconds: input.ElapsedSeconds}, nil
				},
				getCycleTaskIDsFn: func(context.Context, uuid.UUID) ([]uuid.UUID, error) {
					return []uuid.UUID{task1, task2}, nil
				},
				completeCycleTasksFn: func(context.Context, uuid.UUID, time.Time) error {
					return nil
				},
				updateTasksStatusFn: func(context.Context, uuid.UUID, []uuid.UUID, blocktasks.BlockTaskStatus, *time.Time) error {
					return nil
				},
				getActiveTasksByBlockIDFn: func(context.Context, uuid.UUID) ([]blocktasks.BlockTask, error) {
					return []blocktasks.BlockTask{
						{ID: task1, SprintBlockID: blockID, Status: blocktasks.TaskDone},
						{ID: task2, SprintBlockID: blockID, Status: blocktasks.TaskDone},
					}, nil
				},
				updateBlockStatusFn: func(_ context.Context, gotBlockID uuid.UUID, status sprintblocks.SprintBlocksStatus, _, completedAt *time.Time) error {
					if gotBlockID != blockID || status != sprintblocks.Finished || completedAt == nil || !completedAt.Equal(stopAt) {
						t.Fatal("unexpected UpdateBlockStatus call")
					}
					blockFinished = true
					return nil
				},
			}
			repo := &fakeStudyCycleRepository{tx: tx}
			service := NewStudyCycleServiceWithClock(repo, func() time.Time { return stopAt })

			if _, err := service.Stop(context.Background(), cycleID, StopStudyCycleInput{Reason: CycleStopReasonCompleted}); err != nil {
				t.Fatalf("expected stop to succeed, got %v", err)
			}
			if !blockFinished || !repo.committed {
				t.Fatal("expected block to be marked finished and transaction committed")
			}
		})
	})

	t.Run("rolls back when a dependent operation fails", func(t *testing.T) {
		cycleID := uuid.New()
		blockID := uuid.New()
		task1 := uuid.New()
		dependentErr := errors.New("failed to update block tasks")

		tx := &fakeStudyCycleTxRepository{
			lockCycleByIDFn: func(context.Context, uuid.UUID) (*StudyCycle, error) {
				return &StudyCycle{
					ID:            cycleID,
					SprintBlockID: blockID,
					ExecutionMode: ExecutionModeSingleTask,
					Status:        StudyCycleInProgress,
					StartedAt:     startedAt,
					RunningSince:  &runningSince,
				}, nil
			},
			stopCycleConditionalFn: func(_ context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
				return &StudyCycle{ID: cycleID, SprintBlockID: blockID, Status: input.Status}, nil
			},
			getCycleTaskIDsFn: func(context.Context, uuid.UUID) ([]uuid.UUID, error) {
				return []uuid.UUID{task1}, nil
			},
			completeCycleTasksFn: func(context.Context, uuid.UUID, time.Time) error {
				return nil
			},
			updateTasksStatusFn: func(context.Context, uuid.UUID, []uuid.UUID, blocktasks.BlockTaskStatus, *time.Time) error {
				return dependentErr
			},
		}
		repo := &fakeStudyCycleRepository{tx: tx}
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return stopAt })

		_, err := service.Stop(context.Background(), cycleID, StopStudyCycleInput{Reason: CycleStopReasonCompleted})
		if !errors.Is(err, dependentErr) {
			t.Fatalf("expected %v, got %v", dependentErr, err)
		}
		if !repo.rolledBack || repo.committed {
			t.Fatal("expected transaction to roll back when dependent operation fails")
		}
	})
}

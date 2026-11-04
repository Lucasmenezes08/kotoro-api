package studycycles

import (
	"context"
	"errors"
	"fmt"
	"time"

	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
)

type StudyCycleServiceContract interface {
	Play(ctx context.Context, sprintID uuid.UUID, input PlayStudyCycleInput) (StudyCycle, error)
	Pause(ctx context.Context, cycleID uuid.UUID) (StudyCycle, error)
	Resume(ctx context.Context, cycleID uuid.UUID) (StudyCycle, error)
	Stop(ctx context.Context, cycleID uuid.UUID, input StopStudyCycleInput) (StudyCycle, error)
}

type StudyCycleService struct {
	repo StudyCycleRepositoryContract
	now  func() time.Time
}

func NewStudyCycleService(repo StudyCycleRepositoryContract) *StudyCycleService {
	return NewStudyCycleServiceWithClock(repo, func() time.Time {
		return time.Now().UTC()
	})
}

func NewStudyCycleServiceWithClock(repo StudyCycleRepositoryContract, clock func() time.Time) *StudyCycleService {
	if clock == nil {
		clock = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &StudyCycleService{
		repo: repo,
		now:  clock,
	}
}

func (s *StudyCycleService) Play(ctx context.Context, sprintID uuid.UUID, input PlayStudyCycleInput) (StudyCycle, error) {
	if err := validatePlayInput(input); err != nil {
		return StudyCycle{}, err
	}

	var created StudyCycle
	err := s.repo.WithinTransaction(ctx, func(ctx context.Context, tx StudyCycleTxRepositoryContract) error {
		sp, err := tx.LockSprintByID(ctx, sprintID)
		if err != nil {
			return err
		}
		if sp.Status != sprint.InProgress {
			return ErrSprintNotStarted
		}

		hasActive, err := tx.HasActiveCycleBySprintID(ctx, sprintID)
		if err != nil {
			return fmt.Errorf("check active study cycle: %w", err)
		}
		if hasActive {
			return ErrStudyCycleAlreadyActive
		}

		blocks, err := tx.LockBlocksBySprintID(ctx, sprintID)
		if err != nil {
			return fmt.Errorf("lock sprint blocks: %w", err)
		}

		selectedBlock, err := selectExecutableBlock(sprintID, blocks)
		if err != nil {
			return err
		}

		if input.ExecutionMode == ExecutionModeSingleTask || input.ExecutionMode == ExecutionModeMultipleTasks {
			tasks, err := tx.GetActiveTasksByBlockID(ctx, selectedBlock.ID)
			if err != nil {
				return fmt.Errorf("get active block tasks: %w", err)
			}
			if err := validateSelectedTasksForBlock(selectedBlock.ID, input.BlockTaskIDs, tasks); err != nil {
				return err
			}
		}

		nextSeq, err := tx.GetNextSequenceNumber(ctx, sprintID)
		if err != nil {
			return fmt.Errorf("get next sequence number: %w", err)
		}
		if nextSeq <= 0 {
			nextSeq = 1
		}

		now := s.now().UTC()
		cycle, err := tx.CreateCycle(ctx, CreateStudyCycleModel{
			SprintID:               sprintID,
			SprintBlockID:          selectedBlock.ID,
			SequenceNumber:         nextSeq,
			ExecutionMode:          input.ExecutionMode,
			TimerType:              input.TimerType,
			Status:                 StudyCycleInProgress,
			PlannedDurationSeconds: input.PlannedDurationSeconds,
			StartedAt:              now,
			RunningSince:           &now,
			PausedAt:               nil,
			EndedAt:                nil,
			ElapsedSeconds:         0,
			StopReason:             nil,
		})
		if err != nil {
			return err
		}

		if len(input.BlockTaskIDs) > 0 {
			if err := tx.AttachCycleTasks(ctx, cycle.ID, input.BlockTaskIDs); err != nil {
				return fmt.Errorf("attach study cycle tasks: %w", err)
			}
		}

		if selectedBlock.Status == sprintblocks.Pending {
			if err := tx.UpdateBlockStatus(ctx, selectedBlock.ID, sprintblocks.InProgress, &now, nil); err != nil {
				return fmt.Errorf("update sprint block status: %w", err)
			}
		}

		created = *cycle
		return nil
	})
	if err != nil {
		return StudyCycle{}, err
	}

	return created, nil
}

func (s *StudyCycleService) Pause(ctx context.Context, cycleID uuid.UUID) (StudyCycle, error) {
	existing, err := s.repo.GetByID(ctx, cycleID)
	if err != nil {
		return StudyCycle{}, err
	}

	now := s.now().UTC()
	paused, err := existing.ApplyPause(now)
	if err != nil {
		return StudyCycle{}, err
	}

	updated, err := s.repo.PauseConditional(ctx, PauseStudyCycleModel{
		ID:             cycleID,
		ExpectedStatus: StudyCycleInProgress,
		Status:         StudyCyclePaused,
		PausedAt:       now,
		ElapsedSeconds: paused.ElapsedSeconds,
	})
	if err != nil {
		return StudyCycle{}, err
	}

	return *updated, nil
}

func (s *StudyCycleService) Resume(ctx context.Context, cycleID uuid.UUID) (StudyCycle, error) {
	existing, err := s.repo.GetByID(ctx, cycleID)
	if err != nil {
		return StudyCycle{}, err
	}

	now := s.now().UTC()
	resumed, err := existing.ApplyResume(now)
	if err != nil {
		return StudyCycle{}, err
	}

	updated, err := s.repo.ResumeConditional(ctx, ResumeStudyCycleModel{
		ID:             cycleID,
		ExpectedStatus: StudyCyclePaused,
		Status:         StudyCycleInProgress,
		RunningSince:   now,
		StartedAt:      resumed.StartedAt,
		ElapsedSeconds: resumed.ElapsedSeconds,
	})
	if err != nil {
		return StudyCycle{}, err
	}

	return *updated, nil
}

func (s *StudyCycleService) Stop(ctx context.Context, cycleID uuid.UUID, input StopStudyCycleInput) (StudyCycle, error) {
	return StudyCycle{}, errors.New("not implemented")
}

func selectExecutableBlock(sprintID uuid.UUID, blocks []sprintblocks.SprintBlock) (*sprintblocks.SprintBlock, error) {
	var inProgressCandidate *sprintblocks.SprintBlock
	var pendingCandidate *sprintblocks.SprintBlock

	for i := range blocks {
		block := &blocks[i]
		if block.DeletedAt != nil {
			continue
		}
		if block.SprintID != uuid.Nil && block.SprintID != sprintID {
			continue
		}
		switch block.Status {
		case sprintblocks.InProgress:
			if inProgressCandidate == nil || block.Position < inProgressCandidate.Position {
				inProgressCandidate = block
			}
		case sprintblocks.Pending:
			if pendingCandidate == nil || block.Position < pendingCandidate.Position {
				pendingCandidate = block
			}
		case sprintblocks.Finished:
			continue
		}
	}

	if inProgressCandidate != nil {
		return inProgressCandidate, nil
	}
	if pendingCandidate != nil {
		return pendingCandidate, nil
	}
	return nil, ErrSprintHasNoAvailableBlock
}

func validateSelectedTasksForBlock(blockID uuid.UUID, selectedIDs []uuid.UUID, blockTasks []blocktasks.BlockTask) error {
	available := make(map[uuid.UUID]blocktasks.BlockTask, len(blockTasks))
	for _, task := range blockTasks {
		if task.DeletedAt != nil {
			continue
		}
		if task.SprintBlockID != uuid.Nil && task.SprintBlockID != blockID {
			continue
		}
		if task.Status == blocktasks.TaskDone || task.Status == blocktasks.TaskCancelled {
			continue
		}
		available[task.ID] = task
	}

	for _, id := range selectedIDs {
		if _, ok := available[id]; !ok {
			return ErrInvalidSelectedTasks
		}
	}
	return nil
}

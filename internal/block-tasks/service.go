package blocktasks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
)

const maxPostgresInteger = 1<<31 - 1

var (
	ErrBlockTaskNotFound                = errors.New("block task not found")
	ErrBlockTaskTitleRequired           = errors.New("block task title is required")
	ErrBlockTaskTitleTooLong            = errors.New("block task title cannot exceed 255 characters")
	ErrBlockTaskUpdateEmpty             = errors.New("update fields must have at least one value")
	ErrBlockTaskPositionInvalid         = errors.New("block task position must be greater than zero")
	ErrBlockTaskPositionConflict        = errors.New("block task position is already occupied")
	ErrBlockTaskPriorityInvalid         = errors.New("block task priority is invalid")
	ErrBlockTaskEstimatedSecondsInvalid = errors.New("estimated seconds must be greater than zero")
	ErrBlockTaskOrderInvalid            = errors.New("invalid block task order")
	ErrSprintBlockNotInPlanning         = errors.New("block tasks can only be changed while sprint block is pending")
)

type sprintReader interface {
	GetById(context.Context, uuid.UUID) (*sprint.Sprint, error)
}

type sprintBlockReader interface {
	GetByID(context.Context, uuid.UUID, uuid.UUID) (*sprintblocks.SprintBlock, error)
}

type BlockTaskServiceContract interface {
	Create(context.Context, uuid.UUID, uuid.UUID, CreateBlockTaskInput) (*BlockTask, error)
	GetAll(context.Context, uuid.UUID, uuid.UUID) ([]BlockTask, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*BlockTask, error)
	Update(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, BlockTaskUpdateInput) (*BlockTask, error)
	Delete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	Reorder(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) error
}

type BlockTaskService struct {
	repository BlockTaskRepositoryContract
	sprints    sprintReader
	blocks     sprintBlockReader
}

func NewBlockTaskService(repository BlockTaskRepositoryContract, sprints sprintReader, blocks sprintBlockReader) *BlockTaskService {
	return &BlockTaskService{repository: repository, sprints: sprints, blocks: blocks}
}

func (s *BlockTaskService) Create(ctx context.Context, sprintID, blockID uuid.UUID, input CreateBlockTaskInput) (*BlockTask, error) {
	if err := s.requirePlanningHierarchy(ctx, sprintID, blockID); err != nil {
		return nil, err
	}
	title, err := normalizeTaskTitle(input.Title)
	if err != nil {
		return nil, err
	}
	description := normalizeDescription(input.Description)
	if err := validateTaskPosition(input.Position); err != nil {
		return nil, err
	}
	if err := validateEstimatedSeconds(input.EstimatedSeconds); err != nil {
		return nil, err
	}
	priority := PriorityMedium
	if input.Priority != nil {
		priority = *input.Priority
	}
	if !validPriority(priority) {
		return nil, ErrBlockTaskPriorityInvalid
	}
	if err := s.checkPositionConflict(ctx, blockID, input.Position, nil); err != nil {
		return nil, err
	}
	task, err := s.repository.Create(ctx, CreateBlockTaskModel{
		SprintBlockID: blockID, Title: title, Description: description,
		Position: input.Position, Status: TaskPending, Priority: priority,
		EstimatedSeconds: input.EstimatedSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("create block task: %w", err)
	}
	return task, nil
}

func (s *BlockTaskService) GetAll(ctx context.Context, sprintID, blockID uuid.UUID) ([]BlockTask, error) {
	if _, err := s.requireHierarchy(ctx, sprintID, blockID); err != nil {
		return nil, err
	}
	tasks, err := s.repository.GetAllByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("get block tasks: %w", err)
	}
	if tasks == nil {
		return []BlockTask{}, nil
	}
	return tasks, nil
}

func (s *BlockTaskService) GetByID(ctx context.Context, sprintID, blockID, taskID uuid.UUID) (*BlockTask, error) {
	if _, err := s.requireHierarchy(ctx, sprintID, blockID); err != nil {
		return nil, err
	}
	task, err := s.repository.GetByID(ctx, blockID, taskID)
	if err != nil {
		return nil, fmt.Errorf("get block task: %w", err)
	}
	return task, nil
}

func (s *BlockTaskService) Update(ctx context.Context, sprintID, blockID, taskID uuid.UUID, input BlockTaskUpdateInput) (*BlockTask, error) {
	if input.Title == nil && input.Description == nil && input.Position == nil && input.Priority == nil && input.EstimatedSeconds == nil {
		return nil, ErrBlockTaskUpdateEmpty
	}
	if err := s.requirePlanningHierarchy(ctx, sprintID, blockID); err != nil {
		return nil, err
	}
	if _, err := s.repository.GetByID(ctx, blockID, taskID); err != nil {
		return nil, err
	}
	model := BlockTaskUpdateModel{
		Position: input.Position, Priority: input.Priority, EstimatedSeconds: input.EstimatedSeconds,
		DescriptionSet: input.Description != nil,
	}
	if input.Title != nil {
		title, err := normalizeTaskTitle(*input.Title)
		if err != nil {
			return nil, err
		}
		model.Title = &title
	}
	if input.Description != nil {
		model.Description = normalizeDescription(input.Description)
	}
	if input.Position != nil {
		if err := validateTaskPosition(*input.Position); err != nil {
			return nil, err
		}
		if err := s.checkPositionConflict(ctx, blockID, *input.Position, &taskID); err != nil {
			return nil, err
		}
	}
	if input.Priority != nil && !validPriority(*input.Priority) {
		return nil, ErrBlockTaskPriorityInvalid
	}
	if err := validateEstimatedSeconds(input.EstimatedSeconds); err != nil {
		return nil, err
	}
	task, err := s.repository.Update(ctx, blockID, taskID, model)
	if err != nil {
		return nil, fmt.Errorf("update block task: %w", err)
	}
	return task, nil
}

func (s *BlockTaskService) Delete(ctx context.Context, sprintID, blockID, taskID uuid.UUID) error {
	if err := s.requirePlanningHierarchy(ctx, sprintID, blockID); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, blockID, taskID); err != nil {
		return fmt.Errorf("delete block task: %w", err)
	}
	return nil
}

func (s *BlockTaskService) Reorder(ctx context.Context, sprintID, blockID uuid.UUID, taskIDs []uuid.UUID) error {
	if err := s.requirePlanningHierarchy(ctx, sprintID, blockID); err != nil {
		return err
	}
	tasks, err := s.repository.GetAllByBlockID(ctx, blockID)
	if err != nil {
		return fmt.Errorf("get block tasks for reorder: %w", err)
	}
	if len(tasks) != len(taskIDs) {
		return ErrBlockTaskOrderInvalid
	}
	expected := make(map[uuid.UUID]struct{}, len(tasks))
	for _, task := range tasks {
		expected[task.ID] = struct{}{}
	}
	seen := make(map[uuid.UUID]struct{}, len(taskIDs))
	for _, id := range taskIDs {
		if _, duplicate := seen[id]; duplicate {
			return ErrBlockTaskOrderInvalid
		}
		if _, exists := expected[id]; !exists {
			return ErrBlockTaskOrderInvalid
		}
		seen[id] = struct{}{}
	}
	if err := s.repository.Reorder(ctx, blockID, taskIDs); err != nil {
		return fmt.Errorf("reorder block tasks: %w", err)
	}
	return nil
}

func (s *BlockTaskService) requireHierarchy(ctx context.Context, sprintID, blockID uuid.UUID) (*sprintblocks.SprintBlock, error) {
	if _, err := s.sprints.GetById(ctx, sprintID); err != nil {
		return nil, err
	}
	block, err := s.blocks.GetByID(ctx, sprintID, blockID)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (s *BlockTaskService) requirePlanningHierarchy(ctx context.Context, sprintID, blockID uuid.UUID) error {
	sprintValue, err := s.sprints.GetById(ctx, sprintID)
	if err != nil {
		return err
	}
	if sprintValue.Status != sprint.Creating {
		return sprintblocks.ErrSprintNotInPlanning
	}
	block, err := s.blocks.GetByID(ctx, sprintID, blockID)
	if err != nil {
		return err
	}
	if block.Status != sprintblocks.Pending {
		return ErrSprintBlockNotInPlanning
	}
	return nil
}

func (s *BlockTaskService) checkPositionConflict(ctx context.Context, blockID uuid.UUID, position int, excludeID *uuid.UUID) error {
	exists, err := s.repository.ExistsByPosition(ctx, blockID, position, excludeID)
	if err != nil {
		return fmt.Errorf("check block task position: %w", err)
	}
	if exists {
		return ErrBlockTaskPositionConflict
	}
	return nil
}

func normalizeTaskTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", ErrBlockTaskTitleRequired
	}
	if utf8.RuneCountInString(trimmed) > 255 {
		return "", ErrBlockTaskTitleTooLong
	}
	return trimmed, nil
}

func normalizeDescription(description *string) *string {
	if description == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*description)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validateTaskPosition(position int) error {
	if position <= 0 || position > maxPostgresInteger {
		return ErrBlockTaskPositionInvalid
	}
	return nil
}

func validateEstimatedSeconds(value *int) error {
	if value != nil && (*value <= 0 || *value > maxPostgresInteger) {
		return ErrBlockTaskEstimatedSecondsInvalid
	}
	return nil
}

func validPriority(priority TaskPriority) bool {
	switch priority {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityVeryHigh:
		return true
	default:
		return false
	}
}

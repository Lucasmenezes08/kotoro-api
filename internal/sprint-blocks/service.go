package sprintblocks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/google/uuid"
)

const maxPostgresInteger = 1<<31 - 1

var (
	ErrSprintBlockNotFound                = errors.New("sprint block not found")
	ErrSprintBlockUpdateEmpty             = errors.New("update fields must have at least one value")
	ErrSprintBlockPositionInvalid         = errors.New("sprint block position must be greater than zero")
	ErrSprintBlockPositionConflict        = errors.New("sprint block position is already occupied")
	ErrSprintBlockEstimatedSecondsInvalid = errors.New("estimated seconds must be greater than zero")
	ErrSprintBlockSubjectConflict         = errors.New("subject already has an active block in this sprint")
	ErrSprintBlockTitleInvalid            = errors.New("sprint block title cannot be empty")
	ErrSprintBlockTitleTooLong            = errors.New("sprint block title cannot exceed 100 characters")
	ErrSprintNotInPlanning                = errors.New("sprint blocks can only be changed while sprint is creating")
	ErrSprintBlockNotEditable             = errors.New("completed or in-progress sprint blocks cannot be modified")
	ErrInvalidSprintBlockOrder            = errors.New("invalid sprint block order")
)

type sprintReader interface {
	GetById(context.Context, uuid.UUID) (*sprint.Sprint, error)
}

type subjectReader interface {
	GetById(context.Context, uuid.UUID) (*subject.Subject, error)
}

type SprintBlockServiceContract interface {
	Create(context.Context, uuid.UUID, uuid.UUID, CreateSprintBlockInput) (*SprintBlock, error)
	GetAll(context.Context, uuid.UUID, uuid.UUID) ([]SprintBlock, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*SprintBlock, error)
	Update(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, SprintBlockUpdateModel) (*SprintBlock, error)
	Delete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	Reorder(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) error
	UpdateCanvasLayout(context.Context, uuid.UUID, uuid.UUID, UpdateCanvasLayoutInput) error
}

type SprintBlockService struct {
	repository    SprintBlockRepositoryContract
	sprintReader  sprintReader
	subjectReader subjectReader
}

func NewSprintBlockService(repository SprintBlockRepositoryContract, sprintReader sprintReader, subjectReader subjectReader) *SprintBlockService {
	return &SprintBlockService{repository: repository, sprintReader: sprintReader, subjectReader: subjectReader}
}

func (s *SprintBlockService) Create(ctx context.Context, userID, sprintID uuid.UUID, input CreateSprintBlockInput) (*SprintBlock, error) {
	if _, err := s.requireEditableSprint(ctx, userID, sprintID); err != nil {
		return nil, err
	}
	title, err := normalizeTitle(input.Title)
	if err != nil {
		return nil, err
	}
	if input.Position <= 0 || input.Position > maxPostgresInteger {
		return nil, ErrSprintBlockPositionInvalid
	}
	if input.EstimatedSeconds != nil && (*input.EstimatedSeconds <= 0 || *input.EstimatedSeconds > maxPostgresInteger) {
		return nil, ErrSprintBlockEstimatedSecondsInvalid
	}
	if err := s.requireSubject(ctx, userID, input.SubjectID); err != nil {
		return nil, err
	}
	if err := s.checkSubjectConflict(ctx, sprintID, input.SubjectID, nil); err != nil {
		return nil, err
	}
	if err := s.checkPositionConflict(ctx, sprintID, input.Position, nil); err != nil {
		return nil, err
	}
	var xVal, yVal float64
	if input.X != nil {
		xVal = *input.X
	}
	if input.Y != nil {
		yVal = *input.Y
	}
	block, err := s.repository.Create(ctx, CreateSprintBlockModel{
		SprintID: sprintID, SubjectID: input.SubjectID, Title: title,
		Position: input.Position, X: xVal, Y: yVal, Status: Pending, EstimatedSeconds: input.EstimatedSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("create sprint block: %w", err)
	}
	return block, nil
}

func (s *SprintBlockService) GetAll(ctx context.Context, userID, sprintID uuid.UUID) ([]SprintBlock, error) {
	sp, err := s.sprintReader.GetById(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sp.UserID != userID {
		return nil, sprint.ErrSprintNotFound
	}
	blocks, err := s.repository.GetAllBySprintID(ctx, sprintID)
	if err != nil {
		return nil, fmt.Errorf("get sprint blocks: %w", err)
	}
	if blocks == nil {
		return []SprintBlock{}, nil
	}
	return blocks, nil
}

func (s *SprintBlockService) GetByID(ctx context.Context, userID, sprintID, blockID uuid.UUID) (*SprintBlock, error) {
	sp, err := s.sprintReader.GetById(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if sp.UserID != userID {
		return nil, sprint.ErrSprintNotFound
	}
	block, err := s.repository.GetByID(ctx, sprintID, blockID)
	if err != nil {
		return nil, fmt.Errorf("get sprint block: %w", err)
	}
	return block, nil
}

func (s *SprintBlockService) Update(ctx context.Context, userID, sprintID, blockID uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
	if input.SubjectID == nil && input.Title == nil && input.Position == nil && input.EstimatedSeconds == nil {
		return nil, ErrSprintBlockUpdateEmpty
	}
	sp, err := s.requireEditableSprint(ctx, userID, sprintID)
	if err != nil {
		return nil, err
	}
	existingBlock, err := s.repository.GetByID(ctx, sprintID, blockID)
	if err != nil {
		return nil, err
	}
	if sp.Status == sprint.InProgress && existingBlock.Status != Pending {
		return nil, ErrSprintBlockNotEditable
	}
	if input.Title != nil {
		title, err := normalizeTitle(input.Title)
		if err != nil {
			return nil, err
		}
		input.Title = title
	}
	if input.EstimatedSeconds != nil && (*input.EstimatedSeconds <= 0 || *input.EstimatedSeconds > maxPostgresInteger) {
		return nil, ErrSprintBlockEstimatedSecondsInvalid
	}
	if input.Position != nil {
		if *input.Position <= 0 || *input.Position > maxPostgresInteger {
			return nil, ErrSprintBlockPositionInvalid
		}
		if err := s.checkPositionConflict(ctx, sprintID, *input.Position, &blockID); err != nil {
			return nil, err
		}
	}
	if input.SubjectID != nil {
		if err := s.requireSubject(ctx, userID, *input.SubjectID); err != nil {
			return nil, err
		}
		if err := s.checkSubjectConflict(ctx, sprintID, *input.SubjectID, &blockID); err != nil {
			return nil, err
		}
	}
	block, err := s.repository.Update(ctx, sprintID, blockID, input)
	if err != nil {
		return nil, fmt.Errorf("update sprint block: %w", err)
	}
	return block, nil
}

func (s *SprintBlockService) Delete(ctx context.Context, userID, sprintID, blockID uuid.UUID) error {
	sp, err := s.requireEditableSprint(ctx, userID, sprintID)
	if err != nil {
		return err
	}
	if sp.Status == sprint.InProgress {
		existingBlock, err := s.repository.GetByID(ctx, sprintID, blockID)
		if err != nil {
			return err
		}
		if existingBlock.Status != Pending {
			return ErrSprintBlockNotEditable
		}
	}
	if err := s.repository.Delete(ctx, sprintID, blockID); err != nil {
		return fmt.Errorf("delete sprint block: %w", err)
	}
	return nil
}

func (s *SprintBlockService) Reorder(ctx context.Context, userID, sprintID uuid.UUID, blockIDs []uuid.UUID) error {
	sp, err := s.requireEditableSprint(ctx, userID, sprintID)
	if err != nil {
		return err
	}
	blocks, err := s.repository.GetAllBySprintID(ctx, sprintID)
	if err != nil {
		return fmt.Errorf("get sprint blocks for reorder: %w", err)
	}
	if len(blocks) != len(blockIDs) {
		return ErrInvalidSprintBlockOrder
	}
	expected := make(map[uuid.UUID]SprintBlock, len(blocks))
	for _, block := range blocks {
		expected[block.ID] = block
	}
	seen := make(map[uuid.UUID]struct{}, len(blockIDs))
	for newIdx, id := range blockIDs {
		if _, duplicate := seen[id]; duplicate {
			return ErrInvalidSprintBlockOrder
		}
		b, exists := expected[id]
		if !exists {
			return ErrInvalidSprintBlockOrder
		}
		if sp.Status == sprint.InProgress && b.Status != Pending {
			if b.Position != newIdx+1 {
				return ErrInvalidSprintBlockOrder
			}
		}
		seen[id] = struct{}{}
	}
	if err := s.repository.Reorder(ctx, sprintID, blockIDs); err != nil {
		return fmt.Errorf("reorder sprint blocks: %w", err)
	}
	return nil
}

func (s *SprintBlockService) UpdateCanvasLayout(ctx context.Context, userID, sprintID uuid.UUID, input UpdateCanvasLayoutInput) error {
	sp, err := s.requireEditableSprint(ctx, userID, sprintID)
	if err != nil {
		return err
	}
	if len(input.Blocks) == 0 {
		return ErrInvalidSprintBlockOrder
	}
	blocks, err := s.repository.GetAllBySprintID(ctx, sprintID)
	if err != nil {
		return fmt.Errorf("get sprint blocks for canvas layout: %w", err)
	}
	if len(blocks) != len(input.Blocks) {
		return ErrInvalidSprintBlockOrder
	}
	expected := make(map[uuid.UUID]SprintBlock, len(blocks))
	for _, block := range blocks {
		expected[block.ID] = block
	}
	seen := make(map[uuid.UUID]struct{}, len(input.Blocks))
	seenPositions := make(map[int]struct{}, len(input.Blocks))
	for _, it := range input.Blocks {
		if it.Position <= 0 || it.Position > maxPostgresInteger {
			return ErrSprintBlockPositionInvalid
		}
		if _, dup := seen[it.ID]; dup {
			return ErrInvalidSprintBlockOrder
		}
		if _, dupPos := seenPositions[it.Position]; dupPos {
			return ErrSprintBlockPositionConflict
		}
		b, exists := expected[it.ID]
		if !exists {
			return ErrInvalidSprintBlockOrder
		}
		if sp.Status == sprint.InProgress && b.Status != Pending {
			if b.Position != it.Position {
				return ErrInvalidSprintBlockOrder
			}
		}
		seen[it.ID] = struct{}{}
		seenPositions[it.Position] = struct{}{}
	}
	return s.repository.UpdateCanvasLayout(ctx, sprintID, input.Blocks)
}

func (s *SprintBlockService) requireEditableSprint(ctx context.Context, userID, sprintID uuid.UUID) (*sprint.Sprint, error) {
	value, err := s.sprintReader.GetById(ctx, sprintID)
	if err != nil {
		return nil, err
	}
	if value.UserID != userID {
		return nil, sprint.ErrSprintNotFound
	}
	if value.Status != sprint.Creating && value.Status != sprint.InProgress {
		return nil, ErrSprintNotInPlanning
	}
	return value, nil
}

func (s *SprintBlockService) requirePlanningSprint(ctx context.Context, userID, sprintID uuid.UUID) error {
	_, err := s.requireEditableSprint(ctx, userID, sprintID)
	return err
}

func (s *SprintBlockService) requireSubject(ctx context.Context, userID, subjectID uuid.UUID) error {
	value, err := s.subjectReader.GetById(ctx, subjectID)
	if err != nil {
		return err
	}
	if value.UserId != userID {
		return subject.ErrSubjectNotFound
	}
	return nil
}

func (s *SprintBlockService) checkSubjectConflict(ctx context.Context, sprintID, subjectID uuid.UUID, excludeID *uuid.UUID) error {
	exists, err := s.repository.ExistsBySubjectID(ctx, sprintID, subjectID, excludeID)
	if err != nil {
		return fmt.Errorf("check sprint block subject: %w", err)
	}
	if exists {
		return ErrSprintBlockSubjectConflict
	}
	return nil
}

func (s *SprintBlockService) checkPositionConflict(ctx context.Context, sprintID uuid.UUID, position int, excludeID *uuid.UUID) error {
	exists, err := s.repository.ExistsByPosition(ctx, sprintID, position, excludeID)
	if err != nil {
		return fmt.Errorf("check sprint block position: %w", err)
	}
	if exists {
		return ErrSprintBlockPositionConflict
	}
	return nil
}

func normalizeTitle(title *string) (*string, error) {
	if title == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*title)
	if trimmed == "" {
		return nil, ErrSprintBlockTitleInvalid
	}
	if utf8.RuneCountInString(trimmed) > 100 {
		return nil, ErrSprintBlockTitleTooLong
	}
	return &trimmed, nil
}

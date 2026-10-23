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
	ErrInvalidSprintBlockOrder            = errors.New("invalid sprint block order")
)

type sprintReader interface {
	GetById(context.Context, uuid.UUID) (*sprint.Sprint, error)
}

type subjectReader interface {
	GetById(context.Context, uuid.UUID) (*subject.Subject, error)
}

type SprintBlockServiceContract interface {
	Create(context.Context, uuid.UUID, CreateSprintBlockInput) (*SprintBlock, error)
	GetAll(context.Context, uuid.UUID) ([]SprintBlock, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error)
	Update(context.Context, uuid.UUID, uuid.UUID, SprintBlockUpdateModel) (*SprintBlock, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
	Reorder(context.Context, uuid.UUID, []uuid.UUID) error
}

type SprintBlockService struct {
	repository    SprintBlockRepositoryContract
	sprintReader  sprintReader
	subjectReader subjectReader
}

func NewSprintBlockService(repository SprintBlockRepositoryContract, sprintReader sprintReader, subjectReader subjectReader) *SprintBlockService {
	return &SprintBlockService{repository: repository, sprintReader: sprintReader, subjectReader: subjectReader}
}

func (s *SprintBlockService) Create(ctx context.Context, sprintID uuid.UUID, input CreateSprintBlockInput) (*SprintBlock, error) {
	if err := s.requirePlanningSprint(ctx, sprintID); err != nil {
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
	if err := s.requireSubject(ctx, input.SubjectID); err != nil {
		return nil, err
	}
	if err := s.checkSubjectConflict(ctx, sprintID, input.SubjectID, nil); err != nil {
		return nil, err
	}
	if err := s.checkPositionConflict(ctx, sprintID, input.Position, nil); err != nil {
		return nil, err
	}
	block, err := s.repository.Create(ctx, CreateSprintBlockModel{
		SprintID: sprintID, SubjectID: input.SubjectID, Title: title,
		Position: input.Position, Status: Pending, EstimatedSeconds: input.EstimatedSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("create sprint block: %w", err)
	}
	return block, nil
}

func (s *SprintBlockService) GetAll(ctx context.Context, sprintID uuid.UUID) ([]SprintBlock, error) {
	if _, err := s.sprintReader.GetById(ctx, sprintID); err != nil {
		return nil, err
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

func (s *SprintBlockService) GetByID(ctx context.Context, sprintID, blockID uuid.UUID) (*SprintBlock, error) {
	if _, err := s.sprintReader.GetById(ctx, sprintID); err != nil {
		return nil, err
	}
	block, err := s.repository.GetByID(ctx, sprintID, blockID)
	if err != nil {
		return nil, fmt.Errorf("get sprint block: %w", err)
	}
	return block, nil
}

func (s *SprintBlockService) Update(ctx context.Context, sprintID, blockID uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
	if input.SubjectID == nil && input.Title == nil && input.Position == nil && input.EstimatedSeconds == nil {
		return nil, ErrSprintBlockUpdateEmpty
	}
	if err := s.requirePlanningSprint(ctx, sprintID); err != nil {
		return nil, err
	}
	if _, err := s.repository.GetByID(ctx, sprintID, blockID); err != nil {
		return nil, err
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
		if err := s.requireSubject(ctx, *input.SubjectID); err != nil {
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

func (s *SprintBlockService) Delete(ctx context.Context, sprintID, blockID uuid.UUID) error {
	if err := s.requirePlanningSprint(ctx, sprintID); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, sprintID, blockID); err != nil {
		return fmt.Errorf("delete sprint block: %w", err)
	}
	return nil
}

func (s *SprintBlockService) Reorder(ctx context.Context, sprintID uuid.UUID, blockIDs []uuid.UUID) error {
	if err := s.requirePlanningSprint(ctx, sprintID); err != nil {
		return err
	}
	blocks, err := s.repository.GetAllBySprintID(ctx, sprintID)
	if err != nil {
		return fmt.Errorf("get sprint blocks for reorder: %w", err)
	}
	if len(blocks) != len(blockIDs) {
		return ErrInvalidSprintBlockOrder
	}
	expected := make(map[uuid.UUID]struct{}, len(blocks))
	for _, block := range blocks {
		expected[block.ID] = struct{}{}
	}
	seen := make(map[uuid.UUID]struct{}, len(blockIDs))
	for _, id := range blockIDs {
		if _, duplicate := seen[id]; duplicate {
			return ErrInvalidSprintBlockOrder
		}
		if _, exists := expected[id]; !exists {
			return ErrInvalidSprintBlockOrder
		}
		seen[id] = struct{}{}
	}
	if err := s.repository.Reorder(ctx, sprintID, blockIDs); err != nil {
		return fmt.Errorf("reorder sprint blocks: %w", err)
	}
	return nil
}

func (s *SprintBlockService) requirePlanningSprint(ctx context.Context, sprintID uuid.UUID) error {
	value, err := s.sprintReader.GetById(ctx, sprintID)
	if err != nil {
		return err
	}
	if value.Status != sprint.Creating {
		return ErrSprintNotInPlanning
	}
	return nil
}

func (s *SprintBlockService) requireSubject(ctx context.Context, subjectID uuid.UUID) error {
	_, err := s.subjectReader.GetById(ctx, subjectID)
	return err
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

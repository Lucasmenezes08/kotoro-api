package fixedcycles

import (
	"context"
	"strings"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/google/uuid"
)

type SubjectReader interface {
	GetById(ctx context.Context, id uuid.UUID) (*subject.Subject, error)
}

type Service interface {
	Create(ctx context.Context, userID uuid.UUID, input CreateFixedCycleInput) (*FixedCycle, error)
	GetAll(ctx context.Context, userID uuid.UUID, dayOfWeek *string) ([]FixedCycle, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*FixedCycle, error)
	Update(ctx context.Context, userID, id uuid.UUID, input UpdateFixedCycleInput) (*FixedCycle, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type fixedCycleService struct {
	repository    Repository
	subjectReader SubjectReader
}

func NewService(repository Repository, subjectReader SubjectReader) Service {
	return &fixedCycleService{
		repository:    repository,
		subjectReader: subjectReader,
	}
}

func normalizeDayOfWeek(day string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(day))
	switch d {
	case "", "all", "todos", "todos os dias":
		return "all", nil
	case "domingo", "sunday", "dom", "0":
		return "domingo", nil
	case "segunda", "segunda-feira", "seg", "monday", "mon", "1":
		return "segunda", nil
	case "terca", "terça", "terca-feira", "terça-feira", "ter", "tuesday", "tue", "2":
		return "terca", nil
	case "quarta", "quarta-feira", "qua", "wednesday", "wed", "3":
		return "quarta", nil
	case "quinta", "quinta-feira", "qui", "thursday", "thu", "4":
		return "quinta", nil
	case "sexta", "sexta-feira", "sex", "friday", "fri", "5":
		return "sexta", nil
	case "sabado", "sábado", "sab", "saturday", "sat", "6":
		return "sabado", nil
	default:
		return "", ErrFixedCycleInvalidDayOfWeek
	}
}

func (s *fixedCycleService) Create(ctx context.Context, userID uuid.UUID, input CreateFixedCycleInput) (*FixedCycle, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, ErrFixedCycleTitleRequired
	}
	if len(title) > 150 {
		return nil, ErrFixedCycleTitleTooLong
	}

	if input.SubjectID == uuid.Nil {
		return nil, ErrFixedCycleInvalidSubject
	}
	sub, err := s.subjectReader.GetById(ctx, input.SubjectID)
	if err != nil || sub == nil || sub.UserId != userID {
		return nil, ErrFixedCycleInvalidSubject
	}

	dayOfWeek, err := normalizeDayOfWeek(input.DayOfWeek)
	if err != nil {
		return nil, err
	}
	input.DayOfWeek = dayOfWeek

	if input.EstimatedSeconds <= 0 {
		input.EstimatedSeconds = 3600
	}

	cleanedTasks := make([]string, 0, len(input.Tasks))
	for _, t := range input.Tasks {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			cleanedTasks = append(cleanedTasks, trimmed)
		}
	}
	input.Tasks = cleanedTasks
	input.Title = title

	return s.repository.Create(ctx, userID, input)
}

func (s *fixedCycleService) GetAll(ctx context.Context, userID uuid.UUID, dayOfWeek *string) ([]FixedCycle, error) {
	var normalizedDay *string
	if dayOfWeek != nil && strings.TrimSpace(*dayOfWeek) != "" {
		nd, err := normalizeDayOfWeek(*dayOfWeek)
		if err == nil && nd != "all" {
			normalizedDay = &nd
		}
	}
	return s.repository.GetAll(ctx, userID, normalizedDay)
}

func (s *fixedCycleService) GetByID(ctx context.Context, userID, id uuid.UUID) (*FixedCycle, error) {
	return s.repository.GetByID(ctx, userID, id)
}

func (s *fixedCycleService) Update(ctx context.Context, userID, id uuid.UUID, input UpdateFixedCycleInput) (*FixedCycle, error) {
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return nil, ErrFixedCycleTitleRequired
		}
		if len(title) > 150 {
			return nil, ErrFixedCycleTitleTooLong
		}
		input.Title = &title
	}

	if input.SubjectID != nil {
		if *input.SubjectID == uuid.Nil {
			return nil, ErrFixedCycleInvalidSubject
		}
		sub, err := s.subjectReader.GetById(ctx, *input.SubjectID)
		if err != nil || sub == nil || sub.UserId != userID {
			return nil, ErrFixedCycleInvalidSubject
		}
	}

	if input.DayOfWeek != nil {
		nd, err := normalizeDayOfWeek(*input.DayOfWeek)
		if err != nil {
			return nil, err
		}
		input.DayOfWeek = &nd
	}

	if input.EstimatedSeconds != nil && *input.EstimatedSeconds <= 0 {
		return nil, ErrFixedCycleInvalidEstimated
	}

	if input.Tasks != nil {
		cleanedTasks := make([]string, 0, len(*input.Tasks))
		for _, t := range *input.Tasks {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				cleanedTasks = append(cleanedTasks, trimmed)
			}
		}
		input.Tasks = &cleanedTasks
	}

	return s.repository.Update(ctx, userID, id, input)
}

func (s *fixedCycleService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repository.Delete(ctx, userID, id)
}

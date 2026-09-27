package sprint

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSprintNameEmpty     error = errors.New("Sprint name is empty")
	ErrSprintNotFound      error = errors.New("Sprint not found")
	ErrSprintAlreadyExists error = errors.New("Only one sprint per day is allowed")
	ErrSprintUpdateEmpty   error = errors.New("Update fields must have one value at least")
	ErrSprintStatusInvalid error = errors.New("Sprint status is invalid")
	ErrSprintDateInPast    error = errors.New("sprint date cannot be in the past")
	ErrSprintDateRequired  error = errors.New("sprint date is required")
)

type SprintServiceContract interface {
	Create(ctx context.Context, sprint CreateSprintModel) error
	GetAll(ctx context.Context) ([]Sprint, error)
	GetById(ctx context.Context, id uuid.UUID) (*Sprint, error)
	Update(ctx context.Context, id uuid.UUID, payload UpdateSprintModel) error
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type SprintService struct {
	repo SprintRepositoryContract
}

func NewSprintService(repo SprintRepositoryContract) *SprintService {
	return &SprintService{
		repo: repo,
	}
}

func (s *SprintService) Create(ctx context.Context, payload CreateSprintModel) error {
	if payload.SprintDate.IsZero() {
		return ErrSprintDateRequired
	}

	location := payload.SprintDate.Location()
	timeToday := beginningOfDay(time.Now(), location)

	sprintDate := beginningOfDay(payload.SprintDate, location)

	if sprintDate.Before(timeToday) {
		return ErrSprintDateInPast
	}

	if payload.Name != nil {
		if strings.TrimSpace(*payload.Name) == "" {
			return ErrSprintNameEmpty
		}
	}

	exist, err := s.repo.ExistsByDate(ctx, payload.SprintDate)
	if err != nil {
		return err
	}

	if exist {
		return ErrSprintAlreadyExists
	}

	input := CreateSprintModel{
		Name:       payload.Name,
		SprintDate: payload.SprintDate,
		Status:     Creating,
	}

	if sprint := s.repo.Create(ctx, input); sprint != nil {
		return sprint
	}

	return nil
}

func (s *SprintService) GetAll(ctx context.Context) ([]Sprint, error) {
	sprints, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error to get all sprints: %w", err)
	}

	return sprints, nil
}

func (s *SprintService) GetById(ctx context.Context, id uuid.UUID) (*Sprint, error) {
	sprint, err := s.repo.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	return sprint, nil
}

func (s *SprintService) Update(ctx context.Context, id uuid.UUID, payload UpdateSprintModel) error {
	if payload.Name == nil && payload.Status == nil {
		return ErrSprintUpdateEmpty
	}

	if payload.Name != nil && strings.TrimSpace(*payload.Name) == "" {
		return ErrSprintNameEmpty
	}

	if payload.Status != nil && !validSprintStatus(*payload.Status) {
		return ErrSprintStatusInvalid
	}

	if err := s.repo.Update(ctx, id, payload); err != nil {
		return err
	}

	return nil
}

func (s *SprintService) DeleteById(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteById(ctx, id); err != nil {
		return err
	}

	return nil
}

func validSprintStatus(status SprintStatus) bool {
	switch status {
	case Creating, InProgress, Finished, Abandoned:
		return true
	default:
		return false
	}
}

func beginningOfDay(
	value time.Time,
	location *time.Location,
) time.Time {
	localTime := value.In(location)

	return time.Date(
		localTime.Year(),
		localTime.Month(),
		localTime.Day(),
		0,
		0,
		0,
		0,
		location,
	)
}

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
	ErrSprintNameEmpty               error = errors.New("Sprint name is empty")
	ErrSprintNotFound                error = errors.New("Sprint not found")
	ErrSprintAlreadyExists           error = errors.New("Only one sprint per day is allowed")
	ErrSprintUpdateEmpty             error = errors.New("Update fields must have one value at least")
	ErrSprintStatusInvalid           error = errors.New("Sprint status is invalid")
	ErrSprintDateInPast              error = errors.New("sprint date cannot be in the past")
	ErrSprintDateRequired            error = errors.New("sprint date is required")
	ErrSprintDeleteStatusNotCreating error = errors.New("Sprint can only be deleted while its status is 'creating'")
	ErrSprintNotEditable             error = errors.New("sprint is not editable; actions can only be performed on planning or in-progress sprints from today or future")
	ErrInvalidRestMinutes            error = errors.New("rest_minutes must be between 1 and 120")
)

type SprintServiceContract interface {
	Create(ctx context.Context, userID uuid.UUID, sprint CreateSprintModel) error
	GetAll(ctx context.Context, userID uuid.UUID) ([]Sprint, error)
	GetById(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*Sprint, error)
	Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, payload UpdateSprintModel) error
	DeleteById(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type SprintService struct {
	repo SprintRepositoryContract
}

func NewSprintService(repo SprintRepositoryContract) *SprintService {
	return &SprintService{
		repo: repo,
	}
}

func (s *SprintService) Create(ctx context.Context, userID uuid.UUID, payload CreateSprintModel) error {
	if payload.SprintDate.IsZero() {
		return ErrSprintDateRequired
	}

	if IsPastDate(payload.SprintDate) {
		return ErrSprintDateInPast
	}

	if payload.Name != nil {
		if strings.TrimSpace(*payload.Name) == "" {
			return ErrSprintNameEmpty
		}
	}

	exist, err := s.repo.ExistsByDate(ctx, userID, payload.SprintDate)
	if err != nil {
		return err
	}

	if exist {
		return ErrSprintAlreadyExists
	}

	restMinutes := payload.RestMinutes
	if restMinutes <= 0 {
		restMinutes = 10
	}
	if restMinutes > 120 {
		return ErrInvalidRestMinutes
	}

	input := CreateSprintModel{
		UserID:      userID,
		Name:        payload.Name,
		SprintDate:  payload.SprintDate,
		RestMinutes: restMinutes,
		Status:      Creating,
	}

	if err := s.repo.Create(ctx, input); err != nil {
		return err
	}

	return nil
}

func (s *SprintService) GetAll(ctx context.Context, userID uuid.UUID) ([]Sprint, error) {
	sprints, err := s.repo.GetAll(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("Error to get all sprints: %w", err)
	}

	return sprints, nil
}

func (s *SprintService) GetById(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*Sprint, error) {
	sprint, err := s.repo.GetByIdAndUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	return sprint, nil
}

func (s *SprintService) Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, payload UpdateSprintModel) error {
	if payload.Name == nil && payload.Status == nil && payload.RestMinutes == nil {
		return ErrSprintUpdateEmpty
	}

	if payload.Name != nil && strings.TrimSpace(*payload.Name) == "" {
		return ErrSprintNameEmpty
	}

	if payload.RestMinutes != nil {
		if *payload.RestMinutes <= 0 || *payload.RestMinutes > 120 {
			return ErrInvalidRestMinutes
		}
	}

	if payload.Status != nil && !validSprintStatus(*payload.Status) {
		return ErrSprintStatusInvalid
	}

	existing, err := s.repo.GetByIdAndUser(ctx, userID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrSprintNotFound
	}

	if existing.Status != Creating && existing.Status != InProgress {
		return ErrSprintNotEditable
	}

	if IsPastDate(existing.SprintDate) {
		return ErrSprintNotEditable
	}

	if err := s.repo.Update(ctx, userID, id, payload); err != nil {
		return err
	}

	return nil
}

func (s *SprintService) DeleteById(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	sprint, err := s.repo.GetByIdAndUser(ctx, userID, id)
	if err != nil {
		return err
	}

	if sprint.Status != Creating {
		return ErrSprintDeleteStatusNotCreating
	}

	if IsPastDate(sprint.SprintDate) {
		return ErrSprintDateInPast
	}

	if err := s.repo.DeleteById(ctx, userID, id); err != nil {
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

func BeginningOfDay(
	value time.Time,
	location *time.Location,
) time.Time {
	if location == nil {
		location = value.Location()
	}
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

func IsPastDate(date time.Time) bool {
	return IsPastDateAt(date, time.Now(), time.Local)
}

// IsPastDateAt checks whether the civil calendar date stored in date precedes
// the calendar day of now in location. PostgreSQL DATE values arrive in time.UTC
// even when the app runs in a local timezone, so date's stored Y/M/D must not
// be shifted via .In(location).
func IsPastDateAt(date time.Time, now time.Time, location *time.Location) bool {
	if date.IsZero() {
		return false
	}
	if location == nil {
		location = time.Local
	}
	timeToday := BeginningOfDay(now, location)
	year, month, day := date.Date()
	sprintDay := time.Date(year, month, day, 0, 0, 0, 0, location)
	return sprintDay.Before(timeToday)
}

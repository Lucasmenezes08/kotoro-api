package studycycles

import (
	"context"
	"time"

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

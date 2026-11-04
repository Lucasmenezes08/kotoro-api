package studycycles

import (
	"context"
	"time"

	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type StudyCycleTxRepositoryContract interface {
	LockSprintByID(ctx context.Context, sprintID uuid.UUID) (*sprint.Sprint, error)
	HasActiveCycleBySprintID(ctx context.Context, sprintID uuid.UUID) (bool, error)
	LockBlocksBySprintID(ctx context.Context, sprintID uuid.UUID) ([]sprintblocks.SprintBlock, error)
	GetActiveTasksByBlockID(ctx context.Context, blockID uuid.UUID) ([]blocktasks.BlockTask, error)
	GetNextSequenceNumber(ctx context.Context, sprintID uuid.UUID) (int, error)
	CreateCycle(ctx context.Context, input CreateStudyCycleModel) (*StudyCycle, error)
	AttachCycleTasks(ctx context.Context, cycleID uuid.UUID, taskIDs []uuid.UUID) error
	UpdateBlockStatus(ctx context.Context, blockID uuid.UUID, status sprintblocks.SprintBlocksStatus, startedAt, completedAt *time.Time) error
	UpdateTasksStatus(ctx context.Context, blockID uuid.UUID, taskIDs []uuid.UUID, status blocktasks.BlockTaskStatus, completedAt *time.Time) error

	LockCycleByID(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error)
	GetCycleTaskIDs(ctx context.Context, cycleID uuid.UUID) ([]uuid.UUID, error)
	CompleteCycleTasks(ctx context.Context, cycleID uuid.UUID, completedAt time.Time) error
	StopCycleConditional(ctx context.Context, input StopStudyCycleModel) (*StudyCycle, error)
}

type StudyCycleRepositoryContract interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx StudyCycleTxRepositoryContract) error) error
	GetByID(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error)
	PauseConditional(ctx context.Context, input PauseStudyCycleModel) (*StudyCycle, error)
	ResumeConditional(ctx context.Context, input ResumeStudyCycleModel) (*StudyCycle, error)
}

type StudyCycleRepository struct {
	db *sqlx.DB
}

func NewStudyCycleRepository(db *sqlx.DB) *StudyCycleRepository {
	return &StudyCycleRepository{
		db: db,
	}
}

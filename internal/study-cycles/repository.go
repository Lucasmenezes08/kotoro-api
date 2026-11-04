package studycycles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

const (
	studyCycleUniqueViolationCode = "23505"
	activeSprintCycleUniqueIndex  = "uq_study_cycles_active_sprint"
	sprintSequenceUniqueIndex     = "uq_study_cycles_sprint_sequence"
	studyCycleColumns             = `id, sprint_id, sprint_block_id, sequence_number, execution_mode, timer_type, status, planned_duration_seconds, started_at, running_since, paused_at, ended_at, elapsed_seconds, stop_reason, created_at, updated_at`
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

type studyCycleTxRepository struct {
	tx *sqlx.Tx
}

func NewStudyCycleRepository(db *sqlx.DB) *StudyCycleRepository {
	return &StudyCycleRepository{
		db: db,
	}
}

func (r *StudyCycleRepository) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx StudyCycleTxRepositoryContract) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin study cycle transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(ctx, &studyCycleTxRepository{tx: tx}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit study cycle transaction: %w", err)
	}

	return nil
}

func (r *StudyCycleRepository) GetByID(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error) {
	var cycle StudyCycle
	query := `SELECT ` + studyCycleColumns + ` FROM study_cycles WHERE id = $1 LIMIT 1`
	err := r.db.GetContext(ctx, &cycle, query, cycleID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrStudyCycleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get study cycle by id: %w", err)
	}
	return &cycle, nil
}

func (r *StudyCycleRepository) PauseConditional(ctx context.Context, input PauseStudyCycleModel) (*StudyCycle, error) {
	return nil, errors.New("not implemented")
}

func (r *StudyCycleRepository) ResumeConditional(ctx context.Context, input ResumeStudyCycleModel) (*StudyCycle, error) {
	return nil, errors.New("not implemented")
}

func (t *studyCycleTxRepository) LockSprintByID(ctx context.Context, sprintID uuid.UUID) (*sprint.Sprint, error) {
	var sp sprint.Sprint
	query := `
		SELECT
			id,
			name,
			sprint_date,
			status,
			started_at,
			completed_at,
			created_at,
			updated_at,
			deleted_at
		FROM sprints
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`
	err := t.tx.GetContext(ctx, &sp, query, sprintID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sprint.ErrSprintNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock sprint by id: %w", err)
	}
	return &sp, nil
}

func (t *studyCycleTxRepository) HasActiveCycleBySprintID(ctx context.Context, sprintID uuid.UUID) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM study_cycles
			WHERE sprint_id = $1
			  AND status IN ('em_andamento', 'pausado')
		)
	`
	if err := t.tx.GetContext(ctx, &exists, query, sprintID); err != nil {
		return false, fmt.Errorf("check active study cycle: %w", err)
	}
	return exists, nil
}

func (t *studyCycleTxRepository) LockBlocksBySprintID(ctx context.Context, sprintID uuid.UUID) ([]sprintblocks.SprintBlock, error) {
	blocks := make([]sprintblocks.SprintBlock, 0)
	query := `
		SELECT
			id,
			sprint_id,
			subject_id,
			title,
			position,
			status,
			estimated_seconds,
			started_at,
			completed_at,
			created_at,
			updated_at,
			deleted_at
		FROM sprint_block
		WHERE sprint_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC, id ASC
		FOR UPDATE
	`
	if err := t.tx.SelectContext(ctx, &blocks, query, sprintID); err != nil {
		return nil, fmt.Errorf("lock sprint blocks: %w", err)
	}
	return blocks, nil
}

func (t *studyCycleTxRepository) GetActiveTasksByBlockID(ctx context.Context, blockID uuid.UUID) ([]blocktasks.BlockTask, error) {
	tasks := make([]blocktasks.BlockTask, 0)
	query := `
		SELECT
			id,
			sprint_block_id,
			title,
			description,
			position,
			status,
			priority,
			estimated_seconds,
			created_at,
			updated_at,
			completed_at,
			deleted_at
		FROM block_tasks
		WHERE sprint_block_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC, id ASC
		FOR UPDATE
	`
	if err := t.tx.SelectContext(ctx, &tasks, query, blockID); err != nil {
		return nil, fmt.Errorf("lock block tasks: %w", err)
	}
	return tasks, nil
}

func (t *studyCycleTxRepository) GetNextSequenceNumber(ctx context.Context, sprintID uuid.UUID) (int, error) {
	var nextSeq int
	query := `
		SELECT COALESCE(MAX(sequence_number), 0) + 1
		FROM study_cycles
		WHERE sprint_id = $1
	`
	if err := t.tx.GetContext(ctx, &nextSeq, query, sprintID); err != nil {
		return 0, fmt.Errorf("calculate next sequence number: %w", err)
	}
	return nextSeq, nil
}

func (t *studyCycleTxRepository) CreateCycle(ctx context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
	var cycle StudyCycle
	query := `
		INSERT INTO study_cycles (
			sprint_id,
			sprint_block_id,
			sequence_number,
			execution_mode,
			timer_type,
			status,
			planned_duration_seconds,
			started_at,
			running_since,
			paused_at,
			ended_at,
			elapsed_seconds,
			stop_reason
		) VALUES (
			$1,
			$2,
			$3,
			$4::execution_mode,
			$5::timer_type,
			$6::status,
			$7,
			$8::timestamptz,
			$9::timestamptz,
			$10::timestamptz,
			$11::timestamptz,
			$12,
			$13::cycle_stop_reason
		)
		RETURNING ` + studyCycleColumns

	err := t.tx.GetContext(
		ctx,
		&cycle,
		query,
		input.SprintID,
		input.SprintBlockID,
		input.SequenceNumber,
		input.ExecutionMode,
		input.TimerType,
		input.Status,
		input.PlannedDurationSeconds,
		input.StartedAt,
		input.RunningSince,
		input.PausedAt,
		input.EndedAt,
		input.ElapsedSeconds,
		input.StopReason,
	)
	if err != nil {
		return nil, translateStudyCycleConstraintError(err, "create study cycle")
	}
	return &cycle, nil
}

func (t *studyCycleTxRepository) AttachCycleTasks(ctx context.Context, cycleID uuid.UUID, taskIDs []uuid.UUID) error {
	query := `
		INSERT INTO study_cycle_tasks (study_cycle_id, block_task_id)
		VALUES ($1, $2)
	`
	for _, taskID := range taskIDs {
		if _, err := t.tx.ExecContext(ctx, query, cycleID, taskID); err != nil {
			return fmt.Errorf("attach study cycle task: %w", err)
		}
	}
	return nil
}

func (t *studyCycleTxRepository) UpdateBlockStatus(ctx context.Context, blockID uuid.UUID, status sprintblocks.SprintBlocksStatus, startedAt, completedAt *time.Time) error {
	query := `
		UPDATE sprint_block
		SET
			status = $2::sprint_block_status,
			started_at = COALESCE($3::timestamptz, started_at),
			completed_at = COALESCE($4::timestamptz, completed_at),
			updated_at = COALESCE($4::timestamptz, $3::timestamptz, now())
		WHERE id = $1 AND deleted_at IS NULL
	`
	result, err := t.tx.ExecContext(ctx, query, blockID, status, startedAt, completedAt)
	if err != nil {
		return fmt.Errorf("update sprint block status: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated sprint block rows count: %w", err)
	}
	if rows == 0 {
		return sprintblocks.ErrSprintBlockNotFound
	}
	return nil
}

func (t *studyCycleTxRepository) UpdateTasksStatus(ctx context.Context, blockID uuid.UUID, taskIDs []uuid.UUID, status blocktasks.BlockTaskStatus, completedAt *time.Time) error {
	return errors.New("not implemented")
}

func (t *studyCycleTxRepository) LockCycleByID(ctx context.Context, cycleID uuid.UUID) (*StudyCycle, error) {
	return nil, errors.New("not implemented")
}

func (t *studyCycleTxRepository) GetCycleTaskIDs(ctx context.Context, cycleID uuid.UUID) ([]uuid.UUID, error) {
	return nil, errors.New("not implemented")
}

func (t *studyCycleTxRepository) CompleteCycleTasks(ctx context.Context, cycleID uuid.UUID, completedAt time.Time) error {
	return errors.New("not implemented")
}

func (t *studyCycleTxRepository) StopCycleConditional(ctx context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
	return nil, errors.New("not implemented")
}

func translateStudyCycleConstraintError(err error, operation string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == studyCycleUniqueViolationCode {
		switch pgErr.ConstraintName {
		case activeSprintCycleUniqueIndex, sprintSequenceUniqueIndex:
			return ErrStudyCycleAlreadyActive
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

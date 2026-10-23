package blocktasks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

const (
	blockTaskUniqueViolationCode = "23505"
	activeTaskPositionIndex      = "block_tasks_active_position_unique"
	blockTaskColumns             = `id, sprint_block_id, title, description, position, status, priority, estimated_seconds, created_at, updated_at, completed_at, deleted_at`
)

type BlockTaskRepositoryContract interface {
	Create(context.Context, CreateBlockTaskModel) (*BlockTask, error)
	GetAllByBlockID(context.Context, uuid.UUID) ([]BlockTask, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID) (*BlockTask, error)
	Update(context.Context, uuid.UUID, uuid.UUID, BlockTaskUpdateModel) (*BlockTask, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
	Reorder(context.Context, uuid.UUID, []uuid.UUID) error
	ExistsByPosition(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error)
}

type BlockTaskRepository struct{ db *sqlx.DB }

func NewBlockTaskRepository(db *sqlx.DB) *BlockTaskRepository {
	return &BlockTaskRepository{db: db}
}

func (r *BlockTaskRepository) Create(ctx context.Context, input CreateBlockTaskModel) (*BlockTask, error) {
	query := `INSERT INTO block_tasks
		(sprint_block_id, title, description, position, status, priority, estimated_seconds)
		VALUES (:sprint_block_id, :title, :description, :position, :status, :priority, :estimated_seconds)
		RETURNING ` + blockTaskColumns
	rows, err := r.db.NamedQueryContext(ctx, query, input)
	if err != nil {
		return nil, translateBlockTaskConstraintError(err, "create block task")
	}
	defer rows.Close()
	var task BlockTask
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("create block task: %w", err)
		}
		return nil, errors.New("create block task: no row returned")
	}
	if err := rows.StructScan(&task); err != nil {
		return nil, fmt.Errorf("scan created block task: %w", err)
	}
	return &task, nil
}

func (r *BlockTaskRepository) GetAllByBlockID(ctx context.Context, blockID uuid.UUID) ([]BlockTask, error) {
	tasks := make([]BlockTask, 0)
	query := `SELECT ` + blockTaskColumns + ` FROM block_tasks
		WHERE sprint_block_id = $1 AND deleted_at IS NULL ORDER BY position ASC, id ASC`
	if err := r.db.SelectContext(ctx, &tasks, query, blockID); err != nil {
		return nil, fmt.Errorf("get block tasks: %w", err)
	}
	return tasks, nil
}

func (r *BlockTaskRepository) GetByID(ctx context.Context, blockID, taskID uuid.UUID) (*BlockTask, error) {
	var task BlockTask
	query := `SELECT ` + blockTaskColumns + ` FROM block_tasks
		WHERE sprint_block_id = $1 AND id = $2 AND deleted_at IS NULL LIMIT 1`
	err := r.db.GetContext(ctx, &task, query, blockID, taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBlockTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get block task: %w", err)
	}
	return &task, nil
}

func (r *BlockTaskRepository) Update(ctx context.Context, blockID, taskID uuid.UUID, input BlockTaskUpdateModel) (*BlockTask, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update block task: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	query := `UPDATE block_tasks SET
		title = COALESCE($1::varchar, title),
		description = CASE WHEN $2::boolean THEN $3::text ELSE description END,
		position = COALESCE($4::integer, position),
		priority = COALESCE($5::block_task_priority, priority),
		estimated_seconds = COALESCE($6::integer, estimated_seconds),
		updated_at = now()
		WHERE sprint_block_id = $7 AND id = $8 AND deleted_at IS NULL`
	result, err := tx.ExecContext(ctx, query, input.Title, input.DescriptionSet, input.Description,
		input.Position, input.Priority, input.EstimatedSeconds, blockID, taskID)
	if err != nil {
		return nil, translateBlockTaskConstraintError(err, "update block task")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get updated block task rows count: %w", err)
	}
	if rows == 0 {
		return nil, ErrBlockTaskNotFound
	}
	var task BlockTask
	selectQuery := `SELECT ` + blockTaskColumns + ` FROM block_tasks
		WHERE sprint_block_id = $1 AND id = $2 AND deleted_at IS NULL LIMIT 1`
	if err := tx.GetContext(ctx, &task, selectQuery, blockID, taskID); err != nil {
		return nil, fmt.Errorf("get updated block task: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit update block task: %w", err)
	}
	return &task, nil
}

func (r *BlockTaskRepository) Delete(ctx context.Context, blockID, taskID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete block task: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE block_tasks SET deleted_at = now(), updated_at = now()
		WHERE sprint_block_id = $1 AND id = $2 AND deleted_at IS NULL`, blockID, taskID)
	if err != nil {
		return fmt.Errorf("delete block task: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted block task rows count: %w", err)
	}
	if rows == 0 {
		return ErrBlockTaskNotFound
	}
	ids, err := lockActiveTaskIDs(ctx, tx, blockID)
	if err != nil {
		return err
	}
	if err := reorderTasksInTransaction(ctx, tx, blockID, ids); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete block task: %w", err)
	}
	return nil
}

func (r *BlockTaskRepository) Reorder(ctx context.Context, blockID uuid.UUID, taskIDs []uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder block tasks: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	activeIDs, err := lockActiveTaskIDs(ctx, tx, blockID)
	if err != nil {
		return err
	}
	if !sameTaskIDSet(activeIDs, taskIDs) {
		return ErrBlockTaskOrderInvalid
	}
	if err := reorderTasksInTransaction(ctx, tx, blockID, taskIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder block tasks: %w", err)
	}
	return nil
}

func (r *BlockTaskRepository) ExistsByPosition(ctx context.Context, blockID uuid.UUID, position int, excludeID *uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM block_tasks WHERE sprint_block_id = $1
		AND position = $2 AND deleted_at IS NULL AND ($3::uuid IS NULL OR id <> $3))`
	if err := r.db.GetContext(ctx, &exists, query, blockID, position, excludeID); err != nil {
		return false, fmt.Errorf("check block task position: %w", err)
	}
	return exists, nil
}

func lockActiveTaskIDs(ctx context.Context, tx *sqlx.Tx, blockID uuid.UUID) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0)
	query := `SELECT id FROM block_tasks WHERE sprint_block_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC, id ASC FOR UPDATE`
	if err := tx.SelectContext(ctx, &ids, query, blockID); err != nil {
		return nil, fmt.Errorf("lock block tasks: %w", err)
	}
	return ids, nil
}

func reorderTasksInTransaction(ctx context.Context, tx *sqlx.Tx, blockID uuid.UUID, taskIDs []uuid.UUID) error {
	if len(taskIDs) == 0 {
		return nil
	}
	var maxPosition int
	if err := tx.GetContext(ctx, &maxPosition, `SELECT COALESCE(MAX(position), 0) FROM block_tasks
		WHERE sprint_block_id = $1 AND deleted_at IS NULL`, blockID); err != nil {
		return fmt.Errorf("get maximum block task position: %w", err)
	}
	offset := maxPosition + len(taskIDs) + 1
	result, err := tx.ExecContext(ctx, `UPDATE block_tasks SET position = position + $1
		WHERE sprint_block_id = $2 AND deleted_at IS NULL`, offset, blockID)
	if err != nil {
		return fmt.Errorf("move block tasks to temporary positions: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get temporarily moved block task rows count: %w", err)
	}
	if rows != int64(len(taskIDs)) {
		return ErrBlockTaskOrderInvalid
	}
	for index, id := range taskIDs {
		result, err := tx.ExecContext(ctx, `UPDATE block_tasks SET position = $1, updated_at = now()
			WHERE sprint_block_id = $2 AND id = $3 AND deleted_at IS NULL`, index+1, blockID, id)
		if err != nil {
			return translateBlockTaskConstraintError(err, "reorder block task")
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("get reordered block task rows count: %w", err)
		}
		if rows != 1 {
			return ErrBlockTaskOrderInvalid
		}
	}
	return nil
}

func sameTaskIDSet(left, right []uuid.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[uuid.UUID]struct{}, len(left))
	for _, id := range left {
		seen[id] = struct{}{}
	}
	for _, id := range right {
		if _, ok := seen[id]; !ok {
			return false
		}
		delete(seen, id)
	}
	return len(seen) == 0
}

func translateBlockTaskConstraintError(err error, operation string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == blockTaskUniqueViolationCode && pgErr.ConstraintName == activeTaskPositionIndex {
		return ErrBlockTaskPositionConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}

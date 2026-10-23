package sprintblocks

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
	uniqueViolationCode       = "23505"
	activePositionUniqueIndex = "sprint_block_active_position_unique"
	activeSubjectUniqueIndex  = "sprint_block_active_subject_unique"
	sprintBlockColumns        = `id, sprint_id, subject_id, title, position, status, estimated_seconds, started_at, completed_at, created_at, updated_at, deleted_at`
)

type SprintBlockRepositoryContract interface {
	Create(context.Context, CreateSprintBlockModel) (*SprintBlock, error)
	GetAllBySprintID(context.Context, uuid.UUID) ([]SprintBlock, error)
	GetByID(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error)
	Update(context.Context, uuid.UUID, uuid.UUID, SprintBlockUpdateModel) (*SprintBlock, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
	Reorder(context.Context, uuid.UUID, []uuid.UUID) error
	ExistsBySubjectID(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (bool, error)
	ExistsByPosition(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error)
}

type SprintBlockRepository struct{ db *sqlx.DB }

func NewSprintBlockRepository(db *sqlx.DB) *SprintBlockRepository {
	return &SprintBlockRepository{db: db}
}

func (r *SprintBlockRepository) Create(ctx context.Context, input CreateSprintBlockModel) (*SprintBlock, error) {
	query := `INSERT INTO sprint_block (sprint_id, subject_id, title, position, status, estimated_seconds)
		VALUES (:sprint_id, :subject_id, :title, :position, :status, :estimated_seconds)
		RETURNING ` + sprintBlockColumns
	rows, err := r.db.NamedQueryContext(ctx, query, input)
	if err != nil {
		return nil, translateConstraintError(err, "create sprint block")
	}
	defer rows.Close()
	var block SprintBlock
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("create sprint block: %w", err)
		}
		return nil, errors.New("create sprint block: no row returned")
	}
	if err := rows.StructScan(&block); err != nil {
		return nil, fmt.Errorf("scan created sprint block: %w", err)
	}
	return &block, nil
}

func (r *SprintBlockRepository) GetAllBySprintID(ctx context.Context, sprintID uuid.UUID) ([]SprintBlock, error) {
	blocks := make([]SprintBlock, 0)
	query := `SELECT ` + sprintBlockColumns + ` FROM sprint_block
		WHERE sprint_id = $1 AND deleted_at IS NULL ORDER BY position ASC, id ASC`
	if err := r.db.SelectContext(ctx, &blocks, query, sprintID); err != nil {
		return nil, fmt.Errorf("get sprint blocks: %w", err)
	}
	return blocks, nil
}

func (r *SprintBlockRepository) GetByID(ctx context.Context, sprintID, blockID uuid.UUID) (*SprintBlock, error) {
	var block SprintBlock
	query := `SELECT ` + sprintBlockColumns + ` FROM sprint_block
		WHERE sprint_id = $1 AND id = $2 AND deleted_at IS NULL LIMIT 1`
	err := r.db.GetContext(ctx, &block, query, sprintID, blockID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSprintBlockNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get sprint block: %w", err)
	}
	return &block, nil
}

func (r *SprintBlockRepository) Update(ctx context.Context, sprintID, blockID uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update sprint block: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `UPDATE sprint_block SET
		subject_id = COALESCE($1::uuid, subject_id), title = COALESCE($2::varchar, title),
		position = COALESCE($3::integer, position), estimated_seconds = COALESCE($4::integer, estimated_seconds),
		updated_at = now()
		WHERE sprint_id = $5 AND id = $6 AND deleted_at IS NULL`
	result, err := tx.ExecContext(ctx, query, input.SubjectID, input.Title, input.Position, input.EstimatedSeconds, sprintID, blockID)
	if err != nil {
		return nil, translateConstraintError(err, "update sprint block")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get updated sprint block rows count: %w", err)
	}
	if rows == 0 {
		return nil, ErrSprintBlockNotFound
	}

	var block SprintBlock
	selectQuery := `SELECT ` + sprintBlockColumns + ` FROM sprint_block
		WHERE sprint_id = $1 AND id = $2 AND deleted_at IS NULL LIMIT 1`
	if err := tx.GetContext(ctx, &block, selectQuery, sprintID, blockID); err != nil {
		return nil, fmt.Errorf("get updated sprint block: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit update sprint block: %w", err)
	}
	return &block, nil
}

func (r *SprintBlockRepository) Delete(ctx context.Context, sprintID, blockID uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete sprint block: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE sprint_block SET deleted_at = now(), updated_at = now()
		WHERE sprint_id = $1 AND id = $2 AND deleted_at IS NULL`, sprintID, blockID)
	if err != nil {
		return fmt.Errorf("delete sprint block: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted sprint block rows count: %w", err)
	}
	if rows == 0 {
		return ErrSprintBlockNotFound
	}
	activeIDs, err := lockActiveBlockIDs(ctx, tx, sprintID)
	if err != nil {
		return err
	}
	if err := reorderInTransaction(ctx, tx, sprintID, activeIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete sprint block: %w", err)
	}
	return nil
}

func (r *SprintBlockRepository) Reorder(ctx context.Context, sprintID uuid.UUID, blockIDs []uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder sprint blocks: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	activeIDs, err := lockActiveBlockIDs(ctx, tx, sprintID)
	if err != nil {
		return err
	}
	if !sameUUIDSet(activeIDs, blockIDs) {
		return ErrInvalidSprintBlockOrder
	}
	if err := reorderInTransaction(ctx, tx, sprintID, blockIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder sprint blocks: %w", err)
	}
	return nil
}

func (r *SprintBlockRepository) ExistsBySubjectID(ctx context.Context, sprintID, subjectID uuid.UUID, excludeID *uuid.UUID) (bool, error) {
	return r.exists(ctx, `SELECT EXISTS (SELECT 1 FROM sprint_block WHERE sprint_id = $1
		AND subject_id = $2 AND deleted_at IS NULL AND ($3::uuid IS NULL OR id <> $3))`, sprintID, subjectID, excludeID)
}

func (r *SprintBlockRepository) ExistsByPosition(ctx context.Context, sprintID uuid.UUID, position int, excludeID *uuid.UUID) (bool, error) {
	return r.exists(ctx, `SELECT EXISTS (SELECT 1 FROM sprint_block WHERE sprint_id = $1
		AND position = $2 AND deleted_at IS NULL AND ($3::uuid IS NULL OR id <> $3))`, sprintID, position, excludeID)
}

func (r *SprintBlockRepository) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var exists bool
	if err := r.db.GetContext(ctx, &exists, query, args...); err != nil {
		return false, fmt.Errorf("check sprint block conflict: %w", err)
	}
	return exists, nil
}

func lockActiveBlockIDs(ctx context.Context, tx *sqlx.Tx, sprintID uuid.UUID) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0)
	query := `SELECT id FROM sprint_block WHERE sprint_id = $1 AND deleted_at IS NULL
		ORDER BY position ASC, id ASC FOR UPDATE`
	if err := tx.SelectContext(ctx, &ids, query, sprintID); err != nil {
		return nil, fmt.Errorf("lock sprint blocks: %w", err)
	}
	return ids, nil
}

func reorderInTransaction(ctx context.Context, tx *sqlx.Tx, sprintID uuid.UUID, blockIDs []uuid.UUID) error {
	if len(blockIDs) == 0 {
		return nil
	}
	var maxPosition int
	if err := tx.GetContext(ctx, &maxPosition, `SELECT COALESCE(MAX(position), 0) FROM sprint_block
		WHERE sprint_id = $1 AND deleted_at IS NULL`, sprintID); err != nil {
		return fmt.Errorf("get maximum sprint block position: %w", err)
	}
	offset := maxPosition + len(blockIDs) + 1
	result, err := tx.ExecContext(ctx, `UPDATE sprint_block SET position = position + $1
		WHERE sprint_id = $2 AND deleted_at IS NULL`, offset, sprintID)
	if err != nil {
		return fmt.Errorf("move sprint blocks to temporary positions: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get temporarily moved sprint block rows count: %w", err)
	}
	if rows != int64(len(blockIDs)) {
		return ErrInvalidSprintBlockOrder
	}
	for index, id := range blockIDs {
		result, err := tx.ExecContext(ctx, `UPDATE sprint_block SET position = $1, updated_at = now()
			WHERE sprint_id = $2 AND id = $3 AND deleted_at IS NULL`, index+1, sprintID, id)
		if err != nil {
			return translateConstraintError(err, "reorder sprint block")
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("get reordered sprint block rows count: %w", err)
		}
		if rows != 1 {
			return ErrInvalidSprintBlockOrder
		}
	}
	return nil
}

func sameUUIDSet(left, right []uuid.UUID) bool {
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

func translateConstraintError(err error, operation string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
		switch pgErr.ConstraintName {
		case activePositionUniqueIndex:
			return ErrSprintBlockPositionConflict
		case activeSubjectUniqueIndex:
			return ErrSprintBlockSubjectConflict
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

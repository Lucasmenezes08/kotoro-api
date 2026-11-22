package fixedcycles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Create(ctx context.Context, userID uuid.UUID, input CreateFixedCycleInput) (*FixedCycle, error)
	GetAll(ctx context.Context, userID uuid.UUID, dayOfWeek *string) ([]FixedCycle, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*FixedCycle, error)
	Update(ctx context.Context, userID, id uuid.UUID, input UpdateFixedCycleInput) (*FixedCycle, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type sqlxRepository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &sqlxRepository{db: db}
}

func (r *sqlxRepository) Create(ctx context.Context, userID uuid.UUID, input CreateFixedCycleInput) (*FixedCycle, error) {
	tasks := StringList(input.Tasks)
	if tasks == nil {
		tasks = StringList([]string{})
	}

	query := `
		WITH inserted AS (
			INSERT INTO fixed_cycles (user_id, subject_id, title, estimated_seconds, day_of_week, tasks)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, user_id, subject_id, title, estimated_seconds, day_of_week, tasks, created_at, updated_at, deleted_at
		)
		SELECT inserted.id, inserted.user_id, inserted.subject_id, inserted.title, inserted.estimated_seconds,
		       inserted.day_of_week, inserted.tasks, inserted.created_at, inserted.updated_at, inserted.deleted_at,
		       s.name AS subject_name, s.color AS subject_color
		FROM inserted
		LEFT JOIN subjects s ON s.id = inserted.subject_id
	`

	var fc FixedCycle
	err := r.db.GetContext(ctx, &fc, query, userID, input.SubjectID, input.Title, input.EstimatedSeconds, input.DayOfWeek, tasks)
	if err != nil {
		return nil, fmt.Errorf("create fixed cycle: %w", err)
	}
	return &fc, nil
}

func (r *sqlxRepository) GetAll(ctx context.Context, userID uuid.UUID, dayOfWeek *string) ([]FixedCycle, error) {
	query := `
		SELECT fc.id, fc.user_id, fc.subject_id, fc.title, fc.estimated_seconds, fc.day_of_week, fc.tasks,
		       fc.created_at, fc.updated_at, fc.deleted_at,
		       s.name AS subject_name, s.color AS subject_color
		FROM fixed_cycles fc
		LEFT JOIN subjects s ON s.id = fc.subject_id
		WHERE fc.user_id = $1 AND fc.deleted_at IS NULL
		  AND ($2::text IS NULL OR fc.day_of_week = 'all' OR fc.day_of_week = $2)
		ORDER BY fc.created_at ASC
	`
	cycles := make([]FixedCycle, 0)
	err := r.db.SelectContext(ctx, &cycles, query, userID, dayOfWeek)
	if err != nil {
		return nil, fmt.Errorf("get all fixed cycles: %w", err)
	}
	return cycles, nil
}

func (r *sqlxRepository) GetByID(ctx context.Context, userID, id uuid.UUID) (*FixedCycle, error) {
	query := `
		SELECT fc.id, fc.user_id, fc.subject_id, fc.title, fc.estimated_seconds, fc.day_of_week, fc.tasks,
		       fc.created_at, fc.updated_at, fc.deleted_at,
		       s.name AS subject_name, s.color AS subject_color
		FROM fixed_cycles fc
		LEFT JOIN subjects s ON s.id = fc.subject_id
		WHERE fc.id = $1 AND fc.user_id = $2 AND fc.deleted_at IS NULL
	`
	var fc FixedCycle
	err := r.db.GetContext(ctx, &fc, query, id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFixedCycleNotFound
		}
		return nil, fmt.Errorf("get fixed cycle by id: %w", err)
	}
	return &fc, nil
}

func (r *sqlxRepository) Update(ctx context.Context, userID, id uuid.UUID, input UpdateFixedCycleInput) (*FixedCycle, error) {
	current, err := r.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	subjectID := current.SubjectID
	if input.SubjectID != nil {
		subjectID = *input.SubjectID
	}
	title := current.Title
	if input.Title != nil {
		title = *input.Title
	}
	estimated := current.EstimatedSeconds
	if input.EstimatedSeconds != nil {
		estimated = *input.EstimatedSeconds
	}
	dayOfWeek := current.DayOfWeek
	if input.DayOfWeek != nil {
		dayOfWeek = *input.DayOfWeek
	}
	tasks := current.Tasks
	if input.Tasks != nil {
		tasks = StringList(*input.Tasks)
	}

	query := `
		WITH updated AS (
			UPDATE fixed_cycles
			SET subject_id = $1, title = $2, estimated_seconds = $3, day_of_week = $4, tasks = $5, updated_at = now()
			WHERE id = $6 AND user_id = $7 AND deleted_at IS NULL
			RETURNING id, user_id, subject_id, title, estimated_seconds, day_of_week, tasks, created_at, updated_at, deleted_at
		)
		SELECT updated.id, updated.user_id, updated.subject_id, updated.title, updated.estimated_seconds,
		       updated.day_of_week, updated.tasks, updated.created_at, updated.updated_at, updated.deleted_at,
		       s.name AS subject_name, s.color AS subject_color
		FROM updated
		LEFT JOIN subjects s ON s.id = updated.subject_id
	`
	var fc FixedCycle
	err = r.db.GetContext(ctx, &fc, query, subjectID, title, estimated, dayOfWeek, tasks, id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFixedCycleNotFound
		}
		return nil, fmt.Errorf("update fixed cycle: %w", err)
	}
	return &fc, nil
}

func (r *sqlxRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	query := `UPDATE fixed_cycles SET deleted_at = now() WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("delete fixed cycle: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete fixed cycle rows affected: %w", err)
	}
	if rows == 0 {
		return ErrFixedCycleNotFound
	}
	return nil
}

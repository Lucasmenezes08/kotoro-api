package sprint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type SprintRepositoryContract interface {
	Create(ctx context.Context, payload CreateSprintModel) error
	GetAll(ctx context.Context) ([]Sprint, error)
	GetById(ctx context.Context, id uuid.UUID) (*Sprint, error)
	Update(ctx context.Context, id uuid.UUID, payload UpdateSprintModel) error
	DeleteById(ctx context.Context, id uuid.UUID) error
	ExistsByDate(ctx context.Context, date time.Time) (bool, error)
}

type SprintCreateInput struct {
	Name       *string
	SprintDate time.Time
}

type CreateSprintModel struct {
	Name       *string      `db:"name"`
	SprintDate time.Time    `db:"sprint_date"`
	Status     SprintStatus `db:"status"`
}

type UpdateSprintModel struct {
	Name   *string       `db:"name"`
	Status *SprintStatus `db:"status"`
}

type SprintRepository struct {
	db *sqlx.DB
}

func NewSprintRepository(db *sqlx.DB) *SprintRepository {
	return &SprintRepository{
		db: db,
	}
}

func (r *SprintRepository) Create(ctx context.Context, sprint CreateSprintModel) error {
	query := "INSERT INTO sprints (name, sprint_date, status) VALUES(:name, :sprint_date, :status)"

	_, err := r.db.NamedExecContext(ctx, query, sprint)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_sprint_active_date" {
			return ErrSprintAlreadyExists
		}
		return fmt.Errorf("Error to create Sprint: %w", err)
	}

	return nil
}

func (r *SprintRepository) GetAll(ctx context.Context) ([]Sprint, error) {
	sprints := make([]Sprint, 0)

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
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
	`

	if err := r.db.SelectContext(ctx, &sprints, query); err != nil {
		return nil, fmt.Errorf("Error to get all sprints: %w", err)
	}

	return sprints, nil
}

func (r *SprintRepository) GetById(ctx context.Context, id uuid.UUID) (*Sprint, error) {
	var sprint Sprint

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
		LIMIT 1
	`

	err := r.db.GetContext(ctx, &sprint, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSprintNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("Error to get sprint by id: %w", err)
	}

	return &sprint, nil
}

func (r *SprintRepository) Update(ctx context.Context, id uuid.UUID, payload UpdateSprintModel) error {
	query := "UPDATE sprints SET name = COALESCE($1::varchar, name), status = COALESCE($2::sprint_status, status), updated_at = now() WHERE id = $3 AND deleted_at IS NULL"

	result, err := r.db.ExecContext(ctx, query, payload.Name, payload.Status, id)
	if err != nil {
		return fmt.Errorf("Error to update sprint: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Error to get updated sprint rows count: %w", err)
	}

	if rowsAffected == 0 {
		return ErrSprintNotFound
	}

	return nil
}

func (r *SprintRepository) DeleteById(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE sprints
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1
			AND deleted_at IS NULL
			AND status = $2::sprint_status
		RETURNING id
	`

	var deletedID uuid.UUID

	err := r.db.GetContext(ctx, &deletedID, query, id, Creating)
	if errors.Is(err, sql.ErrNoRows) {
		_, getErr := r.GetById(ctx, id)

		switch {
		case errors.Is(getErr, ErrSprintNotFound):
			return ErrSprintNotFound
		case getErr != nil:
			return fmt.Errorf("Error to check sprint after delete: %w", getErr)
		default:
			return ErrSprintDeleteStatusNotCreating
		}
	}

	if err != nil {
		return fmt.Errorf("Error to delete sprint by id: %w", err)
	}

	return nil
}

func (r *SprintRepository) ExistsByDate(ctx context.Context, date time.Time) (bool, error) {
	query := "SELECT EXISTS (SELECT 1 FROM sprints WHERE sprint_date = $1::date AND deleted_at IS NULL)"

	var exists bool

	err := r.db.GetContext(ctx, &exists, query, date)
	if err != nil {
		return false, fmt.Errorf("Error to get sprint_date exists: %w", err)
	}

	return exists, nil
}

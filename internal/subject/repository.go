package subject

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type SubjectRepositoryContract interface {
	Create(ctx context.Context, subject SubjectCreateInput) error
	GetAll(ctx context.Context) ([]Subject, error)
	GetByName(ctx context.Context, name string) (*Subject, error)
	Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput) error
	DeleteById(ctx context.Context, id uuid.UUID) error
}

type SubjectRepository struct {
	db *sqlx.DB
}

type SubjectCreateInput struct {
	Name  string `json:"name" db:"name"`
	Color Color  `json:"color" db:"color"`
}

type SubjectUpdateInput struct {
	Name  *string `json:"name" db:"name"`
	Color *Color  `json:"color" db:"color"`
}

const (
	uniqueViolationCode        = "23505"
	activeNameUniqueIndex      = "subjects_active_name_unique"
	legacyNameUniqueConstraint = "subjects_name_key"
)

func NewSubjectRepository(db *sqlx.DB) *SubjectRepository {
	return &SubjectRepository{
		db: db,
	}
}

func (s *SubjectRepository) GetAll(ctx context.Context) ([]Subject, error) {
	subject := make([]Subject, 0)
	const query = `
		SELECT
			id,
			name,
			color,
			created_at,
			updated_at,
			deleted_at
		FROM subjects
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
	`
	if err := s.db.SelectContext(ctx, &subject, query); err != nil {
		return nil, fmt.Errorf("Error to get all subjects, error, %w", err)
	}
	return subject, nil
}

func (s *SubjectRepository) GetByName(ctx context.Context, name string) (*Subject, error) {
	var subject Subject

	query := "SELECT id,name,color,created_at,updated_at,deleted_at FROM subjects WHERE deleted_at IS NULL AND name = $1 LIMIT 1"

	err := s.db.GetContext(ctx, &subject, query, name)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSubjectNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("Error to get subject by name, error, %w", err)
	}

	return &subject, nil
}

func (s *SubjectRepository) Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput) error {
	query := "UPDATE subjects SET name = COALESCE($1::varchar, name), color = COALESCE($2::colors, color), updated_at = now() WHERE id = $3 AND deleted_at IS NULL"

	result, err := s.db.ExecContext(ctx, query, input.Name, input.Color, id)

	if err != nil {
		if isSubjectNameUniqueViolation(err) {
			return ErrSubjectDuplicated
		}

		return fmt.Errorf("Update subjects failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get updated rows count: %w", err)
	}

	if rowsAffected == 0 {
		return ErrSubjectNotFound
	}

	return nil
}

func (s *SubjectRepository) Create(ctx context.Context, subject SubjectCreateInput) error {
	_, err := s.db.NamedExecContext(ctx, "INSERT INTO subjects (name ,color) VALUES (:name, :color)", &subject)

	if err != nil {
		if isSubjectNameUniqueViolation(err) {
			return ErrSubjectDuplicated
		}

		return fmt.Errorf("Error to create subject, error, %w", err)
	}

	return nil
}

func isSubjectNameUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolationCode {
		return false
	}

	return pgErr.ConstraintName == activeNameUniqueIndex ||
		pgErr.ConstraintName == legacyNameUniqueConstraint
}

func (s *SubjectRepository) DeleteById(ctx context.Context, id uuid.UUID) error {

	query := "UPDATE subjects SET deleted_at = now(), updated_at = now() WHERE id = $1 AND deleted_at IS NULL"
	result, err := s.db.ExecContext(ctx, query, id)

	if err != nil {
		return fmt.Errorf("Error to delete by id, error, %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted rows count: %w", err)
	}

	if rowsAffected == 0 {
		return ErrSubjectNotFound
	}

	return nil
}

package subject

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type SubjectRepository struct {
	db *sqlx.DB
}

type SubjectCreateInput struct {
	Name  string `json:"name" db:"name"`
	Color Color  `json:"color" db:"color"`
}

type SubjectUpdateInput struct {
	Name *string `json:"name" db:"name"`
	Color *Color `json:"color" db:"color"`
}

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

func (s *SubjectRepository)GetByName(ctx context.Context, name string) (*Subject, error){
	var subject Subject

	query := "SELECT id,name,color,created_at,updated_at,deleted_at FROM subjects WHERE deleted_at IS NULL AND name = $1 LIMIT 1"
	
	err := s.db.GetContext(ctx, &subject, query, name)

	if errors.Is(err, sql.ErrNoRows){
		return nil, ErrSubjectNotFound
	}

	if err != nil{
		return nil,fmt.Errorf("Error to get subject by name, error, %w", err)
	}
		
	return &subject,nil
}


func (s *SubjectRepository)Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput) error{
	query := "UPDATE subjects SET name = COALESCE($1::varchar, name), color = COALESCE($2::colors, color), updated_at = now() WHERE id = $3 AND deleted_at IS NULL"

	result, err := s.db.ExecContext(ctx, query, input.Name, input.Color, id)

	if err != nil{
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
		return fmt.Errorf("Error to create subject, error, %w", err)
	}

	return nil
}

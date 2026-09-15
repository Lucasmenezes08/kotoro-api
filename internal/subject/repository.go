package subject

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type SubjectRepository struct {
	db *sqlx.DB
}

type SubjectCreateInput struct {
	Name  string `json:"name" db:"name"`
	Color Color  `json:"color" db:"color"`
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
	query := "SELECT id,name,color,created_at,updated_at,deleted_at FROM subjects WHERE deleted_at IS NULL AND name = ?"
	
	if err := s.db.SelectContext(ctx, name, query); err != nil{
		return nil,fmt.Errorf("Error to get subject by name, error, %w", err)
	}

	return &Subject{},nil
}

func (s *SubjectRepository) Create(ctx context.Context, subject SubjectCreateInput) error {
	_, err := s.db.NamedExecContext(ctx, "INSERT INTO subjects (name ,color) VALUES (:name, :color)", &subject)

	if err != nil {
		return fmt.Errorf("Error to create subject, error, %w", err)
	}

	return nil
}

package subject

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type SubjectRepository struct{
	db *sqlx.DB
}

type SubjectCreateInput struct {
	Name string `json:"name" db:"name"`
	Color Color `json:"color" db:"color"`
}

func NewSubjectRepository(db *sqlx.DB) *SubjectRepository{
	return &SubjectRepository{
		db : db,
	}
}

func (s *SubjectRepository)Create(ctx context.Context, subject SubjectCreateInput)error{
	_, err := s.db.NamedExecContext(ctx, "INSERT INTO subjects (name ,color) VALUES (:name, :color)", &subject)
	
	if err != nil {
		return fmt.Errorf("Error to create subject, error, %w", err)
	}

	return nil
}
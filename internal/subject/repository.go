package subject

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type SubjectRepository struct{
	DB *sqlx.DB
}

func (s *SubjectRepository)NewSubjectRepository(db *sqlx.DB) *SubjectRepository{
	return &SubjectRepository{
		DB : db,
	}
}

func (s *SubjectRepository)Create(ctx context.Context, subject Subject)error{
	_, err := s.DB.NamedExecContext(ctx, "INSERT INTO subjects (name ,color) VALUES (:name, :color)", &subject)
	
	if err != nil {
		return fmt.Errorf("Error to create subject, error, %v", err)
	}

	return nil
}
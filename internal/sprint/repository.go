package sprint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)


type SprintRepositoryContract interface {
	Create(ctx context.Context, payload CreateSprintModel)error
	ExistsByDate(ctx context.Context, date time.Time)(bool, error)
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

type SprintRepository struct{
	db *sqlx.DB
}

func NewSprintRepository(db *sqlx.DB) *SprintRepository{
	return &SprintRepository{
		db: db,
	}
}

func (r *SprintRepository)Create(ctx context.Context ,sprint CreateSprintModel)error{
	query := "INSERT INTO sprints (name, sprint_date, status) VALUES(:name, :sprint_date, :status)"

	_, err := r.db.NamedExecContext(ctx, query, sprint)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_sprint_active_date"{
			return ErrSprintAlreadyExists
		}
		return fmt.Errorf("Error to create Sprint: %w", err)
	}
	
	return nil
}

func (r *SprintRepository)ExistsByDate(ctx context.Context, date time.Time)(bool, error){
	query := "SELECT EXISTS (SELECT 1 FROM sprints WHERE sprint_date = 1::date AND deleted_at IS NOT NULL)"

	var exists bool

	err := r.db.GetContext(ctx, &exists, query, date)
	if err != nil{
		return false, fmt.Errorf("Error to GET sprint_date exists")
	} 

	return exists, nil
}



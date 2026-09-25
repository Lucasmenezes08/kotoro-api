package sprint

import (
	"time"

	"github.com/google/uuid"
)




type SprintStatus string

var (
	Creating SprintStatus = "creating"
	InProgress SprintStatus = "in_progress"
	Finished SprintStatus = "finished"
	Abandoned SprintStatus = "abandoned"
)


type Sprint struct {
	ID          uuid.UUID    `db:"id"`
	Name        *string       `db:"name"`
	SprintDate  time.Time    `db:"sprint_date"`
	Status      SprintStatus `db:"status"`
	StartedAt   *time.Time   `db:"started_at"`
	CompletedAt *time.Time   `db:"completed_at"`
	CreatedAt   time.Time    `db:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"`
	DeletedAt   *time.Time   `db:"deleted_at"`
}


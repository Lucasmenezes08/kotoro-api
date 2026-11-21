package sprint

import (
	"time"

	"github.com/google/uuid"
)

type SprintStatus string

var (
	Creating   SprintStatus = "creating"
	InProgress SprintStatus = "in_progress"
	Finished   SprintStatus = "finished"
	Abandoned  SprintStatus = "abandoned"
)

type Sprint struct {
	ID          uuid.UUID    `db:"id" json:"id"`
	UserID      uuid.UUID    `db:"user_id" json:"user_id"`
	Name        *string      `db:"name" json:"name"`
	SprintDate  time.Time    `db:"sprint_date" json:"sprint_date"`
	RestMinutes int          `db:"rest_minutes" json:"rest_minutes"`
	Status      SprintStatus `db:"status" json:"status"`
	StartedAt   *time.Time   `db:"started_at" json:"started_at"`
	CompletedAt *time.Time   `db:"completed_at" json:"completed_at"`
	CreatedAt   time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time   `db:"deleted_at" json:"deleted_at,omitempty"`
}

package sprintblocks

import (
	"time"

	"github.com/google/uuid"
)

type SprintBlocksStatus string

var (
	Pending    SprintBlocksStatus = "pendente"
	InProgress SprintBlocksStatus = "em andamento"
	Finished   SprintBlocksStatus = "finalizado"
)

type SprintBlock struct {
	ID               uuid.UUID          `db:"id"`
	SprintId         uuid.UUID          `db:"sprint_id"`
	SubjectId        uuid.UUID          `db:"subject_id"`
	Title            *string            `db:"title"`
	Position         int                `db:"position"`
	Status           SprintBlocksStatus `db:"status"`
	EstimatedSeconds *int               `db:"estimated_seconds"`
	StartedAt        *time.Time         `db:"started_at"`
	CompletedAt      *time.Time         `db:"completed_at"`
	CreatedAt        *time.Time         `db:"created_at"`
	UpdatedAt        *time.Time         `db:"updated_at"`
	DeletedAt        *time.Time         `db:"deleted_at"`
}

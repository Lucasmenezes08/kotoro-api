package sprintblocks

import (
	"time"

	"github.com/google/uuid"
)

type SprintBlocksStatus string

const (
	Pending    SprintBlocksStatus = "pendente"
	InProgress SprintBlocksStatus = "em andamento"
	Finished   SprintBlocksStatus = "finalizado"
)

type SprintBlock struct {
	ID               uuid.UUID          `db:"id"`
	SprintID         uuid.UUID          `db:"sprint_id"`
	SubjectID        uuid.UUID          `db:"subject_id"`
	Title            *string            `db:"title"`
	Position         int                `db:"position"`
	X                float64            `db:"x"`
	Y                float64            `db:"y"`
	Status           SprintBlocksStatus `db:"status"`
	EstimatedSeconds *int               `db:"estimated_seconds"`
	StartedAt        *time.Time         `db:"started_at"`
	CompletedAt      *time.Time         `db:"completed_at"`
	CreatedAt        *time.Time         `db:"created_at"`
	UpdatedAt        *time.Time         `db:"updated_at"`
	DeletedAt        *time.Time         `db:"deleted_at"`
}

type CreateSprintBlockInput struct {
	SubjectID        uuid.UUID
	Title            *string
	Position         int
	X                *float64
	Y                *float64
	EstimatedSeconds *int
}

type CreateSprintBlockModel struct {
	SprintID         uuid.UUID          `db:"sprint_id"`
	SubjectID        uuid.UUID          `db:"subject_id"`
	Title            *string            `db:"title"`
	Position         int                `db:"position"`
	X                float64            `db:"x"`
	Y                float64            `db:"y"`
	Status           SprintBlocksStatus `db:"status"`
	EstimatedSeconds *int               `db:"estimated_seconds"`
}

type SprintBlockUpdateModel struct {
	SubjectID        *uuid.UUID          `db:"subject_id"`
	Title            *string             `db:"title"`
	Position         *int                `db:"position"`
	X                *float64            `db:"x"`
	Y                *float64            `db:"y"`
	EstimatedSeconds *int                `db:"estimated_seconds"`
	Status           *SprintBlocksStatus `db:"status"`
	StartedAt        *time.Time          `db:"started_at"`
	CompletedAt      *time.Time          `db:"completed_at"`
}

type CanvasBlockLayoutItem struct {
	ID       uuid.UUID `json:"id"`
	Position int       `json:"position"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
}

type UpdateCanvasLayoutInput struct {
	Blocks []CanvasBlockLayoutItem `json:"blocks"`
}

type SprintBlockResponse struct {
	ID               uuid.UUID          `json:"id"`
	SprintID         uuid.UUID          `json:"sprint_id"`
	SubjectID        uuid.UUID          `json:"subject_id"`
	Title            *string            `json:"title"`
	Position         int                `json:"position"`
	X                float64            `json:"x"`
	Y                float64            `json:"y"`
	Status           SprintBlocksStatus `json:"status"`
	EstimatedSeconds *int               `json:"estimated_seconds"`
	StartedAt        *time.Time         `json:"started_at"`
	CompletedAt      *time.Time         `json:"completed_at"`
	CreatedAt        *time.Time         `json:"created_at"`
	UpdatedAt        *time.Time         `json:"updated_at"`
}

func responseFromModel(block *SprintBlock) SprintBlockResponse {
	return SprintBlockResponse{
		ID:               block.ID,
		SprintID:         block.SprintID,
		SubjectID:        block.SubjectID,
		Title:            block.Title,
		Position:         block.Position,
		X:                block.X,
		Y:                block.Y,
		Status:           block.Status,
		EstimatedSeconds: block.EstimatedSeconds,
		StartedAt:        block.StartedAt,
		CompletedAt:      block.CompletedAt,
		CreatedAt:        block.CreatedAt,
		UpdatedAt:        block.UpdatedAt,
	}
}

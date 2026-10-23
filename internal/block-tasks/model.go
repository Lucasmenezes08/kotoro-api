package blocktasks

import (
	"time"

	"github.com/google/uuid"
)

type BlockTaskStatus string

const (
	TaskPending    BlockTaskStatus = "pendente"
	TaskInProgress BlockTaskStatus = "em_andamento"
	TaskDone       BlockTaskStatus = "feita"
	TaskCancelled  BlockTaskStatus = "cancelada"
)

type TaskPriority string

const (
	PriorityLow      TaskPriority = "baixa"
	PriorityMedium   TaskPriority = "media"
	PriorityHigh     TaskPriority = "alta"
	PriorityVeryHigh TaskPriority = "muito_alta"
)

type BlockTask struct {
	ID               uuid.UUID       `db:"id"`
	SprintBlockID    uuid.UUID       `db:"sprint_block_id"`
	Title            string          `db:"title"`
	Description      *string         `db:"description"`
	Position         int             `db:"position"`
	Status           BlockTaskStatus `db:"status"`
	Priority         TaskPriority    `db:"priority"`
	EstimatedSeconds *int            `db:"estimated_seconds"`
	CreatedAt        time.Time       `db:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"`
	CompletedAt      *time.Time      `db:"completed_at"`
	DeletedAt        *time.Time      `db:"deleted_at"`
}

type CreateBlockTaskInput struct {
	Title            string
	Description      *string
	Position         int
	Priority         *TaskPriority
	EstimatedSeconds *int
}

type CreateBlockTaskModel struct {
	SprintBlockID    uuid.UUID       `db:"sprint_block_id"`
	Title            string          `db:"title"`
	Description      *string         `db:"description"`
	Position         int             `db:"position"`
	Status           BlockTaskStatus `db:"status"`
	Priority         TaskPriority    `db:"priority"`
	EstimatedSeconds *int            `db:"estimated_seconds"`
}

type BlockTaskUpdateInput struct {
	Title            *string
	Description      *string
	Position         *int
	Priority         *TaskPriority
	EstimatedSeconds *int
}

type BlockTaskUpdateModel struct {
	Title            *string       `db:"title"`
	Description      *string       `db:"description"`
	DescriptionSet   bool          `db:"description_set"`
	Position         *int          `db:"position"`
	Priority         *TaskPriority `db:"priority"`
	EstimatedSeconds *int          `db:"estimated_seconds"`
}

type BlockTaskResponse struct {
	ID               uuid.UUID       `json:"id"`
	SprintBlockID    uuid.UUID       `json:"sprint_block_id"`
	Title            string          `json:"title"`
	Description      *string         `json:"description"`
	Position         int             `json:"position"`
	Status           BlockTaskStatus `json:"status"`
	Priority         TaskPriority    `json:"priority"`
	EstimatedSeconds *int            `json:"estimated_seconds"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
}

func responseFromModel(task *BlockTask) BlockTaskResponse {
	return BlockTaskResponse{
		ID: task.ID, SprintBlockID: task.SprintBlockID, Title: task.Title,
		Description: task.Description, Position: task.Position, Status: task.Status,
		Priority: task.Priority, EstimatedSeconds: task.EstimatedSeconds,
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, CompletedAt: task.CompletedAt,
	}
}

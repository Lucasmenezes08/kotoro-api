-- +goose Up

CREATE TYPE block_task_status AS ENUM ('pendente', 'em_andamento', 'feita', 'cancelada');
CREATE TYPE block_task_priority AS ENUM ('baixa', 'media', 'alta', 'muito_alta');

CREATE TABLE block_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sprint_block_id UUID NOT NULL REFERENCES sprint_block(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    position INTEGER NOT NULL CHECK (position > 0),
    status block_task_status NOT NULL DEFAULT 'pendente',
    priority block_task_priority NOT NULL DEFAULT 'media',
    estimated_seconds INTEGER CHECK (estimated_seconds IS NULL OR estimated_seconds > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX block_tasks_active_position_unique
    ON block_tasks (sprint_block_id, position)
    WHERE deleted_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS block_tasks;
DROP TYPE IF EXISTS block_task_priority;
DROP TYPE IF EXISTS block_task_status;

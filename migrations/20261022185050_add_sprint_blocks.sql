-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE sprint_block_status AS ENUM('pendente', 'em andamento', 'finalizado');

CREATE TABLE sprint_block (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sprint_id UUID NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id),
    title VARCHAR(100),
    position INTEGER NOT NULL,
    status sprint_block_status NOT NULL DEFAULT 'pendente',
    estimated_seconds INTEGER, 
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- +goose Down

DROP TYPE IF EXISTS sprint_block_status;
DROP TABLE IF EXISTS sprint_block;

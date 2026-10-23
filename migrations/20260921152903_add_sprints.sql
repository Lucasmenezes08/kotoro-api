-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE sprint_status AS ENUM ('creating', 'in_progress', 'finished', 'abandoned');

CREATE TABLE sprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    sprint_date DATE NOT NULL,
    status sprint_status NOT NULL DEFAULT 'creating',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- +goose Down

DROP TABLE IF EXISTS sprints;
DROP TYPE IF EXISTS sprint_status;

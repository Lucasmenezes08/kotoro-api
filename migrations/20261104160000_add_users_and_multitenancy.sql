-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_users_active_email ON users (LOWER(email)) WHERE deleted_at IS NULL;

-- Multi-tenancy for subjects:
ALTER TABLE subjects ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS subjects_active_name_unique;

CREATE UNIQUE INDEX subjects_active_user_name_unique
    ON subjects (user_id, LOWER(name))
    WHERE deleted_at IS NULL;

-- Multi-tenancy for sprints:
ALTER TABLE sprints ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_sprint_active_date;

CREATE UNIQUE INDEX idx_sprint_active_user_date
    ON sprints (user_id, sprint_date)
    WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_sprint_active_user_date;

CREATE UNIQUE INDEX idx_sprint_active_date
    ON sprints (sprint_date)
    WHERE deleted_at IS NULL;

ALTER TABLE sprints DROP COLUMN IF EXISTS user_id;

DROP INDEX IF EXISTS subjects_active_user_name_unique;

CREATE UNIQUE INDEX subjects_active_name_unique
    ON subjects (name)
    WHERE deleted_at IS NULL;

ALTER TABLE subjects DROP COLUMN IF EXISTS user_id;

DROP INDEX IF EXISTS idx_users_active_email;

DROP TABLE IF EXISTS users;

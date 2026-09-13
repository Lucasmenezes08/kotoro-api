-- +goose Up
CREATE TYPE colors AS ENUM ('green' , 'red', 'blue', 'orange', 'black', 'gray', 'purple', 'white', 'pink', 'yellow');

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE subjects(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    color colors NOT NULL DEFAULT 'gray',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE IF EXISTS subjects;
DROP TYPE IF EXISTS colors;
-- +goose Up
-- +goose StatementBegin
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'teal';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'cyan';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'indigo';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'emerald';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'amber';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'rose';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'lime';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'violet';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'fuchsia';
ALTER TYPE colors ADD VALUE IF NOT EXISTS 'brown';
-- +goose StatementEnd

-- +goose Down
-- PostgreSQL does not support removing values from an ENUM type directly.

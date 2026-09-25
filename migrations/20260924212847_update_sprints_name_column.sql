-- +goose Up
ALTER TABLE sprints ALTER COLUMN name DROP NOT NULL;

-- +goose Down

ALTER TABLE sprints ALTER COLUMN name ADD NOT NULL;
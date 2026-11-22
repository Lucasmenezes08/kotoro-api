-- +goose Up
ALTER TABLE sprints ADD COLUMN rest_minutes INTEGER NOT NULL DEFAULT 10 CHECK (rest_minutes > 0 AND rest_minutes <= 120);

-- +goose Down
ALTER TABLE sprints DROP COLUMN IF EXISTS rest_minutes;

-- +goose Up
CREATE UNIQUE INDEX idx_sprint_active_date ON sprints(sprint_date) WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS idx_sprint_active_date;



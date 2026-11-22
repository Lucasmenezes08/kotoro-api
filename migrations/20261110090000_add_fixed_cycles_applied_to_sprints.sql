-- +goose Up
-- +goose StatementBegin
ALTER TABLE sprints ADD COLUMN IF NOT EXISTS fixed_cycles_applied BOOLEAN NOT NULL DEFAULT FALSE;
-- Mark sprints that already have blocks as having fixed cycles applied if they have blocks
UPDATE sprints SET fixed_cycles_applied = TRUE WHERE id IN (SELECT DISTINCT sprint_id FROM sprint_block WHERE deleted_at IS NULL);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sprints DROP COLUMN IF EXISTS fixed_cycles_applied;
-- +goose StatementEnd

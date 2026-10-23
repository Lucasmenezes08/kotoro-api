-- +goose Up

CREATE UNIQUE INDEX sprint_block_active_position_unique
    ON sprint_block (sprint_id, position)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX sprint_block_active_subject_unique
    ON sprint_block (sprint_id, subject_id)
    WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS sprint_block_active_subject_unique;
DROP INDEX IF EXISTS sprint_block_active_position_unique;

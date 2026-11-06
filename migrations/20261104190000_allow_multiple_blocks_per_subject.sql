-- +goose Up

DROP INDEX IF EXISTS sprint_block_active_subject_unique;

-- +goose Down

CREATE UNIQUE INDEX sprint_block_active_subject_unique
    ON sprint_block (sprint_id, subject_id)
    WHERE deleted_at IS NULL;

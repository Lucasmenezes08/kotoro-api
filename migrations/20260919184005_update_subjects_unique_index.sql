-- +goose Up
ALTER TABLE subjects
  DROP CONSTRAINT subjects_name_key;

CREATE UNIQUE INDEX subjects_active_name_unique
  ON subjects (name)
  WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX subjects_active_name_unique;

ALTER TABLE subjects
  ADD CONSTRAINT subjects_name_key UNIQUE (name);

-- +goose Up

ALTER TABLE subjects ADD CONSTRAINT subjects_name_key UNIQUE (name);

-- +goose Down

ALTER TABLE subjects DROP CONSTRAINT subjects_name_key;

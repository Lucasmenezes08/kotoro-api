-- +goose Up
DROP INDEX IF EXISTS uq_study_cycles_sprint_sequence;

CREATE UNIQUE INDEX uq_study_cycles_sprint_sequence
    ON study_cycles (sprint_id, sequence_number)
    WHERE status != 'cancelado';

-- +goose Down
DROP INDEX IF EXISTS uq_study_cycles_sprint_sequence;

CREATE UNIQUE INDEX uq_study_cycles_sprint_sequence
    ON study_cycles (sprint_id, sequence_number);

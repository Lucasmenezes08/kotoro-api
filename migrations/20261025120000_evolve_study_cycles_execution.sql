-- +goose Up

ALTER TABLE study_cycles
    ADD CONSTRAINT chk_study_cycles_planned_duration_positive
    CHECK (
        planned_duration_seconds IS NULL
        OR planned_duration_seconds > 0
    );

ALTER TABLE study_cycles
    ADD CONSTRAINT chk_study_cycles_ended_at_after_started_at
    CHECK (
        ended_at IS NULL
        OR ended_at >= started_at
    );

CREATE UNIQUE INDEX uq_study_cycles_active_sprint
    ON study_cycles (sprint_id)
    WHERE status IN ('em_andamento', 'pausado');

CREATE TABLE study_cycle_tasks (
    study_cycle_id UUID NOT NULL REFERENCES study_cycles(id) ON DELETE CASCADE,
    block_task_id UUID NOT NULL REFERENCES block_tasks(id) ON DELETE CASCADE,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (study_cycle_id, block_task_id)
);

CREATE INDEX idx_study_cycle_tasks_block_task_id
    ON study_cycle_tasks (block_task_id);

-- +goose Down

DROP INDEX IF EXISTS idx_study_cycle_tasks_block_task_id;
DROP TABLE IF EXISTS study_cycle_tasks;
DROP INDEX IF EXISTS uq_study_cycles_active_sprint;

ALTER TABLE study_cycles
    DROP CONSTRAINT IF EXISTS chk_study_cycles_ended_at_after_started_at;

ALTER TABLE study_cycles
    DROP CONSTRAINT IF EXISTS chk_study_cycles_planned_duration_positive;

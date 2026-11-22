-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;



CREATE TYPE execution_mode AS ENUM('bloco_materia', 'tarefa_individual', 'multiplas_tarefas');
CREATE TYPE timer_type AS ENUM('cronometro_livre', 'temporizado');
CREATE TYPE status AS ENUM('em_andamento', 'pausado', 'finalizado', 'cancelado');
CREATE TYPE cycle_stop_reason AS ENUM('tempo_esgotado', 'concluido', 'cancelado'); 


CREATE TABLE study_cycles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sprint_id UUID NOT NULL REFERENCES sprints(id),
    sprint_block_id UUID NOT NULL REFERENCES sprint_block(id),
    sequence_number INTEGER NOT NULL CHECK (sequence_number > 0),
    execution_mode execution_mode NOT NULL,
    timer_type timer_type NOT NULL,
    status status NOT NULL DEFAULT 'em_andamento',
    planned_duration_seconds INTEGER ,
    started_at TIMESTAMPTZ NOT NULL,
    running_since TIMESTAMPTZ,
    paused_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    elapsed_seconds INTEGER NOT NULL DEFAULT 0 CHECK (elapsed_seconds >= 0),
    stop_reason cycle_stop_reason,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_study_cycles_sprint_id ON study_cycles(sprint_id);
CREATE INDEX idx_study_cycles_sprint_block_id ON study_cycles(sprint_block_id);
CREATE UNIQUE INDEX uq_study_cycles_sprint_sequence ON study_cycles(sprint_id, sequence_number);
CREATE INDEX idx_study_cycles_status ON study_cycles(status);


-- +goose Down

DROP TYPE IF EXISTS execution_mode;
DROP TYPE IF EXISTS timer_type;
DROP TYPE IF EXISTS status;
DROP TYPE IF EXISTS cycle_stop_reason;

DROP TABLE IF EXISTS study_cycles;

DROP INDEX IF EXISTS idx_study_cycles_sprint_id;
DROP INDEX IF EXISTS idx_study_cycles_sprint_block_id;
DROP UNIQUE INDEX IF EXISTS uq_study_cycles_sprint_sequence;
DROP INDEX IF EXISTS idx_study_cycles_status;

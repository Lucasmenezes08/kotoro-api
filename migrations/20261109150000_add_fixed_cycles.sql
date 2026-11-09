-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS fixed_cycles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    title VARCHAR(150) NOT NULL,
    estimated_seconds INT NOT NULL DEFAULT 3600,
    day_of_week VARCHAR(20) NOT NULL DEFAULT 'all',
    tasks JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_fixed_cycles_user ON fixed_cycles(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_fixed_cycles_day ON fixed_cycles(user_id, day_of_week) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fixed_cycles;
-- +goose StatementEnd

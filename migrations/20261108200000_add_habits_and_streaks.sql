-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS water_day_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    total_ml INT NOT NULL DEFAULT 0,
    target_ml INT NOT NULL DEFAULT 2000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_water_date UNIQUE (user_id, date)
);

CREATE TABLE IF NOT EXISTS water_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    water_record_id UUID NOT NULL REFERENCES water_day_records(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_ml INT NOT NULL,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_water_day_records_user_date ON water_day_records(user_id, date);
CREATE INDEX IF NOT EXISTS idx_water_logs_record ON water_logs(water_record_id);

CREATE TABLE IF NOT EXISTS user_streaks (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_streak INT NOT NULL DEFAULT 0,
    longest_streak INT NOT NULL DEFAULT 0,
    last_study_date DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_streaks;
DROP TABLE IF EXISTS water_logs;
DROP TABLE IF EXISTS water_day_records;
-- +goose StatementEnd

-- +goose Up
CREATE TYPE outbox_event_status AS ENUM ('pending', 'processing', 'published', 'failed');

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(150) NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1 CHECK (event_version > 0),
    payload JSONB NOT NULL,
    correlation_id UUID,
    causation_id UUID,
    status outbox_event_status NOT NULL DEFAULT 'pending',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    locked_at TIMESTAMPTZ,
    locked_by VARCHAR(150),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_outbox_events_dispatch
    ON outbox_events (status, available_at, created_at)
    WHERE status = 'pending';

CREATE INDEX idx_outbox_events_aggregate
    ON outbox_events (aggregate_type, aggregate_id, occurred_at);

CREATE INDEX idx_outbox_events_event_type
    ON outbox_events (event_type);

CREATE INDEX idx_outbox_events_published_at
    ON outbox_events (published_at);

CREATE INDEX idx_outbox_events_stale_processing
    ON outbox_events (locked_at)
    WHERE status = 'processing';

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
DROP TYPE IF EXISTS outbox_event_status;

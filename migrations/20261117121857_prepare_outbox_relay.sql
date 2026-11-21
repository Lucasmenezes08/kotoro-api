-- +goose Up

ALTER TABLE outbox_events ADD locked_until TIMESTAMPTZ;
DROP INDEX idx_outbox_events_dispatch;
DROP INDEX idx_outbox_events_stale_processing;

CREATE INDEX idx_outbox_events_pending_available ON outbox_events(available_at, occurred_at, id) WHERE status = 'pending';
CREATE INDEX idx_outbox_events_processing_lease ON outbox_events(locked_until, id) WHERE status = 'processing';

-- +goose Down

DROP INDEX idx_outbox_events_pending_available;
DROP INDEX idx_outbox_events_processing_lease; 

CREATE INDEX idx_outbox_events_dispatch ON outbox_events(status, available_at, created_at) WHERE status = 'pending';
CREATE INDEX idx_outbox_events_stale_processing ON outbox_events(locked_at) WHERE status = 'processing';

ALTER TABLE outbox_events DROP COLUMN locked_until;
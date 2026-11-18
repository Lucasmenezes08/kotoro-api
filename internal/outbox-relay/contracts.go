package outboxrelay

import (
	"context"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
)

type Store interface {
	ClaimBatch(ctx context.Context, relayID string, limit int, leaseDuration time.Duration) ([]outbox.OutboxEvent, error)
	MarkPublished(ctx context.Context, eventID uuid.UUID, relayID string, publishedAt time.Time) error
	ScheduleRetry(ctx context.Context, eventID uuid.UUID, relayID string, availableAt time.Time, lastError string) error
	MarkFailed(ctx context.Context, eventID uuid.UUID, relayID string, lastError string) error
}

type Publisher interface {
	Publish(ctx context.Context, event outbox.OutboxEvent) error
	Close() error
}

var _ Store = (*outbox.RelayStore)(nil)

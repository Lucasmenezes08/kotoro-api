package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Event struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	EventVersion  int
	Payload       json.RawMessage
	CorrelationID *uuid.UUID
	CausationID   *uuid.UUID
	OccurredAt    time.Time
}

type OutboxRepository struct{}

// OutboxWriter persists events using a transaction supplied by its caller.
// It deliberately has no transaction lifecycle methods: the domain repository
// owns the atomic unit of work.
type OutboxWriter interface {
	Create(ctx context.Context, tx *sqlx.Tx, event Event) error
}

var _ OutboxWriter = (*OutboxRepository)(nil)

func NewOutboxRepository() *OutboxRepository {
	return &OutboxRepository{}
}

func (r *OutboxRepository) Create(
	ctx context.Context,
	tx *sqlx.Tx,
	event Event,
) error {
	const query = `
		INSERT INTO outbox_events (
			id,
			user_id,
			aggregate_type,
			aggregate_id,
			event_type,
			event_version,
			payload,
			correlation_id,
			causation_id,
			occurred_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7::jsonb,
			$8,
			$9,
			$10
		)
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		event.ID,
		event.UserID,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.EventVersion,
		event.Payload,
		event.CorrelationID,
		event.CausationID,
		event.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf(
			"insert outbox event: %w",
			err,
		)
	}

	return nil
}

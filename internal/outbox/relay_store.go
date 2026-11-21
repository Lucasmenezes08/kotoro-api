package outbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrInvalidRelayID       = errors.New("relay ID is required")
	ErrInvalidLimit         = errors.New("limit must be greater than zero")
	ErrInvalidLeaseDuration = errors.New("lease duration must be greater than zero")
	ErrLeaseLost            = errors.New("event lease lost or invalid state")
)

type RelayStore struct {
	db *sqlx.DB
}

func NewRelayStore(db *sqlx.DB) *RelayStore {
	return &RelayStore{
		db: db,
	}
}

func (s *RelayStore) ClaimBatch(
	ctx context.Context,
	relayID string,
	limit int,
	leaseDuration time.Duration,
) ([]OutboxEvent, error) {
	if strings.TrimSpace(relayID) == "" {
		return nil, ErrInvalidRelayID
	}
	if limit <= 0 {
		return nil, ErrInvalidLimit
	}
	if leaseDuration <= 0 {
		return nil, ErrInvalidLeaseDuration
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	leaseMillis := leaseDuration.Milliseconds()

	const query = `
		WITH selected AS (
			SELECT id
			FROM outbox_events
			WHERE (status = 'pending' AND available_at <= CURRENT_TIMESTAMP)
			   OR (status = 'processing' AND locked_until <= CURRENT_TIMESTAMP)
			ORDER BY available_at ASC, occurred_at ASC, id ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events e
		SET status = 'processing',
		    locked_by = $2,
		    locked_at = CURRENT_TIMESTAMP,
		    locked_until = CURRENT_TIMESTAMP + ($3 * INTERVAL '1 millisecond'),
		    attempts = e.attempts + 1,
		    updated_at = CURRENT_TIMESTAMP
		FROM selected s
		WHERE e.id = s.id
		RETURNING e.id, e.user_id, e.aggregate_type, e.aggregate_id, e.event_type, e.event_version,
		          e.payload, e.correlation_id, e.causation_id, e.status, e.occurred_at, e.available_at,
		          e.published_at, e.attempts, e.last_error, e.locked_at, e.locked_by, e.locked_until,
		          e.created_at, e.updated_at
	`

	var events []OutboxEvent
	if err := tx.SelectContext(ctx, &events, query, limit, relayID, leaseMillis); err != nil {
		return nil, fmt.Errorf("claim batch query: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim transaction: %w", err)
	}

	if events == nil {
		events = []OutboxEvent{}
	}

	return events, nil
}

const maxLastErrorLength = 2048

func sanitizeLastError(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) > maxLastErrorLength {
		return msg[:maxLastErrorLength]
	}
	return msg
}

func (s *RelayStore) MarkPublished(ctx context.Context, eventID uuid.UUID, relayID string, publishedAt time.Time) error {
	if eventID == uuid.Nil {
		return errors.New("event ID is required")
	}
	if strings.TrimSpace(relayID) == "" {
		return ErrInvalidRelayID
	}
	if publishedAt.IsZero() {
		publishedAt = time.Now().UTC()
	}

	const query = `
		UPDATE outbox_events
		SET status = 'published',
		    published_at = $1,
		    locked_at = NULL,
		    locked_until = NULL,
		    locked_by = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		  AND status = 'processing'
		  AND locked_by = $3
	`

	res, err := s.db.ExecContext(ctx, query, publishedAt, eventID, relayID)
	if err != nil {
		return fmt.Errorf("mark published: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected for mark published: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: event %s not in processing or not owned by %s", ErrLeaseLost, eventID, relayID)
	}

	return nil
}

func (s *RelayStore) ScheduleRetry(ctx context.Context, eventID uuid.UUID, relayID string, availableAt time.Time, lastError string) error {
	if eventID == uuid.Nil {
		return errors.New("event ID is required")
	}
	if strings.TrimSpace(relayID) == "" {
		return ErrInvalidRelayID
	}
	if availableAt.IsZero() {
		return errors.New("availableAt is required")
	}

	sanitizedErr := sanitizeLastError(lastError)

	const query = `
		UPDATE outbox_events
		SET status = 'pending',
		    available_at = $1,
		    last_error = $2,
		    locked_at = NULL,
		    locked_until = NULL,
		    locked_by = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		  AND status = 'processing'
		  AND locked_by = $4
	`

	res, err := s.db.ExecContext(ctx, query, availableAt, sanitizedErr, eventID, relayID)
	if err != nil {
		return fmt.Errorf("schedule retry: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected for schedule retry: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: event %s not in processing or not owned by %s", ErrLeaseLost, eventID, relayID)
	}

	return nil
}

func (s *RelayStore) MarkFailed(ctx context.Context, eventID uuid.UUID, relayID string, lastError string) error {
	if eventID == uuid.Nil {
		return errors.New("event ID is required")
	}
	if strings.TrimSpace(relayID) == "" {
		return ErrInvalidRelayID
	}

	sanitizedErr := sanitizeLastError(lastError)

	const query = `
		UPDATE outbox_events
		SET status = 'failed',
		    last_error = $1,
		    locked_at = NULL,
		    locked_until = NULL,
		    locked_by = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
		  AND status = 'processing'
		  AND locked_by = $3
	`

	res, err := s.db.ExecContext(ctx, query, sanitizedErr, eventID, relayID)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected for mark failed: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: event %s not in processing or not owned by %s", ErrLeaseLost, eventID, relayID)
	}

	return nil
}

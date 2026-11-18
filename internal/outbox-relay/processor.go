package outboxrelay

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
)

var (
	ErrProcessorNilStore     = errors.New("store is required")
	ErrProcessorNilPublisher = errors.New("publisher is required")
)

type JitterFunc func(delay time.Duration) time.Duration

func DefaultJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	maxJitter := int64(delay / 5) // up to 20% jitter
	if maxJitter <= 0 {
		maxJitter = 1
	}
	n, err := rand.Int(rand.Reader, big.NewInt(maxJitter+1))
	if err != nil {
		return delay
	}
	return delay + time.Duration(n.Int64())
}

func CalculateBackoff(
	attempt int,
	baseDelay time.Duration,
	maxDelay time.Duration,
	jitter JitterFunc,
) time.Duration {
	if baseDelay <= 0 {
		baseDelay = defaultRetryBaseDelay
	}
	if maxDelay <= 0 {
		maxDelay = defaultMaxRetryDelay
	}
	if maxDelay < baseDelay {
		maxDelay = baseDelay
	}

	if attempt <= 1 {
		if jitter != nil {
			j := jitter(baseDelay)
			if j > maxDelay {
				return maxDelay
			}
			return j
		}
		return baseDelay
	}

	shift := attempt - 1
	if shift > 30 {
		shift = 30
	}
	multiplier := int64(1) << shift

	var delay time.Duration
	if maxDelay/baseDelay < time.Duration(multiplier) {
		delay = maxDelay
	} else {
		delay = baseDelay * time.Duration(multiplier)
	}

	if delay > maxDelay || delay <= 0 {
		delay = maxDelay
	}

	if jitter != nil {
		delay = jitter(delay)
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	return delay
}

type Processor struct {
	relayID        string
	store          Store
	publisher      Publisher
	publishTimeout time.Duration
	maxAttempts    int
	retryBaseDelay time.Duration
	maxRetryDelay  time.Duration
	now            func() time.Time
	jitter         JitterFunc
}

type ProcessorOption func(*Processor)

func WithClock(now func() time.Time) ProcessorOption {
	return func(p *Processor) {
		p.now = now
	}
}

func WithJitter(jitter JitterFunc) ProcessorOption {
	return func(p *Processor) {
		p.jitter = jitter
	}
}

func NewProcessor(
	relayID string,
	store Store,
	publisher Publisher,
	cfg Config,
	opts ...ProcessorOption,
) (*Processor, error) {
	if store == nil {
		return nil, ErrProcessorNilStore
	}
	if publisher == nil {
		return nil, ErrProcessorNilPublisher
	}

	p := &Processor{
		relayID:        relayID,
		store:          store,
		publisher:      publisher,
		publishTimeout: cfg.PublishTimeout,
		maxAttempts:    cfg.MaxAttempts,
		retryBaseDelay: cfg.RetryBaseDelay,
		maxRetryDelay:  cfg.MaxRetryDelay,
		now:            time.Now,
		jitter:         DefaultJitter,
	}

	if p.publishTimeout <= 0 {
		p.publishTimeout = defaultPublishTimeout
	}
	if p.maxAttempts <= 0 {
		p.maxAttempts = defaultMaxAttempts
	}
	if p.retryBaseDelay <= 0 {
		p.retryBaseDelay = defaultRetryBaseDelay
	}
	if p.maxRetryDelay <= 0 {
		p.maxRetryDelay = defaultMaxRetryDelay
	}

	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}

	return p, nil
}

func (p *Processor) ProcessEvent(ctx context.Context, event outbox.OutboxEvent) error {
	pubCtx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	err := p.publisher.Publish(pubCtx, event)
	if err == nil {
		publishedAt := p.now().UTC()
		if markErr := p.store.MarkPublished(ctx, event.ID, p.relayID, publishedAt); markErr != nil {
			// Note: Failure to MarkPublished after broker ACK creates an at-least-once duplication window.
			// The message cannot be un-published from RabbitMQ; consumers must be idempotent.
			return fmt.Errorf("mark published: %w", markErr)
		}
		slog.Info("event published to rabbitmq", "relay_id", p.relayID, "event_id", event.ID, "event_type", event.EventType)
		return nil
	}

	// Publish failed (e.g. broker error, timeout, or nack)
	if event.Attempts < p.maxAttempts {
		backoff := CalculateBackoff(event.Attempts, p.retryBaseDelay, p.maxRetryDelay, p.jitter)
		availableAt := p.now().UTC().Add(backoff)
		if schedErr := p.store.ScheduleRetry(ctx, event.ID, p.relayID, availableAt, err.Error()); schedErr != nil {
			return fmt.Errorf("schedule retry: %w", schedErr)
		}
		slog.Warn("publish failed; retry scheduled", "relay_id", p.relayID, "event_id", event.ID, "attempts", event.Attempts, "backoff", backoff, "error", err)
		return nil
	}

	// Attempts reached maxAttempts
	if failErr := p.store.MarkFailed(ctx, event.ID, p.relayID, err.Error()); failErr != nil {
		return fmt.Errorf("mark failed: %w", failErr)
	}
	slog.Error("event failed permanently", "relay_id", p.relayID, "event_id", event.ID, "attempts", event.Attempts, "error", err)
	return nil
}

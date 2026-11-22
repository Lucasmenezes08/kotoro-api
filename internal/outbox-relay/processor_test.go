package outboxrelay

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
)

type mockStore struct {
	claimBatchFn    func(ctx context.Context, relayID string, limit int, leaseDuration time.Duration) ([]outbox.OutboxEvent, error)
	markPublishedFn func(ctx context.Context, eventID uuid.UUID, relayID string, publishedAt time.Time) error
	scheduleRetryFn func(ctx context.Context, eventID uuid.UUID, relayID string, availableAt time.Time, lastError string) error
	markFailedFn    func(ctx context.Context, eventID uuid.UUID, relayID string, lastError string) error
}

func (m *mockStore) ClaimBatch(ctx context.Context, relayID string, limit int, leaseDuration time.Duration) ([]outbox.OutboxEvent, error) {
	if m.claimBatchFn != nil {
		return m.claimBatchFn(ctx, relayID, limit, leaseDuration)
	}
	return nil, nil
}

func (m *mockStore) MarkPublished(ctx context.Context, eventID uuid.UUID, relayID string, publishedAt time.Time) error {
	if m.markPublishedFn != nil {
		return m.markPublishedFn(ctx, eventID, relayID, publishedAt)
	}
	return nil
}

func (m *mockStore) ScheduleRetry(ctx context.Context, eventID uuid.UUID, relayID string, availableAt time.Time, lastError string) error {
	if m.scheduleRetryFn != nil {
		return m.scheduleRetryFn(ctx, eventID, relayID, availableAt, lastError)
	}
	return nil
}

func (m *mockStore) MarkFailed(ctx context.Context, eventID uuid.UUID, relayID string, lastError string) error {
	if m.markFailedFn != nil {
		return m.markFailedFn(ctx, eventID, relayID, lastError)
	}
	return nil
}

type mockPublisher struct {
	publishFn func(ctx context.Context, event outbox.OutboxEvent) error
	closeFn   func() error
}

func (m *mockPublisher) Publish(ctx context.Context, event outbox.OutboxEvent) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, event)
	}
	return nil
}

func (m *mockPublisher) Close() error {
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func TestCalculateBackoff(t *testing.T) {
	base := 1 * time.Second
	maxDelay := 30 * time.Second
	noJitter := func(d time.Duration) time.Duration { return d }

	t.Run("crescimento exponencial", func(t *testing.T) {
		expected := []time.Duration{
			1 * time.Second,  // attempt 1 (2^0 = 1)
			2 * time.Second,  // attempt 2 (2^1 = 2)
			4 * time.Second,  // attempt 3 (2^2 = 4)
			8 * time.Second,  // attempt 4 (2^3 = 8)
			16 * time.Second, // attempt 5 (2^4 = 16)
		}

		for i, want := range expected {
			attempt := i + 1
			got := CalculateBackoff(attempt, base, maxDelay, noJitter)
			if got != want {
				t.Errorf("attempt %d: got backoff %v, want %v", attempt, got, want)
			}
		}
	})

	t.Run("limite maximo", func(t *testing.T) {
		// attempt 6 would be 32s, but max is 30s
		got := CalculateBackoff(6, base, maxDelay, noJitter)
		if got != maxDelay {
			t.Errorf("attempt 6: got %v, want max %v", got, maxDelay)
		}

		// attempt 50 (testing overflow avoidance)
		gotLarge := CalculateBackoff(50, base, maxDelay, noJitter)
		if gotLarge != maxDelay {
			t.Errorf("attempt 50: got %v, want max %v", gotLarge, maxDelay)
		}
	})

	t.Run("jitter dentro da faixa", func(t *testing.T) {
		delay := 10 * time.Second
		for i := 0; i < 50; i++ {
			jittered := DefaultJitter(delay)
			// DefaultJitter adds up to 20%: [10s, 12s]
			if jittered < delay || jittered > delay+2*time.Second {
				t.Fatalf("jittered delay %v out of range [%v, %v]", jittered, delay, delay+2*time.Second)
			}
		}
	})
}

func TestProcessorProcessEvent(t *testing.T) {
	relayID := "test-relay-1"
	cfg := Config{
		PublishTimeout: 50 * time.Millisecond,
		MaxAttempts:    3,
		RetryBaseDelay: 1 * time.Second,
		MaxRetryDelay:  10 * time.Second,
	}

	event := outbox.OutboxEvent{
		ID:       uuid.New(),
		Attempts: 1,
	}

	t0 := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	fixedClock := func() time.Time { return t0 }
	noJitter := func(d time.Duration) time.Duration { return d }

	t.Run("sucesso chama MarkPublished", func(t *testing.T) {
		var publishedCalled bool
		var recordedPublishedAt time.Time
		store := &mockStore{
			markPublishedFn: func(ctx context.Context, id uuid.UUID, rid string, pubAt time.Time) error {
				publishedCalled = true
				recordedPublishedAt = pubAt
				if id != event.ID || rid != relayID {
					t.Errorf("unexpected id %s or relay %s", id, rid)
				}
				return nil
			},
		}
		publisher := &mockPublisher{
			publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
				return nil
			},
		}

		p, err := NewProcessor(relayID, store, publisher, cfg, WithClock(fixedClock))
		if err != nil {
			t.Fatalf("new processor: %v", err)
		}

		err = p.ProcessEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("process event failed: %v", err)
		}
		if !publishedCalled {
			t.Fatal("expected MarkPublished to be called")
		}
		if !recordedPublishedAt.Equal(t0) {
			t.Errorf("expected publishedAt %v, got %v", t0, recordedPublishedAt)
		}
	})

	t.Run("falha recuperavel chama ScheduleRetry com relogio deterministico", func(t *testing.T) {
		var retryCalled bool
		var recordedAvailableAt time.Time
		var recordedLastError string
		store := &mockStore{
			scheduleRetryFn: func(ctx context.Context, id uuid.UUID, rid string, availAt time.Time, lastErr string) error {
				retryCalled = true
				recordedAvailableAt = availAt
				recordedLastError = lastErr
				return nil
			},
		}
		publisher := &mockPublisher{
			publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
				return errors.New("amqp connection refused")
			},
		}

		p, err := NewProcessor(relayID, store, publisher, cfg, WithClock(fixedClock), WithJitter(noJitter))
		if err != nil {
			t.Fatalf("new processor: %v", err)
		}

		// event.Attempts is 1, maxAttempts is 3 -> retryable
		err = p.ProcessEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("process event unexpected error: %v", err)
		}
		if !retryCalled {
			t.Fatal("expected ScheduleRetry to be called")
		}
		// For attempt 1, backoff is 1s, so availableAt = t0 + 1s
		expectedAvail := t0.Add(1 * time.Second)
		if !recordedAvailableAt.Equal(expectedAvail) {
			t.Errorf("expected availableAt %v, got %v", expectedAvail, recordedAvailableAt)
		}
		if recordedLastError != "amqp connection refused" {
			t.Errorf("expected error string %q, got %q", "amqp connection refused", recordedLastError)
		}
	})

	t.Run("limite de tentativas chama MarkFailed", func(t *testing.T) {
		var failedCalled bool
		var recordedLastError string
		store := &mockStore{
			markFailedFn: func(ctx context.Context, id uuid.UUID, rid string, lastErr string) error {
				failedCalled = true
				recordedLastError = lastErr
				return nil
			},
		}
		publisher := &mockPublisher{
			publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
				return errors.New("mandatory routing failed: NO_ROUTE")
			},
		}

		p, err := NewProcessor(relayID, store, publisher, cfg, WithClock(fixedClock))
		if err != nil {
			t.Fatalf("new processor: %v", err)
		}

		// event.Attempts == 3, maxAttempts == 3 -> non-retryable
		exhaustedEvent := event
		exhaustedEvent.Attempts = 3

		err = p.ProcessEvent(context.Background(), exhaustedEvent)
		if err != nil {
			t.Fatalf("process event unexpected error: %v", err)
		}
		if !failedCalled {
			t.Fatal("expected MarkFailed to be called")
		}
		if recordedLastError != "mandatory routing failed: NO_ROUTE" {
			t.Errorf("expected lastError %q, got %q", "mandatory routing failed: NO_ROUTE", recordedLastError)
		}
	})

	t.Run("erro em transicao e propagado", func(t *testing.T) {
		t.Run("erro em MarkPublished e propagado", func(t *testing.T) {
			transitionErr := errors.New("lease expired on mark published")
			store := &mockStore{
				markPublishedFn: func(ctx context.Context, id uuid.UUID, rid string, pubAt time.Time) error {
					return transitionErr
				},
			}
			publisher := &mockPublisher{
				publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
					return nil
				},
			}

			p, _ := NewProcessor(relayID, store, publisher, cfg)
			err := p.ProcessEvent(context.Background(), event)
			if !errors.Is(err, transitionErr) {
				t.Fatalf("expected transition error %v, got %v", transitionErr, err)
			}
		})

		t.Run("erro em ScheduleRetry e propagado", func(t *testing.T) {
			transitionErr := errors.New("db connection lost on schedule retry")
			store := &mockStore{
				scheduleRetryFn: func(ctx context.Context, id uuid.UUID, rid string, availAt time.Time, lastErr string) error {
					return transitionErr
				},
			}
			publisher := &mockPublisher{
				publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
					return errors.New("nack received")
				},
			}

			p, _ := NewProcessor(relayID, store, publisher, cfg)
			err := p.ProcessEvent(context.Background(), event)
			if !errors.Is(err, transitionErr) {
				t.Fatalf("expected transition error %v, got %v", transitionErr, err)
			}
		})

		t.Run("erro em MarkFailed e propagado", func(t *testing.T) {
			transitionErr := errors.New("db error on mark failed")
			store := &mockStore{
				markFailedFn: func(ctx context.Context, id uuid.UUID, rid string, lastErr string) error {
					return transitionErr
				},
			}
			publisher := &mockPublisher{
				publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
					return errors.New("nack received")
				},
			}

			exhaustedEvent := event
			exhaustedEvent.Attempts = 3

			p, _ := NewProcessor(relayID, store, publisher, cfg)
			err := p.ProcessEvent(context.Background(), exhaustedEvent)
			if !errors.Is(err, transitionErr) {
				t.Fatalf("expected transition error %v, got %v", transitionErr, err)
			}
		})
	})

	t.Run("timeout de publicacao aciona ScheduleRetry com deadline exceeded", func(t *testing.T) {
		var retryCalled bool
		var recordedLastError string
		store := &mockStore{
			scheduleRetryFn: func(ctx context.Context, id uuid.UUID, rid string, availAt time.Time, lastErr string) error {
				retryCalled = true
				recordedLastError = lastErr
				return nil
			},
		}
		publisher := &mockPublisher{
			publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
				// Block until context times out
				<-ctx.Done()
				return ctx.Err()
			},
		}

		shortTimeoutCfg := cfg
		shortTimeoutCfg.PublishTimeout = 10 * time.Millisecond

		p, _ := NewProcessor(relayID, store, publisher, shortTimeoutCfg)
		err := p.ProcessEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !retryCalled {
			t.Fatal("expected ScheduleRetry to be called on timeout")
		}
		if recordedLastError != context.DeadlineExceeded.Error() {
			t.Errorf("expected deadline exceeded, got %q", recordedLastError)
		}
	})
}

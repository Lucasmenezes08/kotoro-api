package outboxrelay

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
)

func TestNewDispatcherValidation(t *testing.T) {
	cfg := Config{}
	t.Run("nil store returns error", func(t *testing.T) {
		jobs := make(chan outbox.OutboxEvent, 5)
		_, err := NewDispatcher("relay-1", nil, jobs, cfg)
		if !errors.Is(err, ErrDispatcherNilStore) {
			t.Fatalf("expected ErrDispatcherNilStore, got %v", err)
		}
	})

	t.Run("nil jobs returns error", func(t *testing.T) {
		store := &mockStore{}
		_, err := NewDispatcher("relay-1", store, nil, cfg)
		if !errors.Is(err, ErrDispatcherNilJobs) {
			t.Fatalf("expected ErrDispatcherNilJobs, got %v", err)
		}
	})
}

func TestDispatcherAdaptivePollingAndBackpressure(t *testing.T) {
	relayID := "test-relay-dispatcher"
	cfg := Config{
		BatchSize:       8,
		PollInterval:    100 * time.Millisecond,
		MaxPollInterval: 800 * time.Millisecond,
		LeaseDuration:   time.Minute,
	}

	t.Run("canal cheio impede claim (backpressure)", func(t *testing.T) {
		bufferSize := 3
		jobs := make(chan outbox.OutboxEvent, bufferSize)
		// Fill channel to capacity
		for i := 0; i < bufferSize; i++ {
			jobs <- outbox.OutboxEvent{ID: uuid.New()}
		}

		var claimCalled bool
		store := &mockStore{
			claimBatchFn: func(ctx context.Context, rid string, limit int, leaseDur time.Duration) ([]outbox.OutboxEvent, error) {
				claimCalled = true
				return nil, nil
			},
		}

		d, err := NewDispatcher(relayID, store, jobs, cfg)
		if err != nil {
			t.Fatalf("new dispatcher: %v", err)
		}

		count, err := d.PollAndDispatch(context.Background())
		if err != nil {
			t.Fatalf("poll error: %v", err)
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
		if claimCalled {
			t.Fatal("ClaimBatch should NOT be called when jobs channel is full")
		}
	})

	t.Run("claim nunca excede capacidade livre", func(t *testing.T) {
		bufferSize := 5
		jobs := make(chan outbox.OutboxEvent, bufferSize)
		// 2 items already in buffer -> free capacity is 3
		jobs <- outbox.OutboxEvent{ID: uuid.New()}
		jobs <- outbox.OutboxEvent{ID: uuid.New()}

		var requestedLimit int
		store := &mockStore{
			claimBatchFn: func(ctx context.Context, rid string, limit int, leaseDur time.Duration) ([]outbox.OutboxEvent, error) {
				requestedLimit = limit
				return nil, nil
			},
		}

		// BatchSize is 8, but free capacity is 3 -> must request limit 3
		d, err := NewDispatcher(relayID, store, jobs, cfg)
		if err != nil {
			t.Fatalf("new dispatcher: %v", err)
		}

		_, err = d.PollAndDispatch(context.Background())
		if err != nil {
			t.Fatalf("poll error: %v", err)
		}
		if requestedLimit != 3 {
			t.Fatalf("requested limit = %d, want 3 (free capacity)", requestedLimit)
		}
	})

	t.Run("lote vazio aumenta intervalo ate o maximo", func(t *testing.T) {
		jobs := make(chan outbox.OutboxEvent, 10)
		store := &mockStore{
			claimBatchFn: func(ctx context.Context, rid string, limit int, leaseDur time.Duration) ([]outbox.OutboxEvent, error) {
				// return empty batch
				return []outbox.OutboxEvent{}, nil
			},
		}

		d, err := NewDispatcher(relayID, store, jobs, cfg)
		if err != nil {
			t.Fatalf("new dispatcher: %v", err)
		}

		if d.CurrentInterval() != 100*time.Millisecond {
			t.Errorf("initial interval = %v, want 100ms", d.CurrentInterval())
		}

		// Empty batch 1: doubles to 200ms
		_, _ = d.PollAndDispatch(context.Background())
		if d.CurrentInterval() != 200*time.Millisecond {
			t.Errorf("step 1 interval = %v, want 200ms", d.CurrentInterval())
		}

		// Empty batch 2: doubles to 400ms
		_, _ = d.PollAndDispatch(context.Background())
		if d.CurrentInterval() != 400*time.Millisecond {
			t.Errorf("step 2 interval = %v, want 400ms", d.CurrentInterval())
		}

		// Empty batch 3: doubles to 800ms (maxPollInterval)
		_, _ = d.PollAndDispatch(context.Background())
		if d.CurrentInterval() != 800*time.Millisecond {
			t.Errorf("step 3 interval = %v, want 800ms", d.CurrentInterval())
		}

		// Empty batch 4: capped at 800ms
		_, _ = d.PollAndDispatch(context.Background())
		if d.CurrentInterval() != 800*time.Millisecond {
			t.Errorf("step 4 interval = %v, want 800ms (capped)", d.CurrentInterval())
		}
	})

	t.Run("lote encontrado restaura minimo", func(t *testing.T) {
		jobs := make(chan outbox.OutboxEvent, 10)
		var returnEvent bool
		store := &mockStore{
			claimBatchFn: func(ctx context.Context, rid string, limit int, leaseDur time.Duration) ([]outbox.OutboxEvent, error) {
				if returnEvent {
					return []outbox.OutboxEvent{{ID: uuid.New(), Attempts: 1}}, nil
				}
				return []outbox.OutboxEvent{}, nil
			},
		}

		d, err := NewDispatcher(relayID, store, jobs, cfg)
		if err != nil {
			t.Fatalf("new dispatcher: %v", err)
		}

		// Back off to max
		for i := 0; i < 5; i++ {
			_, _ = d.PollAndDispatch(context.Background())
		}
		if d.CurrentInterval() != cfg.MaxPollInterval {
			t.Fatalf("expected interval at max %v, got %v", cfg.MaxPollInterval, d.CurrentInterval())
		}

		// Now events are found
		returnEvent = true
		count, err := d.PollAndDispatch(context.Background())
		if err != nil {
			t.Fatalf("poll error: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 event, got %d", count)
		}
		if d.CurrentInterval() != cfg.PollInterval {
			t.Errorf("interval after batch found = %v, want reset to %v", d.CurrentInterval(), cfg.PollInterval)
		}
	})

	t.Run("jitter permanece na faixa", func(t *testing.T) {
		jobs := make(chan outbox.OutboxEvent, 10)
		store := &mockStore{}
		d, err := NewDispatcher(relayID, store, jobs, cfg)
		if err != nil {
			t.Fatalf("new dispatcher: %v", err)
		}

		for i := 0; i < 50; i++ {
			sleepDur := d.sleepDuration()
			// DefaultJitter adds up to 20%
			if sleepDur < cfg.PollInterval || sleepDur > cfg.PollInterval+cfg.PollInterval/5 {
				t.Fatalf("sleep duration with jitter %v out of range [%v, %v]",
					sleepDur, cfg.PollInterval, cfg.PollInterval+cfg.PollInterval/5)
			}
		}
	})

	t.Run("cancelamento interrompe timer e dispatcher fecha canal exatamente uma vez", func(t *testing.T) {
		jobs := make(chan outbox.OutboxEvent, 10)
		var pollCount int64
		store := &mockStore{
			claimBatchFn: func(ctx context.Context, rid string, limit int, leaseDur time.Duration) ([]outbox.OutboxEvent, error) {
				atomic.AddInt64(&pollCount, 1)
				return []outbox.OutboxEvent{}, nil
			},
		}

		d, err := NewDispatcher(relayID, store, jobs, cfg)
		if err != nil {
			t.Fatalf("new dispatcher: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			d.Run(ctx)
			close(done)
		}()

		// Wait briefly then cancel
		time.Sleep(50 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// Verified: Run exited promptly without hanging on timer
		case <-time.After(1 * time.Second):
			t.Fatal("dispatcher did not exit upon context cancellation")
		}

		// Verify channel is closed
		select {
		case _, ok := <-jobs:
			if ok {
				t.Fatal("expected jobs channel to be closed by dispatcher")
			}
		default:
			t.Fatal("jobs channel was not closed")
		}

		// Calling Close again should be idempotent and not panic
		d.Close()
	})
}

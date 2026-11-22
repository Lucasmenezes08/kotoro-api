package outboxrelay

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
)

func TestNewWorkerPoolValidation(t *testing.T) {
	cfg := Config{
		WorkerCount: 3,
		JobBuffer:   5,
	}

	t.Run("nil store returns error", func(t *testing.T) {
		pubs := []Publisher{&mockPublisher{}, &mockPublisher{}, &mockPublisher{}}
		_, err := NewWorkerPool("relay-1", nil, pubs, cfg)
		if !errors.Is(err, ErrWorkerPoolNilStore) {
			t.Fatalf("expected ErrWorkerPoolNilStore, got %v", err)
		}
	})

	t.Run("publishers count mismatch returns error", func(t *testing.T) {
		store := &mockStore{}
		pubs := []Publisher{&mockPublisher{}} // only 1 publisher for 3 workers
		_, err := NewWorkerPool("relay-1", store, pubs, cfg)
		if !errors.Is(err, ErrWorkerPoolPublishersMismatch) {
			t.Fatalf("expected ErrWorkerPoolPublishersMismatch, got %v", err)
		}
	})

	t.Run("zero worker count returns error", func(t *testing.T) {
		store := &mockStore{}
		zeroCfg := cfg
		zeroCfg.WorkerCount = 0
		_, err := NewWorkerPool("relay-1", store, nil, zeroCfg)
		if !errors.Is(err, ErrWorkerPoolInvalidCount) {
			t.Fatalf("expected ErrWorkerPoolInvalidCount, got %v", err)
		}
	})
}

func TestWorkerPoolExecution(t *testing.T) {
	const workerCount = 4
	cfg := Config{
		WorkerCount:    workerCount,
		JobBuffer:      8,
		PublishTimeout: 100 * time.Millisecond,
		MaxAttempts:    3,
		RetryBaseDelay: 50 * time.Millisecond,
		MaxRetryDelay:  200 * time.Millisecond,
	}

	t.Run("quantidade fixa de workers e varios jobs processados", func(t *testing.T) {
		var activeWorkerIDs sync.Map
		var processedCount int64

		publishers := make([]Publisher, workerCount)
		for i := 0; i < workerCount; i++ {
			idx := i
			publishers[i] = &mockPublisher{
				publishFn: func(ctx context.Context, event outbox.OutboxEvent) error {
					activeWorkerIDs.Store(idx, true)
					atomic.AddInt64(&processedCount, 1)
					return nil
				},
			}
		}

		var publishedCount int64
		store := &mockStore{
			markPublishedFn: func(ctx context.Context, eventID uuid.UUID, relayID string, pubAt time.Time) error {
				atomic.AddInt64(&publishedCount, 1)
				return nil
			},
		}

		wp, err := NewWorkerPool("relay-1", store, publishers, cfg)
		if err != nil {
			t.Fatalf("new worker pool: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := wp.Start(ctx); err != nil {
			t.Fatalf("start worker pool: %v", err)
		}

		const numJobs = 20
		jobsCh := wp.Jobs()
		for i := 0; i < numJobs; i++ {
			jobsCh <- outbox.OutboxEvent{
				ID:       uuid.New(),
				Attempts: 1,
			}
		}

		// Close jobs channel (owned by producer)
		close(wp.jobs)

		// Wait for all workers to finish draining jobs
		wp.Wait()

		if atomic.LoadInt64(&processedCount) != numJobs {
			t.Fatalf("processed %d jobs, want %d", processedCount, numJobs)
		}
		if atomic.LoadInt64(&publishedCount) != numJobs {
			t.Fatalf("published %d jobs, want %d", publishedCount, numJobs)
		}

		// Verify multiple distinct workers participated
		var distinctWorkers int
		for i := 0; i < workerCount; i++ {
			if _, ok := activeWorkerIDs.Load(i); ok {
				distinctWorkers++
			}
		}
		if distinctWorkers == 0 {
			t.Fatal("expected workers to process jobs")
		}
	})

	t.Run("erro em um job nao mata worker", func(t *testing.T) {
		var processedAfterError int64
		publishers := make([]Publisher, 1)

		var attemptsForFirstJob int32
		publishers[0] = &mockPublisher{
			publishFn: func(ctx context.Context, event outbox.OutboxEvent) error {
				if event.EventType == "failing-job" {
					atomic.AddInt32(&attemptsForFirstJob, 1)
					return errors.New("simulated error")
				}
				atomic.AddInt64(&processedAfterError, 1)
				return nil
			},
		}

		store := &mockStore{
			scheduleRetryFn: func(ctx context.Context, id uuid.UUID, rid string, availAt time.Time, lastErr string) error {
				return nil
			},
			markPublishedFn: func(ctx context.Context, id uuid.UUID, rid string, pubAt time.Time) error {
				return nil
			},
		}

		singleWorkerCfg := cfg
		singleWorkerCfg.WorkerCount = 1

		wp, err := NewWorkerPool("relay-1", store, publishers, singleWorkerCfg)
		if err != nil {
			t.Fatalf("new worker pool: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := wp.Start(ctx); err != nil {
			t.Fatalf("start worker pool: %v", err)
		}

		jobsCh := wp.Jobs()
		// Job 1 will fail in publisher
		jobsCh <- outbox.OutboxEvent{ID: uuid.New(), EventType: "failing-job", Attempts: 1}
		// Job 2 should still be processed by the same worker!
		jobsCh <- outbox.OutboxEvent{ID: uuid.New(), EventType: "success-job-1", Attempts: 1}
		jobsCh <- outbox.OutboxEvent{ID: uuid.New(), EventType: "success-job-2", Attempts: 1}

		close(wp.jobs)
		wp.Wait()

		if atomic.LoadInt32(&attemptsForFirstJob) != 1 {
			t.Errorf("failing job was not attempted")
		}
		if atomic.LoadInt64(&processedAfterError) != 2 {
			t.Errorf("worker died after error; processed %d subsequent jobs, want 2", processedAfterError)
		}
	})

	t.Run("cancelamento encerra workers", func(t *testing.T) {
		publishers := make([]Publisher, workerCount)
		for i := 0; i < workerCount; i++ {
			publishers[i] = &mockPublisher{
				publishFn: func(ctx context.Context, event outbox.OutboxEvent) error {
					time.Sleep(10 * time.Millisecond)
					return nil
				},
			}
		}

		store := &mockStore{}
		wp, err := NewWorkerPool("relay-1", store, publishers, cfg)
		if err != nil {
			t.Fatalf("new worker pool: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		if err := wp.Start(ctx); err != nil {
			t.Fatalf("start worker pool: %v", err)
		}

		// Cancel context immediately
		cancel()

		done := make(chan struct{})
		go func() {
			wp.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success: workers exited upon cancellation
		case <-time.After(2 * time.Second):
			t.Fatal("workers did not terminate upon context cancellation (goroutine leak)")
		}
	})

	t.Run("ClosePublishers fecha todos os publishers apos termino dos workers", func(t *testing.T) {
		var closedCount int64
		publishers := make([]Publisher, workerCount)
		for i := 0; i < workerCount; i++ {
			publishers[i] = &mockPublisher{
				closeFn: func() error {
					atomic.AddInt64(&closedCount, 1)
					return nil
				},
			}
		}

		store := &mockStore{}
		wp, err := NewWorkerPool("relay-1", store, publishers, cfg)
		if err != nil {
			t.Fatalf("new worker pool: %v", err)
		}

		close(wp.jobs)
		_ = wp.Start(context.Background())
		wp.Wait()

		if err := wp.ClosePublishers(); err != nil {
			t.Fatalf("close publishers: %v", err)
		}
		if atomic.LoadInt64(&closedCount) != workerCount {
			t.Fatalf("closed %d publishers, want %d", closedCount, workerCount)
		}
	})
}

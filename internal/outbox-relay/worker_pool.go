package outboxrelay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
)

var (
	ErrWorkerPoolPublishersMismatch = errors.New("number of publishers must match worker count")
	ErrWorkerPoolNilStore           = errors.New("store is required")
	ErrWorkerPoolInvalidCount       = errors.New("worker count must be greater than zero")
	ErrWorkerPoolAlreadyStarted     = errors.New("worker pool is already started")
)

type WorkerPool struct {
	relayID     string
	store       Store
	publishers  []Publisher
	cfg         Config
	workerCount int
	jobBuffer   int
	jobs        chan outbox.OutboxEvent
	wg          sync.WaitGroup
	mu          sync.Mutex
	started     bool
}

func NewWorkerPool(
	relayID string,
	store Store,
	publishers []Publisher,
	cfg Config,
) (*WorkerPool, error) {
	if store == nil {
		return nil, ErrWorkerPoolNilStore
	}
	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		return nil, ErrWorkerPoolInvalidCount
	}
	if len(publishers) != workerCount {
		return nil, fmt.Errorf("%w: got %d publishers for %d workers", ErrWorkerPoolPublishersMismatch, len(publishers), workerCount)
	}

	jobBuffer := cfg.JobBuffer
	if jobBuffer <= 0 {
		jobBuffer = defaultJobBuffer
	}

	return &WorkerPool{
		relayID:     relayID,
		store:       store,
		publishers:  publishers,
		cfg:         cfg,
		workerCount: workerCount,
		jobBuffer:   jobBuffer,
		jobs:        make(chan outbox.OutboxEvent, jobBuffer),
	}, nil
}

// Jobs returns the channel where the dispatcher pushes claimed events.
// Only the dispatcher owns and closes this channel.
func (wp *WorkerPool) Jobs() chan<- outbox.OutboxEvent {
	return wp.jobs
}

// JobsChannel returns the receive channel (for internal use by workers/tests).
func (wp *WorkerPool) JobsChannel() <-chan outbox.OutboxEvent {
	return wp.jobs
}

// Start launches the fixed pool of workers. Each worker is assigned its own
// dedicated Publisher to guarantee no concurrent AMQP channel sharing.
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.started {
		return ErrWorkerPoolAlreadyStarted
	}
	wp.started = true

	for i := 0; i < wp.workerCount; i++ {
		workerID := i
		publisher := wp.publishers[i]

		proc, err := NewProcessor(wp.relayID, wp.store, publisher, wp.cfg)
		if err != nil {
			return fmt.Errorf("create worker processor %d: %w", workerID, err)
		}

		wp.wg.Add(1)
		go func(id int, processor *Processor) {
			defer wp.wg.Done()
			wp.runWorker(ctx, id, processor)
		}(workerID, proc)
	}

	return nil
}

func (wp *WorkerPool) runWorker(ctx context.Context, workerID int, processor *Processor) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-wp.jobs:
			if !ok {
				return
			}
			if err := processor.ProcessEvent(ctx, event); err != nil {
				slog.Error("failed to process outbox event",
					"worker_id", workerID,
					"event_id", event.ID,
					"error", err,
				)
			}
		}
	}
}

// Wait blocks until all workers finish.
func (wp *WorkerPool) Wait() {
	wp.wg.Wait()
}

// ClosePublishers closes all worker publishers. Must only be called after
// Wait has completed to ensure no worker is publishing concurrently.
func (wp *WorkerPool) ClosePublishers() error {
	var errs []error
	for i, pub := range wp.publishers {
		if pub != nil {
			if err := pub.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close publisher %d: %w", i, err))
			}
		}
	}
	return errors.Join(errs...)
}

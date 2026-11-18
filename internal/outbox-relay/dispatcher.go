package outboxrelay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
)

var (
	ErrDispatcherNilStore = errors.New("store is required")
	ErrDispatcherNilJobs  = errors.New("jobs channel is required")
)

type Dispatcher struct {
	relayID         string
	store           Store
	jobs            chan outbox.OutboxEvent
	cfg             Config
	batchSize       int
	pollInterval    time.Duration
	maxPollInterval time.Duration
	leaseDuration   time.Duration
	jitter          JitterFunc
	closeOnce       sync.Once
	currentInterval time.Duration
}

type DispatcherOption func(*Dispatcher)

func WithDispatcherJitter(jitter JitterFunc) DispatcherOption {
	return func(d *Dispatcher) {
		d.jitter = jitter
	}
}

func NewDispatcher(
	relayID string,
	store Store,
	jobs chan outbox.OutboxEvent,
	cfg Config,
	opts ...DispatcherOption,
) (*Dispatcher, error) {
	if store == nil {
		return nil, ErrDispatcherNilStore
	}
	if jobs == nil {
		return nil, ErrDispatcherNilJobs
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	maxPollInterval := cfg.MaxPollInterval
	if maxPollInterval <= 0 {
		maxPollInterval = defaultMaxPollInterval
	}
	if maxPollInterval < pollInterval {
		maxPollInterval = pollInterval
	}

	leaseDuration := cfg.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = defaultLeaseDuration
	}

	d := &Dispatcher{
		relayID:         relayID,
		store:           store,
		jobs:            jobs,
		cfg:             cfg,
		batchSize:       batchSize,
		pollInterval:    pollInterval,
		maxPollInterval: maxPollInterval,
		leaseDuration:   leaseDuration,
		jitter:          DefaultJitter,
		currentInterval: pollInterval,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}

	return d, nil
}

// FreeCapacity calculates available buffer slots in the jobs channel.
func (d *Dispatcher) FreeCapacity() int {
	return cap(d.jobs) - len(d.jobs)
}

// CurrentInterval returns the current adaptive polling interval (before jitter).
func (d *Dispatcher) CurrentInterval() time.Duration {
	return d.currentInterval
}

// Close closes the jobs channel. Safe to call multiple times; executes exactly once.
func (d *Dispatcher) Close() {
	d.closeOnce.Do(func() {
		close(d.jobs)
	})
}

// PollAndDispatch executes a single poll-and-dispatch step.
// Returns the number of claimed events and any error encountered.
func (d *Dispatcher) PollAndDispatch(ctx context.Context) (int, error) {
	freeCap := d.FreeCapacity()
	if freeCap <= 0 {
		// Backpressure: worker pool is busy and channel is full.
		// Do not query database until workers consume existing jobs.
		return 0, nil
	}

	limit := d.batchSize
	if limit > freeCap {
		limit = freeCap
	}

	events, err := d.store.ClaimBatch(ctx, d.relayID, limit, d.leaseDuration)
	if err != nil {
		return 0, fmt.Errorf("claim batch: %w", err)
	}

	if len(events) == 0 {
		// Empty batch: back off interval up to maxPollInterval
		d.increaseInterval()
		return 0, nil
	}

	// Batch found: reset adaptive interval to minimum
	d.currentInterval = d.pollInterval

	for _, event := range events {
		select {
		case <-ctx.Done():
			return len(events), ctx.Err()
		case d.jobs <- event:
		}
	}

	return len(events), nil
}

func (d *Dispatcher) increaseInterval() {
	next := d.currentInterval * 2
	if next > d.maxPollInterval {
		next = d.maxPollInterval
	}
	d.currentInterval = next
}

func (d *Dispatcher) sleepDuration() time.Duration {
	dur := d.currentInterval
	if d.jitter != nil {
		dur = d.jitter(dur)
		if dur > d.maxPollInterval {
			dur = d.maxPollInterval
		}
	}
	return dur
}

// Run executes the continuous adaptive polling loop until ctx is cancelled.
// Upon termination, it closes the jobs channel exactly once to signal workers.
func (d *Dispatcher) Run(ctx context.Context) {
	defer d.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		count, err := d.PollAndDispatch(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			// On transient error, back off to avoid tight error loop
			d.increaseInterval()
		}

		// If events were found and there is still free capacity in the channel,
		// poll again immediately without waiting the full sleep duration.
		if count > 0 && d.FreeCapacity() > 0 {
			continue
		}

		sleepDur := d.sleepDuration()
		timer := time.NewTimer(sleepDur)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
	}
}

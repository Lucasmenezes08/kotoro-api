package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/database"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/messaging/rabbitmq"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	outboxrelay "github.com/Lucasmenezes08/kotoro-api.git/internal/outbox-relay"
	"github.com/Lucasmenezes08/kotoro-api.git/migrations"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
)

func main() {
	if err := run(); err != nil {
		slog.Error("outbox relay terminated with error", "error", err)
		os.Exit(1)
	}
	slog.Info("outbox relay stopped cleanly")
}

func run() error {
	_ = godotenv.Load()

	cfg, err := outboxrelay.LoadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	relayID := os.Getenv("OUTBOX_RELAY_ID")
	if relayID == "" {
		hostname, err := os.Hostname()
		if err != nil || hostname == "" {
			hostname = "relay"
		}
		relayID = fmt.Sprintf("%s-%s", hostname, uuid.NewString()[:8])
	}

	slog.Info("starting outbox relay",
		"relay_id", relayID,
		"workers", cfg.WorkerCount,
		"batch_size", cfg.BatchSize,
		"job_buffer", cfg.JobBuffer,
		"poll_interval", cfg.PollInterval,
		"max_poll_interval", cfg.MaxPollInterval,
		"lease_duration", cfg.LeaseDuration,
	)

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer startupCancel()

	// 1. Connect PostgreSQL
	db, err := database.ConnectDatabase(startupCtx, database.Config{
		URL:             cfg.DatabaseURL,
		MaxOpenConns:    cfg.WorkerCount + 4,
		MaxIdleConns:    cfg.WorkerCount,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("connect postgresql: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("failed to close postgresql connection", "error", closeErr)
		}
	}()
	slog.Info("connected to postgresql")

	// 2. Ensure migrations are applied
	if err := database.RunMigrations(db, migrations.Files, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("verified database migrations")

	// 3. Connect RabbitMQ
	amqpConn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		return fmt.Errorf("connect rabbitmq: %w", err)
	}
	defer func() {
		if closeErr := amqpConn.Close(); closeErr != nil {
			slog.Error("failed to close rabbitmq connection", "error", closeErr)
		}
	}()
	slog.Info("connected to rabbitmq broker")

	// 4. Declare exchange and demonstration queue topology
	if err := rabbitmq.SetupTopology(amqpConn, cfg.Exchange); err != nil {
		return fmt.Errorf("setup rabbitmq topology: %w", err)
	}
	slog.Info("verified rabbitmq topology", "exchange", cfg.Exchange)

	// 5. Create dedicated confirmed publisher per worker (no channel sharing)
	publishers := make([]outboxrelay.Publisher, cfg.WorkerCount)
	for i := 0; i < cfg.WorkerCount; i++ {
		pub, err := rabbitmq.NewPublisher(amqpConn, cfg.Exchange)
		if err != nil {
			return fmt.Errorf("create publisher for worker %d: %w", i, err)
		}
		publishers[i] = pub
	}

	// 6. Build store, worker pool and dispatcher
	store := outbox.NewRelayStore(db)

	pool, err := outboxrelay.NewWorkerPool(relayID, store, publishers, cfg)
	if err != nil {
		return fmt.Errorf("create worker pool: %w", err)
	}

	dispatcher, err := outboxrelay.NewDispatcher(relayID, store, pool.RawJobsChannel(), cfg)
	if err != nil {
		return fmt.Errorf("create dispatcher: %w", err)
	}

	// 7. Context for signal handling and graceful shutdown
	rootCtx, rootStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer rootStop()

	// 8. Start worker pool
	if err := pool.Start(rootCtx); err != nil {
		return fmt.Errorf("start worker pool: %w", err)
	}
	slog.Info("worker pool started", "workers", cfg.WorkerCount)

	// 9. Run dispatcher in background
	dispatcherDone := make(chan struct{})
	go func() {
		dispatcher.Run(rootCtx)
		close(dispatcherDone)
	}()
	slog.Info("dispatcher started; relay is active")

	// 10. Wait for shutdown signal
	<-rootCtx.Done()
	slog.Info("shutdown signal received; commencing graceful shutdown")
	rootStop()

	// Wait for dispatcher to terminate and close jobs channel
	<-dispatcherDone
	slog.Info("dispatcher terminated; jobs channel closed")

	// 11. Graceful shutdown waiting for workers to drain jobs
	const shutdownTimeout = 25 * time.Second
	workerDone := make(chan struct{})
	go func() {
		pool.Wait()
		close(workerDone)
	}()

	var shutdownErr error
	select {
	case <-workerDone:
		slog.Info("all workers finished in-flight jobs cleanly")
	case <-time.After(shutdownTimeout):
		shutdownErr = errors.New("shutdown deadline exceeded waiting for worker pool")
		slog.Error("shutdown deadline exceeded; incomplete events remain leased and will be recovered after lease expiry")
	}

	// 12. Close publishers
	if pubCloseErr := pool.ClosePublishers(); pubCloseErr != nil {
		slog.Error("failed to close publishers cleanly", "error", pubCloseErr)
		if shutdownErr == nil {
			shutdownErr = pubCloseErr
		}
	}

	return shutdownErr
}

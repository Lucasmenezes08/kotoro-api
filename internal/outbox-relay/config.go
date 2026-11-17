package outboxrelay

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultExchange        = "kotoro.domain-events"
	defaultWorkerCount     = 4
	defaultBatchSize       = 8
	defaultJobBuffer       = 8
	defaultPollInterval    = 250 * time.Millisecond
	defaultMaxPollInterval = 5 * time.Second
	defaultLeaseDuration   = 60 * time.Second
	defaultPublishTimeout  = 10 * time.Second
	defaultMaxAttempts     = 10
	defaultRetryBaseDelay  = time.Second
	defaultMaxRetryDelay   = 5 * time.Minute
)

type Config struct {
	DatabaseURL     string
	RabbitMQURL     string
	Exchange        string
	WorkerCount     int
	BatchSize       int
	JobBuffer       int
	PollInterval    time.Duration
	MaxPollInterval time.Duration
	LeaseDuration   time.Duration
	PublishTimeout  time.Duration
	MaxAttempts     int
	RetryBaseDelay  time.Duration
	MaxRetryDelay   time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	return LoadConfig(os.Getenv)
}

func LoadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("environment getter is required")
	}

	databaseURL, err := requiredEnv(getenv, "DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	rabbitMQURL, err := requiredEnv(getenv, "RABBITMQ_URL")
	if err != nil {
		return Config{}, err
	}

	exchange := strings.TrimSpace(getenv("OUTBOX_EXCHANGE"))
	if exchange == "" {
		exchange = defaultExchange
	}

	workerCount, err := positiveIntEnv(getenv, "OUTBOX_WORKER_COUNT", defaultWorkerCount)
	if err != nil {
		return Config{}, err
	}
	batchSize, err := positiveIntEnv(getenv, "OUTBOX_BATCH_SIZE", defaultBatchSize)
	if err != nil {
		return Config{}, err
	}
	jobBuffer, err := positiveIntEnv(getenv, "OUTBOX_JOB_BUFFER", defaultJobBuffer)
	if err != nil {
		return Config{}, err
	}
	pollInterval, err := positiveDurationEnv(getenv, "OUTBOX_POLL_INTERVAL", defaultPollInterval)
	if err != nil {
		return Config{}, err
	}
	maxPollInterval, err := positiveDurationEnv(getenv, "OUTBOX_MAX_POLL_INTERVAL", defaultMaxPollInterval)
	if err != nil {
		return Config{}, err
	}
	leaseDuration, err := positiveDurationEnv(getenv, "OUTBOX_LEASE_DURATION", defaultLeaseDuration)
	if err != nil {
		return Config{}, err
	}
	publishTimeout, err := positiveDurationEnv(getenv, "OUTBOX_PUBLISH_TIMEOUT", defaultPublishTimeout)
	if err != nil {
		return Config{}, err
	}
	maxAttempts, err := positiveIntEnv(getenv, "OUTBOX_MAX_ATTEMPTS", defaultMaxAttempts)
	if err != nil {
		return Config{}, err
	}
	retryBaseDelay, err := positiveDurationEnv(getenv, "OUTBOX_RETRY_BASE_DELAY", defaultRetryBaseDelay)
	if err != nil {
		return Config{}, err
	}
	maxRetryDelay, err := positiveDurationEnv(getenv, "OUTBOX_MAX_RETRY_DELAY", defaultMaxRetryDelay)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		DatabaseURL:     databaseURL,
		RabbitMQURL:     rabbitMQURL,
		Exchange:        exchange,
		WorkerCount:     workerCount,
		BatchSize:       batchSize,
		JobBuffer:       jobBuffer,
		PollInterval:    pollInterval,
		MaxPollInterval: maxPollInterval,
		LeaseDuration:   leaseDuration,
		PublishTimeout:  publishTimeout,
		MaxAttempts:     maxAttempts,
		RetryBaseDelay:  retryBaseDelay,
		MaxRetryDelay:   maxRetryDelay,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	if strings.TrimSpace(c.RabbitMQURL) == "" {
		return errors.New("RABBITMQ_URL is required")
	}
	if strings.TrimSpace(c.Exchange) == "" {
		return errors.New("OUTBOX_EXCHANGE is required")
	}
	if c.WorkerCount <= 0 {
		return errors.New("OUTBOX_WORKER_COUNT must be greater than zero")
	}
	if c.BatchSize <= 0 {
		return errors.New("OUTBOX_BATCH_SIZE must be greater than zero")
	}
	if c.JobBuffer <= 0 {
		return errors.New("OUTBOX_JOB_BUFFER must be greater than zero")
	}
	if c.PollInterval <= 0 {
		return errors.New("OUTBOX_POLL_INTERVAL must be greater than zero")
	}
	if c.MaxPollInterval <= 0 {
		return errors.New("OUTBOX_MAX_POLL_INTERVAL must be greater than zero")
	}
	if c.LeaseDuration <= 0 {
		return errors.New("OUTBOX_LEASE_DURATION must be greater than zero")
	}
	if c.PublishTimeout <= 0 {
		return errors.New("OUTBOX_PUBLISH_TIMEOUT must be greater than zero")
	}
	if c.MaxAttempts <= 0 {
		return errors.New("OUTBOX_MAX_ATTEMPTS must be greater than zero")
	}
	if c.RetryBaseDelay <= 0 {
		return errors.New("OUTBOX_RETRY_BASE_DELAY must be greater than zero")
	}
	if c.MaxRetryDelay <= 0 {
		return errors.New("OUTBOX_MAX_RETRY_DELAY must be greater than zero")
	}
	if c.PollInterval > c.MaxPollInterval {
		return errors.New("OUTBOX_POLL_INTERVAL must not exceed OUTBOX_MAX_POLL_INTERVAL")
	}
	if c.RetryBaseDelay > c.MaxRetryDelay {
		return errors.New("OUTBOX_RETRY_BASE_DELAY must not exceed OUTBOX_MAX_RETRY_DELAY")
	}

	maxInFlight := c.WorkerCount + c.JobBuffer
	waves := (maxInFlight + c.WorkerCount - 1) / c.WorkerCount
	minimumProcessingWindow := time.Duration(waves) * c.PublishTimeout
	if c.LeaseDuration <= minimumProcessingWindow {
		return fmt.Errorf(
			"OUTBOX_LEASE_DURATION must exceed the maximum processing window of %s",
			minimumProcessingWindow,
		)
	}

	return nil
}

func requiredEnv(getenv func(string) string, key string) (string, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func positiveIntEnv(getenv func(string) string, key string, fallback int) (int, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return parsed, nil
}

func positiveDurationEnv(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return parsed, nil
}

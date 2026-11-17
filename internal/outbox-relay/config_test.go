package outboxrelay

import (
	"strings"
	"testing"
	"time"
)

func TestLoadConfigUsesDefaults(t *testing.T) {
	got, err := LoadConfig(mapGetter(map[string]string{
		"DATABASE_URL": "postgres://database",
		"RABBITMQ_URL": "amqp://broker",
	}))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	want := Config{
		DatabaseURL:     "postgres://database",
		RabbitMQURL:     "amqp://broker",
		Exchange:        defaultExchange,
		WorkerCount:     defaultWorkerCount,
		BatchSize:       defaultBatchSize,
		JobBuffer:       defaultJobBuffer,
		PollInterval:    defaultPollInterval,
		MaxPollInterval: defaultMaxPollInterval,
		LeaseDuration:   defaultLeaseDuration,
		PublishTimeout:  defaultPublishTimeout,
		MaxAttempts:     defaultMaxAttempts,
		RetryBaseDelay:  defaultRetryBaseDelay,
		MaxRetryDelay:   defaultMaxRetryDelay,
	}

	if got != want {
		t.Fatalf("LoadConfig() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigUsesOverrides(t *testing.T) {
	got, err := LoadConfig(mapGetter(map[string]string{
		"DATABASE_URL":             " postgres://custom-database ",
		"RABBITMQ_URL":             " amqp://custom-broker ",
		"OUTBOX_EXCHANGE":          " custom.domain-events ",
		"OUTBOX_WORKER_COUNT":      "2",
		"OUTBOX_BATCH_SIZE":        "3",
		"OUTBOX_JOB_BUFFER":        "4",
		"OUTBOX_POLL_INTERVAL":     "500ms",
		"OUTBOX_MAX_POLL_INTERVAL": "10s",
		"OUTBOX_LEASE_DURATION":    "1m",
		"OUTBOX_PUBLISH_TIMEOUT":   "5s",
		"OUTBOX_MAX_ATTEMPTS":      "3",
		"OUTBOX_RETRY_BASE_DELAY":  "2s",
		"OUTBOX_MAX_RETRY_DELAY":   "1m",
	}))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	want := Config{
		DatabaseURL:     "postgres://custom-database",
		RabbitMQURL:     "amqp://custom-broker",
		Exchange:        "custom.domain-events",
		WorkerCount:     2,
		BatchSize:       3,
		JobBuffer:       4,
		PollInterval:    500 * time.Millisecond,
		MaxPollInterval: 10 * time.Second,
		LeaseDuration:   time.Minute,
		PublishTimeout:  5 * time.Second,
		MaxAttempts:     3,
		RetryBaseDelay:  2 * time.Second,
		MaxRetryDelay:   time.Minute,
	}

	if got != want {
		t.Fatalf("LoadConfig() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		values      map[string]string
		wantMessage string
	}{
		{
			name: "missing database URL",
			values: map[string]string{
				"RABBITMQ_URL": "amqp://broker",
			},
			wantMessage: "DATABASE_URL is required",
		},
		{
			name: "missing RabbitMQ URL",
			values: map[string]string{
				"DATABASE_URL": "postgres://database",
			},
			wantMessage: "RABBITMQ_URL is required",
		},
		{
			name: "non-integer worker count",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_WORKER_COUNT": "abc",
			}),
			wantMessage: "OUTBOX_WORKER_COUNT must be an integer",
		},
		{
			name: "zero worker count",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_WORKER_COUNT": "0",
			}),
			wantMessage: "OUTBOX_WORKER_COUNT must be greater than zero",
		},
		{
			name: "negative batch size",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_BATCH_SIZE": "-1",
			}),
			wantMessage: "OUTBOX_BATCH_SIZE must be greater than zero",
		},
		{
			name: "zero job buffer",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_JOB_BUFFER": "0",
			}),
			wantMessage: "OUTBOX_JOB_BUFFER must be greater than zero",
		},
		{
			name: "invalid duration",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_POLL_INTERVAL": "soon",
			}),
			wantMessage: "OUTBOX_POLL_INTERVAL must be a valid duration",
		},
		{
			name: "zero duration",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_PUBLISH_TIMEOUT": "0s",
			}),
			wantMessage: "OUTBOX_PUBLISH_TIMEOUT must be greater than zero",
		},
		{
			name: "poll interval exceeds maximum",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_POLL_INTERVAL":     "6s",
				"OUTBOX_MAX_POLL_INTERVAL": "5s",
			}),
			wantMessage: "OUTBOX_POLL_INTERVAL must not exceed OUTBOX_MAX_POLL_INTERVAL",
		},
		{
			name: "retry base exceeds maximum",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_RETRY_BASE_DELAY": "6m",
				"OUTBOX_MAX_RETRY_DELAY":  "5m",
			}),
			wantMessage: "OUTBOX_RETRY_BASE_DELAY must not exceed OUTBOX_MAX_RETRY_DELAY",
		},
		{
			name: "lease has no processing margin",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_LEASE_DURATION": "30s",
			}),
			wantMessage: "OUTBOX_LEASE_DURATION must exceed the maximum processing window of 30s",
		},
		{
			name: "custom lease equals processing window",
			values: validRequiredEnv(map[string]string{
				"OUTBOX_WORKER_COUNT":    "2",
				"OUTBOX_JOB_BUFFER":      "8",
				"OUTBOX_PUBLISH_TIMEOUT": "10s",
				"OUTBOX_LEASE_DURATION":  "50s",
			}),
			wantMessage: "OUTBOX_LEASE_DURATION must exceed the maximum processing window of 50s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(mapGetter(tt.values))
			if err == nil {
				t.Fatalf("LoadConfig() error = nil, want error containing %q", tt.wantMessage)
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("LoadConfig() error = %q, want message containing %q", err, tt.wantMessage)
			}
		})
	}
}

func TestLoadConfigRejectsNilGetter(t *testing.T) {
	_, err := LoadConfig(nil)
	if err == nil || err.Error() != "environment getter is required" {
		t.Fatalf("LoadConfig(nil) error = %v, want environment getter error", err)
	}
}

func TestConfigValidateRejectsZeroValueWithoutPanicking(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("Config{}.Validate() error = nil, want validation error")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://database")
	t.Setenv("RABBITMQ_URL", "amqp://broker")
	t.Setenv("OUTBOX_EXCHANGE", "")
	t.Setenv("OUTBOX_WORKER_COUNT", "")
	t.Setenv("OUTBOX_BATCH_SIZE", "")
	t.Setenv("OUTBOX_JOB_BUFFER", "")
	t.Setenv("OUTBOX_POLL_INTERVAL", "")
	t.Setenv("OUTBOX_MAX_POLL_INTERVAL", "")
	t.Setenv("OUTBOX_LEASE_DURATION", "")
	t.Setenv("OUTBOX_PUBLISH_TIMEOUT", "")
	t.Setenv("OUTBOX_MAX_ATTEMPTS", "")
	t.Setenv("OUTBOX_RETRY_BASE_DELAY", "")
	t.Setenv("OUTBOX_MAX_RETRY_DELAY", "")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}
	if cfg.Exchange != defaultExchange || cfg.WorkerCount != defaultWorkerCount {
		t.Fatalf("LoadConfigFromEnv() did not apply defaults: %#v", cfg)
	}
}

func mapGetter(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func validRequiredEnv(overrides map[string]string) map[string]string {
	values := map[string]string{
		"DATABASE_URL": "postgres://database",
		"RABBITMQ_URL": "amqp://broker",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return values
}

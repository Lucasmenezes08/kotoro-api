package outboxrelay

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/database"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/messaging/rabbitmq"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/Lucasmenezes08/kotoro-api.git/migrations"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rabbitmq/amqp091-go"
)

func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = "postgres://koroto_dba_admin:kotoro_dev@localhost:5432/kotoro_db?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.ConnectDatabase(ctx, database.Config{
		URL:             dsn,
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	})
	if err != nil {
		t.Skipf("PostgreSQL not reachable (%v); skipping integration test", err)
	}

	if err := database.RunMigrations(db, migrations.Files, "."); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func openTestRabbitMQ(t *testing.T) *amqp091.Connection {
	t.Helper()
	url := os.Getenv("TEST_RABBITMQ_URL")
	if url == "" {
		url = os.Getenv("RABBITMQ_URL")
	}
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	conn, err := amqp091.Dial(url)
	if err != nil {
		t.Skipf("RabbitMQ not reachable (%v); skipping integration test", err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

func createTestUser(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	var userID uuid.UUID
	email := "e2e-test-" + uuid.NewString() + "@example.com"
	err := db.GetContext(context.Background(), &userID,
		`INSERT INTO users (name, email, password_hash) VALUES ('E2E User', $1, 'hash') RETURNING id`, email)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

func insertOutboxEvent(t *testing.T, db *sqlx.DB, e outbox.OutboxEvent) uuid.UUID {
	t.Helper()
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.EventVersion <= 0 {
		e.EventVersion = 1
	}
	if e.Payload == nil {
		e.Payload = []byte(`{"status":"test"}`)
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC().Add(-10 * time.Second)
	}
	if e.AvailableAt.IsZero() {
		e.AvailableAt = e.OccurredAt
	}

	const q = `
		INSERT INTO outbox_events (
			id, user_id, aggregate_type, aggregate_id, event_type, event_version,
			payload, correlation_id, causation_id, status, occurred_at, available_at,
			attempts, locked_at, locked_by, locked_until
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16
		)
	`
	_, err := db.ExecContext(context.Background(), q,
		e.ID, e.UserID, e.AggregateType, e.AggregateID, e.EventType, e.EventVersion,
		e.Payload, e.CorrelationID, e.CausationID, e.Status, e.OccurredAt, e.AvailableAt,
		e.Attempts, e.LockedAt, e.LockedBy, e.LockedUntil,
	)
	if err != nil {
		t.Fatalf("insert outbox event: %v", err)
	}
	return e.ID
}

// TestEndToEndDeliveryAndRecovery covers the full matrix of failure, delivery,
// lease expiration, retry backoff, and at-least-once duplication scenarios.
func TestEndToEndDeliveryAndRecovery(t *testing.T) {
	db := openTestDB(t)
	amqpConn := openTestRabbitMQ(t)
	ctx := context.Background()

	const exchange = "kotoro.domain-events"
	if err := rabbitmq.SetupTopology(amqpConn, exchange); err != nil {
		t.Fatalf("setup rabbitmq topology: %v", err)
	}

	store := outbox.NewRelayStore(db)

	t.Run("cenario 1 a 3: evento pending -> publicado no RabbitMQ com confirm -> status published", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC().Add(-10 * time.Second)

		eventID := insertOutboxEvent(t, db, outbox.OutboxEvent{
			UserID:        userID,
			AggregateType: "study_cycle",
			AggregateID:   uuid.New(),
			EventType:     "study_cycle.finished",
			EventVersion:  1,
			Payload:       json.RawMessage(`{"study_time_seconds":1800}`),
			Status:        outbox.StatusPending,
			OccurredAt:    now,
			AvailableAt:   now,
		})

		// 1. Claim batch
		relayID := "e2e-relay-happy"
		events, err := store.ClaimBatch(ctx, relayID, 10, time.Minute)
		if err != nil {
			t.Fatalf("claim batch failed: %v", err)
		}

		var claimed *outbox.OutboxEvent
		for i := range events {
			if events[i].ID == eventID {
				claimed = &events[i]
				break
			}
		}
		if claimed == nil {
			t.Fatalf("event %s not claimed", eventID)
		}
		if claimed.Status != outbox.StatusProcessing {
			t.Errorf("status = %s, want processing", claimed.Status)
		}

		// 2. Publish to RabbitMQ using confirmed publisher
		pub, err := rabbitmq.NewPublisher(amqpConn, exchange)
		if err != nil {
			t.Fatalf("new publisher: %v", err)
		}
		defer func() { _ = pub.Close() }()

		cfg := Config{
			PublishTimeout: 5 * time.Second,
			MaxAttempts:    3,
			RetryBaseDelay: 100 * time.Millisecond,
			MaxRetryDelay:  1 * time.Second,
		}

		proc, err := NewProcessor(relayID, store, pub, cfg)
		if err != nil {
			t.Fatalf("new processor: %v", err)
		}

		err = proc.ProcessEvent(ctx, *claimed)
		if err != nil {
			t.Fatalf("process event failed: %v", err)
		}

		// 3. Verify event is now marked as published in database
		var dbEvent outbox.OutboxEvent
		err = db.GetContext(ctx, &dbEvent, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get event from db: %v", err)
		}

		if dbEvent.Status != outbox.StatusPublished {
			t.Errorf("db status = %s, want published", dbEvent.Status)
		}
		if dbEvent.PublishedAt == nil || dbEvent.PublishedAt.IsZero() {
			t.Errorf("db published_at not set: %v", dbEvent.PublishedAt)
		}
		if dbEvent.LockedBy != nil || dbEvent.LockedAt != nil || dbEvent.LockedUntil != nil {
			t.Errorf("locks not cleared: %v, %v, %v", dbEvent.LockedBy, dbEvent.LockedAt, dbEvent.LockedUntil)
		}
	})

	t.Run("cenario 4 a 7: broker indisponivel -> retry reagendado -> broker retorna -> publicado", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC().Add(-10 * time.Second)
		eventID := insertOutboxEvent(t, db, outbox.OutboxEvent{
			UserID:        userID,
			AggregateType: "study_cycle",
			AggregateID:   uuid.New(),
			EventType:     "study_cycle.finished",
			EventVersion:  1,
			Payload:       json.RawMessage(`{"study_time_seconds":900}`),
			Status:        outbox.StatusPending,
			OccurredAt:    now,
			AvailableAt:   now,
		})

		relayID := "e2e-relay-broker-fail"
		events, err := store.ClaimBatch(ctx, relayID, 10, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		var claimed *outbox.OutboxEvent
		for i := range events {
			if events[i].ID == eventID {
				claimed = &events[i]
				break
			}
		}
		if claimed == nil {
			t.Fatalf("event %s not claimed", eventID)
		}

		// Simulate broker unavailable
		failingPublisher := &mockPublisher{
			publishFn: func(ctx context.Context, e outbox.OutboxEvent) error {
				return errors.New("amqp dial: connection refused (broker unavailable)")
			},
		}

		cfg := Config{
			PublishTimeout: 2 * time.Second,
			MaxAttempts:    3,
			RetryBaseDelay: 50 * time.Millisecond,
			MaxRetryDelay:  500 * time.Millisecond,
		}

		fixedTime := now
		proc, _ := NewProcessor(relayID, store, failingPublisher, cfg,
			WithClock(func() time.Time { return fixedTime }),
			WithJitter(func(d time.Duration) time.Duration { return d }),
		)

		err = proc.ProcessEvent(ctx, *claimed)
		if err != nil {
			t.Fatalf("process event unexpected error: %v", err)
		}

		// Event must be back in pending with available_at in the future and last_error recorded
		var retryEvent outbox.OutboxEvent
		err = db.GetContext(ctx, &retryEvent, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get event: %v", err)
		}
		if retryEvent.Status != outbox.StatusPending {
			t.Errorf("status = %s, want pending", retryEvent.Status)
		}
		if retryEvent.LastError == nil || *retryEvent.LastError != "amqp dial: connection refused (broker unavailable)" {
			t.Errorf("last_error = %v, want connection refused", retryEvent.LastError)
		}
		if !retryEvent.AvailableAt.After(now) {
			t.Errorf("available_at %v not in future compared to %v", retryEvent.AvailableAt, now)
		}

		// Now simulate broker recovered: set available_at to now, claim again and publish with real RabbitMQ
		_, err = db.ExecContext(ctx, `UPDATE outbox_events SET available_at = CURRENT_TIMESTAMP WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("update available_at: %v", err)
		}

		reclaimedEvents, err := store.ClaimBatch(ctx, relayID, 10, time.Minute)
		if err != nil {
			t.Fatalf("re-claim failed: %v", err)
		}

		var reclaimed *outbox.OutboxEvent
		for i := range reclaimedEvents {
			if reclaimedEvents[i].ID == eventID {
				reclaimed = &reclaimedEvents[i]
				break
			}
		}
		if reclaimed == nil {
			t.Fatalf("expected reclaimed event %s", eventID)
		}
		if reclaimed.Attempts != 2 {
			t.Errorf("attempts = %d, want 2", reclaimed.Attempts)
		}

		// Real publisher succeeds
		realPub, err := rabbitmq.NewPublisher(amqpConn, exchange)
		if err != nil {
			t.Fatalf("new real publisher: %v", err)
		}
		defer func() { _ = realPub.Close() }()

		procRecovered, _ := NewProcessor(relayID, store, realPub, cfg)
		err = procRecovered.ProcessEvent(ctx, *reclaimed)
		if err != nil {
			t.Fatalf("process recovered event: %v", err)
		}

		// Confirmed published
		var finalEvent outbox.OutboxEvent
		err = db.GetContext(ctx, &finalEvent, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get final event: %v", err)
		}
		if finalEvent.Status != outbox.StatusPublished {
			t.Errorf("final status = %s, want published", finalEvent.Status)
		}
	})

	t.Run("cenario 8 a 11: relay morre apos claim -> lease vence -> outra instancia recupera sem colisao", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC().Add(-10 * time.Second)
		eventID := insertOutboxEvent(t, db, outbox.OutboxEvent{
			UserID:        userID,
			AggregateType: "study_cycle",
			AggregateID:   uuid.New(),
			EventType:     "study_cycle.finished",
			EventVersion:  1,
			Payload:       json.RawMessage(`{"cycle":"lease-test"}`),
			Status:        outbox.StatusPending,
			OccurredAt:    now,
			AvailableAt:   now,
		})

		relayCrashed := "relay-crashed-1"
		relayAlive := "relay-alive-2"

		// 1. relayCrashed claims with very short lease (100ms)
		claimedList, err := store.ClaimBatch(ctx, relayCrashed, 10, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("crashed relay claim failed: %v", err)
		}
		var claimed *outbox.OutboxEvent
		for i := range claimedList {
			if claimedList[i].ID == eventID {
				claimed = &claimedList[i]
				break
			}
		}
		if claimed == nil {
			t.Fatalf("event %s not claimed by crashed relay", eventID)
		}

		// 2. relayCrashed "dies" immediately (does not call MarkPublished)

		// 3. relayAlive attempts claim immediately: lease is still valid, so event must NOT be recovered
		earlyClaim, err := store.ClaimBatch(ctx, relayAlive, 10, time.Minute)
		if err != nil {
			t.Fatalf("early claim error: %v", err)
		}
		for _, e := range earlyClaim {
			if e.ID == eventID {
				t.Fatalf("event %s claimed prematurely while lease was still active", eventID)
			}
		}

		// 4. Wait for lease to expire
		time.Sleep(200 * time.Millisecond)

		// 5. relayAlive recovers the expired event
		recoveredList, err := store.ClaimBatch(ctx, relayAlive, 10, time.Minute)
		if err != nil {
			t.Fatalf("recovery claim failed: %v", err)
		}
		var recovered *outbox.OutboxEvent
		for i := range recoveredList {
			if recoveredList[i].ID == eventID {
				recovered = &recoveredList[i]
				break
			}
		}
		if recovered == nil {
			t.Fatalf("expected event %s to be recovered after lease expiration", eventID)
		}
		if recovered.LockedBy == nil || *recovered.LockedBy != relayAlive {
			t.Errorf("locked_by = %v, want %s", recovered.LockedBy, relayAlive)
		}
		if recovered.Attempts != 2 {
			t.Errorf("attempts = %d, want 2", recovered.Attempts)
		}

		// 6. Complete the recovered event
		err = store.MarkPublished(ctx, eventID, relayAlive, time.Now().UTC())
		if err != nil {
			t.Fatalf("mark published on recovered event failed: %v", err)
		}
	})

	t.Run("cenario 12: mensagem nao roteada (mandatory=true) falha com retorno", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC().Add(-10 * time.Second)

		// An event with a routing key that does NOT match any queue binding
		eventID := insertOutboxEvent(t, db, outbox.OutboxEvent{
			UserID:        userID,
			AggregateType: "unknown",
			AggregateID:   uuid.New(),
			EventType:     "completely.unrouted.topic.nonexistent",
			EventVersion:  99,
			Payload:       json.RawMessage(`{}`),
			Status:        outbox.StatusPending,
			OccurredAt:    now,
			AvailableAt:   now,
		})

		relayID := "relay-mandatory-test"
		events, err := store.ClaimBatch(ctx, relayID, 10, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		var claimed *outbox.OutboxEvent
		for i := range events {
			if events[i].ID == eventID {
				claimed = &events[i]
				break
			}
		}
		if claimed == nil {
			t.Fatalf("event %s not claimed", eventID)
		}

		pub, err := rabbitmq.NewPublisher(amqpConn, exchange)
		if err != nil {
			t.Fatalf("new publisher: %v", err)
		}
		defer func() { _ = pub.Close() }()

		cfg := Config{
			PublishTimeout: 2 * time.Second,
			MaxAttempts:    1, // Only 1 attempt -> triggers MarkFailed
			RetryBaseDelay: time.Second,
			MaxRetryDelay:  time.Second,
		}

		proc, _ := NewProcessor(relayID, store, pub, cfg)
		_ = proc.ProcessEvent(ctx, *claimed)

		// Unroutable message must fail and reach failed status (or be rescheduled if attempts < max)
		var dbEvent outbox.OutboxEvent
		err = db.GetContext(ctx, &dbEvent, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get db event: %v", err)
		}

		if dbEvent.Status != outbox.StatusFailed {
			t.Errorf("expected status failed for unroutable message with max attempts reached, got %s", dbEvent.Status)
		}
		if dbEvent.LastError == nil || *dbEvent.LastError == "" {
			t.Error("expected last_error to be recorded for unroutable message")
		}
	})

	t.Run("cenario 13 e 14: duplicacao possivel apos ACK antes de MarkPublished (at-least-once)", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC().Add(-10 * time.Second)
		eventID := insertOutboxEvent(t, db, outbox.OutboxEvent{
			UserID:        userID,
			AggregateType: "study_cycle",
			AggregateID:   uuid.New(),
			EventType:     "study_cycle.finished",
			EventVersion:  1,
			Payload:       json.RawMessage(`{"at_least_once_test":true}`),
			Status:        outbox.StatusPending,
			OccurredAt:    now,
			AvailableAt:   now,
		})

		relay1 := "relay-duplicate-test-1"
		relay2 := "relay-duplicate-test-2"

		// 1. relay1 claims the event with a 150ms lease
		claimedList, err := store.ClaimBatch(ctx, relay1, 10, 150*time.Millisecond)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}
		var claimed *outbox.OutboxEvent
		for i := range claimedList {
			if claimedList[i].ID == eventID {
				claimed = &claimedList[i]
				break
			}
		}
		if claimed == nil {
			t.Fatalf("event %s not claimed", eventID)
		}

		// 2. Real publish to RabbitMQ succeeds (broker confirms ACK)
		pub1, err := rabbitmq.NewPublisher(amqpConn, exchange)
		if err != nil {
			t.Fatalf("new publisher: %v", err)
		}
		defer func() { _ = pub1.Close() }()

		err = pub1.Publish(ctx, *claimed)
		if err != nil {
			t.Fatalf("first publish to rabbitmq failed: %v", err)
		}

		// 3. Simulated crash!
		// The message is now safely stored in the RabbitMQ queue, but relay1 crashes
		// BEFORE calling store.MarkPublished!

		// 4. Wait for lease to expire in PostgreSQL
		time.Sleep(250 * time.Millisecond)

		// 5. relay2 recovers the expired event
		reclaimedList, err := store.ClaimBatch(ctx, relay2, 10, time.Minute)
		if err != nil {
			t.Fatalf("reclaim failed: %v", err)
		}
		var reclaimed *outbox.OutboxEvent
		for i := range reclaimedList {
			if reclaimedList[i].ID == eventID {
				reclaimed = &reclaimedList[i]
				break
			}
		}
		if reclaimed == nil {
			t.Fatalf("expected event %s to be reclaimed by relay2", eventID)
		}

		// 6. relay2 publishes the event a second time to RabbitMQ (duplicate delivery!)
		pub2, err := rabbitmq.NewPublisher(amqpConn, exchange)
		if err != nil {
			t.Fatalf("new publisher: %v", err)
		}
		defer func() { _ = pub2.Close() }()

		err = pub2.Publish(ctx, *reclaimed)
		if err != nil {
			t.Fatalf("second publish failed: %v", err)
		}

		// 7. relay2 successfully marks the event published in the database
		err = store.MarkPublished(ctx, eventID, relay2, time.Now().UTC())
		if err != nil {
			t.Fatalf("mark published failed: %v", err)
		}

		// 8. Verify the event in DB is finally published with attempts = 2
		var finalEvent outbox.OutboxEvent
		err = db.GetContext(ctx, &finalEvent, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get db event: %v", err)
		}
		if finalEvent.Status != outbox.StatusPublished {
			t.Errorf("status = %s, want published", finalEvent.Status)
		}
		if finalEvent.Attempts != 2 {
			t.Errorf("attempts = %d, want 2 (demonstrating duplicate publication)", finalEvent.Attempts)
		}
	})
}

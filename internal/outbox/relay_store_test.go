package outbox

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/database"
	"github.com/Lucasmenezes08/kotoro-api.git/migrations"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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

func createTestUser(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	var userID uuid.UUID
	email := "relay-test-" + uuid.NewString() + "@example.com"
	err := db.GetContext(context.Background(), &userID,
		`INSERT INTO users (name, email, password_hash) VALUES ('Test User', $1, 'hash') RETURNING id`, email)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

func insertTestEvent(t *testing.T, db *sqlx.DB, e OutboxEvent) uuid.UUID {
	t.Helper()
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.EventVersion <= 0 {
		e.EventVersion = 1
	}
	if e.Payload == nil {
		e.Payload = []byte(`{"test":true}`)
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
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
		t.Fatalf("failed to insert test event: %v", err)
	}
	return e.ID
}

func TestRelayStoreValidation(t *testing.T) {
	store := NewRelayStore(nil)
	ctx := context.Background()

	t.Run("empty relayID returns error", func(t *testing.T) {
		_, err := store.ClaimBatch(ctx, "", 10, time.Minute)
		if err != ErrInvalidRelayID {
			t.Fatalf("expected ErrInvalidRelayID, got %v", err)
		}
	})

	t.Run("zero limit returns error", func(t *testing.T) {
		_, err := store.ClaimBatch(ctx, "relay-1", 0, time.Minute)
		if err != ErrInvalidLimit {
			t.Fatalf("expected ErrInvalidLimit, got %v", err)
		}
	})

	t.Run("negative limit returns error", func(t *testing.T) {
		_, err := store.ClaimBatch(ctx, "relay-1", -5, time.Minute)
		if err != ErrInvalidLimit {
			t.Fatalf("expected ErrInvalidLimit, got %v", err)
		}
	})

	t.Run("zero leaseDuration returns error", func(t *testing.T) {
		_, err := store.ClaimBatch(ctx, "relay-1", 10, 0)
		if err != ErrInvalidLeaseDuration {
			t.Fatalf("expected ErrInvalidLeaseDuration, got %v", err)
		}
	})

	t.Run("negative leaseDuration returns error", func(t *testing.T) {
		_, err := store.ClaimBatch(ctx, "relay-1", 10, -time.Second)
		if err != ErrInvalidLeaseDuration {
			t.Fatalf("expected ErrInvalidLeaseDuration, got %v", err)
		}
	})
}

func TestRelayStoreClaimBatchIntegration(t *testing.T) {
	db := openTestDB(t)
	store := NewRelayStore(db)
	ctx := context.Background()

	t.Run("lote vazio retorna slice vazio", func(t *testing.T) {
		userID := createTestUser(t, db)
		_ = userID

		// Claim when no eligible events exist
		events, err := store.ClaimBatch(ctx, "relay-empty-test", 10, time.Minute)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if events == nil {
			t.Fatal("expected empty slice, got nil")
		}
	})

	t.Run("pending disponivel e reivindicado", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusPending,
			OccurredAt:    now.Add(-10 * time.Second),
			AvailableAt:   now.Add(-5 * time.Second),
			Attempts:      0,
		})

		events, err := store.ClaimBatch(ctx, "relay-available-test", 10, 30*time.Second)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		var claimed *OutboxEvent
		for i := range events {
			if events[i].ID == eventID {
				claimed = &events[i]
				break
			}
		}

		if claimed == nil {
			t.Fatalf("expected event %s to be claimed", eventID)
		}
		if claimed.Status != StatusProcessing {
			t.Errorf("expected status processing, got %s", claimed.Status)
		}
		if claimed.LockedBy == nil || *claimed.LockedBy != "relay-available-test" {
			t.Errorf("expected locked_by relay-available-test, got %v", claimed.LockedBy)
		}
		if claimed.Attempts != 1 {
			t.Errorf("expected attempts 1, got %d", claimed.Attempts)
		}
		if claimed.LockedUntil == nil || !claimed.LockedUntil.After(now) {
			t.Errorf("expected locked_until in the future, got %v", claimed.LockedUntil)
		}
	})

	t.Run("pending futuro nao e reivindicado", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeSprint,
			AggregateID:   uuid.New(),
			EventType:     EventTypeSprintStarted,
			Status:        StatusPending,
			OccurredAt:    now,
			AvailableAt:   now.Add(1 * time.Hour), // Future available_at
			Attempts:      0,
		})

		events, err := store.ClaimBatch(ctx, "relay-future-test", 10, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		for _, e := range events {
			if e.ID == eventID {
				t.Fatalf("future pending event %s was claimed unexpectedly", eventID)
			}
		}

		// Verify event remains pending in database
		var status OutboxEventStatus
		err = db.GetContext(ctx, &status, `SELECT status FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("query status: %v", err)
		}
		if status != StatusPending {
			t.Errorf("expected status pending, got %s", status)
		}
	})

	t.Run("processing com lease valido nao e recuperado", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		lockedBy := "other-relay"
		lockedUntil := now.Add(5 * time.Minute)
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeSprint,
			AggregateID:   uuid.New(),
			EventType:     EventTypeSprintStarted,
			Status:        StatusProcessing,
			OccurredAt:    now.Add(-10 * time.Minute),
			AvailableAt:   now.Add(-10 * time.Minute),
			LockedAt:      &now,
			LockedBy:      &lockedBy,
			LockedUntil:   &lockedUntil,
			Attempts:      1,
		})

		events, err := store.ClaimBatch(ctx, "relay-valid-lease-test", 10, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		for _, e := range events {
			if e.ID == eventID {
				t.Fatalf("event with valid lease %s was claimed unexpectedly", eventID)
			}
		}
	})

	t.Run("processing com lease vencido e recuperado", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		lockedBy := "crashed-relay"
		expiredUntil := now.Add(-10 * time.Second)
		lockedAt := now.Add(-70 * time.Second)
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusProcessing,
			OccurredAt:    now.Add(-80 * time.Second),
			AvailableAt:   now.Add(-80 * time.Second),
			LockedAt:      &lockedAt,
			LockedBy:      &lockedBy,
			LockedUntil:   &expiredUntil,
			Attempts:      1,
		})

		events, err := store.ClaimBatch(ctx, "recovery-relay", 10, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		var recovered *OutboxEvent
		for i := range events {
			if events[i].ID == eventID {
				recovered = &events[i]
				break
			}
		}

		if recovered == nil {
			t.Fatalf("expected expired processing event %s to be recovered", eventID)
		}
		if recovered.LockedBy == nil || *recovered.LockedBy != "recovery-relay" {
			t.Errorf("expected locked_by recovery-relay, got %v", recovered.LockedBy)
		}
		if recovered.Attempts != 2 {
			t.Errorf("expected attempts incremented to 2, got %d", recovered.Attempts)
		}
	})

	t.Run("attempts e incrementado", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusPending,
			OccurredAt:    now.Add(-10 * time.Second),
			AvailableAt:   now.Add(-5 * time.Second),
			Attempts:      3,
		})

		events, err := store.ClaimBatch(ctx, "relay-attempts-test", 10, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		var found *OutboxEvent
		for i := range events {
			if events[i].ID == eventID {
				found = &events[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("event %s not found in claimed events", eventID)
		}
		if found.Attempts != 4 {
			t.Errorf("expected attempts 4, got %d", found.Attempts)
		}
	})

	t.Run("limite do lote e respeitado", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		for i := 0; i < 5; i++ {
			insertTestEvent(t, db, OutboxEvent{
				UserID:        userID,
				AggregateType: AggregateTypeStudyCycle,
				AggregateID:   uuid.New(),
				EventType:     EventTypeStudyCycleFinished,
				Status:        StatusPending,
				OccurredAt:    now.Add(time.Duration(-10+i) * time.Second),
				AvailableAt:   now.Add(time.Duration(-5+i) * time.Second),
			})
		}

		limit := 3
		events, err := store.ClaimBatch(ctx, "relay-limit-test", limit, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}
		if len(events) > limit {
			t.Fatalf("expected at most %d events, got %d", limit, len(events))
		}
	})

	t.Run("commit persiste ownership", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusPending,
			OccurredAt:    now.Add(-10 * time.Second),
			AvailableAt:   now.Add(-5 * time.Second),
		})

		relayID := "relay-ownership-test"
		events, err := store.ClaimBatch(ctx, relayID, 10, 2*time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		var claimed bool
		for _, e := range events {
			if e.ID == eventID {
				claimed = true
				break
			}
		}
		if !claimed {
			t.Fatalf("event %s not claimed", eventID)
		}

		// Read directly from DB in a separate query to verify persistence
		var dbStatus OutboxEventStatus
		var dbLockedBy string
		var dbAttempts int
		err = db.QueryRowContext(ctx,
			`SELECT status, locked_by, attempts FROM outbox_events WHERE id = $1`, eventID).
			Scan(&dbStatus, &dbLockedBy, &dbAttempts)
		if err != nil {
			t.Fatalf("query db: %v", err)
		}
		if dbStatus != StatusProcessing {
			t.Errorf("db status = %s, want processing", dbStatus)
		}
		if dbLockedBy != relayID {
			t.Errorf("db locked_by = %s, want %s", dbLockedBy, relayID)
		}
		if dbAttempts != 1 {
			t.Errorf("db attempts = %d, want 1", dbAttempts)
		}
	})

	t.Run("rollback ocorre em erro", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusPending,
			OccurredAt:    now.Add(-10 * time.Second),
			AvailableAt:   now.Add(-5 * time.Second),
		})

		// Use a context that is cancelled immediately
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, err := store.ClaimBatch(cancelledCtx, "relay-rollback-test", 10, time.Minute)
		if err == nil {
			t.Fatal("expected error on cancelled context, got nil")
		}

		// Verify event was NOT updated and remains pending
		var status OutboxEventStatus
		var lockedBy *string
		err = db.QueryRowContext(ctx, `SELECT status, locked_by FROM outbox_events WHERE id = $1`, eventID).
			Scan(&status, &lockedBy)
		if err != nil {
			t.Fatalf("query event: %v", err)
		}
		if status != StatusPending {
			t.Errorf("status was altered despite rollback: %s", status)
		}
		if lockedBy != nil {
			t.Errorf("locked_by was altered despite rollback: %v", lockedBy)
		}
	})

	t.Run("dois claimers concorrentes nao recebem o mesmo ID", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()

		const totalEvents = 20
		insertedIDs := make([]uuid.UUID, totalEvents)
		for i := 0; i < totalEvents; i++ {
			insertedIDs[i] = insertTestEvent(t, db, OutboxEvent{
				UserID:        userID,
				AggregateType: AggregateTypeStudyCycle,
				AggregateID:   uuid.New(),
				EventType:     EventTypeStudyCycleFinished,
				Status:        StatusPending,
				OccurredAt:    now.Add(time.Duration(-totalEvents+i) * time.Second),
				AvailableAt:   now.Add(time.Duration(-totalEvents+i) * time.Second),
			})
		}

		startBarrier := make(chan struct{})
		var wg sync.WaitGroup

		var mu sync.Mutex
		claimedIDs := make(map[uuid.UUID]string)
		duplicates := make([]uuid.UUID, 0)

		claimWorker := func(relayID string) {
			defer wg.Done()
			<-startBarrier

			events, err := store.ClaimBatch(ctx, relayID, totalEvents, time.Minute)
			if err != nil {
				t.Errorf("%s claim failed: %v", relayID, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, e := range events {
				if prevOwner, exists := claimedIDs[e.ID]; exists {
					duplicates = append(duplicates, e.ID)
					t.Errorf("duplicate claim for event %s: claimed by %s, then %s", e.ID, prevOwner, relayID)
				} else {
					claimedIDs[e.ID] = relayID
				}
			}
		}

		wg.Add(2)
		go claimWorker("relay-alpha")
		go claimWorker("relay-beta")

		// Release both workers simultaneously
		close(startBarrier)
		wg.Wait()

		if len(duplicates) > 0 {
			t.Fatalf("detected %d duplicate claims across concurrent workers: %v", len(duplicates), duplicates)
		}

		// Ensure all inserted events were claimed between alpha and beta
		for _, id := range insertedIDs {
			if _, ok := claimedIDs[id]; !ok {
				t.Errorf("event %s was not claimed by any worker", id)
			}
		}
	})
}

func TestRelayStoreTransitionsIntegration(t *testing.T) {
	db := openTestDB(t)
	store := NewRelayStore(db)
	ctx := context.Background()

	t.Run("MarkPublished atualiza status, limpa locks e preenche published_at", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		relayID := "relay-pub-owner"
		lockedUntil := now.Add(time.Minute)
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusProcessing,
			OccurredAt:    now.Add(-time.Minute),
			AvailableAt:   now.Add(-time.Minute),
			LockedAt:      &now,
			LockedBy:      &relayID,
			LockedUntil:   &lockedUntil,
			Attempts:      1,
		})

		publishedAt := now.Add(2 * time.Second)
		err := store.MarkPublished(ctx, eventID, relayID, publishedAt)
		if err != nil {
			t.Fatalf("MarkPublished failed: %v", err)
		}

		var event OutboxEvent
		err = db.GetContext(ctx, &event, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get event: %v", err)
		}

		if event.Status != StatusPublished {
			t.Errorf("expected status published, got %s", event.Status)
		}
		if event.PublishedAt == nil || event.PublishedAt.IsZero() {
			t.Errorf("expected published_at populated, got %v", event.PublishedAt)
		}
		if event.LockedAt != nil || event.LockedBy != nil || event.LockedUntil != nil {
			t.Errorf("expected locks cleared, got locked_at=%v locked_by=%v locked_until=%v",
				event.LockedAt, event.LockedBy, event.LockedUntil)
		}
	})

	t.Run("ScheduleRetry reagenda evento, limpa locks e grava last_error", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		relayID := "relay-retry-owner"
		lockedUntil := now.Add(time.Minute)
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeSprint,
			AggregateID:   uuid.New(),
			EventType:     EventTypeSprintStarted,
			Status:        StatusProcessing,
			OccurredAt:    now.Add(-time.Minute),
			AvailableAt:   now.Add(-time.Minute),
			LockedAt:      &now,
			LockedBy:      &relayID,
			LockedUntil:   &lockedUntil,
			Attempts:      1,
		})

		retryAt := now.Add(10 * time.Second)
		err := store.ScheduleRetry(ctx, eventID, relayID, retryAt, "connection timeout to broker")
		if err != nil {
			t.Fatalf("ScheduleRetry failed: %v", err)
		}

		var event OutboxEvent
		err = db.GetContext(ctx, &event, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get event: %v", err)
		}

		if event.Status != StatusPending {
			t.Errorf("expected status pending, got %s", event.Status)
		}
		if event.LastError == nil || *event.LastError != "connection timeout to broker" {
			t.Errorf("expected last_error populated, got %v", event.LastError)
		}
		if event.LockedAt != nil || event.LockedBy != nil || event.LockedUntil != nil {
			t.Errorf("expected locks cleared, got locked_at=%v locked_by=%v locked_until=%v",
				event.LockedAt, event.LockedBy, event.LockedUntil)
		}
	})

	t.Run("MarkFailed define status failed, preserva erro e limpa locks", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		relayID := "relay-fail-owner"
		lockedUntil := now.Add(time.Minute)
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusProcessing,
			OccurredAt:    now.Add(-time.Minute),
			AvailableAt:   now.Add(-time.Minute),
			LockedAt:      &now,
			LockedBy:      &relayID,
			LockedUntil:   &lockedUntil,
			Attempts:      10,
		})

		err := store.MarkFailed(ctx, eventID, relayID, "max attempts reached: permanent broker failure")
		if err != nil {
			t.Fatalf("MarkFailed failed: %v", err)
		}

		var event OutboxEvent
		err = db.GetContext(ctx, &event, `SELECT * FROM outbox_events WHERE id = $1`, eventID)
		if err != nil {
			t.Fatalf("get event: %v", err)
		}

		if event.Status != StatusFailed {
			t.Errorf("expected status failed, got %s", event.Status)
		}
		if event.LastError == nil || *event.LastError != "max attempts reached: permanent broker failure" {
			t.Errorf("expected last_error populated, got %v", event.LastError)
		}
		if event.LockedAt != nil || event.LockedBy != nil || event.LockedUntil != nil {
			t.Errorf("expected locks cleared, got locked_at=%v locked_by=%v locked_until=%v",
				event.LockedAt, event.LockedBy, event.LockedUntil)
		}
	})

	t.Run("relayID incorreto nao atualiza e retorna ErrLeaseLost", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		correctOwner := "relay-correct"
		wrongOwner := "relay-wrong"
		lockedUntil := now.Add(time.Minute)
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusProcessing,
			OccurredAt:    now.Add(-time.Minute),
			AvailableAt:   now.Add(-time.Minute),
			LockedAt:      &now,
			LockedBy:      &correctOwner,
			LockedUntil:   &lockedUntil,
			Attempts:      1,
		})

		err := store.MarkPublished(ctx, eventID, wrongOwner, now)
		if !errors.Is(err, ErrLeaseLost) {
			t.Fatalf("expected ErrLeaseLost for wrong relay ID, got %v", err)
		}

		err = store.ScheduleRetry(ctx, eventID, wrongOwner, now.Add(time.Second), "err")
		if !errors.Is(err, ErrLeaseLost) {
			t.Fatalf("expected ErrLeaseLost for wrong relay ID, got %v", err)
		}

		err = store.MarkFailed(ctx, eventID, wrongOwner, "err")
		if !errors.Is(err, ErrLeaseLost) {
			t.Fatalf("expected ErrLeaseLost for wrong relay ID, got %v", err)
		}
	})

	t.Run("status incorreto nao atualiza e retorna ErrLeaseLost", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		relayID := "relay-status-test"

		// Event in pending status
		pendingID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeSprint,
			AggregateID:   uuid.New(),
			EventType:     EventTypeSprintStarted,
			Status:        StatusPending,
			OccurredAt:    now,
			AvailableAt:   now,
		})

		err := store.MarkPublished(ctx, pendingID, relayID, now)
		if !errors.Is(err, ErrLeaseLost) {
			t.Fatalf("expected ErrLeaseLost when marking non-processing event, got %v", err)
		}
	})

	t.Run("worker atrasado nao altera evento recuperado por outro relay", func(t *testing.T) {
		userID := createTestUser(t, db)
		now := time.Now().UTC()
		slowRelay := "relay-slow"
		fastRelay := "relay-fast"

		// 1. Event was claimed by relay-slow, but its lease expired
		expiredUntil := now.Add(-10 * time.Second)
		lockedAt := now.Add(-70 * time.Second)
		eventID := insertTestEvent(t, db, OutboxEvent{
			UserID:        userID,
			AggregateType: AggregateTypeStudyCycle,
			AggregateID:   uuid.New(),
			EventType:     EventTypeStudyCycleFinished,
			Status:        StatusProcessing,
			OccurredAt:    now.Add(-80 * time.Second),
			AvailableAt:   now.Add(-80 * time.Second),
			LockedAt:      &lockedAt,
			LockedBy:      &slowRelay,
			LockedUntil:   &expiredUntil,
			Attempts:      1,
		})

		// 2. relay-fast recovers the event via ClaimBatch
		events, err := store.ClaimBatch(ctx, fastRelay, 10, time.Minute)
		if err != nil {
			t.Fatalf("claim failed: %v", err)
		}

		var claimed bool
		for _, e := range events {
			if e.ID == eventID {
				claimed = true
				break
			}
		}
		if !claimed {
			t.Fatalf("expected event %s to be recovered by relay-fast", eventID)
		}

		// 3. relay-slow wakes up late and tries to acknowledge completion or retry
		err = store.MarkPublished(ctx, eventID, slowRelay, now)
		if !errors.Is(err, ErrLeaseLost) {
			t.Fatalf("expected ErrLeaseLost when slow worker attempts MarkPublished, got %v", err)
		}

		err = store.ScheduleRetry(ctx, eventID, slowRelay, now.Add(time.Minute), "late failure")
		if !errors.Is(err, ErrLeaseLost) {
			t.Fatalf("expected ErrLeaseLost when slow worker attempts ScheduleRetry, got %v", err)
		}

		// 4. Verify in DB that event is still intact and owned by fastRelay
		var dbStatus OutboxEventStatus
		var dbLockedBy string
		err = db.QueryRowContext(ctx, `SELECT status, locked_by FROM outbox_events WHERE id = $1`, eventID).
			Scan(&dbStatus, &dbLockedBy)
		if err != nil {
			t.Fatalf("query db: %v", err)
		}
		if dbStatus != StatusProcessing {
			t.Errorf("db status altered by slow worker: got %s, want processing", dbStatus)
		}
		if dbLockedBy != fastRelay {
			t.Errorf("ownership stolen by slow worker: got %s, want %s", dbLockedBy, fastRelay)
		}
	})
}

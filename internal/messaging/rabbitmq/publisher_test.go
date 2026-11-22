package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

type mockAMQPChannel struct {
	publishFunc func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error
	closeFunc   func() error

	confirms chan amqp091.Confirmation
	returns  chan amqp091.Return
	closes   chan *amqp091.Error
}

func newMockAMQPChannel() *mockAMQPChannel {
	return &mockAMQPChannel{
		confirms: make(chan amqp091.Confirmation, 10),
		returns:  make(chan amqp091.Return, 10),
		closes:   make(chan *amqp091.Error, 10),
	}
}

func (m *mockAMQPChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, exchange, key, mandatory, immediate, msg)
	}
	return nil
}

func (m *mockAMQPChannel) NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation {
	return m.confirms
}

func (m *mockAMQPChannel) NotifyReturn(returns chan amqp091.Return) chan amqp091.Return {
	return m.returns
}

func (m *mockAMQPChannel) NotifyClose(c chan *amqp091.Error) chan *amqp091.Error {
	return m.closes
}

func (m *mockAMQPChannel) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestRabbitMQPublisherEnvelopeRoutingAndProperties(t *testing.T) {
	mockCh := newMockAMQPChannel()

	var recordedExchange string
	var recordedKey string
	var recordedMandatory bool
	var recordedMsg amqp091.Publishing

	mockCh.publishFunc = func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
		recordedExchange = exchange
		recordedKey = key
		recordedMandatory = mandatory
		recordedMsg = msg
		// Send ACK
		mockCh.confirms <- amqp091.Confirmation{DeliveryTag: 1, Ack: true}
		return nil
	}

	pub, err := NewPublisherFromChannel(mockCh, "kotoro.domain-events")
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	eventID := uuid.New()
	userID := uuid.New()
	aggID := uuid.New()
	corrID := uuid.New()
	causID := uuid.New()
	occurredAt := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

	event := outbox.OutboxEvent{
		ID:            eventID,
		UserID:        userID,
		AggregateType: "study_cycle",
		AggregateID:   aggID,
		EventType:     "study_cycle.finished",
		EventVersion:  1,
		Payload:       json.RawMessage(`{"elapsed_seconds":3600}`),
		CorrelationID: &corrID,
		CausationID:   &causID,
		OccurredAt:    occurredAt,
	}

	err = pub.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	// 1. Verify routing key
	expectedKey := "study_cycle.finished.v1"
	if recordedKey != expectedKey {
		t.Errorf("routing key = %q, want %q", recordedKey, expectedKey)
	}

	// 2. Verify exchange
	if recordedExchange != "kotoro.domain-events" {
		t.Errorf("exchange = %q, want %q", recordedExchange, "kotoro.domain-events")
	}

	// 3. Verify mandatory flag
	if !recordedMandatory {
		t.Error("expected mandatory=true")
	}

	// 4. Verify AMQP properties
	if recordedMsg.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want application/json", recordedMsg.ContentType)
	}
	if recordedMsg.DeliveryMode != amqp091.Persistent {
		t.Errorf("DeliveryMode = %d, want persistent (%d)", recordedMsg.DeliveryMode, amqp091.Persistent)
	}
	if recordedMsg.MessageId != eventID.String() {
		t.Errorf("MessageId = %q, want %q", recordedMsg.MessageId, eventID.String())
	}
	if recordedMsg.Type != "study_cycle.finished" {
		t.Errorf("Type = %q, want study_cycle.finished", recordedMsg.Type)
	}
	if !recordedMsg.Timestamp.Equal(occurredAt) {
		t.Errorf("Timestamp = %v, want %v", recordedMsg.Timestamp, occurredAt)
	}

	// 5. Verify JSON envelope structure
	var env EventEnvelope
	if err := json.Unmarshal(recordedMsg.Body, &env); err != nil {
		t.Fatalf("unmarshal body to envelope: %v", err)
	}
	if env.ID != eventID {
		t.Errorf("envelope ID = %s, want %s", env.ID, eventID)
	}
	if env.UserID != userID {
		t.Errorf("envelope UserID = %s, want %s", env.UserID, userID)
	}
	if env.AggregateType != "study_cycle" {
		t.Errorf("envelope AggregateType = %s, want study_cycle", env.AggregateType)
	}
	if env.AggregateID != aggID {
		t.Errorf("envelope AggregateID = %s, want %s", env.AggregateID, aggID)
	}
	if env.EventType != "study_cycle.finished" {
		t.Errorf("envelope EventType = %s, want study_cycle.finished", env.EventType)
	}
	if env.EventVersion != 1 {
		t.Errorf("envelope EventVersion = %d, want 1", env.EventVersion)
	}
	if string(env.Payload) != `{"elapsed_seconds":3600}` {
		t.Errorf("envelope Payload = %s, want %s", string(env.Payload), `{"elapsed_seconds":3600}`)
	}
	if env.CorrelationID == nil || *env.CorrelationID != corrID {
		t.Errorf("envelope CorrelationID = %v, want %v", env.CorrelationID, corrID)
	}
	if env.CausationID == nil || *env.CausationID != causID {
		t.Errorf("envelope CausationID = %v, want %v", env.CausationID, causID)
	}
	if !env.OccurredAt.Equal(occurredAt) {
		t.Errorf("envelope OccurredAt = %v, want %v", env.OccurredAt, occurredAt)
	}
}

func TestRabbitMQPublisherConfirmsAndErrors(t *testing.T) {
	event := outbox.OutboxEvent{
		ID:            uuid.New(),
		UserID:        uuid.New(),
		AggregateType: "study_cycle",
		AggregateID:   uuid.New(),
		EventType:     "study_cycle.finished",
		EventVersion:  1,
		Payload:       json.RawMessage(`{}`),
	}

	t.Run("ACK retorna nil", func(t *testing.T) {
		mockCh := newMockAMQPChannel()
		mockCh.publishFunc = func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
			mockCh.confirms <- amqp091.Confirmation{DeliveryTag: 1, Ack: true}
			return nil
		}

		pub, _ := NewPublisherFromChannel(mockCh, DefaultExchange)
		err := pub.Publish(context.Background(), event)
		if err != nil {
			t.Fatalf("expected nil for ACK, got: %v", err)
		}
	})

	t.Run("NACK retorna erro ErrBrokerNack", func(t *testing.T) {
		mockCh := newMockAMQPChannel()
		mockCh.publishFunc = func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
			mockCh.confirms <- amqp091.Confirmation{DeliveryTag: 1, Ack: false}
			return nil
		}

		pub, _ := NewPublisherFromChannel(mockCh, DefaultExchange)
		err := pub.Publish(context.Background(), event)
		if !errors.Is(err, ErrBrokerNack) {
			t.Fatalf("expected ErrBrokerNack, got: %v", err)
		}
	})

	t.Run("retorno obrigatorio (mandatory unroutable) retorna erro", func(t *testing.T) {
		mockCh := newMockAMQPChannel()
		mockCh.publishFunc = func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
			mockCh.returns <- amqp091.Return{
				ReplyCode:  312,
				ReplyText:  "NO_ROUTE",
				Exchange:   exchange,
				RoutingKey: key,
			}
			return nil
		}

		pub, _ := NewPublisherFromChannel(mockCh, DefaultExchange)
		err := pub.Publish(context.Background(), event)
		if !errors.Is(err, ErrMessageUnroutable) {
			t.Fatalf("expected ErrMessageUnroutable, got: %v", err)
		}
	})

	t.Run("timeout retorna erro", func(t *testing.T) {
		mockCh := newMockAMQPChannel()
		mockCh.publishFunc = func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
			// Do not send confirmation or return; let context time out
			return nil
		}

		pub, _ := NewPublisherFromChannel(mockCh, DefaultExchange)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err := pub.Publish(ctx, event)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded in error, got: %v", err)
		}
	})

	t.Run("channel fechado retorna erro", func(t *testing.T) {
		mockCh := newMockAMQPChannel()
		mockCh.publishFunc = func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error {
			mockCh.closes <- &amqp091.Error{
				Code:   504,
				Reason: "channel exception",
			}
			return nil
		}

		pub, _ := NewPublisherFromChannel(mockCh, DefaultExchange)
		err := pub.Publish(context.Background(), event)
		if !errors.Is(err, ErrChannelClosed) {
			t.Fatalf("expected ErrChannelClosed, got: %v", err)
		}
	})

	t.Run("publisher fechado impede novas publicacoes", func(t *testing.T) {
		mockCh := newMockAMQPChannel()
		pub, _ := NewPublisherFromChannel(mockCh, DefaultExchange)
		if err := pub.Close(); err != nil {
			t.Fatalf("close publisher: %v", err)
		}

		err := pub.Publish(context.Background(), event)
		if !errors.Is(err, ErrPublisherClosed) {
			t.Fatalf("expected ErrPublisherClosed, got: %v", err)
		}
	})
}

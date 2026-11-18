package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/outbox"
	"github.com/rabbitmq/amqp091-go"
)

const (
	DefaultExchange        = "kotoro.domain-events"
	AnalyticsQueueName     = "kotoro.analytics.events"
	DefaultDemoBindingKey  = "study_cycle.finished.v1"
	DefaultWildcardBinding = "*.v*"
)

var (
	ErrBrokerNack        = errors.New("rabbitmq broker nacked the message")
	ErrMessageUnroutable = errors.New("message returned by broker as unroutable (no matching queue)")
	ErrChannelClosed     = errors.New("amqp channel closed")
	ErrPublisherClosed   = errors.New("publisher is closed")
)

// AMQPChannel defines the channel operations used by RabbitMQPublisher.
// *amqp091.Channel implements this interface directly.
type AMQPChannel interface {
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp091.Publishing) error
	NotifyPublish(confirm chan amqp091.Confirmation) chan amqp091.Confirmation
	NotifyReturn(returns chan amqp091.Return) chan amqp091.Return
	NotifyClose(c chan *amqp091.Error) chan *amqp091.Error
	Close() error
}

type RabbitMQPublisher struct {
	exchange string
	ch       AMQPChannel
	confirms chan amqp091.Confirmation
	returns  chan amqp091.Return
	closes   chan *amqp091.Error
	closed   bool
	mu       sync.Mutex
}

func NewPublisherFromChannel(ch AMQPChannel, exchange string) (*RabbitMQPublisher, error) {
	if ch == nil {
		return nil, errors.New("amqp channel is required")
	}
	if exchange == "" {
		exchange = DefaultExchange
	}

	confirms := ch.NotifyPublish(make(chan amqp091.Confirmation, 1))
	returns := ch.NotifyReturn(make(chan amqp091.Return, 1))
	closes := ch.NotifyClose(make(chan *amqp091.Error, 1))

	return &RabbitMQPublisher{
		exchange: exchange,
		ch:       ch,
		confirms: confirms,
		returns:  returns,
		closes:   closes,
	}, nil
}

// NewPublisher opens a new channel on the shared connection and configures publisher confirms.
// Each worker MUST receive its own publisher instance to avoid concurrent channel sharing.
func NewPublisher(conn *amqp091.Connection, exchange string) (*RabbitMQPublisher, error) {
	if conn == nil {
		return nil, errors.New("amqp connection is required")
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}

	return NewPublisherFromChannel(ch, exchange)
}

// SetupTopology declares the domain topic exchange and demonstration queue with bindings.
// This ensures mandatory=true publications to study_cycle.finished.v1 find a destination queue.
func SetupTopology(conn *amqp091.Connection, exchange string) error {
	if conn == nil {
		return errors.New("amqp connection is required")
	}
	if exchange == "" {
		exchange = DefaultExchange
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open setup channel: %w", err)
	}
	defer func() {
		_ = ch.Close()
	}()

	err = ch.ExchangeDeclare(
		exchange,
		amqp091.ExchangeTopic,
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare exchange %s: %w", exchange, err)
	}

	_, err = ch.QueueDeclare(
		AnalyticsQueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare queue %s: %w", AnalyticsQueueName, err)
	}

	for _, key := range []string{DefaultDemoBindingKey, DefaultWildcardBinding} {
		if err := ch.QueueBind(AnalyticsQueueName, key, exchange, false, nil); err != nil {
			return fmt.Errorf("bind queue %s to %s with key %s: %w", AnalyticsQueueName, exchange, key, err)
		}
	}

	return nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, event outbox.OutboxEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPublisherClosed
	}

	envelope, err := BuildEnvelope(event)
	if err != nil {
		return fmt.Errorf("build envelope: %w", err)
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	routingKey := BuildRoutingKey(event.EventType, event.EventVersion)

	msg := amqp091.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp091.Persistent,
		MessageId:    event.ID.String(),
		Type:         event.EventType,
		Timestamp:    envelope.OccurredAt,
		Body:         body,
	}

	err = p.ch.PublishWithContext(ctx, p.exchange, routingKey, true, false, msg)
	if err != nil {
		return fmt.Errorf("amqp publish: %w", err)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("publish timeout or cancelled: %w", ctx.Err())
	case ret, ok := <-p.returns:
		if !ok {
			return ErrChannelClosed
		}
		return fmt.Errorf("%w: replyCode=%d replyText=%s exchange=%s routingKey=%s",
			ErrMessageUnroutable, ret.ReplyCode, ret.ReplyText, ret.Exchange, ret.RoutingKey)
	case closeErr, ok := <-p.closes:
		if !ok {
			return ErrChannelClosed
		}
		return fmt.Errorf("%w: %v", ErrChannelClosed, closeErr)
	case confirm, ok := <-p.confirms:
		if !ok {
			return ErrChannelClosed
		}
		if !confirm.Ack {
			return ErrBrokerNack
		}
		// Check if basic.return arrived simultaneously
		select {
		case ret := <-p.returns:
			return fmt.Errorf("%w: replyCode=%d replyText=%s exchange=%s routingKey=%s",
				ErrMessageUnroutable, ret.ReplyCode, ret.ReplyText, ret.Exchange, ret.RoutingKey)
		default:
			return nil
		}
	}
}

func (p *RabbitMQPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	if p.ch != nil {
		return p.ch.Close()
	}
	return nil
}

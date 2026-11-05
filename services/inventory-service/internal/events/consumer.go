package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName                 = "ecommerce.events"
	DeadLetterExchangeName       = "ecommerce.events.dlx"
	DefaultQueueName             = "inventory.order-events"
	RoutingKeyOrderConfirmed     = "order.confirmed"
	RoutingKeyReservationRelease = "order.reservation.release_requested"
	maxDeliveryAttempts          = 5
)

// ErrPermanentMessage marks unparseable or invalid event payloads that must go
// straight to the dead-letter queue without retries.
var ErrPermanentMessage = errors.New("permanent event payload error")

// ReservationHandler defines the repository operations invoked by order events.
type ReservationHandler interface {
	ConfirmReservation(ctx context.Context, orderID string) (string, error)
	ReleaseReservation(ctx context.Context, orderID, reason string) (string, error)
}

type eventEnvelope struct {
	EventID       string    `json:"eventId"`
	EventType     string    `json:"eventType"`
	EventVersion  int       `json:"eventVersion"`
	OccurredAt    string    `json:"occurredAt"`
	AggregateID   string    `json:"aggregateId"`
	CorrelationID string    `json:"correlationId"`
	Data          eventData `json:"data"`
}

type eventData struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

// ConsumerOption configures a Consumer instance.
type ConsumerOption func(*Consumer)

// WithQueuePrefix overrides the default queue name (useful for isolated tests).
func WithQueuePrefix(queueName string) ConsumerOption {
	return func(c *Consumer) {
		if trimmed := strings.TrimSpace(queueName); trimmed != "" {
			c.queueName = trimmed
		}
	}
}

// Consumer subscribes to order lifecycle events from RabbitMQ and applies
// reservation confirmation or compensation idempotently.
type Consumer struct {
	rabbitURL string
	queueName string
	handler   ReservationHandler
	readyOnce sync.Once
	readyCh   chan struct{}
}

// NewConsumer creates an order-event Consumer backed by RabbitMQ.
func NewConsumer(rabbitURL string, handler ReservationHandler, opts ...ConsumerOption) (*Consumer, error) {
	if strings.TrimSpace(rabbitURL) == "" {
		return nil, errors.New("rabbitURL is required")
	}
	if handler == nil {
		return nil, errors.New("reservation handler is required")
	}
	c := &Consumer{
		rabbitURL: strings.TrimSpace(rabbitURL),
		queueName: DefaultQueueName,
		handler:   handler,
		readyCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c, nil
}

// DispatchMessage parses a raw RabbitMQ JSON envelope and routes it to handler.
func DispatchMessage(ctx context.Context, handler ReservationHandler, routingKey string, body []byte) error {
	var env eventEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("%w: decode envelope: %v", ErrPermanentMessage, err)
	}

	orderID := strings.TrimSpace(env.Data.OrderID)
	if orderID == "" {
		orderID = strings.TrimSpace(env.AggregateID)
	}
	if orderID == "" {
		return fmt.Errorf("%w: missing orderId", ErrPermanentMessage)
	}

	switch {
	case routingKey == RoutingKeyOrderConfirmed || env.EventType == "OrderConfirmed":
		_, err := handler.ConfirmReservation(ctx, orderID)
		return err
	case routingKey == RoutingKeyReservationRelease || env.EventType == "ReservationReleaseRequested":
		_, err := handler.ReleaseReservation(ctx, orderID, env.Data.Reason)
		return err
	default:
		return nil
	}
}

// Run maintains the AMQP subscription until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := c.consumeSession(ctx); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "inventory event consumer disconnected; retrying", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(1 * time.Second):
			}
		}
	}
}

func (c *Consumer) consumeSession(ctx context.Context) error {
	conn, err := amqp.Dial(c.rabbitURL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := declareTopology(ch, c.queueName); err != nil {
		return err
	}
	if err := ch.Qos(10, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := ch.Consume(c.queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %q: %w", c.queueName, err)
	}

	c.readyOnce.Do(func() {
		close(c.readyCh)
	})

	connClose := conn.NotifyClose(make(chan *amqp.Error, 1))
	for {
		select {
		case <-ctx.Done():
			return nil
		case closeErr := <-connClose:
			if closeErr != nil {
				return closeErr
			}
			return errors.New("amqp connection closed")
		case msg, ok := <-deliveries:
			if !ok {
				return errors.New("amqp deliveries channel closed")
			}
			c.handleDelivery(ctx, ch, msg)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, ch *amqp.Channel, msg amqp.Delivery) {
	err := DispatchMessage(ctx, c.handler, msg.RoutingKey, msg.Body)
	if err == nil {
		_ = msg.Ack(false)
		return
	}
	if errors.Is(err, ErrPermanentMessage) {
		slog.ErrorContext(ctx, "inventory permanent event failure; dead-lettering", "routing_key", msg.RoutingKey, "error", err)
		_ = msg.Nack(false, false)
		return
	}

	attempt := deliveryAttempt(msg.Headers) + 1
	if attempt >= maxDeliveryAttempts {
		slog.ErrorContext(ctx, "inventory event exceeded max attempts; dead-lettering", "attempt", attempt, "routing_key", msg.RoutingKey, "error", err)
		_ = msg.Nack(false, false)
		return
	}

	headers := cloneHeaders(msg.Headers)
	headers["x-attempt"] = int32(attempt)
	repubErr := ch.PublishWithContext(ctx, "", c.queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Type:         msg.Type,
		Body:         msg.Body,
	})
	if repubErr != nil {
		_ = msg.Nack(false, true)
		return
	}
	_ = msg.Ack(false)
}

// PublishForTest waits for consumer topology readiness and publishes a test event.
func (c *Consumer) PublishForTest(ctx context.Context, routingKey string, body []byte) error {
	select {
	case <-c.readyCh:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return errors.New("timed out waiting for consumer readiness")
	}

	conn, err := amqp.Dial(c.rabbitURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	return ch.PublishWithContext(ctx, ExchangeName, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func declareTopology(ch *amqp.Channel, queueName string) error {
	if err := ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %q: %w", ExchangeName, err)
	}
	if err := ch.ExchangeDeclare(DeadLetterExchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx %q: %w", DeadLetterExchangeName, err)
	}

	dlqName := queueName + ".dlq"
	if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlq %q: %w", dlqName, err)
	}
	if err := ch.QueueBind(dlqName, queueName, DeadLetterExchangeName, false, nil); err != nil {
		return fmt.Errorf("bind dlq %q: %w", dlqName, err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange":    DeadLetterExchangeName,
		"x-dead-letter-routing-key": queueName,
	}
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, args); err != nil {
		return fmt.Errorf("declare queue %q: %w", queueName, err)
	}
	for _, key := range []string{RoutingKeyOrderConfirmed, RoutingKeyReservationRelease} {
		if err := ch.QueueBind(queueName, key, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("bind queue %q to %q: %w", queueName, key, err)
		}
	}
	return nil
}

func deliveryAttempt(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	switch v := headers["x-attempt"].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	default:
		return 0
	}
}

func cloneHeaders(in amqp.Table) amqp.Table {
	out := amqp.Table{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

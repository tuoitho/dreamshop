package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const maxDeliveryAttempts = 5

type CompensationHandler interface {
	CancelPayment(ctx context.Context, orderID, reason string) (string, error)
	MarkRefundRequired(ctx context.Context, orderID, reason string) (string, error)
}

type eventEnvelope struct {
	EventID       string          `json:"eventId"`
	EventType     string          `json:"eventType"`
	AggregateID   string          `json:"aggregateId"`
	CorrelationID string          `json:"correlationId"`
	Data          json.RawMessage `json:"data"`
}

type compensationData struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

type Consumer struct {
	rabbitURL string
	handler   CompensationHandler
}

func NewConsumer(rabbitURL string, handler CompensationHandler) (*Consumer, error) {
	if strings.TrimSpace(rabbitURL) == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("compensation handler is required")
	}
	return &Consumer{
		rabbitURL: rabbitURL,
		handler:   handler,
	}, nil
}

func DispatchMessage(ctx context.Context, handler CompensationHandler, routingKey string, body []byte) error {
	var env eventEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("unmarshal compensation envelope: %w", err)
	}
	var payload compensationData
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			return fmt.Errorf("unmarshal compensation payload: %w", err)
		}
	}
	orderID := strings.TrimSpace(payload.OrderID)
	if orderID == "" {
		orderID = strings.TrimSpace(env.AggregateID)
	}
	if orderID == "" {
		return fmt.Errorf("compensation event missing orderId")
	}

	switch routingKey {
	case RoutingKeyPaymentCancelRequested:
		reason := strings.TrimSpace(payload.Reason)
		if reason == "" {
			reason = "ORDER_CANCELLED"
		}
		_, err := handler.CancelPayment(ctx, orderID, reason)
		return err
	case RoutingKeyRefundRequested:
		reason := strings.TrimSpace(payload.Reason)
		if reason == "" {
			reason = "LATE_PAYMENT_ON_CANCELLED_ORDER"
		}
		_, err := handler.MarkRefundRequired(ctx, orderID, reason)
		return err
	default:
		return nil
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := c.consumeSession(ctx); err != nil && ctx.Err() == nil {
			log.Printf("payment event consumer session ended: %v; reconnecting in 1s", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
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

	if err := ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if err := ch.ExchangeDeclare(DLXName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx: %w", err)
	}
	if _, err := ch.QueueDeclare(DLQName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlq: %w", err)
	}
	if err := ch.QueueBind(DLQName, QueueName, DLXName, false, nil); err != nil {
		return fmt.Errorf("bind dlq: %w", err)
	}

	qArgs := amqp.Table{
		"x-dead-letter-exchange":    DLXName,
		"x-dead-letter-routing-key": QueueName,
	}
	if _, err := ch.QueueDeclare(QueueName, true, false, false, false, qArgs); err != nil {
		return fmt.Errorf("declare queue %s: %w", QueueName, err)
	}
	for _, rk := range []string{RoutingKeyPaymentCancelRequested, RoutingKeyRefundRequested} {
		if err := ch.QueueBind(QueueName, rk, ExchangeName, false, nil); err != nil {
			return fmt.Errorf("bind queue %s -> %s: %w", QueueName, rk, err)
		}
	}

	if err := ch.Qos(10, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := ch.Consume(QueueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", QueueName, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			c.handleDelivery(ctx, ch, d)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, ch *amqp.Channel, d amqp.Delivery) {
	if err := DispatchMessage(ctx, c.handler, d.RoutingKey, d.Body); err == nil {
		_ = d.Ack(false)
		return
	} else {
		attempt := extractAttempt(d.Headers) + 1
		if attempt >= maxDeliveryAttempts {
			log.Printf("payment event exhausted %d attempts (rk=%s): %v; routing to DLQ", attempt, d.RoutingKey, err)
			_ = d.Nack(false, false)
			return
		}

		headers := amqp.Table{}
		for k, v := range d.Headers {
			headers[k] = v
		}
		headers["x-attempt"] = int32(attempt)

		pubErr := ch.PublishWithContext(ctx, ExchangeName, d.RoutingKey, false, false, amqp.Publishing{
			Headers:      headers,
			ContentType:  d.ContentType,
			DeliveryMode: amqp.Persistent,
			MessageId:    d.MessageId,
			Body:         d.Body,
		})
		if pubErr != nil {
			_ = d.Nack(false, true)
			return
		}
		_ = d.Ack(false)
	}
}

func extractAttempt(headers amqp.Table) int {
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

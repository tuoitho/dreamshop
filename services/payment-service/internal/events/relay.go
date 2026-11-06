package events

import (
	"context"
	"fmt"
	"log"
	"time"

	"ecommerce.local/payment-service/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName                     = "ecommerce.events"
	DLXName                          = "ecommerce.events.dlx"
	QueueName                        = "payment.order-events"
	DLQName                          = "payment.order-events.dlq"
	RoutingKeyPaymentCancelRequested = "order.payment.cancel_requested"
	RoutingKeyRefundRequested        = "order.payment.refund_requested"
	confirmTimeout                   = 3 * time.Second
	outboxBatchSize                  = 50
)

type OutboxStore interface {
	ClaimPendingOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkOutboxPublished(ctx context.Context, id string) error
	RecordOutboxFailure(ctx context.Context, id, errMsg string, backoff time.Duration) error
}

type OutboxRelay struct {
	store        OutboxStore
	rabbitURL    string
	pollInterval time.Duration
	wakeupCh     chan struct{}
}

func NewOutboxRelay(store OutboxStore, rabbitURL string, pollInterval time.Duration) *OutboxRelay {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	return &OutboxRelay{
		store:        store,
		rabbitURL:    rabbitURL,
		pollInterval: pollInterval,
		wakeupCh:     make(chan struct{}, 1),
	}
}

func (r *OutboxRelay) Notify() {
	if r == nil {
		return
	}
	select {
	case r.wakeupCh <- struct{}{}:
	default:
	}
}

func (r *OutboxRelay) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := r.runConnectedSession(ctx); err != nil && ctx.Err() == nil {
			log.Printf("payment outbox relay session ended: %v; reconnecting in 1s", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
		}
	}
}

func (r *OutboxRelay) runConnectedSession(ctx context.Context) error {
	conn, err := amqp.Dial(r.rabbitURL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("enable publisher confirms: %w", err)
	}
	if err := ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", ExchangeName, err)
	}

	closeCh := ch.NotifyClose(make(chan *amqp.Error, 1))
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	if err := r.flushPending(ctx, ch); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case amqpErr := <-closeCh:
			if amqpErr != nil {
				return fmt.Errorf("amqp channel closed: %w", amqpErr)
			}
			return fmt.Errorf("amqp channel closed")
		case <-r.wakeupCh:
			if err := r.flushPending(ctx, ch); err != nil {
				return err
			}
		case <-ticker.C:
			if err := r.flushPending(ctx, ch); err != nil {
				return err
			}
		}
	}
}

func (r *OutboxRelay) flushPending(ctx context.Context, ch *amqp.Channel) error {
	for {
		batch, err := r.store.ClaimPendingOutbox(ctx, outboxBatchSize)
		if err != nil {
			return fmt.Errorf("claim outbox batch: %w", err)
		}
		if len(batch) == 0 {
			return nil
		}

		for _, ev := range batch {
			pubCtx, cancel := context.WithTimeout(ctx, confirmTimeout)
			confirm, pubErr := ch.PublishWithDeferredConfirmWithContext(
				pubCtx,
				ExchangeName,
				ev.RoutingKey,
				false,
				false,
				amqp.Publishing{
					ContentType:  "application/json",
					DeliveryMode: amqp.Persistent,
					MessageId:    ev.ID,
					Type:         ev.EventType,
					Timestamp:    time.Now().UTC(),
					Body:         ev.Payload,
				},
			)
			if pubErr == nil {
				acked, waitErr := confirm.WaitContext(pubCtx)
				if waitErr != nil {
					pubErr = waitErr
				} else if !acked {
					pubErr = fmt.Errorf("broker nacked outbox event %s", ev.ID)
				}
			}
			cancel()

			if pubErr != nil {
				_ = r.store.RecordOutboxFailure(ctx, ev.ID, pubErr.Error(), 2*time.Second)
				return fmt.Errorf("publish outbox event %s: %w", ev.ID, pubErr)
			}

			if err := r.store.MarkOutboxPublished(ctx, ev.ID); err != nil {
				return err
			}
		}

		if len(batch) < outboxBatchSize {
			return nil
		}
	}
}

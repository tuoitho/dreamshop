package events

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"ecommerce.local/payment-service/internal/domain"
	"ecommerce.local/payment-service/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

type fakeCompensationHandler struct {
	mu        sync.Mutex
	cancelled map[string]string
	refunded  map[string]string
	notifyCh  chan struct{}
}

func (f *fakeCompensationHandler) CancelPayment(_ context.Context, orderID, reason string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancelled == nil {
		f.cancelled = make(map[string]string)
	}
	f.cancelled[orderID] = reason
	if f.notifyCh != nil {
		select {
		case f.notifyCh <- struct{}{}:
		default:
		}
	}
	return "CANCELLED", nil
}

func (f *fakeCompensationHandler) RequestRefund(_ context.Context, orderID, reason string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refunded == nil {
		f.refunded = make(map[string]string)
	}
	f.refunded[orderID] = reason
	if f.notifyCh != nil {
		select {
		case f.notifyCh <- struct{}{}:
		default:
		}
	}
	return true, nil
}

func TestDispatchCompensationRoutesCancelAndRefundEvents(t *testing.T) {
	h := &fakeCompensationHandler{}
	cancelBody, _ := json.Marshal(map[string]any{
		"eventId":     "evt-pay-cancel-1",
		"eventType":   "PaymentCancelRequested",
		"aggregateId": "ord-c-1",
		"data": map[string]any{
			"orderId": "ord-c-1",
			"reason":  "USER_CANCELLED",
		},
	})
	if err := DispatchMessage(context.Background(), h, RoutingKeyPaymentCancelRequested, cancelBody); err != nil {
		t.Fatalf("DispatchMessage(cancel): %v", err)
	}

	refundBody, _ := json.Marshal(map[string]any{
		"eventId":     "evt-pay-refund-1",
		"eventType":   "RefundRequested",
		"aggregateId": "ord-r-1",
		"data": map[string]any{
			"orderId": "ord-r-1",
			"reason":  "LATE_MOMO_SUCCESS",
		},
	})
	if err := DispatchMessage(context.Background(), h, RoutingKeyRefundRequested, refundBody); err != nil {
		t.Fatalf("DispatchMessage(refund): %v", err)
	}

	if h.cancelled["ord-c-1"] != "USER_CANCELLED" || h.refunded["ord-r-1"] != "LATE_MOMO_SUCCESS" {
		t.Fatalf("unexpected compensation state: cancelled=%v refunded=%v", h.cancelled, h.refunded)
	}
}

type fakeRefundRequestStore struct {
	queued      bool
	lastOrderID string
	lastReason  string
	lastMaxAuto int
}

func (f *fakeRefundRequestStore) CancelPayment(_ context.Context, _, _ string) (string, error) {
	return "CANCELLED", nil
}

func (f *fakeRefundRequestStore) RequestRefund(_ context.Context, orderID, reason string, maxAutoAttempts int) (bool, error) {
	f.lastOrderID = orderID
	f.lastReason = reason
	f.lastMaxAuto = maxAutoAttempts
	return f.queued, nil
}

func TestCompensationAdapterWakesRefundWorkerWhenQueued(t *testing.T) {
	st := &fakeRefundRequestStore{queued: true}
	notifier := &fakeOutboxNotifier{}
	adapter := NewCompensationAdapter(st, 3, notifier)

	body, _ := json.Marshal(map[string]any{
		"eventId":     "evt-ref-wake-1",
		"eventType":   "RefundRequested",
		"aggregateId": "ord-wake-1",
		"data": map[string]any{
			"orderId": "ord-wake-1",
			"reason":  "USER_CANCELLED",
		},
	})
	if err := DispatchMessage(context.Background(), adapter, RoutingKeyRefundRequested, body); err != nil {
		t.Fatalf("DispatchMessage: %v", err)
	}
	if st.lastOrderID != "ord-wake-1" || st.lastReason != "USER_CANCELLED" || st.lastMaxAuto != 3 {
		t.Fatalf("unexpected RequestRefund args: %+v", st)
	}
	if notifier.count != 1 {
		t.Fatalf("expected refund notifier to wake once when queued=true, got %d", notifier.count)
	}
}

func TestOutboxRelayAndConsumerLiveIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if databaseURL == "" || rabbitURL == "" {
		t.Skip("DATABASE_URL or RABBITMQ_URL not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	defer adminPool.Close()

	schemaName := "payment_outbox_live_test"
	_, _ = adminPool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	}()

	cfg, _ := pgxpool.ParseConfig(databaseURL)
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	defer pool.Close()

	if err := store.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := store.New(pool)

	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		t.Fatalf("dial amqp: %v", err)
	}
	defer func() { _ = conn.Close() }()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("open amqp channel: %v", err)
	}
	defer func() { _ = ch.Close() }()
	_ = ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil)
	tmpQ, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatalf("declare temp queue: %v", err)
	}
	if err := ch.QueueBind(tmpQ.Name, "payment.succeeded", ExchangeName, false, nil); err != nil {
		t.Fatalf("bind temp queue: %v", err)
	}
	deliveries, err := ch.Consume(tmpQ.Name, "", true, true, false, false, nil)
	if err != nil {
		t.Fatalf("consume temp queue: %v", err)
	}

	relay := NewOutboxRelay(repo, rabbitURL, 5*time.Second)
	compConsumer, err := NewConsumer(rabbitURL, NewCompensationAdapter(repo, 3, nil))
	if err != nil {
		t.Fatalf("NewConsumer: %v", err)
	}
	go func() { _ = relay.Run(ctx) }()
	go func() { _ = compConsumer.Run(ctx) }()

	expires := time.Now().UTC().Add(15 * time.Minute)
	if _, err := repo.BeginCreateSession(ctx, domain.CreateSessionInput{
		OrderID: "ord-relay-1", OrderCode: "ORD-RELAY-1", UserID: "user-1",
		AmountMinor: 99000, Currency: "VND", MomoRequestID: "req-relay-1", ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("BeginCreateSession: %v", err)
	}
	if _, err := repo.CompleteCreateSession(ctx, "ord-relay-1", "https://momo.test/pay/relay", 0, "OK", nil); err != nil {
		t.Fatalf("CompleteCreateSession: %v", err)
	}
	if _, err := repo.RecordMomoCallback(ctx, domain.CallbackInput{
		MomoOrderID: "ord-relay-1", MomoTransID: "trans-relay-1",
		AmountMinor: 99000, ResultCode: 0, ResultMessage: "Successful.",
	}); err != nil {
		t.Fatalf("RecordMomoCallback: %v", err)
	}
	relay.Notify()

	select {
	case msg := <-deliveries:
		var parsed map[string]any
		if err := json.Unmarshal(msg.Body, &parsed); err != nil {
			t.Fatalf("unmarshal published event: %v", err)
		}
		if parsed["eventType"] != "PaymentSucceeded" || parsed["aggregateId"] != "ord-relay-1" {
			t.Fatalf("unexpected published event: %+v", parsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OutboxRelay Notify() publication")
	}
}

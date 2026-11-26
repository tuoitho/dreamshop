package events

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"ecommerce.local/cart-service/internal/domain"
)

type fakeCartCleanup struct {
	mu      sync.Mutex
	eventID string
	userID  string
	items   []domain.OrderedItemRef
	doneCh  chan struct{}
}

func (f *fakeCartCleanup) RemoveOrderedItems(_ context.Context, eventID, userID string, items []domain.OrderedItemRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventID = eventID
	f.userID = userID
	f.items = append([]domain.OrderedItemRef(nil), items...)
	if f.doneCh != nil {
		select {
		case f.doneCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func TestDispatchMessageExtractsOrderedItemRefs(t *testing.T) {
	cleanup := &fakeCartCleanup{}
	payload, err := json.Marshal(map[string]any{
		"eventId":      "evt-cart-1",
		"eventType":    "OrderConfirmed",
		"eventVersion": 1,
		"aggregateId":  "ord-1",
		"data": map[string]any{
			"orderId": "ord-1",
			"userId":  "00000000-0000-0000-0000-000000000001",
			"items": []map[string]any{
				{"cartItemId": "11111111-1111-1111-1111-111111111111", "productId": "dt-1", "skuId": "dt-1-v1", "quantity": 2},
				{"cartItemId": "", "productId": "dt-2", "skuId": "dt-2-v1", "quantity": 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if err := DispatchMessage(context.Background(), cleanup, RoutingKeyOrderConfirmed, payload); err != nil {
		t.Fatalf("DispatchMessage: %v", err)
	}
	if cleanup.eventID != "order-cleanup:ord-1" || cleanup.userID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected cleanup metadata: eventID=%q userID=%q", cleanup.eventID, cleanup.userID)
	}
	if len(cleanup.items) != 2 || cleanup.items[0].SkuID != "dt-1-v1" || cleanup.items[1].SkuID != "dt-2-v1" {
		t.Fatalf("unexpected cleanup items: %+v", cleanup.items)
	}
}

func TestDispatchMessageHandlesOrderPlaced(t *testing.T) {
	cleanup := &fakeCartCleanup{}
	payload := []byte(`{"eventId":"evt-placed-1","eventType":"OrderPlaced","eventVersion":1,"aggregateId":"ord-2",
		"data":{"orderId":"ord-2","userId":"00000000-0000-0000-0000-000000000002","items":[
			{"cartItemId":"22222222-2222-2222-2222-222222222222","productId":"dt-1","skuId":"dt-1-v1","quantity":1},
			{"cartItemId":"33333333-3333-3333-3333-333333333333","productId":"dt-3","skuId":"dt-3-v2","quantity":2}]}}`)

	if err := DispatchMessage(context.Background(), cleanup, "order.placed", payload); err != nil {
		t.Fatalf("DispatchMessage(order.placed): %v", err)
	}
	if cleanup.eventID != "order-cleanup:ord-2" || cleanup.userID != "00000000-0000-0000-0000-000000000002" {
		t.Fatalf("unexpected cleanup metadata: eventID=%q userID=%q", cleanup.eventID, cleanup.userID)
	}
	if len(cleanup.items) != 2 || cleanup.items[1].CartItemID != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("unexpected cleanup items: %+v", cleanup.items)
	}
}

func TestOrderPlacedAndOrderConfirmedShareOneCleanupKeyPerOrder(t *testing.T) {
	placed := &fakeCartCleanup{}
	confirmed := &fakeCartCleanup{}
	placedBody := []byte(`{"eventId":"evt-a","eventType":"OrderPlaced","data":{"orderId":"ord-9","userId":"00000000-0000-0000-0000-000000000009","items":[]}}`)
	confirmedBody := []byte(`{"eventId":"evt-b","eventType":"OrderConfirmed","data":{"orderId":"ord-9","userId":"00000000-0000-0000-0000-000000000009","items":[]}}`)

	if err := DispatchMessage(context.Background(), placed, "order.placed", placedBody); err != nil {
		t.Fatalf("DispatchMessage(order.placed): %v", err)
	}
	if err := DispatchMessage(context.Background(), confirmed, RoutingKeyOrderConfirmed, confirmedBody); err != nil {
		t.Fatalf("DispatchMessage(order.confirmed): %v", err)
	}
	if placed.eventID == "" || placed.eventID != confirmed.eventID {
		t.Fatalf("both events of one order must use the same cleanup key, got placed=%q confirmed=%q", placed.eventID, confirmed.eventID)
	}
}

func TestDispatchMessageIgnoresUnrelatedRoutingKey(t *testing.T) {
	cleanup := &fakeCartCleanup{}
	payload := []byte(`{"eventId":"evt-other-1","eventType":"PaymentCancelRequested","data":{"orderId":"ord-3","reason":"X"}}`)

	if err := DispatchMessage(context.Background(), cleanup, "order.payment.cancel_requested", payload); err != nil {
		t.Fatalf("DispatchMessage(unrelated): %v", err)
	}
	if cleanup.eventID != "" {
		t.Fatalf("unrelated event must not trigger cleanup, got eventID=%q", cleanup.eventID)
	}
}

func TestCartConsumerLiveRabbitMQRoundTrip(t *testing.T) {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		t.Skip("RABBITMQ_URL not set")
	}

	doneCh := make(chan struct{}, 1)
	cleanup := &fakeCartCleanup{doneCh: doneCh}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer, err := NewConsumer(rabbitURL, cleanup, WithQueuePrefix("cart.test."+time.Now().UTC().Format("150405.000000")))
	if err != nil {
		t.Fatalf("NewConsumer: %v", err)
	}
	go func() { _ = consumer.Run(ctx) }()

	body, _ := json.Marshal(map[string]any{
		"eventId":     "evt-live-cart",
		"eventType":   "OrderConfirmed",
		"aggregateId": "ord-live-cart",
		"data": map[string]any{
			"orderId": "ord-live-cart",
			"userId":  "00000000-0000-0000-0000-000000000001",
			"items": []map[string]any{
				{"cartItemId": "", "productId": "dt-1", "skuId": "dt-1-v1", "quantity": 1},
			},
		},
	})
	if err := consumer.PublishForTest(ctx, RoutingKeyOrderConfirmed, body); err != nil {
		t.Fatalf("PublishForTest: %v", err)
	}

	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cart consumer delivery")
	}
	cancel()
}

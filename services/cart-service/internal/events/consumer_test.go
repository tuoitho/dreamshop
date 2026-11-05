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
	if cleanup.eventID != "evt-cart-1" || cleanup.userID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected cleanup metadata: eventID=%q userID=%q", cleanup.eventID, cleanup.userID)
	}
	if len(cleanup.items) != 2 || cleanup.items[0].SkuID != "dt-1-v1" || cleanup.items[1].SkuID != "dt-2-v1" {
		t.Fatalf("unexpected cleanup items: %+v", cleanup.items)
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

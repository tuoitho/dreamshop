package events

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

type fakeReservationHandler struct {
	mu          sync.Mutex
	confirmed   []string
	released    map[string]string
	restocked   map[string]string
	confirmErr  error
	releaseErr  error
	doneNotifyC chan struct{}
}

func (f *fakeReservationHandler) ConfirmReservation(_ context.Context, orderID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.confirmErr != nil {
		return "", f.confirmErr
	}
	f.confirmed = append(f.confirmed, orderID)
	if f.doneNotifyC != nil {
		select {
		case f.doneNotifyC <- struct{}{}:
		default:
		}
	}
	return "CONFIRMED", nil
}

func (f *fakeReservationHandler) ReleaseReservation(_ context.Context, orderID, reason string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.releaseErr != nil {
		return "", f.releaseErr
	}
	if f.released == nil {
		f.released = make(map[string]string)
	}
	f.released[orderID] = reason
	if f.doneNotifyC != nil {
		select {
		case f.doneNotifyC <- struct{}{}:
		default:
		}
	}
	return "RELEASED", nil
}

func (f *fakeReservationHandler) RestockReservation(_ context.Context, orderID, reason string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.restocked == nil {
		f.restocked = make(map[string]string)
	}
	f.restocked[orderID] = reason
	return "RESTOCKED", nil
}

func TestDispatchMessageRoutesStockRestockRequested(t *testing.T) {
	handler := &fakeReservationHandler{}
	body := []byte(`{"eventId":"e1","eventType":"StockRestockRequested","data":{"orderId":"o-1","reason":"ADMIN_CANCELLED"}}`)

	if err := DispatchMessage(context.Background(), handler, "order.stock.restock_requested", body); err != nil {
		t.Fatalf("DispatchMessage(order.stock.restock_requested): %v", err)
	}

	if got := handler.restocked["o-1"]; got != "ADMIN_CANCELLED" {
		t.Fatalf("restocked[o-1] = %q, want ADMIN_CANCELLED (restocked=%v)", got, handler.restocked)
	}
	if len(handler.confirmed) != 0 || len(handler.released) != 0 {
		t.Fatalf("restock must not confirm or release: confirmed=%v released=%v", handler.confirmed, handler.released)
	}
}

func TestHandleMessageRoutesOrderConfirmedAndReleaseRequested(t *testing.T) {
	handler := &fakeReservationHandler{}

	confirmPayload, err := json.Marshal(map[string]any{
		"eventId":       "evt-conf-1",
		"eventType":     "OrderConfirmed",
		"eventVersion":  1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339),
		"aggregateId":   "ord-100",
		"correlationId": "trace-1",
		"data": map[string]any{
			"orderId": "ord-100",
		},
	})
	if err != nil {
		t.Fatalf("marshal confirmPayload: %v", err)
	}
	if err := DispatchMessage(context.Background(), handler, RoutingKeyOrderConfirmed, confirmPayload); err != nil {
		t.Fatalf("DispatchMessage(order.confirmed): %v", err)
	}

	releasePayload, err := json.Marshal(map[string]any{
		"eventId":       "evt-rel-1",
		"eventType":     "ReservationReleaseRequested",
		"eventVersion":  1,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339),
		"aggregateId":   "ord-200",
		"correlationId": "trace-2",
		"data": map[string]any{
			"orderId": "ord-200",
			"reason":  "PAYMENT_EXPIRED",
		},
	})
	if err != nil {
		t.Fatalf("marshal releasePayload: %v", err)
	}
	if err := DispatchMessage(context.Background(), handler, RoutingKeyReservationRelease, releasePayload); err != nil {
		t.Fatalf("DispatchMessage(order.reservation.release_requested): %v", err)
	}

	if len(handler.confirmed) != 1 || handler.confirmed[0] != "ord-100" {
		t.Fatalf("confirmed = %v, want [ord-100]", handler.confirmed)
	}
	if handler.released["ord-200"] != "PAYMENT_EXPIRED" {
		t.Fatalf("released = %v, want ord-200 -> PAYMENT_EXPIRED", handler.released)
	}
}

func TestHandleMessageRejectsMalformedJSONAsPermanentError(t *testing.T) {
	handler := &fakeReservationHandler{}
	err := DispatchMessage(context.Background(), handler, RoutingKeyOrderConfirmed, []byte("{broken"))
	if err == nil || !errors.Is(err, ErrPermanentMessage) {
		t.Fatalf("expected ErrPermanentMessage on malformed json, got %v", err)
	}
}

func TestConsumerLiveRabbitMQRoundTrip(t *testing.T) {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		t.Skip("RABBITMQ_URL not set")
	}

	notify := make(chan struct{}, 2)
	handler := &fakeReservationHandler{doneNotifyC: notify}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumer, err := NewConsumer(rabbitURL, handler, WithQueuePrefix("inventory.test."+time.Now().UTC().Format("150405.000000")))
	if err != nil {
		t.Fatalf("NewConsumer: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- consumer.Run(ctx)
	}()

	body, _ := json.Marshal(map[string]any{
		"eventId":     "evt-live-1",
		"eventType":   "ReservationReleaseRequested",
		"aggregateId": "ord-live-1",
		"data": map[string]any{
			"orderId": "ord-live-1",
			"reason":  "SAGA_TIMEOUT",
		},
	})
	if err := consumer.PublishForTest(ctx, RoutingKeyReservationRelease, body); err != nil {
		t.Fatalf("PublishForTest: %v", err)
	}

	select {
	case <-notify:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consumer delivery")
	}

	handler.mu.Lock()
	gotReason := handler.released["ord-live-1"]
	handler.mu.Unlock()
	if gotReason != "SAGA_TIMEOUT" {
		t.Fatalf("gotReason = %q, want SAGA_TIMEOUT", gotReason)
	}
	cancel()
}

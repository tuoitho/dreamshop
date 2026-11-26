package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"ecommerce.local/catalog-service/internal/domain"
)

type spyDeliveredHandler struct {
	calls int
	last  struct {
		eventID      string
		orderID      string
		orderCode    string
		userID       string
		customerName string
		deliveredAt  time.Time
		items        []domain.DeliveredOrderItem
	}
}

func (s *spyDeliveredHandler) RecordDeliveredOrder(
	_ context.Context,
	eventID, orderID, orderCode, userID, customerName string,
	deliveredAt time.Time,
	items []domain.DeliveredOrderItem,
) error {
	s.calls++
	s.last.eventID = eventID
	s.last.orderID = orderID
	s.last.orderCode = orderCode
	s.last.userID = userID
	s.last.customerName = customerName
	s.last.deliveredAt = deliveredAt
	s.last.items = items
	return nil
}

func TestDispatchMessageRecordsDeliveredOrder(t *testing.T) {
	handler := &spyDeliveredHandler{}
	payload := []byte(`{
		"eventId": "evt-del-1",
		"eventType": "OrderDelivered",
		"eventVersion": 1,
		"occurredAt": "2026-11-26T04:00:00Z",
		"data": {
			"orderId": "ord-123",
			"orderCode": "ORD-20261126-001",
			"userId": "user-1",
			"recipientName": "Nguyen Van A",
			"deliveredAt": "2026-11-26T04:00:05Z",
			"items": [
				{"productId": "dt-1", "skuId": "dt-1-v1", "variantLabel": "256GB", "quantity": 1}
			]
		}
	}`)

	if err := DispatchMessage(context.Background(), handler, RoutingKeyOrderDelivered, payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.calls != 1 {
		t.Fatalf("expected 1 call, got %d", handler.calls)
	}
	if handler.last.orderID != "ord-123" || handler.last.userID != "user-1" || handler.last.customerName != "Nguyen Van A" {
		t.Fatalf("unexpected decoded fields: %+v", handler.last)
	}
	if len(handler.last.items) != 1 || handler.last.items[0].ProductID != "dt-1" || handler.last.items[0].VariantLabel != "256GB" {
		t.Fatalf("unexpected items: %+v", handler.last.items)
	}
}

func TestDispatchMessageIgnoresUnrelatedAndRejectsMalformed(t *testing.T) {
	handler := &spyDeliveredHandler{}

	if err := DispatchMessage(context.Background(), handler, "order.placed", []byte(`{"eventId":"e1","eventType":"OrderPlaced"}`)); err != nil {
		t.Fatalf("expected unrelated event to be ignored, got %v", err)
	}
	if handler.calls != 0 {
		t.Fatalf("expected 0 calls for unrelated event, got %d", handler.calls)
	}

	err := DispatchMessage(context.Background(), handler, RoutingKeyOrderDelivered, []byte(`{"eventId":"e2","eventType":"OrderDelivered","data":{"orderId":""}}`))
	if !errors.Is(err, ErrPermanentMessage) {
		t.Fatalf("expected ErrPermanentMessage for missing orderId/userId, got %v", err)
	}
}

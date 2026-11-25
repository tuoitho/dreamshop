package events

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ecommerce.local/payment-service/internal/domain"
	"ecommerce.local/payment-service/internal/momo"
)

type fakeRefundStore struct {
	mu               sync.Mutex
	recoverCalls     int
	recoverOutbox    bool
	dueJobs          [][]domain.RefundJob
	completedResults []domain.RefundAttemptResult
	completeOutbox   []bool
}

func (f *fakeRefundStore) RecoverStaleRefunds(_ context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recoverCalls++
	return f.recoverOutbox, nil
}

func (f *fakeRefundStore) ClaimDueRefunds(_ context.Context, _ int) ([]domain.RefundJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.dueJobs) == 0 {
		return nil, nil
	}
	next := f.dueJobs[0]
	f.dueJobs = f.dueJobs[1:]
	return next, nil
}

func (f *fakeRefundStore) CompleteAutoRefundAttempt(_ context.Context, _ string, res domain.RefundAttemptResult, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completedResults = append(f.completedResults, res)
	outbox := false
	if len(f.completeOutbox) > 0 {
		outbox = f.completeOutbox[0]
		f.completeOutbox = f.completeOutbox[1:]
	}
	return outbox, nil
}

type fakeRefundGateway struct {
	mu       sync.Mutex
	requests []momo.RefundRequest
	outcomes []struct {
		resp *momo.RefundResponse
		err  error
	}
}

func (g *fakeRefundGateway) Refund(_ context.Context, req momo.RefundRequest) (*momo.RefundResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests = append(g.requests, req)
	if len(g.outcomes) == 0 {
		return &momo.RefundResponse{ResultCode: 0, Message: "OK", RefundTransID: "ref-default"}, nil
	}
	next := g.outcomes[0]
	g.outcomes = g.outcomes[1:]
	return next.resp, next.err
}

type fakeOutboxNotifier struct {
	mu    sync.Mutex
	count int
}

func (n *fakeOutboxNotifier) Notify() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.count++
}

func TestRefundWorkerExecutes3AttemptsThenEmitsFailedAndTriggersOutbox(t *testing.T) {
	job := domain.RefundJob{
		OrderID:         "ord-w-1",
		OrderCode:       "ORD-W-1",
		MomoOrderID:     "ord-w-1",
		MomoTransID:     "12345678",
		RefundOrderID:   "REFUNDW1",
		RefundRequestID: "REFREQW1",
		Description:     "Hoan tien don hang ORD-W-1",
		AmountMinor:     150000,
		Currency:        "VND",
		Attempts:        1,
		MaxAutoAttempts: 3,
	}

	st := &fakeRefundStore{
		dueJobs: [][]domain.RefundJob{
			{job},
			{job},
			{job},
		},
		completeOutbox: []bool{false, false, true},
	}
	gw := &fakeRefundGateway{
		outcomes: []struct {
			resp *momo.RefundResponse
			err  error
		}{
			{err: errors.New("network timeout")}, // ambiguous -> RotateIDsOnNextAttempt=false
			{err: &momo.GatewayRejectedError{ResultCode: 1080, Message: "Rejected"}}, // definite -> RotateIDsOnNextAttempt=true
			{err: &momo.GatewayRejectedError{ResultCode: 7000, Message: "Processing"}}, // ambiguous -> RotateIDsOnNextAttempt=false
		},
	}
	notifier := &fakeOutboxNotifier{}

	worker := NewRefundWorker(st, gw, notifier, 50*time.Millisecond, 10*time.Millisecond, time.Second)
	for range 3 {
		if err := worker.processOnce(context.Background()); err != nil {
			t.Fatalf("processOnce: %v", err)
		}
	}

	if st.recoverCalls != 3 {
		t.Fatalf("expected 3 RecoverStaleRefunds calls, got %d", st.recoverCalls)
	}
	if len(gw.requests) != 3 {
		t.Fatalf("expected 3 gateway Refund calls, got %d", len(gw.requests))
	}
	for i, req := range gw.requests {
		if req.Description != "Hoan tien don hang ORD-W-1" {
			t.Fatalf("call %d used mutated description %q", i+1, req.Description)
		}
	}
	if len(st.completedResults) != 3 {
		t.Fatalf("expected 3 completed results, got %d", len(st.completedResults))
	}
	if st.completedResults[0].RotateIDsOnNextAttempt {
		t.Fatal("attempt 1 ambiguous error must not rotate IDs")
	}
	if !st.completedResults[1].RotateIDsOnNextAttempt {
		t.Fatal("attempt 2 definite rejection must rotate IDs")
	}
	if st.completedResults[2].RotateIDsOnNextAttempt {
		t.Fatal("attempt 3 code 7000 ambiguous error must not rotate IDs")
	}
	if notifier.count != 1 {
		t.Fatalf("expected outbox notifier to fire once when outboxInserted=true, got %d", notifier.count)
	}
}

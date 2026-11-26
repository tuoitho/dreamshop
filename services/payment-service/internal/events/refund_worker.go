package events

import (
	"context"
	"errors"
	"log"
	"time"

	"ecommerce.local/payment-service/internal/domain"
	"ecommerce.local/payment-service/internal/momo"
)

const refundBatchSize = 10

type RefundStore interface {
	RecoverStaleRefunds(ctx context.Context) (bool, error)
	ClaimDueRefunds(ctx context.Context, limit int) ([]domain.RefundJob, error)
	CompleteAutoRefundAttempt(ctx context.Context, orderID string, res domain.RefundAttemptResult, retryBackoffBase time.Duration) (bool, error)
}

type RefundGateway interface {
	Refund(ctx context.Context, req momo.RefundRequest) (*momo.RefundResponse, error)
}

type OutboxNotifier interface {
	Notify()
}

type RefundWorker struct {
	store            RefundStore
	momoClient       RefundGateway
	outboxNotifier   OutboxNotifier
	pollInterval     time.Duration
	retryBackoffBase time.Duration
	callTimeout      time.Duration
	wakeupCh         chan struct{}
}

func NewRefundWorker(
	store RefundStore,
	momoClient RefundGateway,
	outboxNotifier OutboxNotifier,
	pollInterval, retryBackoffBase, callTimeout time.Duration,
) *RefundWorker {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if retryBackoffBase <= 0 {
		retryBackoffBase = 10 * time.Second
	}
	if callTimeout <= 0 {
		callTimeout = 15 * time.Second
	}
	return &RefundWorker{
		store:            store,
		momoClient:       momoClient,
		outboxNotifier:   outboxNotifier,
		pollInterval:     pollInterval,
		retryBackoffBase: retryBackoffBase,
		callTimeout:      callTimeout,
		wakeupCh:         make(chan struct{}, 1),
	}
}

func (w *RefundWorker) Notify() {
	if w == nil {
		return
	}
	select {
	case w.wakeupCh <- struct{}{}:
	default:
	}
}

func (w *RefundWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.processOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("payment refund worker initial cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.wakeupCh:
			if err := w.processOnce(ctx); err != nil && ctx.Err() == nil {
				log.Printf("payment refund worker wakeup cycle failed: %v", err)
			}
		case <-ticker.C:
			if err := w.processOnce(ctx); err != nil && ctx.Err() == nil {
				log.Printf("payment refund worker poll cycle failed: %v", err)
			}
		}
	}
}

func (w *RefundWorker) processOnce(ctx context.Context) error {
	recoveredOutbox, err := w.store.RecoverStaleRefunds(ctx)
	if err != nil {
		return err
	}
	if recoveredOutbox && w.outboxNotifier != nil {
		w.outboxNotifier.Notify()
	}

	jobs, err := w.store.ClaimDueRefunds(ctx, refundBatchSize)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		w.executeJob(ctx, job)
	}
	return nil
}

func (w *RefundWorker) executeJob(ctx context.Context, job domain.RefundJob) {
	callCtx, cancel := context.WithTimeout(ctx, w.callTimeout)
	resp, callErr := w.momoClient.Refund(callCtx, momo.RefundRequest{
		OrderID:     job.RefundOrderID,
		RequestID:   job.RefundRequestID,
		OrigTransID: job.MomoTransID,
		AmountMinor: job.AmountMinor,
		Description: job.Description,
	})
	cancel()

	var attemptRes domain.RefundAttemptResult
	if callErr == nil {
		attemptRes = domain.RefundAttemptResult{
			Succeeded:     true,
			RefundTransID: resp.RefundTransID,
			ResultCode:    resp.ResultCode,
			ResultMessage: resp.Message,
		}
	} else {
		attemptRes = domain.RefundAttemptResult{
			Succeeded:              false,
			ResultCode:             -1,
			LastError:              callErr.Error(),
			RotateIDsOnNextAttempt: !momo.IsAmbiguousRefundError(callErr),
		}
		var rej *momo.GatewayRejectedError
		if errors.As(callErr, &rej) {
			attemptRes.ResultCode = rej.ResultCode
			attemptRes.ResultMessage = rej.Message
		}
	}

	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	outboxInserted, persistErr := w.store.CompleteAutoRefundAttempt(persistCtx, job.OrderID, attemptRes, w.retryBackoffBase)
	persistCancel()
	if persistErr != nil {
		log.Printf("payment refund worker failed to persist attempt for order %s: %v", job.OrderID, persistErr)
		return
	}
	if outboxInserted && w.outboxNotifier != nil {
		w.outboxNotifier.Notify()
	}
}

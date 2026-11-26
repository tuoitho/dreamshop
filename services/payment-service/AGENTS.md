# Payment Service Instructions

These instructions apply to `services/payment-service/` in addition to the root
[`AGENTS.md`](../../AGENTS.md).

## Ownership

This service owns:

- MoMo v2 payment session creation, full-order refund execution (`POST /v2/gateway/api/refund`), `HmacSHA256` request signing, and constant-time callback signature verification (`hmac.Equal`).
- Payment lifecycle state (`INITIATING`, `PENDING`, `SUCCEEDED`, `FAILED`, `CANCELLED`, `REFUND_REQUIRED`, `REFUNDED`, `REFUND_FAILED`), `payment_refunds` auto-retry state (`PENDING`, `PROCESSING`, `SUCCEEDED`, `FAILED`), and cancellation tombstones in `payment_db`.
- Transactional Outbox publication (`PaymentSucceeded`, `PaymentFailed`, `PaymentRefunded`, `PaymentRefundFailed`) and compensation event consumption (`PaymentCancelRequested`, `RefundRequested`).

This service does not own:

- Order lifecycle state, shipping calculation, or checkout snapshot validation.
- Catalog product metadata or Inventory stock reservations.
- Public HTTP ingress routing (BFF adapts public MoMo callbacks to private gRPC).

## Invariants and Workflows

- Never hold a PostgreSQL transaction open across an outbound MoMo HTTPS call (`CreatePaymentSession` splits `TX #A` and `TX #B` with `SELECT ... FOR UPDATE`).
- Verify `HmacSHA256` callback signatures with `hmac.Equal` before touching storage.
- Enforce `INSERT ... ON CONFLICT (order_id) DO NOTHING` tombstones so late session creation cannot bypass prior cancellation.
- Claim Outbox batches in a short transaction (`FOR UPDATE SKIP LOCKED`, `next_attempt_at` lease), commit before RabbitMQ publisher confirms (`3s`), and wake the relay immediately via `Notify()`.

## Current Layout

- `cmd/server/`: startup, PostgreSQL migration, Outbox relay, RabbitMQ consumer, gRPC (`:50056`), and `/healthz` (`:8086`).
- `internal/config/`: environment-backed configuration (`HTTP_ADDR`, `GRPC_ADDR`, `DATABASE_URL`, `RABBITMQ_URL`, `OUTBOX_POLL_INTERVAL`, `MOMO_*`, `PAYMENT_MOMO_MOCK_ENABLED`).
- `internal/domain/`: payment entities, outcomes, and domain error sentinels.
- `internal/momo/`: MoMo v2 canonical HMAC signer/verifier and bounded HTTP/mock client.
- `internal/store/`: embedded SQL migrations (`000001_create_payments_and_outbox.up.sql`, `000002_create_payment_refunds.up.sql`) and PostgreSQL adapter.
- `internal/events/`: transactional Outbox relay (`ecommerce.events`), background `RefundWorker` (up to 3 auto-retry attempts), and `payment.order-events` consumer.
- `internal/grpc/`: `PaymentServiceServer` (`CreatePaymentSession`, `HandleMomoCallback`, `RetryRefund`) handler.
- `internal/http/`: `/healthz` readiness/liveness handler.

## Verification

```powershell
go test ./services/payment-service/...
go run ./scripts/verify-service -service payment-service
```

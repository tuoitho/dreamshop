# MoMo v2 Refund (Auto-Retry + Admin Manual Fallback) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement end-to-end MoMo v2 full-order refunds with 3 automatic retry attempts in `payment-service`, a manual admin fallback gated strictly to `REFUND_FAILED` orders (`admin-fe -> bff-service -> order-service -> payment-service`), and backfill of pre-existing `REFUND_REQUIRED` records to `REFUND_FAILED`.

**Architecture:** `payment-service` owns `payment_refunds` and runs a 2-TX background `RefundWorker` (up to 3 auto attempts with exponential backoff) plus private gRPC `RetryRefund` for synchronous manual retries; outcomes propagate asynchronously to `order-service` via Transactional Outbox (`payment.refunded`, `payment.refund_failed`). `order-service` enforces `order.admin.update` RBAC and order preconditions before calling `PaymentService.RetryRefund` outside any DB transaction, projects admin-only refund audit/error fields separately from shopper responses, and `bff-service` exposes `POST /api/v1/admin/orders/{orderId}/refund` under the existing Kong `/api/v1/admin/orders` route.

**Tech Stack:** Protobuf/Buf, Go 1.24 (`pgx/v5`, `amqp091-go`, `google.golang.org/grpc`), Java 21 / Spring Boot / Flyway / Testcontainers, React / Vite / TypeScript / Vitest.

**Spec:** `docs/superpowers/specs/2026-11-25-momo-refund-design.md`

## Global Constraints

- Follow ADR-001, ADR-002, ADR-005, ADR-006, ADR-014, ADR-015: zero cross-service imports or cross-DB access; no MoMo secrets/logic in `order-service`; never hold a PostgreSQL transaction open across gRPC/HTTPS/RabbitMQ calls.
- All contract changes (protobuf, events, REST) are strictly additive; never hand-edit `contracts/go/` or `contracts/java/src/main/generated/`.
- Existing Kong routes (`/api/v1/admin/orders`) and `identity-service` permissions (`order.admin.update`) are reused without modification.
- MoMo v2 Refund HMAC-SHA256 `rawHash` is exact alphabetical order: `accessKey=$accessKey&amount=$amount&description=$description&orderId=$orderId&partnerCode=$partnerCode&requestId=$requestId&transId=$transId`.
- `refund_order_id` (`REFUND` + 32 hex) and `refund_request_id` (`REFREQ` + 24 hex) are strictly alphanumeric and distinct from original `momo_order_id`.
- Pre-existing `REFUND_REQUIRED` records are backfilled to `REFUND_FAILED` (Option 1), never `PENDING`, so migrations do not fire automatic payouts on boot.
- Production rollout order: `order-service` (bindings + Flyway V3) before `payment-service` (emits new routing keys), then `bff-service`, then frontends.

## Review Focus

1. **Migration backfill safety (`000002` & `V3`):** pre-existing `REFUND_REQUIRED` payments with `amount_minor > 0` and `momo_trans_id <> ''` backfill to `REFUND_FAILED` + `payment_refunds(status='FAILED', attempts=3)` without queuing auto-payouts; tombstones with empty `momo_trans_id` are NOT inserted into `payment_refunds`. (Pinned in Tasks 3 & 5.)
2. **Stale `PROCESSING` crash recovery:** a crashed worker lease (`status='PROCESSING'` with `next_attempt_at <= now()`) returns to `PENDING` if `attempts < max_auto_attempts` or transitions to `FAILED` + `PaymentRefundFailed` outbox if `attempts >= max_auto_attempts` without running a 4th automatic attempt. (Pinned in Task 3.)
3. **Out-of-order & duplicate event safety:** `PaymentRefundFailed` arriving after `REFUNDED` never downgrades `order_db`; duplicate `PaymentRefunded` never overwrites `ADMIN` audit metadata with `SYSTEM`; duplicate `RefundRequested` never resets `REFUND_FAILED` or `REFUNDED` back to auto-retry. (Pinned in Tasks 3 & 5.)
4. **Shopper API redaction boundary:** customer `GetOrder`/`ListOrders` responses never populate `refund_last_error`, `refund_actor_type`, or `refund_reason` even when the order is `REFUND_FAILED` or manually refunded. (Pinned in Task 5.)
5. **MoMo retry identity & frozen `description`:** ambiguous failures (timeout/5xx/crash/`7000`/`9000`) preserve `refund_order_id`, `refund_request_id`, and frozen `description`; definite rejections (`resultCode != 0` not in `{7000, 9000}`) rotate `refund_order_id` and `refund_request_id` on the next attempt while still preserving frozen `description` even when admin supplies a manual audit `reason`. (Pinned in Tasks 2, 3 & 4.)

## Surface Map & Dependency Order

| Task | Surface | Impact Class | Gate |
| --- | --- | --- | --- |
| 1 | `contracts` | `CONTRACT` (additive) | `make verify SERVICE=contracts` |
| 2–4 | `payment-service` | `DIRECT`, `INFRA`, `TEST` | `make verify SERVICE=payment-service` |
| 5–6 | `order-service` | `DIRECT`, `DEPENDENT`, `INFRA`, `TEST` | `make verify SERVICE=order-service` |
| 7 | `bff-service` | `DIRECT`, `DEPENDENT`, `CONTRACT`, `TEST` | `make verify SERVICE=bff-service` |
| 8 | `admin-fe` | `DIRECT`, `DEPENDENT`, `TEST` | `make verify SERVICE=admin-fe` |
| 9 | `ecommerce-fe` | `DIRECT`, `DEPENDENT`, `TEST` | `make verify SERVICE=ecommerce-fe` |
| 10 | `scripts` / `deployments` / `docs` | `INFRA`, `TEST` | `go test ./scripts/... && make check-boundaries` |

---

### Task 1: Additive Protobuf Contracts (`contracts`)

**Files:**
- Modify: `contracts/proto/payment/v1/payment.proto`
- Modify: `contracts/proto/order/v1/order.proto`
- Regenerate: `contracts/go/payment/v1/`, `contracts/go/order/v1/`, `contracts/java/src/main/generated/`

**Interfaces:**
- Produces (`payment.v1`): `PaymentService.RetryRefund(RetryRefundRequest{order_id, reason}) returns (RetryRefundResponse{order_id, payment_status, refund_trans_id, message})`
- Produces (`order.v1`): `OrderDetail` fields `22: refund_trans_id`, `23: refund_last_error`, `24: refunded_at`, `25: refund_actor_type`, `26: refund_reason`; `OrderService.AdminRetryRefund(AdminRetryRefundRequest{user_id, order_id, reason}) returns (AdminRetryRefundResponse{order, allowed_next_statuses})`

- [ ] **Step 1: Add `RetryRefund` RPC and messages to `contracts/proto/payment/v1/payment.proto`**
  Add `rpc RetryRefund(RetryRefundRequest) returns (RetryRefundResponse);` to `PaymentService`, and append `RetryRefundRequest` (`order_id = 1`, `reason = 2`) and `RetryRefundResponse` (`order_id = 1`, `payment_status = 2`, `refund_trans_id = 3`, `message = 4`).

- [ ] **Step 2: Add refund fields and `AdminRetryRefund` RPC to `contracts/proto/order/v1/order.proto`**
  Append fields `22`–`26` (`refund_trans_id`, `refund_last_error`, `refunded_at`, `refund_actor_type`, `refund_reason`) to `OrderDetail`; add `rpc AdminRetryRefund(AdminRetryRefundRequest) returns (AdminRetryRefundResponse);` to `OrderService`; append `AdminRetryRefundRequest` (`user_id = 1`, `order_id = 2`, `reason = 3`) and `AdminRetryRefundResponse` (`order = 1`, `allowed_next_statuses = 2`).

- [ ] **Step 3: Format, generate, and run contract verification gate**
  Run: `buf format -w contracts/proto && buf generate && make verify SERVICE=contracts`
  Expected: exit 0 (format, lint, breaking check vs `main`, and generated output sync all pass).

- [ ] **Step 4: Commit**
  ```bash
  git add contracts/proto contracts/go contracts/java/src/main/generated
  git commit -m "feat(contracts): add MoMo refund retry RPCs and order refund metadata"
  ```

---

### Task 2: `payment-service` Config, MoMo v2 Refund Signer & HTTP Client

**Files:**
- Modify: `services/payment-service/internal/config/config.go`
- Modify: `services/payment-service/internal/config/config_test.go`
- Modify: `services/payment-service/internal/momo/signer.go`
- Modify: `services/payment-service/internal/momo/signer_test.go`
- Modify: `services/payment-service/internal/momo/client.go`
- Modify: `services/payment-service/internal/momo/client_test.go`

**Interfaces:**
- Produces (`config`): `Config` fields `MomoRefundEndpoint string`, `RefundMaxAutoAttempts int`, `RefundPollInterval time.Duration`, `RefundRetryBackoff time.Duration`.
- Produces (`momo`):
  - `RefundSignParams{AccessKey, Description, OrderID, PartnerCode, RequestID string; Amount, TransID int64}`
  - `SignRefundRequest(p RefundSignParams, secret string) string`
  - `RefundRequest{OrderID, RequestID, OrigTransID, Description string; AmountMinor int64}`
  - `RefundResponse{ResultCode int32; Message, RefundTransID string}`
  - `(*Client).Refund(ctx context.Context, req RefundRequest) (*RefundResponse, error)`
  - `IsAmbiguousRefundError(err error) bool` (true for `ErrGatewayUnavailable`, non-`GatewayRejectedError`, or `GatewayRejectedError` with `ResultCode` in `{7000, 9000}`; false for definite `GatewayRejectedError` rejections).

- [ ] **Step 1: Write failing config, signer, and client tests**
  - In `config_test.go`: assert `MOMO_REFUND_ENDPOINT` derives `https://payment.momo.vn/v2/gateway/api/refund` when `MOMO_API_ENDPOINT=https://payment.momo.vn/v2/gateway/api/create`, honors explicit `MOMO_REFUND_ENDPOINT`, defaults `RefundMaxAutoAttempts=3`, `RefundPollInterval=2s`, `RefundRetryBackoff=10s`, and rejects invalid values (`<= 0`).
  - In `signer_test.go`: `TestSignRefundRequestMatchesMoMoV2RawHash` asserting exact HMAC-SHA256 over `accessKey=...&amount=...&description=...&orderId=...&partnerCode=...&requestId=...&transId=...`.
  - In `client_test.go`:
    - `TestClientRefundMockModeSucceedsWithoutHTTP`: `MockEnabled=true` returns `ResultCode=0` and non-empty `RefundTransID` even when `OrigTransID` is `"mock-123"`.
    - `TestClientRefundRealModeSendsSignedPayloadAndParsesNumericTransID`: `httptest.NewServer` asserts JSON keys (`partnerCode`, `orderId`, `requestId`, `amount`, numeric `transId`, `lang: "vi"`, `description`, valid `signature`), responds `{"resultCode":0,"message":"Thành công","transId":144518121}`, expects `RefundTransID == "144518121"`.
    - `TestClientRefundRealModeHandlesErrorsAndClassification`: tests non-numeric `OrigTransID` (`GatewayRejectedError`, non-ambiguous), HTTP 503 (`ErrGatewayUnavailable`, ambiguous), `resultCode: 1080` (`GatewayRejectedError`, non-ambiguous), and `resultCode: 7000` (`GatewayRejectedError`, ambiguous via `IsAmbiguousRefundError`).

- [ ] **Step 2: Run tests to verify failure**
  Run: `go test ./services/payment-service/internal/config/... ./services/payment-service/internal/momo/...`
  Expected: FAIL (undefined fields/methods).

- [ ] **Step 3: Implement config fields, `SignRefundRequest`, `Client.Refund`, and `IsAmbiguousRefundError`**
  - In `config.go`: add `deriveRefundEndpoint(apiEndpoint, explicit string) string` (explicit -> replace `/create` suffix with `/refund` -> fallback `https://test-payment.momo.vn/v2/gateway/api/refund`) and `intEnv` helper for `PAYMENT_REFUND_MAX_AUTO_ATTEMPTS`.
  - In `signer.go`: implement `RefundSignParams` and `SignRefundRequest` using `computeHMAC`.
  - In `client.go`: add `RefundEndpoint string` to `ClientConfig`, implement `(*Client).Refund` supporting both JSON number and string for response `transId` via `json.RawMessage`, and implement `IsAmbiguousRefundError`.

- [ ] **Step 4: Run tests to verify pass**
  Run: `go test -v ./services/payment-service/internal/config/... ./services/payment-service/internal/momo/...`
  Expected: PASS.

- [ ] **Step 5: Commit**
  ```bash
  git add services/payment-service/internal/config services/payment-service/internal/momo
  git commit -m "feat(payment): add MoMo v2 refund config, HMAC signer, and client"
  ```

---

### Task 3: `payment-service` Migration `000002` (Backfill), Refund Store & `RefundWorker`

**Files:**
- Create: `services/payment-service/internal/store/migrations/000002_create_payment_refunds.up.sql`
- Create: `services/payment-service/internal/store/refunds.go`
- Create: `services/payment-service/internal/store/refunds_test.go`
- Create: `services/payment-service/internal/events/refund_worker.go`
- Create: `services/payment-service/internal/events/refund_worker_test.go`
- Modify: `services/payment-service/internal/domain/payment.go`
- Modify: `services/payment-service/internal/store/store.go`

**Interfaces:**
- Produces (`domain`):
  - Payment statuses `StatusRefunded = "REFUNDED"`, `StatusRefundFailed = "REFUND_FAILED"`
  - Refund job statuses `RefundStatusPending = "PENDING"`, `RefundStatusProcessing = "PROCESSING"`, `RefundStatusSucceeded = "SUCCEEDED"`, `RefundStatusFailed = "FAILED"`
  - Sentinel errors: `ErrRefundNotFound`, `ErrRefundAutoRetryInProgress`, `ErrRefundInProgress`
  - Types: `RefundJob{OrderID, OrderCode, UserID, MomoOrderID, MomoTransID, RefundOrderID, RefundRequestID, Description, Reason, Currency string; AmountMinor int64; Attempts, MaxAutoAttempts int}`, `RefundAttemptResult{Succeeded bool; RefundTransID string; ResultCode int32; ResultMessage, LastError string; RotateIDsOnNextAttempt bool}`, `ManualRefundBeginOutcome{AlreadyRefunded bool; RefundTransID string; Job *RefundJob}`.
- Produces (`store`):
  - `(*Store).RequestRefund(ctx context.Context, orderID, reason string, maxAutoAttempts int) (queued bool, err error)`
  - `(*Store).RecoverStaleRefunds(ctx context.Context) (outboxInserted bool, err error)`
  - `(*Store).ClaimDueRefunds(ctx context.Context, limit int) ([]domain.RefundJob, error)`
  - `(*Store).CompleteAutoRefundAttempt(ctx context.Context, orderID string, res domain.RefundAttemptResult, retryBackoffBase time.Duration) (outboxInserted bool, err error)`
  - `(*Store).BeginManualRefund(ctx context.Context, orderID, reason string) (*domain.ManualRefundBeginOutcome, error)`
  - `(*Store).CompleteManualRefund(ctx context.Context, orderID string, res domain.RefundAttemptResult) (outboxInserted bool, err error)`
- Produces (`events`):
  - Routing keys `RoutingKeyPaymentRefunded = "payment.refunded"`, `RoutingKeyPaymentRefundFailed = "payment.refund_failed"`
  - `NewRefundWorker(store RefundStore, momoClient RefundGateway, outboxNotifier OutboxNotifier, pollInterval, retryBackoffBase, callTimeout time.Duration) *RefundWorker` with `Notify()` and `Run(ctx context.Context) error`.

- [ ] **Step 1: Write failing store & worker tests**
  - In `store/refunds_test.go` (using `isolatedPaymentPool`, runs against Postgres):
    - `TestMigration000002BackfillsExistingRefundRequiredToFailedAndSkipsTombstones`: applies migration `000001`, seeds (a) a `REFUND_REQUIRED` payment with `amount_minor=150000, momo_trans_id='998877'`, (b) a `REFUND_REQUIRED` tombstone with `amount_minor=0, momo_trans_id=''`, and (c) a `SUCCEEDED` payment; runs `Migrate` (`000002`); asserts (a) became `payments.status='REFUND_FAILED'` with a `payment_refunds` row (`status='FAILED'`, `attempts=3`, alphanumeric `refund_order_id`/`refund_request_id`, `next_attempt_at IS NULL`), (b) has no `payment_refunds` row, and (c) is untouched.
    - `TestRefundLifecycleAuto3AttemptsRotationAndManualRetry`: tests `RequestRefund` -> 3 auto attempts (`ClaimDueRefunds` + `CompleteAutoRefundAttempt`): attempt 1 ambiguous failure keeps IDs fixed; attempt 2 definite failure (`RotateIDsOnNextAttempt=true`) causes attempt 3 claim to mint fresh alphanumeric IDs while keeping frozen `description`; attempt 3 failure marks `FAILED` + `payments.status='REFUND_FAILED'` + inserts `PaymentRefundFailed` outbox; duplicate `RequestRefund` is no-op; `BeginManualRefund` on `FAILED` claims attempt 4 with local `reason` without mutating frozen `description`, and `CompleteManualRefund(Succeeded=true)` marks `REFUNDED` + inserts `PaymentRefunded` outbox; repeat `BeginManualRefund` returns `AlreadyRefunded=true`.
    - `TestRecoverStaleRefundsHandlesUnderAndAtMaxAttempts`: seeds two `PROCESSING` rows with expired `next_attempt_at` (`attempts=1` and `attempts=3`); asserts `RecoverStaleRefunds` resets `attempts=1` to `PENDING` and finalizes `attempts=3` to `FAILED` + `payments.status='REFUND_FAILED'` + `PaymentRefundFailed` outbox.
  - In `events/refund_worker_test.go` (pure unit test with in-memory fake store & gateway, runs even without Postgres):
    - `TestRefundWorkerExecutes3AttemptsThenEmitsFailedAndTriggersOutbox`: verifies `RefundWorker.processOnce` calls `RecoverStaleRefunds`, claims due jobs, invokes `momoClient.Refund` with frozen `Description`, maps `IsAmbiguousRefundError` to `RotateIDsOnNextAttempt = !ambiguous`, and calls `outboxNotifier.Notify()` when `outboxInserted == true`.

- [ ] **Step 2: Run unit test to verify failure**
  Run: `go test ./services/payment-service/internal/events/... ./services/payment-service/internal/store/...`
  Expected: FAIL (missing migration/types/methods).

- [ ] **Step 3: Create migration `000002_create_payment_refunds.up.sql`, update `domain/payment.go` & `store/store.go`, implement `store/refunds.go` and `events/refund_worker.go`**
  - Migration `000002` drops/recreates `payments_status_check` adding `'REFUNDED'`, `'REFUND_FAILED'`; creates `payment_refunds` table + `ix_payment_refunds_due`; executes the Option 1 backfill SQL (`INSERT INTO payment_refunds ... FROM payments WHERE status = 'REFUND_REQUIRED' AND amount_minor > 0 AND momo_trans_id <> ''` and `UPDATE payments SET status = 'REFUND_FAILED' WHERE status = 'REFUND_REQUIRED' AND amount_minor > 0 AND momo_trans_id <> ''`).
  - Update `store/store.go`: treat `StatusRefunded` and `StatusRefundFailed` as terminal in `BeginCreateSession`, `CompleteCreateSession`, `CancelPayment`, and `RecordMomoCallback`; in `RecordMomoCallback`, if `status == StatusRefundRequired` and existing `momo_trans_id == ''` and `in.ResultCode == 0`, populate `momo_trans_id` (and `amount_minor` if tombstone) and emit `PaymentSucceeded`.
  - Implement `store/refunds.go` with alphanumeric `newRefundOrderID()` (`"REFUND" + 32 hex`) and `newRefundRequestID()` (`"REFREQ" + 24 hex`), `RequestRefund`, `RecoverStaleRefunds`, `ClaimDueRefunds`, `CompleteAutoRefundAttempt`, `BeginManualRefund`, `CompleteManualRefund`, plus outbox helpers for `PaymentRefunded` (`payment.refunded`) and `PaymentRefundFailed` (`payment.refund_failed`).
  - Implement `events/refund_worker.go` mirroring `OutboxRelay`'s `wakeupCh` + ticker loop.

- [ ] **Step 4: Run tests to verify pass**
  Run: `go test -v ./services/payment-service/internal/events/... ./services/payment-service/internal/store/...`
  Expected: PASS (with `DATABASE_URL` set against local Postgres for full `store` coverage).

- [ ] **Step 5: Commit**
  ```bash
  git add services/payment-service/internal/domain services/payment-service/internal/store services/payment-service/internal/events
  git commit -m "feat(payment): add payment_refunds migration with backfill, store, and RefundWorker"
  ```

---

### Task 4: `payment-service` Consumer Hook, gRPC `RetryRefund` & Main Wiring

**Files:**
- Modify: `services/payment-service/internal/events/consumer.go`
- Modify: `services/payment-service/internal/events/events_test.go`
- Modify: `services/payment-service/internal/grpc/server.go`
- Modify: `services/payment-service/internal/grpc/server_test.go`
- Modify: `services/payment-service/cmd/server/main.go`
- Modify: `services/payment-service/AGENTS.md`

**Interfaces:**
- Consumes: `store` refund methods, `events.RefundWorker`, `momo.Client.Refund`, `paymentv1.RetryRefundRequest/Response`.
- Produces: `PaymentService.RetryRefund` gRPC handler wired in `cmd/server/main.go`.

- [ ] **Step 1: Write failing consumer & gRPC `RetryRefund` tests**
  - In `events/events_test.go`: assert `DispatchMessage` on `RoutingKeyRefundRequested` calls `RequestRefund(ctx, orderID, reason)` and wakes the refund notifier when queued.
  - In `grpc/server_test.go`:
    - `TestRetryRefundSucceedsAndNotifiesOutbox`: mocks `BeginManualRefund` returning a job, runs `momoClient.Refund` (mock mode), asserts `CompleteManualRefund` is called with `Succeeded=true`, `notifier.Notify()` fires, and response has `PaymentStatus == "REFUNDED"` + non-empty `RefundTransId`.
    - `TestRetryRefundMapsPreconditionAndGatewayErrors`: verifies `ErrRefundNotFound` -> `NotFound/PAYMENT_REFUND_NOT_FOUND`, `ErrRefundAutoRetryInProgress` -> `FailedPrecondition/REFUND_AUTO_RETRY_IN_PROGRESS`, `ErrRefundInProgress` -> `Aborted/REFUND_IN_PROGRESS`, already-refunded returns idempotent `REFUNDED` without calling MoMo, and gateway 503 / rejection persists failure via `CompleteManualRefund` and returns `Unavailable/PAYMENT_GATEWAY_UNAVAILABLE` or `FailedPrecondition/PAYMENT_GATEWAY_REJECTED`.

- [ ] **Step 2: Run tests to verify failure**
  Run: `go test ./services/payment-service/internal/events/... ./services/payment-service/internal/grpc/...`
  Expected: FAIL.

- [ ] **Step 3: Implement consumer wiring, `Server.RetryRefund`, `cmd/server/main.go` wiring, and `AGENTS.md` update**
  - Update `events/consumer.go`: `CompensationHandler` calls `RequestRefund` (via adapter/method passing `maxAutoAttempts` and notifying `RefundWorker` when queued).
  - Update `grpc/server.go`: extend `PaymentStore` interface with `BeginManualRefund` and `CompleteManualRefund`, implement `(*Server).RetryRefund` using 2-TX pattern (`context.WithoutCancel(ctx)` + `3s` timeout on TX #B), map new domain errors in `mapPaymentError`.
  - Update `cmd/server/main.go`: pass `RefundEndpoint` into `momo.ClientConfig`, construct and launch `RefundWorker` goroutine, wire consumer -> `RequestRefund` + `refundWorker.Notify()`.
  - Update `services/payment-service/AGENTS.md` to document `REFUNDED`, `REFUND_FAILED`, `payment_refunds`, `RetryRefund`, and `PaymentRefunded`/`PaymentRefundFailed`.

- [ ] **Step 4: Run `payment-service` verification gate**
  Run: `make verify SERVICE=payment-service`
  Expected: exit 0.

- [ ] **Step 5: Commit**
  ```bash
  git add services/payment-service
  git commit -m "feat(payment): wire RefundWorker, RetryRefund gRPC, and consumer dispatch"
  ```

---

### Task 5: `order-service` Flyway `V3` (Backfill), `PaymentStatus` & Refund Event Consumer

**Files:**
- Create: `services/order-service/src/main/resources/db/migration/V3__add_refund_payment_statuses.sql`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/domain/PaymentStatus.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/persistence/OrderEntity.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/application/OrderProtoMapper.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/application/OrderAdminQueryService.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/application/OrderLifecycleService.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/config/RabbitMqConfig.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/application/PaymentEventConsumer.java`
- Modify: `services/order-service/src/test/java/com/ecommerce/order/application/PaymentEventConsumerTest.java`
- Modify: `services/order-service/src/test/java/com/ecommerce/order/persistence/OrderMigrationAndLockingTest.java`

**Interfaces:**
- Produces (`order-service`):
  - `PaymentStatus.REFUNDED`, `PaymentStatus.REFUND_FAILED`
  - `OrderEntity` fields: `refundTransId`, `refundLastError`, `refundedAt`, `refundActorType`, `refundActorId`, `refundReason`
  - `OrderProtoMapper.toOrderDetail(OrderEntity[, history])` (customer-safe: omits `refundLastError`, `refundActorType`, `refundReason`)
  - `OrderProtoMapper.toAdminOrderDetail(OrderEntity[, history])` (includes all admin refund fields)
  - `RabbitMqConfig.RK_PAYMENT_REFUNDED = "payment.refunded"`, `RK_PAYMENT_REFUND_FAILED = "payment.refund_failed"` bound to `order.payment-events`.

- [ ] **Step 1: Write failing migration, event consumer, and shopper-redaction tests**
  - In `OrderMigrationAndLockingTest.java`: verify `V3` allows `REFUNDED`/`REFUND_FAILED` in `orders.payment_status` and that `V3` backfill query logic transitions pre-existing `CANCELLED` + `MOMO` + `REFUND_REQUIRED` rows to `REFUND_FAILED` with non-empty `refund_last_error`.
  - In `PaymentEventConsumerTest.java`:
    - `paymentRefundedTransitionsRefundRequiredAndRefundFailedToRefunded`: asserts `payment.refunded` moves `REFUND_REQUIRED` -> `REFUNDED`, stores `refundTransId`, sets `refundActorType = SYSTEM` and `refundedAt`, clears `refundLastError`; and duplicate `PaymentRefunded` does NOT overwrite `ADMIN` audit metadata.
    - `paymentRefundFailedMarksRefundFailedAndNeverDowngradesRefunded`: asserts `payment.refund_failed` moves `REFUND_REQUIRED` -> `REFUND_FAILED` storing truncated `refundLastError`, while an out-of-order `PaymentRefundFailed` on an already `REFUNDED` order is ignored.
    - `customerOrderQueryRedactsAdminRefundErrorAndAuditFields`: asserts `OrderQueryService.getOrder` returns `paymentStatus = REFUND_FAILED` with empty `refundLastError`, `refundActorType`, and `refundReason`, whereas `OrderAdminQueryService.getOrder` exposes them.

- [ ] **Step 2: Run focused tests to verify failure**
  Run: `mvn -pl services/order-service -Dtest=PaymentEventConsumerTest,OrderMigrationAndLockingTest test`
  Expected: FAIL.

- [ ] **Step 3: Implement Flyway `V3`, entity fields, customer vs admin proto mappers, RabbitMQ bindings, and `PaymentEventConsumer`**
  - Create `V3__add_refund_payment_statuses.sql` updating `orders_payment_status_check`, adding `refund_trans_id`, `refund_last_error`, `refunded_at`, `refund_actor_type`, `refund_actor_id`, `refund_reason`, and running Option 1 backfill `UPDATE orders SET payment_status = 'REFUND_FAILED', refund_last_error = '...' WHERE status = 'CANCELLED' AND payment_method = 'MOMO' AND payment_status = 'REFUND_REQUIRED'`.
  - Add `toAdminOrderDetail` in `OrderProtoMapper` and switch `OrderAdminQueryService` + `OrderLifecycleService.updateStatusByAdmin` to use `toAdminOrderDetail` while customer queries keep `toOrderDetail`.
  - Bind `RK_PAYMENT_REFUNDED` and `RK_PAYMENT_REFUND_FAILED` in `RabbitMqConfig` and handle both in `PaymentEventConsumer`.

- [ ] **Step 4: Run focused tests to verify pass**
  Run: `mvn -pl services/order-service -Dtest=PaymentEventConsumerTest,OrderMigrationAndLockingTest test`
  Expected: PASS.

- [ ] **Step 5: Commit**
  ```bash
  git add services/order-service
  git commit -m "feat(order): add V3 refund statuses with backfill, bindings, and event consumer"
  ```

---

### Task 6: `order-service` Synchronous `AdminRetryRefund` RPC & `PaymentGrpcAdapter.retryRefund`

**Files:**
- Modify: `services/order-service/src/main/java/com/ecommerce/order/integration/PaymentGrpcAdapter.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/application/OrderLifecycleService.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/grpc/OrderGrpcService.java`
- Modify: `services/order-service/src/test/java/com/ecommerce/order/application/OrderAdminServiceTest.java`
- Modify: `services/order-service/src/test/java/com/ecommerce/order/grpc/OrderGrpcIntegrationTest.java`

**Interfaces:**
- Consumes: `PaymentService.RetryRefund` gRPC contract, `OrderProtoMapper.toAdminOrderDetail`.
- Produces: `PaymentGrpcAdapter.retryRefund(UUID orderId, String reason) -> RetryRefundOutcome`, `OrderLifecycleService.retryRefundByAdmin(rawUserId, rawOrderId, reason) -> AdminRetryRefundResponse`, `OrderGrpcService.adminRetryRefund`.

- [ ] **Step 1: Write failing `OrderAdminServiceTest` & `OrderGrpcIntegrationTest` cases**
  - In `OrderAdminServiceTest.java` (with `@MockitoBean PaymentGrpcAdapter`):
    - `adminRetryRefundRequiresPermissionAndRefundFailedStatus`: asserts missing `order.admin.update` -> `ORDER_PERMISSION_DENIED`; `REFUND_REQUIRED` -> `REFUND_AUTO_RETRY_IN_PROGRESS`; `PAID` / `COD` / `REFUNDED` -> `ORDER_REFUND_NOT_ELIGIBLE` (zero calls to `paymentGrpcAdapter`).
    - `adminRetryRefundSuccessUpdatesOrderToRefundedWithAdminAudit`: mocks `paymentGrpcAdapter.retryRefund` returning `("REFUNDED", "momo-ref-1")`; asserts `order_db` updates immediately to `REFUNDED`, `refundTransId = "momo-ref-1"`, `refundActorType = "ADMIN"`, `refundActorId = adminId`, `refundReason = "MoMo recovered"`, `refundLastError = ""`, and `order_status_history` count does NOT increase.
    - `adminRetryRefundGatewayFailurePersistsLatestErrorAndRethrows`: mocks `paymentGrpcAdapter.retryRefund` throwing `PAYMENT_GATEWAY_UNAVAILABLE`; asserts `orders.payment_status` remains `REFUND_FAILED`, `orders.refund_last_error` updates, and exception propagates.
  - In `OrderGrpcIntegrationTest.java`: verify `OrderServiceBlockingStub.adminRetryRefund` end-to-end over gRPC stub.

- [ ] **Step 2: Run focused tests to verify failure**
  Run: `mvn -pl services/order-service -Dtest=OrderAdminServiceTest,OrderGrpcIntegrationTest test`
  Expected: FAIL.

- [ ] **Step 3: Implement `PaymentGrpcAdapter.retryRefund`, `OrderLifecycleService.retryRefundByAdmin`, and `OrderGrpcService.adminRetryRefund`**
  - In `PaymentGrpcAdapter`: add `public record RetryRefundOutcome(String paymentStatus, String refundTransId, String message)` and `retryRefund(UUID orderId, String reason)` mapping `PAYMENT_REFUND_NOT_FOUND` -> `ORDER_REFUND_NOT_ELIGIBLE`, `REFUND_AUTO_RETRY_IN_PROGRESS` -> `REFUND_AUTO_RETRY_IN_PROGRESS`, `REFUND_IN_PROGRESS` -> `REFUND_IN_PROGRESS`, `PAYMENT_GATEWAY_REJECTED` -> `PAYMENT_GATEWAY_REJECTED`, else `PAYMENT_GATEWAY_UNAVAILABLE`.
  - In `OrderLifecycleService`: inject `PaymentGrpcAdapter`, implement `retryRefundByAdmin` with RBAC outside TX, short read TX for preconditions, gRPC call outside TX, short write TX for `REFUNDED` + audit fields (or persisting latest `refundLastError` on gateway failure).
  - In `OrderGrpcService`: override `adminRetryRefund`.

- [ ] **Step 4: Run `order-service` verification gate**
  Run: `make verify SERVICE=order-service`
  Expected: exit 0.

- [ ] **Step 5: Commit**
  ```bash
  git add services/order-service
  git commit -m "feat(order): implement synchronous AdminRetryRefund gRPC flow"
  ```

---

### Task 7: `bff-service` Admin Manual Refund Endpoint & DTO Mapping

**Files:**
- Modify: `services/bff-service/internal/order/client.go`
- Modify: `services/bff-service/internal/http/orders.go`
- Modify: `services/bff-service/internal/http/admin_orders.go`
- Modify: `services/bff-service/internal/http/admin_orders_test.go`
- Modify: `services/bff-service/internal/http/orders_test.go`
- Modify: `services/bff-service/cmd/server/main_test.go`

**Interfaces:**
- Consumes: `orderv1.OrderServiceClient.AdminRetryRefund`, new `OrderDetail` fields `22`–`26`.
- Produces: `POST /api/v1/admin/orders/{orderId}/refund` returning `{ "data": { "order": OrderDetail, "allowedNextStatuses": [...] }, "message": "Hoàn tiền đơn hàng thành công" }`, plus optional JSON fields `refundTransId`, `refundLastError`, `refundedAt`, `refundActorType`, `refundReason` on `orderDetailJSON`.

- [ ] **Step 1: Write failing BFF tests**
  - In `admin_orders_test.go`:
    - `TestAdminRetryRefundForwardsReasonAndReturnsUpdatedOrder`: sends `POST /api/v1/admin/orders/ord-123/refund` with `{"reason":"MoMo đã hoạt động lại"}` and also empty body `""`; asserts `AdminRetryRefund` receives caller/orderId/reason, returns `200 OK` with updated `paymentStatus: "REFUNDED"`, `refundTransId`, `refundActorType`, `refundedAt`, `refundReason`, and message `"Hoàn tiền đơn hàng thành công"`.
    - `TestAdminRetryRefundMapsConflictAndUnavailableErrors`: asserts `REFUND_AUTO_RETRY_IN_PROGRESS` -> `409`, `ORDER_REFUND_NOT_ELIGIBLE` -> `409`, `REFUND_IN_PROGRESS` -> `409`, `PAYMENT_GATEWAY_REJECTED` -> `409`, `PAYMENT_GATEWAY_UNAVAILABLE` -> `503`, unknown JSON field -> `400 INVALID_PARAMETER`.
  - In `cmd/server/main_test.go`: add route auth check for `POST /api/v1/admin/orders/ord-1/refund` (authenticated `200`, unauthenticated `401`).

- [ ] **Step 2: Run BFF tests to verify failure**
  Run: `go test ./services/bff-service/...`
  Expected: FAIL.

- [ ] **Step 3: Implement `AdminRetryRefund` in `order/client.go`, DTO fields & error mapping in `http/orders.go`, and handler in `http/admin_orders.go`**
  - Update `fakeOrderClient` in `orders_test.go` and `main_test.go` to implement `AdminRetryRefund`.

- [ ] **Step 4: Run `bff-service` verification gate**
  Run: `make verify SERVICE=bff-service`
  Expected: exit 0.

- [ ] **Step 5: Commit**
  ```bash
  git add services/bff-service
  git commit -m "feat(bff): expose POST /api/v1/admin/orders/{orderId}/refund endpoint"
  ```

---

### Task 8: `admin-fe` Refund Statuses & Manual Refund Fallback UI

**Files:**
- Modify: `admin-fe/src/lib/types.ts`
- Modify: `admin-fe/src/lib/order-status.ts`
- Modify: `admin-fe/src/lib/api.ts`
- Modify: `admin-fe/src/lib/api.test.ts`
- Modify: `admin-fe/src/pages/order-detail-sheet.tsx`
- Modify: `admin-fe/src/pages/orders-page.test.tsx`

**Interfaces:**
- Consumes: `POST /admin/orders/{orderId}/refund`, `AdminOrder` fields `paymentStatus` (`REFUNDED` | `REFUND_FAILED`), `refundTransId`, `refundLastError`, `refundedAt`, `refundActorType`, `refundReason`.
- Produces: `retryAdminOrderRefund(orderId: string, reason?: string): Promise<AdminOrderView>`, manual refund confirmation UX inside `OrderDetailSheet`.

- [ ] **Step 1: Write failing `admin-fe` tests**
  - In `admin-fe/src/lib/api.test.ts`: assert `retryAdminOrderRefund("ord-1", "Thử lại sau sự cố")` sends `POST /admin/orders/ord-1/refund` with `{ reason: "Thử lại sau sự cố" }`.
  - In `admin-fe/src/pages/orders-page.test.tsx`:
    - Shows badge `"Hoàn tiền thất bại"`, error alert with `refundLastError`, and button **"Hoàn tiền thủ công"** when `order.status === "CANCELLED"`, `paymentMethod === "MOMO"`, `paymentStatus === "REFUND_FAILED"`, and user has `order.admin.update`; clicking confirm invokes `retryAdminOrderRefund`, updates the sheet immediately to `"Đã hoàn tiền"` + `refundTransId`, and refreshes the order list.
    - Hides **"Hoàn tiền thủ công"** when `paymentStatus === "REFUND_REQUIRED"`, `paymentStatus === "REFUNDED"`, or user lacks `order.admin.update`.

- [ ] **Step 2: Run tests to verify failure**
  Run: `npm --prefix admin-fe test -- src/lib/api.test.ts src/pages/orders-page.test.tsx`
  Expected: FAIL.

- [ ] **Step 3: Implement types, labels, API helper, and `OrderDetailSheet` manual refund UI**
  - Update `types.ts`, `order-status.ts`, `api.ts`.
  - In `order-detail-sheet.tsx`: add refund failure alert + gated **"Hoàn tiền thủ công"** button + lightweight confirmation dialog (with optional reason input and loading disable state) + refund metadata block when `REFUNDED` (`refundTransId`, `refundedAt`, `ACTOR_LABELS[refundActorType]`, `refundReason`).

- [ ] **Step 4: Run `admin-fe` verification gate**
  Run: `make verify SERVICE=admin-fe`
  Expected: exit 0 (tests, lint, TypeScript build, boundary check).

- [ ] **Step 5: Commit**
  ```bash
  git add admin-fe
  git commit -m "feat(admin-fe): display refund statuses and manual MoMo refund fallback"
  ```

---

### Task 9: `ecommerce-fe` Storefront Refund Statuses & Banners

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Modify: `ecommerce-fe/src/lib/order-status.ts`
- Modify: `ecommerce-fe/src/pages/order-detail-page.tsx`
- Modify: `ecommerce-fe/src/pages/payment-result-page.tsx`
- Modify: `ecommerce-fe/src/pages/payment-result-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/order-detail-page.test.tsx` (or create if absent)

**Interfaces:**
- Consumes: `OrderDetail` `paymentStatus` (`REFUNDED` | `REFUND_FAILED`) and optional `refundTransId`.

- [ ] **Step 1: Write failing storefront tests**
  - In `payment-result-page.test.tsx`: assert `CANCELLED` + `REFUNDED` renders green refunded banner `"Đã hoàn tiền"`, and `CANCELLED` + `REFUND_FAILED` renders customer-friendly refund-in-progress notice without exposing raw gateway errors.
  - In `order-detail-page.test.tsx`: assert `REFUNDED` displays `"Đã hoàn tiền"` badge/banner (and `refundTransId` if present), and `REFUND_FAILED` displays `"Đang xử lý hoàn tiền"` without any raw error text.

- [ ] **Step 2: Run tests to verify failure**
  Run: `npm --prefix ecommerce-fe test -- src/pages/payment-result-page.test.tsx src/pages/order-detail-page.test.tsx`
  Expected: FAIL.

- [ ] **Step 3: Update storefront types, status helpers, `order-detail-page.tsx`, and `payment-result-page.tsx`**
  - `order-status.ts`: `CANCELLED` + `REFUNDED` -> `"Đã hủy • Đã hoàn tiền"` (emerald); `CANCELLED` + (`REFUND_REQUIRED` | `REFUND_FAILED`) -> `"Đã hủy • Đang hoàn tiền"` (amber).
  - `order-detail-page.tsx` & `payment-result-page.tsx`: add green `REFUNDED` state and treat `REFUND_FAILED` with the same friendly pending-refund messaging as `REFUND_REQUIRED`.

- [ ] **Step 4: Run `ecommerce-fe` verification gate**
  Run: `make verify SERVICE=ecommerce-fe`
  Expected: exit 0.

- [ ] **Step 5: Commit**
  ```bash
  git add ecommerce-fe
  git commit -m "feat(ecommerce-fe): show refunded and pending-refund states cleanly"
  ```

---

### Task 10: Local Scripts, Env Templates, Compose & Documentation + Final Verification

**Files:**
- Modify: `.env.example`
- Modify: `.env.production.example`
- Modify: `deployments/docker/compose.production.yml`
- Modify: `scripts/start-local/main.go`
- Modify: `scripts/start-local/main_test.go`
- Modify: `docs/integrations/momo-payment.md`

**Interfaces:**
- Documents and wires `MOMO_REFUND_ENDPOINT`, `PAYMENT_REFUND_MAX_AUTO_ATTEMPTS`, `PAYMENT_REFUND_POLL_INTERVAL`, `PAYMENT_REFUND_RETRY_BACKOFF`.

- [ ] **Step 1: Update `scripts/start-local/main_test.go` with failing assertion for default refund env vars**
  Assert `start-local` for `payment-service` includes `MOMO_REFUND_ENDPOINT=https://test-payment.momo.vn/v2/gateway/api/refund` and `PAYMENT_REFUND_MAX_AUTO_ATTEMPTS=3`.

- [ ] **Step 2: Run script test to verify failure**
  Run: `go test ./scripts/...`
  Expected: FAIL.

- [ ] **Step 3: Update `.env.example`, `.env.production.example`, `deployments/docker/compose.production.yml`, `scripts/start-local/main.go`, and `docs/integrations/momo-payment.md`**
  Document auto-refund (3 attempts), Option 1 backfill behavior, MoMo v2 Refund payload/HMAC signature, and manual admin retry on `admin-fe`.

- [ ] **Step 4: Run script tests, boundary check, and layer-2 verification across all touched surfaces**
  Run:
  - `go test ./scripts/...`
  - `make check-boundaries`
  - `make verify SERVICE=contracts`
  - `make verify SERVICE=payment-service`
  - `make verify SERVICE=order-service`
  - `make verify SERVICE=bff-service`
  - `make verify SERVICE=admin-fe`
  - `make verify SERVICE=ecommerce-fe`
  Expected: all exit 0.

- [ ] **Step 5: Commit & run repository final reviews**
  Commit docs/infra updates, then dispatch two fresh read-only subagents in parallel per `fullstack-feature`:
  - `reviewing-code`
  - `reviewing-service-boundaries`
  Resolve any blocking/major findings, re-verify affected surfaces, and produce the final evidence report.

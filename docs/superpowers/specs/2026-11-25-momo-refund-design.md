# MoMo v2 Refund (Auto-Retry + Admin Manual Fallback) Design

## 1. Intent & Why

When a paid MoMo order is cancelled (by customer or admin) or receives a late MoMo payment callback after cancellation, `order-service` already transitions `payment_status` to `REFUND_REQUIRED` and emits `RefundRequested` (`order.payment.refund_requested`). Today, `payment-service` only marks the payment row `REFUND_REQUIRED` and stops there—money is never refunded to the customer through MoMo, and `order-service` / frontends have no `REFUNDED` or `REFUND_FAILED` lifecycle.

This feature completes the end-to-end MoMo v2 refund workflow:
1. **Automatic refund with up to 3 attempts** in `payment-service` as soon as a paid MoMo order requires a refund, supporting both local mock mode (`PAYMENT_MOMO_MOCK_ENABLED=true`) and real MoMo Sandbox/Production (`PAYMENT_MOMO_MOCK_ENABLED=false`).
2. **Manual admin fallback in `admin-fe`** strictly gated to orders whose 3 automatic refund attempts failed (e.g., MoMo outage/timeout), giving operators immediate synchronous feedback from MoMo and converging `order-service` to `REFUNDED` right away.

## 2. Goals, Non-Goals, and Anti-Goals

### Goals
- Integrate MoMo v2 Refund API (`POST /v2/gateway/api/refund`) with HMAC-SHA256 signing, unique alphanumeric `orderId` (`refund_order_id`) distinct from the original payment's `orderId`, unique `requestId`, and original `transId` (`int64`).
- Support full-order refunds in both `PAYMENT_MOMO_MOCK_ENABLED=true` (simulated immediate success) and `PAYMENT_MOMO_MOCK_ENABLED=false` (live HTTP call to MoMo Sandbox/Production).
- Guarantee refund idempotency across retries and crashes: persist `refund_order_id` and `refund_request_id` before calling MoMo, reuse them on retry, and never hold a PostgreSQL transaction open across an HTTP/gRPC network call (ADR-014).
- Automatically execute up to **3 attempts** with exponential backoff; transition to `REFUND_FAILED` and stop auto-retrying once all 3 attempts fail.
- Expose a manual admin retry action (`POST /api/v1/admin/orders/{orderId}/refund`) routed through `bff-service -> order-service -> payment-service`, reusing existing `order.admin.update` RBAC and existing Kong `/api/v1/admin/orders` routing.
- Reflect `REFUNDED` and `REFUND_FAILED` clearly across `order-service`, `admin-fe`, and `ecommerce-fe`.

### Non-Goals
- Partial refunds (per-item or custom refund amount). Every refund refunds the full `amount_minor` captured on the original payment.
- MoMo `/v2/gateway/api/refund/query` reconciliation polling (left as a future operational enhancement if needed).
- Manual "mark refunded offline" override without calling MoMo.
- Adding a new RBAC permission in `identity-service` (reuses `order.admin.update`).
- Modifying Kong route definitions (existing `/api/v1/admin/orders` prefix already covers `/api/v1/admin/orders/{orderId}/refund`).

### Anti-Goals
- **Double refund**: never generate a fresh `refund_order_id` on an ambiguous timeout retry or allow concurrent worker + admin HTTP calls for the same `order_id`.
- **Bypassing service boundaries**: no direct DB access across `order_db` and `payment_db`; no BFF-to-`payment-service` admin bypass that skips `order-service` RBAC or leaves `order_db` stale on immediate UI reload.
- **Leaking gateway internals to shoppers**: `ecommerce-fe` never displays raw MoMo error messages or stack traces.

## 3. Architectural Constraints

- **ADR-001**: Strict service ownership—`payment-service` owns `payment_db`, MoMo credentials/signing, and refund execution; `order-service` owns `order_db`, order payment status projection, and admin order authorization.
- **ADR-002**: Transactional Outbox for `PaymentRefunded` (`payment.refunded`) and `PaymentRefundFailed` (`payment.refund_failed`), with idempotent `processed_events` deduplication in `order-service`.
- **ADR-005**: Late payment on cancelled order converges to `CANCELLED` + `REFUND_REQUIRED` -> `REFUNDED` (or `REFUND_FAILED`).
- **ADR-014**: Never embed MoMo secrets/logic in `order-service`; never hold DB transactions open across gRPC/HTTPS/RabbitMQ I/O (use 2-TX pattern: TX #A claim/mark `PROCESSING` -> commit -> HTTPS call -> TX #B persist outcome + outbox).
- **Existing timeout budget**: `BFF_ORDER_GRPC_TIMEOUT` (`15s`) > `PAYMENT_GRPC_TIMEOUT` (`12s`) > `MOMO_READ_TIMEOUT` (`10s`) > `MOMO_CONNECT_TIMEOUT` (`3s`).

## 4. Approaches Considered

| Approach | Description | Trade-offs |
| --- | --- | --- |
| **Approach 1 (Chosen)** | Dedicated `payment_refunds` table + background `RefundWorker` (max 3 auto attempts) in `payment-service`; `REFUND_FAILED` & `REFUNDED` statuses in `order-service`; synchronous manual retry via `admin-fe -> bff-service -> order-service (AdminRetryRefund) -> payment-service (RetryRefund)`. | **Pros:** Immediate synchronous result (success or exact MoMo failure) in `admin-fe`; 1 click = 1 attempt; reuses `order.admin.update` RBAC in `order-service` without adding an `identity-service` dependency to `payment-service`; `order_db` updates immediately so UI reload never shows stale `REFUND_FAILED`. **Cons:** Adds 1 additive RPC to `PaymentService` and 1 additive RPC to `OrderService`; admin request waits up to MoMo timeout (`~10s`). |
| **Approach 2 (Rejected)** | Same auto-retry worker, but manual admin retry only resets `REFUND_FAILED -> REFUND_REQUIRED` in `order-service` and re-emits `RefundRequested` over RabbitMQ. | **Pros:** No new `PaymentService` RPC; fast `200`/`202` response. **Cons:** Admin gets no immediate outcome and must poll/refresh to learn whether MoMo is healthy; harder to distinguish 1 manual click from another 3-attempt auto loop. |
| **Approach 3 (Dropped)** | BFF calls `payment-service` directly for admin manual refund. | Ruled out because `payment-service` has no `identity-service` RBAC client and mutating `payment_db` ahead of `order_db` causes immediate UI desynchronization when `admin-fe` reloads from `order-service`. |

## 5. Surface Map & Dependency Order

Build/implementation dependency order:
`contracts` -> `payment-service` -> `order-service` -> `bff-service` -> `admin-fe` & `ecommerce-fe` (parallel) -> `infra/docs`

**Production rollout order (design call to prevent silent RabbitMQ drops):**
Deploy `order-service` (with Flyway `V3` and durable RabbitMQ bindings for `payment.refunded` and `payment.refund_failed`, while `AdminRetryRefund` is not yet exposed by BFF and gRPC client channels are lazy) **before** `payment-service` starts emitting those routing keys, then `bff-service`, then `admin-fe` & `ecommerce-fe`. Because `payment-service/internal/events/relay.go` publishes with `mandatory = false`, binding the queue first guarantees zero dropped refund events during rollout.

**Backfill policy for pre-existing `REFUND_REQUIRED` rows (user-approved Option 1):**
Migrations `000002` (`payment_db`) and `V3` (`order_db`) backfill pre-existing `REFUND_REQUIRED` records to `REFUND_FAILED` (rather than `PENDING`), so they never trigger unintended automatic payouts on deploy while immediately appearing on `admin-fe` with the **"Hoàn tiền thủ công"** action for operator-controlled processing:
- `payment_db` (`000002`): for every `payments` row with `status = 'REFUND_REQUIRED'`, `amount_minor > 0`, and `momo_trans_id <> ''`, insert a `payment_refunds` row with deterministic alphanumeric `refund_order_id` (`'REFUND' || upper(md5(order_id))`), `refund_request_id` (`'REFREQ' || substr(upper(md5(order_id || ':req')), 1, 24)`), frozen `description`, `status = 'FAILED'`, `attempts = 3`, `max_auto_attempts = 3`, `next_attempt_at = NULL`, `last_error = 'Đơn cần hoàn tiền phát sinh trước khi bật hoàn tiền tự động'`, and update `payments.status = 'REFUND_FAILED'`.
- `order_db` (`V3`): for every `orders` row with `status = 'CANCELLED'`, `payment_method = 'MOMO'`, and `payment_status = 'REFUND_REQUIRED'`, update `payment_status = 'REFUND_FAILED'` and `refund_last_error = 'Đơn cần hoàn tiền phát sinh trước khi bật hoàn tiền tự động'`.

| Surface | Impact Class | Owner | Paths Touched | Verification Gate |
| --- | --- | --- | --- | --- |
| `contracts` | `CONTRACT` (additive) | Contracts | `contracts/proto/payment/v1/payment.proto`, `contracts/proto/order/v1/order.proto`, generated `contracts/go/`, `contracts/java/src/main/generated/` | `make verify SERVICE=contracts` |
| `payment-service` | `DIRECT`, `INFRA` (DB migration, env), `TEST` | Payment | `services/payment-service/internal/{config,domain,momo,store,events,grpc}/`, `services/payment-service/cmd/server/main.go`, `services/payment-service/internal/store/migrations/000002_create_payment_refunds.up.sql`, `services/payment-service/AGENTS.md` | `make verify SERVICE=payment-service` |
| `order-service` | `DIRECT`, `DEPENDENT` (Payment proto & events), `INFRA` (Flyway V3, RabbitMQ bindings), `TEST` | Order | `services/order-service/src/main/resources/db/migration/V3__add_refund_payment_statuses.sql`, `services/order-service/src/main/java/com/ecommerce/order/{domain,persistence,config,integration,application,grpc}/`, `services/order-service/src/test/java/...` | `make verify SERVICE=order-service` |
| `bff-service` | `DIRECT`, `DEPENDENT` (Order proto), `CONTRACT` (REST), `TEST` | BFF | `services/bff-service/internal/order/client.go`, `services/bff-service/internal/http/{orders,admin_orders}*.go`, `services/bff-service/cmd/server/main_test.go` | `make verify SERVICE=bff-service` |
| `admin-fe` | `DIRECT`, `DEPENDENT` (BFF REST), `TEST` | Admin FE | `admin-fe/src/lib/{types,order-status,api}*.ts`, `admin-fe/src/pages/{order-detail-sheet,orders-page}*.tsx` | `make verify SERVICE=admin-fe` |
| `ecommerce-fe` | `DIRECT`, `DEPENDENT` (BFF REST), `TEST` | Storefront FE | `ecommerce-fe/src/lib/{types,order-status}.ts`, `ecommerce-fe/src/pages/{order-detail-page,payment-result-page}*.tsx` | `make verify SERVICE=ecommerce-fe` |
| `deployments` & `scripts` & `docs` | `INFRA` (env defaults/docs) | Repo Ops | `.env.example`, `.env.production.example`, `deployments/docker/compose.production.yml`, `scripts/start-local/main.go`, `docs/integrations/momo-payment.md` | `go test ./scripts/...` & `make check-boundaries` |

**Dependents inspected and intentionally unchanged:**
- `deployments/kong/kong.yaml` & `kong-production.yaml`: existing `/api/v1/admin/orders` prefix route already proxies `POST /api/v1/admin/orders/{orderId}/refund` to `bff-service` with JWT verification.
- `services/identity-service/`: reuses existing `order.admin.update` permission.
- `services/catalog-service/`, `services/inventory-service/`, `services/cart-service/`: do not import `payment/v1` or `order/v1` or consume payment refund events.

## 6. Detailed Design

### 6.1. Contracts (All Additive)

#### `contracts/proto/payment/v1/payment.proto`
Add RPC and messages:
```protobuf
service PaymentService {
  rpc CreatePaymentSession(CreatePaymentSessionRequest) returns (CreatePaymentSessionResponse);
  rpc HandleMomoCallback(HandleMomoCallbackRequest) returns (HandleMomoCallbackResponse);
  rpc RetryRefund(RetryRefundRequest) returns (RetryRefundResponse);
}

message RetryRefundRequest {
  string order_id = 1;
  string reason = 2;
}

message RetryRefundResponse {
  string order_id = 1;
  string payment_status = 2; // "REFUNDED"
  string refund_trans_id = 3;
  string message = 4;
}
```
Error reasons under domain `ecommerce.payment.v1`:
- `PAYMENT_REFUND_NOT_FOUND` (`NotFound`)
- `REFUND_AUTO_RETRY_IN_PROGRESS` (`FailedPrecondition`) — refund is still in automatic retry (`PENDING`, attempts < 3)
- `REFUND_IN_PROGRESS` (`Aborted`) — a refund HTTP call is currently executing (`PROCESSING`)
- `PAYMENT_GATEWAY_REJECTED` (`FailedPrecondition`) — MoMo returned `resultCode != 0` or invalid `transId`
- `PAYMENT_GATEWAY_UNAVAILABLE` (`Unavailable`) — MoMo unreachable, non-2xx, timeout, or missing credentials

#### `contracts/proto/order/v1/order.proto`
Add optional fields to `OrderDetail` and new RPC:
```protobuf
// Inside OrderDetail:
  string refund_trans_id = 22;
  // Admin-only fields: omitted (empty) on shopper-facing GetOrder / ListOrders responses.
  string refund_last_error = 23;
  string refunded_at = 24;
  string refund_actor_type = 25; // "SYSTEM" (automatic) or "ADMIN" (manual fallback)
  string refund_reason = 26;

// Inside OrderService:
  rpc AdminRetryRefund(AdminRetryRefundRequest) returns (AdminRetryRefundResponse);

message AdminRetryRefundRequest {
  string user_id = 1;
  string order_id = 2;
  string reason = 3;
}

message AdminRetryRefundResponse {
  OrderDetail order = 1;
  repeated string allowed_next_statuses = 2;
}
```
Error reasons under domain `ecommerce.order.v1`:
- Existing: `ORDER_NOT_FOUND`, `ORDER_PERMISSION_DENIED`, `INVALID_PARAMETER`, `PAYMENT_GATEWAY_REJECTED`, `PAYMENT_GATEWAY_UNAVAILABLE`
- New: `ORDER_REFUND_NOT_ELIGIBLE` (`FailedPrecondition`), `REFUND_AUTO_RETRY_IN_PROGRESS` (`FailedPrecondition`), `REFUND_IN_PROGRESS` (`Aborted`)

#### RabbitMQ Events (`ecommerce.events` topic exchange)
Both events follow the existing JSON envelope (`eventId`, `eventType`, `eventVersion: 1`, `occurredAt`, `aggregateId`, `correlationId`, `data`) published via `payment_db.outbox_events`:
- **`PaymentRefunded`** (routing key `payment.refunded`):
  ```json
  {
    "orderId": "<uuid>",
    "orderCode": "ORD-...",
    "userId": "<uuid>",
    "momoOrderId": "<orig>",
    "momoTransId": "<orig-trans-id>",
    "refundOrderId": "REFUND...",
    "refundTransId": "<momo-refund-trans-id>",
    "amountMinor": 150000,
    "currency": "VND",
    "resultCode": 0,
    "message": "Successful."
  }
  ```
- **`PaymentRefundFailed`** (routing key `payment.refund_failed`): emitted **only** when automatic retries are exhausted (`attempts >= max_auto_attempts`, i.e., after 3 failed attempts):
  ```json
  {
    "orderId": "<uuid>",
    "orderCode": "ORD-...",
    "userId": "<uuid>",
    "momoOrderId": "<orig>",
    "momoTransId": "<orig-trans-id>",
    "refundOrderId": "REFUND...",
    "amountMinor": 150000,
    "currency": "VND",
    "attempts": 3,
    "resultCode": -1,
    "message": "<human/gateway error summary>"
  }
  ```

#### Public Admin REST Endpoint (`bff-service`)
- `POST /api/v1/admin/orders/{orderId}/refund`
- Request body (optional JSON): `{ "reason": "..." }` (empty body allowed; unknown fields rejected).
- Response (`200 OK`): standard envelope `{ "data": { "order": OrderDetail, "allowedNextStatuses": [...] }, "message": "Hoàn tiền đơn hàng thành công" }`.
- `OrderDetail` JSON gains optional `refundTransId` and `refundLastError` (omitted when empty).

---

### 6.2. `payment-service` Design

#### Configuration (`internal/config/config.go`)
- `MOMO_REFUND_ENDPOINT`: optional env var. Resolution order:
  1. Explicit `MOMO_REFUND_ENDPOINT` if non-empty.
  2. If `MOMO_API_ENDPOINT` ends with `/create`, replace suffix `/create` with `/refund`.
  3. Fallback `https://test-payment.momo.vn/v2/gateway/api/refund`.
- `PAYMENT_REFUND_MAX_AUTO_ATTEMPTS`: positive integer, default `3`.
- `PAYMENT_REFUND_POLL_INTERVAL`: positive duration, default `2s`.
- `PAYMENT_REFUND_RETRY_BACKOFF`: positive duration, default `10s` (backoff after attempt `k`: `base * 2^(k-1)`, so attempt 1 failure waits `10s`, attempt 2 failure waits `20s`, attempt 3 failure marks `FAILED`).

#### MoMo v2 Refund Signer & Client (`internal/momo/signer.go`, `internal/momo/client.go`)
- `SignRefundRequest(p RefundSignParams, secret string) string`:
  ```text
  accessKey=%s&amount=%d&description=%s&orderId=%s&partnerCode=%s&requestId=%s&transId=%d
  ```
  Signed with `HMAC-SHA256` hex-encoded, identical to `SignCreateRequest`.
- `Client.Refund(ctx, RefundRequest) (*RefundResponse, error)`:
  - Builds description from the frozen `req.Description` stored at refund creation (e.g., `"Hoan tien don hang " + orderCode`, truncated to <= 255 runes). Manual admin `reason` is saved locally for audit only and does **not** mutate the signed MoMo payload (design call so retries with the same `requestId` always carry an identical HMAC signature).
  - **Mock mode (`MockEnabled == true`)**: returns immediately with `ResultCode: 0`, `Message: "Successful."`, `RefundTransID: fmt.Sprintf("mock-refund-%d", time.Now().UnixNano())`.
  - **Real mode (`MockEnabled == false`)**:
    - Validates `AccessKey` and `SecretKey` non-empty (else `ErrGatewayUnavailable`).
    - Parses `req.OrigTransID` as positive `int64` via `strconv.ParseInt` (else returns `&GatewayRejectedError{ResultCode: -1, Message: "invalid original momo transId"}`).
    - `POST` JSON `{ partnerCode, orderId, requestId, amount, transId, lang: "vi", description, signature }` to `RefundEndpoint`.
    - Non-2xx or transport/timeout/decode error -> `ErrGatewayUnavailable` (ambiguous failure).
    - `parsed.ResultCode != 0` -> `&GatewayRejectedError{ResultCode, Message}`.
    - `parsed.ResultCode == 0` -> parses `transId` from JSON (supporting both JSON number and string) and returns `RefundResponse`.

#### MoMo Retry Identity Policy (Design Call)
- **Ambiguous failures** (HTTP timeout, network/EOF error, HTTP 5xx, process crash, or MoMo processing codes `7000` / `9000`): **keep `refund_order_id` and `refund_request_id` fixed** across retries so MoMo treats the retry idempotently and never double-refunds.
- **Definite gateway rejection** (HTTP 200 with a definite non-zero `resultCode` other than `7000`/`9000`, e.g., temporary MoMo maintenance `10` or system error `99`): mark `rotate_ids_on_next_attempt = true` so the next attempt (auto or manual) mints a fresh alphanumeric `refund_order_id` + `refund_request_id` in TX #A before calling MoMo, preventing permanent duplicate-`orderId` rejection once MoMo recovers.
- **Known limitation (Non-Goal)**: if an ambiguous timeout actually succeeded on MoMo and MoMo later rejects the fixed-ID retry as already refunded, the order remains in `REFUND_FAILED` for operator reconciliation outside the app (since `/v2/gateway/api/refund/query` and offline mark-refunded are explicit Non-Goals).

#### Database Migration (`000002_create_payment_refunds.up.sql`)
- Drop & recreate `payments_status_check` to include `'REFUNDED'` and `'REFUND_FAILED'`.
- Create `payment_refunds`:
  - `order_id text PRIMARY KEY REFERENCES payments(order_id) ON DELETE CASCADE`
  - `refund_order_id text NOT NULL UNIQUE` (alphanumeric, e.g. `REFUND` + 32 hex chars)
  - `refund_request_id text NOT NULL UNIQUE` (alphanumeric, e.g. `REFREQ` + 24 hex chars)
  - `amount_minor bigint NOT NULL CHECK (amount_minor >= 0)`
  - `currency text NOT NULL DEFAULT 'VND'`
  - `momo_trans_id text NOT NULL DEFAULT ''`
  - `refund_trans_id text NOT NULL DEFAULT ''`
  - `description text NOT NULL DEFAULT ''` (frozen MoMo refund description)
  - `reason text NOT NULL DEFAULT ''` (cancellation / latest manual admin audit reason)
  - `rotate_ids_on_next_attempt boolean NOT NULL DEFAULT false`
  - `status text NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCEEDED', 'FAILED'))`
  - `attempts integer NOT NULL DEFAULT 0`
  - `max_auto_attempts integer NOT NULL DEFAULT 3`
  - `next_attempt_at timestamptz`
  - `result_code integer`
  - `result_message text NOT NULL DEFAULT ''`
  - `last_error text NOT NULL DEFAULT ''`
  - `created_at timestamptz NOT NULL DEFAULT now()`
  - `updated_at timestamptz NOT NULL DEFAULT now()`
  - Partial index `ix_payment_refunds_due ON payment_refunds (next_attempt_at, created_at) WHERE status IN ('PENDING', 'PROCESSING') AND next_attempt_at IS NOT NULL`.
  - Backfill existing `payments` rows with `status = 'REFUND_REQUIRED'`, `amount_minor > 0`, and `momo_trans_id <> ''` into `payment_refunds` with `status = 'FAILED'`, `attempts = 3`, `next_attempt_at = NULL`, and transition those `payments` rows to `status = 'REFUND_FAILED'`.

#### Refund Request Ingestion & State Transitions (`internal/store/store.go`, `internal/events/consumer.go`)
- Update `Consumer` handler on `RoutingKeyRefundRequested`: calls `store.RequestRefund(ctx, orderID, reason, maxAutoAttempts)` and wakes `RefundWorker.Notify()` when a refund job is queued.
- `RequestRefund` (single atomic TX):
  - If no `payments` row exists: inserts `REFUND_REQUIRED` tombstone (same safety as before) without creating a `payment_refunds` row (cannot refund without captured payment).
  - Locks existing `payments` row (`FOR UPDATE`).
  - If `payments.status` is already `REFUNDED` or `REFUND_FAILED`: idempotent no-op (do not re-enqueue auto-retries on duplicate RabbitMQ deliveries!).
  - Otherwise sets `payments.status = 'REFUND_REQUIRED'`, preserves `cancel_reason`.
  - If `amount_minor > 0` and `momo_trans_id != ''`: inserts into `payment_refunds` with `status = 'PENDING'`, `attempts = 0`, `max_auto_attempts`, `next_attempt_at = now()`, `ON CONFLICT (order_id) DO NOTHING`.
- Also update `RecordMomoCallback`: if a late success callback arrives when `payments.status == 'REFUND_REQUIRED'` AND `payments.momo_trans_id == ''` (meaning `RefundRequested` arrived slightly ahead of the callback), save `momo_trans_id`, ensure `payment_refunds` is created if `amount_minor > 0`, and emit `PaymentSucceeded`. If `payments.status` is already `REFUNDED` or `REFUND_FAILED` (or `REFUND_REQUIRED` with `momo_trans_id != ''`), treat callback as idempotent no-op.
- Also ensure `BeginCreateSession`, `CompleteCreateSession`, and `CancelPayment` treat `REFUNDED` and `REFUND_FAILED` as terminal (like `REFUND_REQUIRED`).

#### Background `RefundWorker` (`internal/events/refund_worker.go` or `internal/refund/worker.go`)
- Runs in a goroutine started from `cmd/server/main.go` (even when `RABBITMQ_URL` is empty if desired, though normally alongside `OutboxRelay` and `Consumer`; starting it unconditionally against DB + MoMo client is clean and makes testing easy).
- Wakeup channel `Notify()` + ticker (`PAYMENT_REFUND_POLL_INTERVAL`, default `2s`).
- Each cycle:
  1. **Stale lease recovery**: recovers any `PROCESSING` rows whose `next_attempt_at <= now()` (e.g., process crashed mid-HTTP):
     - If `attempts < max_auto_attempts`: reset `status = 'PENDING'`, `next_attempt_at = now()`, `last_error = 'previous attempt timed out or crashed'`.
     - If `attempts >= max_auto_attempts`: mark `payment_refunds.status = 'FAILED'`, `next_attempt_at = NULL`, `payments.status = 'REFUND_FAILED'`, insert `PaymentRefundFailed` outbox event, notify outbox relay.
  2. **Claim due refunds (TX #A)**: selects up to batch size (e.g. `10`) rows `WHERE status = 'PENDING' AND attempts < max_auto_attempts AND next_attempt_at <= now() ORDER BY next_attempt_at ASC, created_at ASC LIMIT $1 FOR UPDATE SKIP LOCKED`, rotates `refund_order_id` + `refund_request_id` if `rotate_ids_on_next_attempt = true` (resetting the flag to `false`), updates `status = 'PROCESSING'`, `attempts = attempts + 1`, `next_attempt_at = now() + interval '2 minutes'` (lease), commits TX #A.
  3. **Execute MoMo Refund HTTP call** outside DB TX for each claimed job (with per-call timeout derived from `MOMO_READ_TIMEOUT + 5s`).
  4. **Complete attempt (TX #B)** with `context.WithoutCancel(ctx)` + `3s` timeout:
     - On **success**: update `payment_refunds` (`status = 'SUCCEEDED'`, `refund_trans_id`, `result_code = 0`, `result_message`, `last_error = ''`, `rotate_ids_on_next_attempt = false`, `next_attempt_at = NULL`), update `payments.status = 'REFUNDED'`, insert `PaymentRefunded` outbox event, commit, call `outboxRelay.Notify()`.
     - On **failure**: set `rotate_ids_on_next_attempt = true` iff failure was a definite non-ambiguous MoMo rejection (`GatewayRejectedError` with `resultCode` not in `{7000, 9000}`):
       - If `attempts < max_auto_attempts`: compute backoff `base * 2^(attempts-1)`, update `payment_refunds` (`status = 'PENDING'`, `next_attempt_at = now() + backoff`, `result_code`, `result_message`, `last_error`, `rotate_ids_on_next_attempt`), commit.
       - If `attempts >= max_auto_attempts`: update `payment_refunds` (`status = 'FAILED'`, `next_attempt_at = NULL`, `result_code`, `result_message`, `last_error`, `rotate_ids_on_next_attempt`), update `payments.status = 'REFUND_FAILED'`, insert `PaymentRefundFailed` outbox event, commit, call `outboxRelay.Notify()`.

#### Manual Admin Retry RPC (`internal/grpc/server.go`)
- `RetryRefund(ctx, req)`:
  1. Validate non-empty `order_id`.
  2. **TX #A (`BeginManualRefund`)**: lock `payment_refunds` joined with `payments` (`FOR UPDATE`).
     - Not found -> `PAYMENT_REFUND_NOT_FOUND`.
     - `status == 'SUCCEEDED'` -> return immediately (`PaymentStatus: "REFUNDED"`, `RefundTransId`, `Message: "Already refunded"`).
     - `status == 'PENDING'` -> `REFUND_AUTO_RETRY_IN_PROGRESS`.
     - `status == 'PROCESSING'` -> `REFUND_IN_PROGRESS`.
     - `status == 'FAILED'` -> if `rotate_ids_on_next_attempt = true`, mint fresh alphanumeric `refund_order_id` + `refund_request_id` and reset the flag to `false`; update `status = 'PROCESSING'`, `attempts = attempts + 1`, `next_attempt_at = now() + interval '2 minutes'`, store optional audit `reason` without changing frozen `description`, commit TX #A.
  3. **Call MoMo Refund HTTP** outside DB TX using the claimed `refund_order_id`, `refund_request_id`, and frozen `description`.
  4. **TX #B (`CompleteManualRefund`)** with `context.WithoutCancel(ctx)` + `3s` timeout:
     - On success: set `payment_refunds.status = 'SUCCEEDED'`, `payments.status = 'REFUNDED'`, insert `PaymentRefunded` outbox event (idempotent safety net), notify outbox relay, return `RetryRefundResponse`.
     - On failure: set `payment_refunds.status = 'FAILED'`, `next_attempt_at = NULL`, save `result_code`, `result_message`, `last_error`, update `rotate_ids_on_next_attempt` per retry identity policy, keep `payments.status = 'REFUND_FAILED'`, return mapped gRPC error (`PAYMENT_GATEWAY_REJECTED` or `PAYMENT_GATEWAY_UNAVAILABLE`) with the failure detail.

---

### 6.3. `order-service` Design

#### Database Migration (`V3__add_refund_payment_statuses.sql`)
- Drop & recreate `orders_payment_status_check` to allow:
  `'UNPAID', 'PENDING', 'PAID', 'FAILED', 'REFUND_REQUIRED', 'REFUNDED', 'REFUND_FAILED'`.
- Add dedicated refund audit columns to `orders` (design call: keeps `order_status_history` strictly for `OrderStatus` transitions so neither storefront nor admin renders duplicate `"Đã hủy"` timeline entries):
  - `refund_trans_id VARCHAR(64) NOT NULL DEFAULT ''`
  - `refund_last_error VARCHAR(255) NOT NULL DEFAULT ''`
  - `refunded_at TIMESTAMPTZ`
  - `refund_actor_type VARCHAR(16) NOT NULL DEFAULT '' CHECK (refund_actor_type IN ('', 'SYSTEM', 'ADMIN'))`
  - `refund_actor_id UUID`
  - `refund_reason VARCHAR(128) NOT NULL DEFAULT ''`
- Backfill existing `orders` rows with `status = 'CANCELLED'`, `payment_method = 'MOMO'`, and `payment_status = 'REFUND_REQUIRED'` to `payment_status = 'REFUND_FAILED'` with `refund_last_error = 'Đơn cần hoàn tiền phát sinh trước khi bật hoàn tiền tự động'`.

#### Domain & Messaging
- `PaymentStatus` enum: add `REFUNDED`, `REFUND_FAILED`.
- `RabbitMqConfig`:
  - Add constants `RK_PAYMENT_REFUNDED = "payment.refunded"` and `RK_PAYMENT_REFUND_FAILED = "payment.refund_failed"`.
  - Bind both routing keys to `order.payment-events` queue on `ecommerce.events`.
- `PaymentEventConsumer`:
  - **`payment.refunded` / `PaymentRefunded`**:
    - If `order.paymentStatus` is `REFUND_REQUIRED` or `REFUND_FAILED` (or `PAID` if refund event arrived ahead of local update): set `paymentStatus = REFUNDED`, save `refundTransId`, clear `refundLastError = ""`, set `refundedAt = now` (if null) and `refundActorType = "SYSTEM"` (if empty), save.
    - If already `REFUNDED`: idempotent no-op (preserves `ADMIN` audit fields if manual retry already completed).
  - **`payment.refund_failed` / `PaymentRefundFailed`**:
    - Only if `order.paymentStatus == REFUND_REQUIRED`: set `paymentStatus = REFUND_FAILED`, set `refundLastError` (trimmed to 255 chars), save.
    - Never downgrade `REFUNDED` back to `REFUND_FAILED` on out-of-order delivery!

#### Shopper vs Admin Projection (Design Call)
- `OrderProtoMapper.toOrderDetail(...)` defaults to **customer-safe** mapping: omits `refund_last_error`, `refund_actor_type`, and `refund_reason` (leaving only `payment_status`, `refund_trans_id`, and `refunded_at`).
- `OrderProtoMapper.toAdminOrderDetail(...)` (used by `OrderAdminQueryService` and `OrderLifecycleService` admin endpoints) populates the full admin fields (`refund_last_error`, `refund_actor_type`, `refund_reason`, `refunded_at`, `refund_trans_id`). This enforces Anti-Goal #3 (no gateway error leaks to shoppers) directly at the backend API boundary.

#### Synchronous Admin Manual Refund (`OrderLifecycleService.retryRefundByAdmin`)
1. Validate UUIDs (`user_id`, `order_id`) and trim optional `reason` (max 128 chars).
2. Check RBAC `order.admin.update` via `AuthorizationGrpcAdapter` (outside DB TX).
3. Short read TX to inspect order preconditions:
   - Not found -> `ORDER_NOT_FOUND`.
   - `status != CANCELLED` or `paymentMethod != MOMO` -> `ORDER_REFUND_NOT_ELIGIBLE`.
   - `paymentStatus == REFUND_REQUIRED` -> `REFUND_AUTO_RETRY_IN_PROGRESS`.
   - `paymentStatus != REFUND_FAILED` (e.g. `PAID`, `UNPAID`, `PENDING`, `FAILED`, `REFUNDED`) -> `ORDER_REFUND_NOT_ELIGIBLE`.
4. Call `PaymentGrpcAdapter.retryRefund(orderId, reason)` over private gRPC (`12s` timeout) **outside DB TX**.
   - Maps `payment-service` errors cleanly (`PAYMENT_REFUND_NOT_FOUND` -> `ORDER_REFUND_NOT_ELIGIBLE`, `REFUND_AUTO_RETRY_IN_PROGRESS` -> `REFUND_AUTO_RETRY_IN_PROGRESS`, `REFUND_IN_PROGRESS` -> `REFUND_IN_PROGRESS`, `PAYMENT_GATEWAY_REJECTED` -> `PAYMENT_GATEWAY_REJECTED` preserving MoMo message, `PAYMENT_GATEWAY_UNAVAILABLE` -> `PAYMENT_GATEWAY_UNAVAILABLE`).
   - If the gRPC call fails with `PAYMENT_GATEWAY_REJECTED` or `PAYMENT_GATEWAY_UNAVAILABLE`, persist the latest truncated error into `orders.refund_last_error` in a short TX before rethrowing, so the admin sees the newest error even after reload.
5. On gRPC success (`REFUNDED`):
   - Short write TX: lock order (`findByIdForUpdate`), set `paymentStatus = REFUNDED`, save `refundTransId`, clear `refundLastError = ""`, set `refundedAt = now`, `refundActorType = "ADMIN"`, `refundActorId = adminId`, `refundReason = cleanReason`, set `updatedAt = now`, and return `AdminRetryRefundResponse` with `OrderProtoMapper.toAdminOrderDetail(...)` and `allowedNextStatuses`.

---

### 6.4. `bff-service` Design

- `internal/order/client.go`: add `AdminRetryRefund(ctx, req) (*orderv1.AdminRetryRefundResponse, error)` to `Client` interface and gRPC implementation.
- `internal/http/orders.go`:
  - Add optional `RefundTransID`, `RefundLastError`, `RefundedAt`, `RefundActorType`, and `RefundReason` (`omitempty`) to `orderDetailJSON` and `newOrderDetailJSON` (customer endpoints naturally omit admin-only fields because `order-service` leaves them empty on customer RPCs).
  - Update `writeOrderServiceError` to map `ORDER_REFUND_NOT_ELIGIBLE`, `REFUND_AUTO_RETRY_IN_PROGRESS`, `REFUND_IN_PROGRESS` to `409 Conflict` (keeping stable `code` and `message`).
- `internal/http/admin_orders.go`:
  - Register `POST /api/v1/admin/orders/{orderId}/refund` wrapped in `correlateTrace(RequireCaller(...))`.
  - Accept optional JSON body `{ "reason": "..." }` (allow empty body / `io.EOF` like customer cancel).
  - Return `200 OK` with `adminOrderDataJSON` and message `"Hoàn tiền đơn hàng thành công"`.

---

### 6.5. Frontend Design (`admin-fe` & `ecommerce-fe`)

#### `admin-fe`
- `types.ts`: extend `OrderPaymentStatus` with `"REFUNDED" | "REFUND_FAILED"`; add optional `refundTransId?: string`, `refundLastError?: string`, `refundedAt?: string`, `refundActorType?: "SYSTEM" | "ADMIN"`, and `refundReason?: string` to `AdminOrder`.
- `order-status.ts`:
  - `PAYMENT_STATUS_LABELS`: `REFUNDED: "Đã hoàn tiền"`, `REFUND_FAILED: "Hoàn tiền thất bại"`.
  - `paymentStatusBadgeVariant`: `REFUNDED -> "success"`, `REFUND_FAILED -> "destructive"`.
- `api.ts`: add `retryAdminOrderRefund(orderId: string, reason?: string): Promise<AdminOrderView>` calling `POST /admin/orders/${orderId}/refund`.
- `order-detail-sheet.tsx`:
  - When `order.paymentStatus === "REFUND_FAILED"`: render an alert callout explaining that automatic refund failed after 3 attempts and showing `order.refundLastError` (if present).
  - Show button **"Hoàn tiền thủ công"** **only** when `canUpdate && order.status === "CANCELLED" && order.paymentMethod === "MOMO" && order.paymentStatus === "REFUND_FAILED"`.
  - Clicking the button opens a lightweight confirmation dialog (with optional reason note) and disables the button while in-flight (`Đang hoàn tiền…`) to prevent double-submission. On success, updates the sheet's `view` immediately and calls `onMutated()`. On failure, refreshes the order detail to show the latest `refundLastError` (toast is already handled by `api.ts` interceptor).
  - When `order.paymentStatus === "REFUNDED"`: display refund metadata in the payment details section (`refundTransId`, `refundedAt`, whether executed automatically by `Hệ thống` or manually by `Quản trị viên`, and `refundReason` if any).

#### `ecommerce-fe`
- `types.ts`: extend `PaymentStatus` with `"REFUNDED" | "REFUND_FAILED"`; add optional `refundTransId?: string` to `OrderDetail` (note: do not render `refundLastError` on storefront).
- `order-status.ts`:
  - `getOrderStatusMeta`: when `status === "CANCELLED" && paymentStatus === "REFUNDED"` -> `"Đã hủy • Đã hoàn tiền"` (emerald badge); when `status === "CANCELLED" && (paymentStatus === "REFUND_REQUIRED" || paymentStatus === "REFUND_FAILED")` -> `"Đã hủy • Đang hoàn tiền"` (amber badge).
- `order-detail-page.tsx` & `payment-result-page.tsx`:
  - `REFUNDED`: show `"Đã hoàn tiền"` payment status label and a clear green confirmation banner that the MoMo payment has been refunded.
  - `REFUND_REQUIRED` / `REFUND_FAILED`: show friendly amber banner that the refund is being processed (do not expose technical failure details to end customers).

---

### 6.6. Configuration, Local Scripts & Docs
- Update `.env.example`, `.env.production.example`, `deployments/docker/compose.production.yml`, and `scripts/start-local/main.go` with `MOMO_REFUND_ENDPOINT` (optional/documented) and refund retry defaults.
- Update `docs/integrations/momo-payment.md` with a dedicated **Hoàn tiền MoMo v2 (Tự động 3 lần + Thủ công Admin)** section covering payload, HMAC signature, retry behavior, and Sandbox notes.
- Update `services/payment-service/AGENTS.md` to reflect `REFUNDED`, `REFUND_FAILED`, `payment_refunds`, `RetryRefund` RPC, and `PaymentRefunded` / `PaymentRefundFailed` events.

## 7. What's Left to the Builder
- Exact UI dialog/inline confirmation styling for the **"Hoàn tiền thủ công"** button in `admin-fe` (consistent with existing `OrderStatusDialog` / shadcn `Dialog` components).
- Exact Go helper structure inside `payment-service` (e.g., placing `RefundWorker` in `internal/events` vs `internal/refund`).
- Exact unit/integration test helper fixtures and mock HTTP servers (`httptest.NewServer` in Go, Mockito/Testcontainers in Java, Vitest/Testing Library in React).

## 8. Acceptance Criteria
1. **Mock auto-refund**: With `PAYMENT_MOMO_MOCK_ENABLED=true`, cancelling a paid MoMo order emits `RefundRequested`, `payment-service` automatically refunds via mock, emits `PaymentRefunded`, and `order-service` transitions `orders.payment_status` to `REFUNDED`.
2. **Real MoMo v2 signature & payload**: `SignRefundRequest` and `Client.Refund` produce the exact HMAC-SHA256 signature (`accessKey&amount&description&orderId&partnerCode&requestId&transId`), send a unique alphanumeric `orderId` distinct from the original payment's `orderId`, send numeric `int64` `transId`, and succeed iff HTTP 2xx and `resultCode == 0`.
3. **3 auto-retries then `REFUND_FAILED`**: When MoMo fails (HTTP 5xx, timeout, or `resultCode != 0`), `payment-service` retries up to 3 total attempts—reusing the same `refund_order_id` and `refund_request_id` on ambiguous failures (timeout/5xx/crash/`7000`/`9000`) and rotating them only after a definite non-ambiguous MoMo `resultCode != 0` rejection, while keeping MoMo `description` frozen. After attempt 3 fails, auto-retry stops, `payment_refunds.status = FAILED`, `payments.status = REFUND_FAILED`, `PaymentRefundFailed` is emitted, and `order-service` updates `orders.payment_status = REFUND_FAILED` with `refund_last_error`.
4. **Strict gating of manual refund**: Both backend (`order-service` + `payment-service`) and `admin-fe` allow manual refund **only** when the order is `CANCELLED` + `MOMO` + `REFUND_FAILED` (after 3 failed auto attempts) and the caller has `order.admin.update`. Calling manual refund while auto-retry is still `PENDING` returns `409 REFUND_AUTO_RETRY_IN_PROGRESS`.
5. **Manual refund success & failure**: When Admin triggers manual refund on a `REFUND_FAILED` order:
   - If MoMo succeeds: `payment-service` marks `REFUNDED` + emits `PaymentRefunded`, `order-service` immediately updates `order_db` to `REFUNDED` (with `refund_actor_type = ADMIN`, `refund_actor_id`, `refunded_at`, `refund_reason`) and returns the updated order to `admin-fe`. Subsequent duplicate `PaymentRefunded` event is an idempotent no-op.
   - If MoMo still fails: `order_db` stays `REFUND_FAILED` with updated `refund_last_error` and `admin-fe` displays the error immediately.
6. **Frontend display & API privacy**: `admin-fe` displays `REFUNDED` and `REFUND_FAILED` badges, refund metadata (`refundTransId`, `refundActorType`, `refundedAt`, `refundReason`), failure reason, and manual refund action only for `REFUND_FAILED` orders when permitted; `ecommerce-fe` displays `REFUNDED` clearly and keeps `REFUND_FAILED` customer-friendly; customer `GetOrder`/`ListOrders` responses never expose `refundLastError` or internal admin refund notes.
7. **Verification gates**: `make verify SERVICE=<surface>` passes (exit 0) for `contracts`, `payment-service`, `order-service`, `bff-service`, `admin-fe`, `ecommerce-fe`, plus `go test ./scripts/...` and repository reviews (`reviewing-code` + `reviewing-service-boundaries`).

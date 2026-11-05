# Event-Driven Order Checkout (`202 Accepted` + Polling), Snapshot Consistency, `payment-service` (COD/MoMo) & RabbitMQ Saga Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a pure Event-Driven multi-SKU Order & Checkout system with a dedicated Go `payment-service` (`payment_db`) that immediately accepts orders (`HTTP 202 Accepted` in a single local DB transaction), processes Catalog snapshot verification, SKU stock reservation, MoMo v2 `captureWallet` session creation, payment callback settlement, stock confirmation/release, and selective cart cleanup asynchronously over Transactional Outbox + RabbitMQ, and converges on the storefront via adaptive HTTP Polling and a multi-threaded concurrency simulation suite.

**Architecture:** `POST /api/v1/orders` checks `order.create` via `identity-service` (`:50054`) and atomically inserts `orders(PENDING_VALIDATION)`, `order_items`, `idempotency_records(COMPLETED)`, and `outbox_events(OrderSubmitted)` in `order_db` before returning `202 Accepted`. Background consumers in `order-service` (`:8083`, mgmt `:8093`, private gRPC `:50055`) isolate Catalog snapshot validation (`order.validation-events` with Circuit Breaker + 5-step `~51s` Exponential Backoff + `MessageRecoverer` early `PROCESSING_TIMEOUT` cancellation) from fast local Saga state transitions (`order.saga-events`), coordinating 100% asynchronously over RabbitMQ (`ecommerce.events`) with `inventory-service`, `cart-service`, and the new Go `payment-service` (`:8086` health, `:50056` gRPC for `Kong -> bff-service -> HandleMomoCallback`, isolating `payment.session-events` MoMo HTTPS I/O from `payment.order-events` Tombstone/refund updates), backed by post-commit in-memory wakeups (`AFTER_COMMIT` / `wakeupCh`) plus `1s` `FOR UPDATE SKIP LOCKED` sweeps and centralized `OrderReconciliationScheduler`.

**Tech Stack:** Protobuf / Buf, Java 21 / Spring Boot 4.1.0 / Spring Data JPA / Flyway / Spring AMQP / Resilience4j CircuitBreaker / `grpc-netty-shaded` 1.65.1 / Testcontainers, Go 1.24 (`pgx/v5`, `sqlc`, `google.golang.org/grpc` v1.65.0, `github.com/rabbitmq/amqp091-go` v1.10.0), Kong 3.8 declarative config, React 19 / TypeScript 6 / Vite 8 / Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-11-03-event-driven-order-checkout-saga-design.md`

## Global Constraints

- **Service boundaries (`AGENTS.md`, `scripts/internal/boundaries/boundaries.go`, `services/catalog-service/AGENTS.md`):** Zero cross-service code imports, zero order workflow logic inside `catalog-service`, zero synchronous gRPC calls between `order-service` and `inventory-service` or `payment-service`, zero foreign `<other>_db` strings (even in SQL/code comments — `payment-service` must never mention `order_db` and vice versa), and zero `:8000` literals (`localhost:8000`, `127.0.0.1:8000`, `kong:8000`, `host.docker.internal:8000`) anywhere in `.go`, `.java`, `.yml`, `.yaml`, `.properties`, or `.env` under `services/*`.
- **Contract compatibility (`docs/steering/api-contracts.md`, `ADR-013`):** Keep `contracts/proto/inventory/v1/inventory.proto` completely unchanged (zero synchronous reservation RPCs); create `contracts/proto/payment/v1/payment.proto` with **only** `HandleMomoCallback` (for `bff-service`); keep `catalog.proto`, `cart.proto`, and `order.proto` (4 order RPCs only) strictly additive (`lower_snake_case`), preserving existing field numbers (`1..4` in `OrderItem`, `1..2` in `CreateOrderRequest`, `1..3` in `CreateOrderResponse`); pass `buf format --diff --exit-code`, `buf lint`, and commit synchronized `buf generate` outputs in `contracts/go` and `contracts/java/src/main/generated`.
- **Pure async intake, queue isolation & transaction discipline (`services/order-service/AGENTS.md`, `ADR-002`, `ADR-003`, `ADR-005`):**
  - Never call `Catalog`, `Inventory`, `Payment`, or `MoMo` inside `CreateOrder`; never hold a DB transaction open across gRPC, MoMo HTTPS (`3s` connect / `10s` read timeout), or RabbitMQ publisher confirms (`3s` timeout).
  - Isolate slow external-I/O queues from fast local-DB queues: `order.validation-events` (separate from `order.saga-events` in `order-service`) and `payment.session-events` (separate from `payment.order-events` in `payment-service`), all configured with max `5` attempts using Exponential Backoff (`1s -> 3s -> 7s -> 15s -> 25s`, `~51s` window) into `.dlq`.
  - All three Outbox relays (`order-service`, `inventory-service`, `payment-service`) claim short leases (`FOR UPDATE SKIP LOCKED`, `next_attempt_at += 15s`), publish outside TX, and mark `published_at = now()` in a second short TX, paired with non-blocking post-commit wakeups (`AFTER_COMMIT` / `wakeupCh`).
  - Lock `orders` via `SELECT ... FOR UPDATE` verifying expected source `OrderStatus` on every transition; never reopen terminal states (`CONFIRMED`, `CANCELLED`).
- **REST & frontend conventions (`AGENTS.md`, `ecommerce-fe/src/lib/api.ts`):** Keep Kong to its existing 2 application upstreams (`identity-service`, `bff-service`); return `202 Accepted` on `POST /api/v1/orders` with `_silentToast: true` in `api.ts`; expose `rejectionDetails` on `GET /api/v1/orders/{orderId}`; poll `OrderPreviewPage` (`300ms -> 500ms -> 1s -> 2s`, max `30s`) with `sessionStorage` F5 recovery and friendly fallback UI for `PROCESSING_TIMEOUT` / `STOCK_RESERVATION_TIMEOUT` / `PAYMENT_INITIATION_TIMEOUT`; poll `/thanh-toan/ket-qua` (`500ms`, max `10s`) while `PENDING_PAYMENT` settles; add `Idempotency-Key` to Kong CORS headers; never commit live MoMo secrets.
- **Windows & CI compatibility (`services/cart-service/AGENTS.md`):** Keep `services/cart-service/cmd/server/main_test.go` Linux signal tests intact; run `go test ./services/cart-service/internal/...` on Windows and `go run ./scripts/verify-service -service <surface>` for every touched surface (including `payment-service`).

## Review Focus

1. **Duplicate `sku_id` aggregation, atomic `SAVEPOINT` rejection outbox & `sku_id ASC` lock ordering in `inventory-service` (`Task 4`):** Repeating the same `sku_id` across lines (`2 + 2 = 4 > 3`) must roll back to `SAVEPOINT before_reserve`, leave `reserved == 0`, set `stock_reservations(status = 'RELEASED', release_reason = 'INVENTORY_INSUFFICIENT')`, and atomically insert `InventoryReservationRejected` in `inventory_db.outbox_events` in one commit, while concurrent `(sku-b, sku-a)` vs `(sku-a, sku-b)` orders lock in `sku_id ASC` order without deadlocking.
2. **Queue isolation, `CatalogService` Circuit Breaker fail-fast & 5th-retry `MessageRecoverer` early `PROCESSING_TIMEOUT` cancellation (`Task 7`):** `OrderSubmitted` runs on isolated `order.validation-events` so a down `catalog-service` never blocks `order.saga-events`; transient `UNAVAILABLE` / `DEADLINE_EXCEEDED` / `CallNotPermittedException` (Circuit Breaker open) retries across `1s -> 3s -> 7s -> 15s -> 25s`, and on the 5th exhausted attempt `ValidationDeadLetterRecoverer` locks `SELECT ... FOR UPDATE` to transition `PENDING_VALIDATION -> CANCELLED (PROCESSING_TIMEOUT)` immediately before routing to `.dlq` (and any later `.dlq` replay sees `status != PENDING_VALIDATION` and exits as a no-op).
3. **DB-free MoMo HTTPS split, dual queue isolation, Tombstones & forged-callback DoS protection in `payment-service` (`Task 6`):** `PaymentSessionRequested` on `payment.session-events` commits `TX #A` (`INITIATING`) before MoMo HTTPS and checks `status == 'INITIATING'` in `TX #B` (emitting `PaymentSessionFailed` on business rejection or 5th exhausted network retry), while `PaymentCancelRequested` on fast `payment.order-events` plants `CANCELLED` tombstones immediately without waiting behind slow MoMo calls; `HandleMomoCallback` verifies constant-time `HmacSHA256` **before** any DB query so neither forged `resultCode=0` nor forged `resultCode=1006` can hijack or cancel an order.
4. **Bidirectional timeout/late-event compensation across `order-service`, `inventory-service`, and `payment-service` (`Task 4`, `Task 6` & `Task 7`, `ADR-005`):** Late `InventoryReserved` or `PaymentSessionCreated` arriving at `order-service` after `orders.status == CANCELLED` emits compensating `ReservationReleaseRequested` / `PaymentCancelRequested` without saving `payUrl`; late valid MoMo `resultCode=0` after `CANCELLED` converges both `orders` and `payments` to `REFUND_REQUIRED` (via `PaymentSucceeded` / `RefundRequested`) with zero `OrderConfirmed` emitted.
5. **Silent `202` toast, `PENDING_PAYMENT_SESSION` & `*_TIMEOUT` fallback UX, F5 `sessionStorage` recovery & `/thanh-toan/ket-qua` convergence (`Task 9`):** `createOrder` (`202`) suppresses premature green toasts; `OrderPreviewPage` displays distinct progress copy for `PENDING_VALIDATION`, `PENDING_STOCK`, and `PENDING_PAYMENT_SESSION`, renders the friendly retryable fallback banner when `cancelReason` is `PROCESSING_TIMEOUT` / `STOCK_RESERVATION_TIMEOUT` / `PAYMENT_INITIATION_TIMEOUT` with empty `rejectionDetails`, resumes polling after F5 via `sessionStorage`, and polls `/thanh-toan/ket-qua` (`500ms` up to `10s`) while `PENDING_PAYMENT` settles.

---

## File Structure

- **Contracts, DB Init & Architecture Docs (`Task 1`):**
  - Create: `contracts/proto/payment/v1/payment.proto`, `docs/adr/014-event-driven-order-checkout-saga-and-payment-service.md`
  - Modify: `contracts/proto/{catalog,cart,order}/v1/*.proto` (`inventory.proto` untouched), `deployments/postgres/init/01-create-databases.sql`, `AGENTS.md`, `docs/steering/service-catalog.md`, `services/{order,inventory,cart,bff}-service/AGENTS.md`
  - Modify (generated): `contracts/go/{catalog,cart,order,payment}/v1/*`, `contracts/java/src/main/generated/com/ecommerce/contracts/{catalog,cart,order,payment}/v1/*`
- **Identity RBAC (`Task 2`):**
  - Create: `services/identity-service/src/main/resources/db/migration/V12__add_order_permissions.sql`
  - Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`, `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`
- **Catalog Product Version (`Task 3`):**
  - Create: `services/catalog-service/internal/store/migrations/000006_add_product_version.up.sql`
  - Modify: `services/catalog-service/internal/domain/{product,admin}.go`, `services/catalog-service/internal/store/{store,admin,store_test}.go`, `services/catalog-service/internal/grpc/{server,admin,server_test}.go`
- **Inventory Event-Driven Reservations, `SAVEPOINT` Rejections, Tombstones & Outbox (`Task 4`):**
  - Create: `services/inventory-service/internal/store/migrations/000003_create_stock_reservations_and_outbox.up.sql`, `services/inventory-service/internal/store/{reservation_store,reservation_store_test}.go`, `services/inventory-service/internal/events/{envelope,consumer,outbox_relay,events_test}.go`
  - Modify: `services/inventory-service/go.mod`, `services/inventory-service/internal/config/config.go`, `services/inventory-service/internal/domain/stock.go`, `services/inventory-service/cmd/server/main.go`
- **Cart Version Enrichment & Event Cleanup (`Task 5`):**
  - Create: `services/cart-service/internal/store/migrations/000004_create_processed_events.up.sql`, `services/cart-service/internal/events/{consumer,consumer_test}.go`
  - Modify: `services/cart-service/go.mod`, `services/cart-service/internal/config/config.go`, `services/cart-service/internal/domain/cart.go`, `services/cart-service/internal/catalog/client.go`, `services/cart-service/internal/store/queries/cart.sql`, `services/cart-service/internal/store/db/*`, `services/cart-service/internal/store/{store,store_test}.go`, `services/cart-service/internal/grpc/{server,server_test}.go`, `services/cart-service/cmd/server/main.go`
- **`payment-service` Event-Driven Go Microservice (`Task 6`):**
  - Create:
    - `services/payment-service/{go.mod,Dockerfile,AGENTS.md}`
    - `services/payment-service/cmd/server/main.go`
    - `services/payment-service/internal/config/{config,config_test}.go`
    - `services/payment-service/internal/domain/payment.go`
    - `services/payment-service/internal/momo/{signer,signer_test,client,client_test}.go`
    - `services/payment-service/internal/store/migrations/000001_create_payments_and_outbox.up.sql`
    - `services/payment-service/internal/store/{store,store_test}.go`
    - `services/payment-service/internal/events/{envelope,relay,session_consumer,order_consumer,events_test}.go`
    - `services/payment-service/internal/grpc/{server,server_test}.go`
    - `services/payment-service/internal/http/health.go`
  - Modify: `go.work`, `scripts/verify-foundation/main.go`
- **`order-service` `202` Intake, Isolated Validation/Saga Consumers, Circuit Breaker, `MessageRecoverer`, Outbox & gRPC (`Task 7`):**
  - Modify: `services/order-service/pom.xml`, `services/order-service/Dockerfile`, `services/order-service/src/main/resources/application.yml`, `services/order-service/src/main/java/com/ecommerce/order/OrderApplication.java`
  - Create:
    - `services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql`
    - `services/order-service/src/main/java/com/ecommerce/order/config/{OrderProperties,GrpcClientConfig,RabbitMqConfig,ResilienceConfig}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/common/{TraceIdProvider,GrpcTraceIdInterceptor,DomainException,RetryableDependencyException}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/domain/{OrderStatus,PaymentMethod,PaymentStatus,ShippingPolicy,SnapshotDiffEvaluator}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/persistence/{OrderEntity,OrderItemEntity,OrderRepository,IdempotencyRecordEntity,IdempotencyRecordRepository,OutboxEventEntity,OutboxEventRepository,ProcessedEventEntity,ProcessedEventRepository}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/integration/{AuthorizationGrpcAdapter,CatalogGrpcAdapter}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/messaging/{SagaEventEnvelope,OutboxWakeupEvent,OutboxRelayScheduler,OrderValidationConsumer,ValidationDeadLetterRecoverer,OrderSagaEventConsumer}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/application/{IdempotencyService,OrderTransactionStepService,OrderIntakeService,OrderSagaCoordinator,OrderQueryService,OrderReconciliationScheduler}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/grpc/{OrderGrpcService,GrpcServerLifecycle,GrpcErrorMapper}.java`
  - Test: `services/order-service/src/test/java/com/ecommerce/order/{support/PostgresRabbitIntegrationTestSupport,persistence/OrderMigrationAndLockingTest,application/IdempotencyServiceTest,application/OrderIntakeAndSagaCoordinatorTest,application/ValidationRetryAndRecovererTest,application/OrderReconciliationSchedulerTest,grpc/OrderGrpcAndRabbitSagaIntegrationTest}.java`
- **BFF (`202 Accepted` + `rejectionDetails`), Kong, Docker Compose, CI, `Makefile` & `start-local` (`Task 8`):**
  - Create: `services/bff-service/internal/order/{client,client_test}.go`, `services/bff-service/internal/payment/{client,client_test}.go`, `services/bff-service/internal/http/{orders,orders_test,payments,payments_test}.go`
  - Modify: `services/bff-service/internal/http/{cart,cart_test}.go`, `services/bff-service/internal/config/{config,config_test,kong_development_test,kong_production_test,compose_production_test}.go`, `services/bff-service/cmd/server/{main,main_test}.go`, `deployments/kong/{kong,kong-production}.yaml`, `deployments/docker/compose.production.yml`, `.github/workflows/ci.yml`, `Makefile`, `.env.example`, `.env.production.example`, `scripts/start-local/{main,main_test}.go`
- **Storefront Adaptive Polling, `*_TIMEOUT` Fallback UX, `sessionStorage` Recovery & Payment Result Convergence (`Task 9`):**
  - Create: `ecommerce-fe/src/components/checkout/shipping-address-dialog.tsx`, `ecommerce-fe/src/pages/{payment-result-page,payment-result-page.test,orders-page,orders-page.test,order-detail-page}.tsx`
  - Modify: `ecommerce-fe/src/lib/{types,api,api.test}.ts`, `ecommerce-fe/src/pages/{order-preview-page,order-preview-page.test}.tsx`, `ecommerce-fe/src/components/layout/{site-header,site-header.test}.tsx`, `ecommerce-fe/src/{App,App.test}.tsx`
- **Event-Driven Concurrency & Race-Condition Simulation Suite (`Task 10`):**
  - Create: `scripts/concurrency-sim/{main,scenarios,sim_test}.go`, `scripts/run-concurrency-sim.ps1`

---

### Task 1: Shared Protobuf Contracts, Event Envelope Specification, Database Init & `ADR-014`

**Files:**
- Modify: `contracts/proto/catalog/v1/catalog.proto`
- Modify: `contracts/proto/cart/v1/cart.proto`
- Create: `contracts/proto/payment/v1/payment.proto`
- Modify: `contracts/proto/order/v1/order.proto` (`inventory.proto` untouched)
- Modify: `contracts/go/{catalog,cart,order,payment}/v1/*` (generated)
- Modify: `contracts/java/src/main/generated/com/ecommerce/contracts/{catalog,cart,order,payment}/v1/*` (generated)
- Modify: `deployments/postgres/init/01-create-databases.sql`
- Create: `docs/adr/014-event-driven-order-checkout-saga-and-payment-service.md`
- Modify: `AGENTS.md`, `docs/steering/service-catalog.md`, `services/order-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, `services/bff-service/AGENTS.md`

**Interfaces:**
- Consumes: `ecommerce.common.v1.Money` (`common/v1/common.proto`)
- Produces:
  - `ecommerce.catalog.v1`: `GetProductResponse.version = 24` (`int64`), `ProductSummary.version = 19` (`int64`), `AdminProductSummary.version = 24` (`int64`)
  - `ecommerce.cart.v1`: `CartItem.product_version = 17` (`int64`)
  - `ecommerce.inventory.v1`: **Unchanged** (zero synchronous reservation RPCs).
  - `ecommerce.payment.v1` (`go_package = "ecommerce.local/contracts/go/payment/v1;paymentv1"`, `java_package = "com.ecommerce.contracts.payment.v1"`):
    - `PaymentService`: **1 RPC only** — `HandleMomoCallback(HandleMomoCallbackRequest) returns (HandleMomoCallbackResponse)` (`CreatePaymentSession` is event-driven via `PaymentSessionRequested`)
    - `HandleMomoCallbackRequest { 1: partner_code, 2: order_id, 3: request_id, 4: amount (int64), 5: order_info, 6: order_type, 7: trans_id, 8: result_code (int32), 9: message, 10: pay_type, 11: response_time (int64), 12: extra_data, 13: signature, 14: source }`, `HandleMomoCallbackResponse { 1: order_id, 2: payment_status, 3: redirect_result }`
  - `ecommerce.order.v1`:
    - `OrderService` (4 order RPCs only): `CreateOrder`, `GetOrder`, `ListOrders`, `CancelOrder`
    - `ShippingAddressSnapshot` (`1..8`), `OrderItem` (`1..4` unchanged + `5..15` per Spec Section 5.3.e), `CreateOrderRequest` (`1..2` unchanged + `3: idempotency_key, 4: shipping_address, 5: payment_method, 6: note`), `OrderDetail` (`1..17` + `18: map<string, string> rejection_details`, `status` in `PENDING_VALIDATION | PENDING_STOCK | PENDING_PAYMENT_SESSION | PENDING_PAYMENT | CONFIRMED | CANCELLED`), `CreateOrderResponse` (`1..3` unchanged + `4: OrderDetail order`)
    - `GetOrderRequest/Response`, `ListOrdersRequest/Response`, `CancelOrderRequest/Response`
  - Canonical RabbitMQ JSON Envelope on durable topic exchange `ecommerce.events` (DLX `ecommerce.events.dlx`, max 5 retries with Exponential Backoff `1s -> 3s -> 7s -> 15s -> 25s` ~`51s`), shared across Tasks 4, 5, 6, 7:
    ```json
    {
      "eventId": "uuid",
      "eventType": "OrderSubmitted | StockReservationRequested | InventoryReserved | InventoryReservationRejected | PaymentSessionRequested | PaymentSessionCreated | PaymentSessionFailed | PaymentSucceeded | PaymentFailed | OrderConfirmed | ReservationReleaseRequested | InventoryReleased | PaymentCancelRequested | RefundRequested",
      "schemaVersion": 1,
      "occurredAt": "RFC3339",
      "aggregateId": "<orderId>",
      "correlationId": "<traceId>",
      "payload": {}
    }
    ```
    Exact isolated queues & routing keys per Spec Section 5.1:
    - `order.validation-events` (`order-service`, + `.dlq`): `order.submitted`
    - `order.saga-events` (`order-service`, + `.dlq`): `inventory.stock.reserved`, `inventory.stock.rejected`, `inventory.stock.released`, `payment.session.created`, `payment.session.failed`, `payment.succeeded`, `payment.failed`
    - `inventory.order-events` (`inventory-service`, + `.dlq`): `order.stock.reserve_requested`, `order.confirmed`, `order.reservation.release_requested`
    - `payment.session-events` (`payment-service`, + `.dlq`): `order.payment.session_requested`
    - `payment.order-events` (`payment-service`, + `.dlq`): `order.payment.cancel_requested`, `order.payment.refund_requested`
    - `cart.order-events` (`cart-service`, + `.dlq`): `order.confirmed`

- [ ] **Step 1: Add `payment.proto`, update `catalog.proto`, `cart.proto`, `order.proto` (leaving `inventory.proto` untouched), and append `CREATE DATABASE payment_db;` to `01-create-databases.sql`**
  - Preserve every existing field number and name (`OrderItem` `1..4`, `CreateOrderRequest` `1..2`, `CreateOrderResponse` `1..3`).

- [ ] **Step 2: Format, lint, and regenerate Go & Java contract modules**
  - Run: `buf format -w; buf generate`
  - Stage generated files so `git ls-files --others` check in `verify-service` sees tracked outputs: `git add contracts`
  - Run: `go run ./scripts/verify-service -service contracts`
  - Expected: `PASSED: contracts` (`exit 0`).

- [ ] **Step 3: Record `ADR-014` and align repository steering/service docs**
  - Create `docs/adr/014-event-driven-order-checkout-saga-and-payment-service.md` documenting: `payment-service` (`:8086` health, `:50056` gRPC for `HandleMomoCallback`, `payment_db`); `order-service` `:50055` `202 Accepted` intake; queue isolation (`order.validation-events` vs `order.saga-events`, `payment.session-events` vs `payment.order-events`) + 5-step `~51s` Exponential Backoff + `CatalogService` Circuit Breaker + `MessageRecoverer` early `PROCESSING_TIMEOUT` cancellation; three-service Outbox with post-commit wakeups; Tombstones on `stock_reservations` & `payments`; and centralized `OrderReconciliationScheduler` + `REFUND_REQUIRED` late-payment convergence.
  - Update `AGENTS.md`, `docs/steering/service-catalog.md`, `services/order-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, and `services/bff-service/AGENTS.md`.

- [ ] **Step 4: Verify boundary check and downstream compilation**
  - Run: `go run ./scripts/check-boundaries && go build ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/... ./services/bff-service/... && mvn --batch-mode -pl contracts/java -am test-compile`
  - Expected: Exit code `0`.

- [ ] **Step 5: Commit**
  - Run: `git add contracts deployments/postgres/init/01-create-databases.sql docs AGENTS.md services/order-service/AGENTS.md services/inventory-service/AGENTS.md services/cart-service/AGENTS.md services/bff-service/AGENTS.md && git commit -m "feat(contracts): add event-driven order checkout, payment-service, and snapshot version contracts"`

---

### Task 2: `identity-service` Order RBAC Permissions Migration

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V12__add_order_permissions.sql`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`

**Interfaces:**
- Produces: System permissions `order.create`, `order.view`, `order.cancel` (`type = 'PERMISSION'`, `system_item = true`) assigned to `CUSTOMER` in `auth_item_children`.

- [ ] **Step 1: Write failing migration and RBAC inheritance tests**
  - Assert `"order.cancel"`, `"order.create"`, `"order.view"` are seeded and inherited by `CUSTOMER` and `SUPER_ADMIN`, but not `ADMIN`.

- [ ] **Step 2: Run Identity tests to verify RED failure**
  - Run: `mvn -f services/identity-service/pom.xml test -Dtest=IdentityDatabaseMigrationTest,RbacPersistenceTest`
  - Expected: FAIL.

- [ ] **Step 3: Create `V12__add_order_permissions.sql`**
  - Upsert `order.create`, `order.view`, `order.cancel` into `auth_items` (`system_item = true`) and link to `CUSTOMER` in `auth_item_children` (`on conflict do nothing`).

- [ ] **Step 4: Run `identity-service` verification gate**
  - Run: `go run ./scripts/verify-service -service identity-service`
  - Expected: `PASSED: identity-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/identity-service && git commit -m "feat(identity): seed order RBAC permissions for customers"`

---

### Task 3: `catalog-service` Monotonic Product `version`

**Files:**
- Create: `services/catalog-service/internal/store/migrations/000006_add_product_version.up.sql`
- Modify: `services/catalog-service/internal/domain/{product,admin}.go`, `services/catalog-service/internal/store/{store,admin,store_test}.go`, `services/catalog-service/internal/grpc/{server,admin,server_test}.go`

**Interfaces:**
- Consumes: `catalogv1` `version` fields from Task 1.
- Produces: `domain.Product.Version int64` (default `1`, incremented `version = version + 1` on `UpdateProduct` and `SoftDelete`) mapped across all Catalog gRPC read/mutation responses.

- [ ] **Step 1: Write failing store & gRPC tests (`TestProductVersionIncrementsOnUpdateAndSoftDelete`)**
- [ ] **Step 2: Run `go test ./services/catalog-service/...` to verify RED failure**
- [ ] **Step 3: Add `000006_add_product_version.up.sql` and wire `version` across domain, store, and gRPC**
- [ ] **Step 4: Run `go run ./scripts/verify-service -service catalog-service` (`PASSED: catalog-service`)**
- [ ] **Step 5: Commit (`feat(catalog): track monotonic product version on commercial updates`)**

---

### Task 4: `inventory-service` Event-Driven Reservations, `SAVEPOINT` Rejections, Tombstones & Outbox Relay

**Files:**
- Modify: `services/inventory-service/go.mod`, `services/inventory-service/internal/config/config.go`, `services/inventory-service/internal/domain/stock.go`, `services/inventory-service/cmd/server/main.go`
- Create: `services/inventory-service/internal/store/migrations/000003_create_stock_reservations_and_outbox.up.sql`, `services/inventory-service/internal/store/{reservation_store,reservation_store_test}.go`, `services/inventory-service/internal/events/{envelope,consumer,outbox_relay,events_test}.go`

**Interfaces:**
- Consumes: RabbitMQ events `StockReservationRequested` (`order.stock.reserve_requested`), `OrderConfirmed` (`order.confirmed`), `ReservationReleaseRequested` (`order.reservation.release_requested`) on `inventory.order-events`.
- Produces: RabbitMQ events via `inventory_db.outbox_events` (`wakeupCh` + `1s` sweep): `InventoryReserved` (`inventory.stock.reserved`), `InventoryReservationRejected` (`inventory.stock.rejected`), `InventoryReleased` (`inventory.stock.released`).

- [ ] **Step 1: Write failing reservation, `SAVEPOINT` rejection outbox, deadlock-free `sku_id ASC` ordering, conflict-free tombstone, and outbox lease tests**
  - Cover duplicate `sku_id` aggregation + `SAVEPOINT before_reserve` rollback writing `InventoryReservationRejected`, duplicate delivery idempotency, `ReleaseReservation` planting `RELEASED` tombstone before late `ProcessReservationRequest`, concurrent reserve/release with zero `23505`, cross-order `(sku-b, sku-a)` vs `(sku-a, sku-b)` zero deadlocks, and `FOR UPDATE SKIP LOCKED` outbox lease claiming.

- [ ] **Step 2: Run `go test ./services/inventory-service/...` to verify RED failure**

- [ ] **Step 3: Implement migration `000003`, `reservation_store.go`, consumer (`1s -> 3s -> 7s -> 15s -> 25s` retry backoff to `.dlq`), and `OutboxRelay` (`wakeupCh` + `3s` AMQP confirm outside DB TX)**

- [ ] **Step 4: Run `go run ./scripts/verify-service -service inventory-service` (`PASSED: inventory-service`)**

- [ ] **Step 5: Commit (`feat(inventory): implement event-driven SKU reservations, tombstones, and transactional outbox`)**

---

### Task 5: `cart-service` `product_version` Enrichment & `OrderConfirmed` Cleanup Consumer

**Files:**
- Modify: `services/cart-service/go.mod`, `services/cart-service/internal/config/config.go`, `services/cart-service/internal/domain/cart.go`, `services/cart-service/internal/catalog/client.go`, `services/cart-service/internal/store/queries/cart.sql`, `services/cart-service/internal/store/db/*`, `services/cart-service/internal/store/{store,store_test}.go`, `services/cart-service/internal/grpc/{server,server_test}.go`, `services/cart-service/cmd/server/main.go`
- Create: `services/cart-service/internal/store/migrations/000004_create_processed_events.up.sql`, `services/cart-service/internal/events/{consumer,consumer_test}.go`

**Interfaces:**
- Consumes: `catalogv1.GetProductResponse.version` (Task 3), `cartv1.CartItem.product_version` & `OrderConfirmed` RabbitMQ event (Task 1).
- Produces: `CartItem.ProductVersion` enrichment and `Store.RemoveOrderedItems` deduplicated by `processed_events(event_id)` on `cart.order-events`.

- [ ] **Step 1: Write failing tests (`TestHandleOrderConfirmedRemovesOnlyOrderedItemsIdempotently` & `CartItem.GetProductVersion()`)**
- [ ] **Step 2: Run `go test ./services/cart-service/internal/...` to verify RED failure**
- [ ] **Step 3: Implement migration `000004`, `product_version` mapping, `RemoveOrderedItems`, and `cart.order-events` consumer inside `buildProductionService`**
- [ ] **Step 4: Run `go run ./scripts/verify-service -service cart-service` (`PASSED: cart-service`)**
- [ ] **Step 5: Commit (`feat(cart): expose product version and clean up confirmed order items`)**

---

### Task 6: `payment-service` (New Go 1.24 Microservice) — Event-Driven Session Creation, Queue Isolation, MoMo v2 HMAC, Tombstones & Outbox Relay

**Files:**
- Modify: `go.work`, `scripts/verify-foundation/main.go`
- Create:
  - `services/payment-service/{go.mod,Dockerfile,AGENTS.md}`
  - `services/payment-service/internal/config/{config,config_test}.go`
  - `services/payment-service/internal/domain/payment.go`
  - `services/payment-service/internal/momo/{signer,signer_test,client,client_test}.go`
  - `services/payment-service/internal/store/migrations/000001_create_payments_and_outbox.up.sql`
  - `services/payment-service/internal/store/{store,store_test}.go`
  - `services/payment-service/internal/events/{envelope,relay,session_consumer,order_consumer,events_test}.go`
  - `services/payment-service/internal/grpc/{server,server_test}.go`
  - `services/payment-service/internal/http/health.go`
  - `services/payment-service/cmd/server/main.go`

**Interfaces:**
- Consumes:
  - `paymentv1.PaymentServiceServer.HandleMomoCallback` from Task 1.
  - Isolated RabbitMQ queues:
    - `payment.session-events`: `PaymentSessionRequested` (`order.payment.session_requested`)
    - `payment.order-events`: `PaymentCancelRequested` (`order.payment.cancel_requested`), `RefundRequested` (`order.payment.refund_requested`)
- Produces:
  - Go module `ecommerce.local/payment-service` listening on HTTP `:8086` (`/healthz`) and private gRPC `127.0.0.1:50056` (`0.0.0.0:50056` in container), owning `payment_db` (zero mention of `order_db` or `:8000` anywhere in `services/payment-service/`).
  - Outbox events via `payment_db.outbox_events` (`wakeupCh` + `1s` `FOR UPDATE SKIP LOCKED` sweep): `PaymentSessionCreated` (`payment.session.created`), `PaymentSessionFailed` (`payment.session.failed`), `PaymentSucceeded` (`payment.succeeded`), `PaymentFailed` (`payment.failed`).

- [ ] **Step 1: Register `./services/payment-service` in `go.work` & `verify-foundation`, and write failing HMAC, two-phase session consumer, Tombstone, callback, and dual-queue tests**
  - `signer_test.go` & `client_test.go`: MoMo v2 `captureWallet` signature, constant-time `hmac.Equal` verification rejecting forged `resultCode=0`, forged `resultCode=1006`, or tampered `amount`, `3s/10s` client timeouts, and `PAYMENT_MOMO_MOCK_ENABLED=true` local signed URL generation.
  - `store_test.go` & `events_test.go`:
    - `TestSessionConsumerTwoPhaseCommitRetriesAndFifthAttemptFailureOutbox`: `TX #A` (`INITIATING`) commits before MoMo HTTPS; transient `5xx`/timeout returns retryable error across attempts `1..4` (`1s -> 3s -> 7s -> 15s -> 25s`), and on attempt `5` (or immediate MoMo business `resultCode != 0`) `TX #B` marks `FAILED` and writes `PaymentSessionFailed` outbox row; on MoMo `payUrl` success, `TX #B` verifies `status == 'INITIATING'`, marks `PENDING` + `pay_url`, and writes `PaymentSessionCreated` outbox row.
    - `TestOrderConsumerTombstoneBlocksLateSessionRequestAndInFlightMoMo`: `PaymentCancelRequested` on `payment.order-events` plants `CANCELLED` tombstone (`momo_request_id = 'tombstone:' || order_id`) so a delayed `PaymentSessionRequested` exits at `TX #A` without calling MoMo, and an in-flight `PaymentSessionRequested` keeps `CANCELLED` at `TX #B` without emitting `PaymentSessionCreated`.
    - `TestHandleMomoCallbackIdempotencyAndLatePaymentRefundRequired`: forged signature rejected with `codes.InvalidArgument` (`INVALID_MOMO_SIGNATURE`) before touching DB; concurrent valid callbacks emit exactly `1` `PaymentSucceeded` outbox row; valid `resultCode=0` on `CANCELLED` transitions `CANCELLED -> REFUND_REQUIRED` and emits `PaymentSucceeded` so `order-service` syncs `REFUND_REQUIRED`.

- [ ] **Step 2: Run `go test ./services/payment-service/...` to verify RED failure**

- [ ] **Step 3: Implement `payment-service` migration `000001`, MoMo v2 client/signer, store, `OutboxRelay` (`wakeupCh`), isolated `session_consumer.go` & `order_consumer.go`, `HandleMomoCallback` gRPC server, `Dockerfile` (`EXPOSE 8086 50056`), and `AGENTS.md`**

- [ ] **Step 4: Run `payment-service` verification gate**
  - Run: `go run ./scripts/verify-service -service payment-service`
  - Expected: `PASSED: payment-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add go.work go.work.sum scripts/verify-foundation/main.go services/payment-service && git commit -m "feat(payment): create event-driven Go payment-service with isolated queues, MoMo v2 HMAC, and outbox"`

---

### Task 7: `order-service` `202` Intake, Isolated Validation/Saga Consumers, Circuit Breaker, `MessageRecoverer`, Outbox & gRPC

**Files:**
- Modify: `services/order-service/pom.xml`, `services/order-service/Dockerfile`, `services/order-service/src/main/resources/application.yml`, `services/order-service/src/main/java/com/ecommerce/order/OrderApplication.java`
- Create:
  - `services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql`
  - `services/order-service/src/main/java/com/ecommerce/order/config/{OrderProperties,GrpcClientConfig,RabbitMqConfig,ResilienceConfig}.java`
  - `services/order-service/src/main/java/com/ecommerce/order/common/{TraceIdProvider,GrpcTraceIdInterceptor,DomainException,RetryableDependencyException}.java`
  - `services/order-service/src/main/java/com/ecommerce/order/domain/{OrderStatus,PaymentMethod,PaymentStatus,ShippingPolicy,SnapshotDiffEvaluator}.java`
  - `services/order-service/src/main/java/com/ecommerce/order/persistence/{OrderEntity,OrderItemEntity,OrderRepository,IdempotencyRecordEntity,IdempotencyRecordRepository,OutboxEventEntity,OutboxEventRepository,ProcessedEventEntity,ProcessedEventRepository}.java`
  - `services/order-service/src/main/java/com/ecommerce/order/integration/{AuthorizationGrpcAdapter,CatalogGrpcAdapter}.java`
  - `services/order-service/src/main/java/com/ecommerce/order/messaging/{SagaEventEnvelope,OutboxWakeupEvent,OutboxRelayScheduler,OrderValidationConsumer,ValidationDeadLetterRecoverer,OrderSagaEventConsumer}.java`
  - `services/order-service/src/main/java/com/ecommerce/order/application/{IdempotencyService,OrderTransactionStepService,OrderIntakeService,OrderSagaCoordinator,OrderQueryService,OrderReconciliationScheduler}.java`
  - `services/order-service/src/main/java/com/ecommerce/order/grpc/{OrderGrpcService,GrpcServerLifecycle,GrpcErrorMapper}.java`
- Test: `services/order-service/src/test/java/com/ecommerce/order/{support/PostgresRabbitIntegrationTestSupport,persistence/OrderMigrationAndLockingTest,application/IdempotencyServiceTest,application/OrderIntakeAndSagaCoordinatorTest,application/ValidationRetryAndRecovererTest,application/OrderReconciliationSchedulerTest,grpc/OrderGrpcAndRabbitSagaIntegrationTest}.java`

**Interfaces:**
- Consumes: `identity.v1.AuthorizationService` (`:50054`), `catalog.v1.CatalogService` (`:50051`, wrapped in Resilience4j `CircuitBreaker`), `order.v1.OrderService` (4 RPCs), and isolated RabbitMQ queues `order.validation-events` (`OrderSubmitted`) & `order.saga-events` (`InventoryReserved`, `InventoryReservationRejected`, `InventoryReleased`, `PaymentSessionCreated`, `PaymentSessionFailed`, `PaymentSucceeded`, `PaymentFailed`).
- Produces:
  - Private gRPC server on `${ORDER_GRPC_HOST:127.0.0.1}:${ORDER_GRPC_PORT:50055}` implementing `CreateOrder`, `GetOrder`, `ListOrders`, `CancelOrder` (zero `Inventory` or `Payment` gRPC client, zero `MOMO_*` secrets, zero `payment_db` or `:8000` strings).
  - Outbox events: `OrderSubmitted`, `StockReservationRequested`, `PaymentSessionRequested`, `OrderConfirmed`, `ReservationReleaseRequested`, `PaymentCancelRequested`, `RefundRequested`.

- [ ] **Step 1: Add Spring Boot 4.1 + `resilience4j-circuitbreaker` dependencies to `services/order-service/pom.xml` and write failing unit & Testcontainers tests**
  - Write failing tests covering:
    1. `IdempotencyServiceTest`: `20` concurrent `OrderIntakeService.createOrder` calls with identical `(userId, idempotencyKey)` insert `1` `orders(PENDING_VALIDATION)` + `1` `OrderSubmitted` outbox row via `ON CONFLICT DO NOTHING` and return the same `orderId`; mutated payload throws `IDEMPOTENCY_KEY_REUSED`.
    2. `ValidationRetryAndRecovererTest`:
       - `OrderSubmitted` runs on isolated `order.validation-events` container factory with 5-attempt Exponential Backoff (`1s -> 3s -> 7s -> 15s -> 25s`).
       - Repeated `CatalogGrpcAdapter` failures open the Resilience4j `CircuitBreaker` so subsequent calls fail fast with `CallNotPermittedException` wrapped in `RetryableDependencyException` without waiting `2s` per call.
       - On the 5th exhausted attempt, `ValidationDeadLetterRecoverer` locks `SELECT ... FOR UPDATE`, transitions `PENDING_VALIDATION -> CANCELLED` (`paymentStatus = FAILED`, `cancelReason = "PROCESSING_TIMEOUT"`), and publishes the failed message to `order.validation-events.dlq`; replaying that `.dlq` message later sees `status == CANCELLED` and exits as an idempotent no-op.
    3. `OrderIntakeAndSagaCoordinatorTest`:
       - Snapshot mismatch transitions `PENDING_VALIDATION -> CANCELLED` (`PRODUCT_SNAPSHOT_MISMATCH`, `rejectionDetails` per `skuId`); matching snapshot transitions `PENDING_VALIDATION -> PENDING_STOCK` + `StockReservationRequested`.
       - `InventoryReserved`: `COD` -> `CONFIRMED` + `OrderConfirmed`; `MOMO` -> `PENDING_PAYMENT_SESSION` + `PaymentSessionRequested`; late `InventoryReserved` on `CANCELLED` -> emits `ReservationReleaseRequested`.
       - `PaymentSessionCreated`: transitions `PENDING_PAYMENT_SESSION -> PENDING_PAYMENT` saving `payUrl`; if already `CANCELLED`, discards `payUrl` and emits `PaymentCancelRequested` + `ReservationReleaseRequested`.
       - `PaymentSessionFailed`: transitions `PENDING_PAYMENT_SESSION -> CANCELLED` (`PAYMENT_INITIATION_FAILED`) + `ReservationReleaseRequested`.
       - `PaymentSucceeded`: transitions `PENDING_PAYMENT -> CONFIRMED` (`PAID`, `paymentTransId`) + `OrderConfirmed`; if already `CANCELLED` (Late Payment), keeps `CANCELLED`, sets `paymentStatus = REFUND_REQUIRED`, records `paymentTransId`, and emits `RefundRequested` (`order.payment.refund_requested`).
       - `PaymentFailed`: transitions `PENDING_PAYMENT -> CANCELLED` (`PAYMENT_FAILED`) + `ReservationReleaseRequested` (no-op if already `CONFIRMED`).
       - `CancelOrder`: allowed only on `PENDING_PAYMENT` -> `CANCELLED` + emits both `ReservationReleaseRequested` and `PaymentCancelRequested`; throws `ORDER_NOT_CANCELLABLE` on `PENDING_VALIDATION`, `PENDING_STOCK`, `PENDING_PAYMENT_SESSION`, or `CONFIRMED`.
    4. `OrderReconciliationSchedulerTest`: sweeps `PENDING_VALIDATION > 2m` (`PROCESSING_TIMEOUT`), `PENDING_STOCK > 2m` (`STOCK_RESERVATION_TIMEOUT` + `ReservationReleaseRequested`), `PENDING_PAYMENT_SESSION > 2m` (`PAYMENT_INITIATION_TIMEOUT` + `ReservationReleaseRequested` + `PaymentCancelRequested`), and `PENDING_PAYMENT` past `payment_expires_at` (`PAYMENT_EXPIRED` + `ReservationReleaseRequested` + `PaymentCancelRequested`).
    5. `OrderGrpcAndRabbitSagaIntegrationTest`: full Testcontainers PostgreSQL + RabbitMQ round-trip verifying `< 500ms` `AFTER_COMMIT` wakeup convergence and queue isolation between `order.validation-events` and `order.saga-events`.

- [ ] **Step 2: Run `mvn -f services/order-service/pom.xml test` to verify RED failure**

- [ ] **Step 3: Implement `V1` Flyway schema, JPA entities, `CatalogGrpcAdapter` with Circuit Breaker, `OrderValidationConsumer` + `ValidationDeadLetterRecoverer`, `OrderSagaEventConsumer`, `AFTER_COMMIT` `OutboxRelayScheduler`, `OrderReconciliationScheduler`, and gRPC server (`EXPOSE 8083 8093 50055`)**

- [ ] **Step 4: Run `go run ./scripts/verify-service -service order-service` (`PASSED: order-service`)**

- [ ] **Step 5: Commit (`feat(order): implement 202 async intake, isolated validation/saga queues, circuit breaker, and outbox`)**

---

### Task 8: `bff-service` (`202 Accepted` + `rejectionDetails`), Kong Gateway, Docker Compose, CI, `Makefile` & `start-local` Wiring

**Files:**
- Create: `services/bff-service/internal/order/{client,client_test}.go`, `services/bff-service/internal/payment/{client,client_test}.go`, `services/bff-service/internal/http/{orders,orders_test,payments,payments_test}.go`
- Modify: `services/bff-service/internal/http/{cart,cart_test}.go`, `services/bff-service/internal/config/{config,config_test,kong_development_test,kong_production_test,compose_production_test}.go`, `services/bff-service/cmd/server/{main,main_test}.go`, `deployments/kong/{kong,kong-production}.yaml`, `deployments/docker/compose.production.yml`, `.github/workflows/ci.yml`, `Makefile`, `.env.example`, `.env.production.example`, `scripts/start-local/{main,main_test}.go`

**Interfaces:**
- Consumes: `cartv1.CartItem.product_version` (Task 5), `paymentv1.PaymentServiceClient.HandleMomoCallback` (Task 6), `orderv1.OrderServiceClient` (Task 7).
- Produces:
  - Protected REST routes -> `OrderService` (`BFF_ORDER_GRPC_ADDR = "127.0.0.1:50055"`):
    - `POST /api/v1/orders` (`Idempotency-Key` required -> **`202 Accepted`** `{ data: { orderId, orderCode, status: "PENDING_VALIDATION", totalAmount, order }, message: "Yêu cầu đặt hàng đã được tiếp nhận và đang xử lý." }`)
    - `GET /api/v1/orders`, `GET /api/v1/orders/{orderId}` (includes `payUrl`, `cancelReason`, `rejectionDetails`), `POST /api/v1/orders/{orderId}/cancel`
  - Public MoMo callback routes (`request-transformer` strips `X-User-Id` & `X-User-Roles`) -> `PaymentService` (`BFF_PAYMENT_GRPC_ADDR = "127.0.0.1:50056"`):
    - `GET /api/v1/payments/momo-redirect` -> `302 Found` to `${BFF_STOREFRONT_BASE_URL}/thanh-toan/ket-qua?orderId=...&result=success|failed` (or `400 INVALID_MOMO_SIGNATURE`)
    - `POST /api/v1/payments/momo-ipn` -> `204 No Content` (or `400`)

- [ ] **Step 1: Write failing BFF (`202` + `rejectionDetails` + `PaymentService` callbacks), Kong 2-upstream route, `compose.production.yml` (`payment-service` + zero sync `inventory`/`payment` gRPC env on `order-service`), and `start-local` tests**
- [ ] **Step 2: Run `go test ./services/bff-service/... ./scripts/...` to verify RED failure**
- [ ] **Step 3: Implement BFF order/payment clients & handlers, Kong configs, `compose.production.yml`, `.github/workflows/ci.yml` (`payment_db` test + build), `Makefile` (`payment`), `.env*.example`, and `start-local` `.env` loader**
- [ ] **Step 4: Run `go run ./scripts/verify-service -service bff-service && go test ./scripts/...` (`PASSED: bff-service`)**
- [ ] **Step 5: Commit (`feat(bff,deploy): expose 202 async order intake, payment-service callbacks, compose, and CI`)**

---

### Task 9: `ecommerce-fe` Adaptive Polling, `*_TIMEOUT` Fallback UX, `sessionStorage` Recovery & Payment Result Convergence

**Files:**
- Modify: `ecommerce-fe/src/lib/{types,api,api.test}.ts`, `ecommerce-fe/src/pages/{order-preview-page,order-preview-page.test}.tsx`, `ecommerce-fe/src/components/layout/{site-header,site-header.test}.tsx`, `ecommerce-fe/src/{App,App.test}.tsx`
- Create: `ecommerce-fe/src/components/checkout/shipping-address-dialog.tsx`, `ecommerce-fe/src/pages/{payment-result-page,payment-result-page.test,orders-page,orders-page.test,order-detail-page}.tsx`

**Interfaces:**
- Consumes: `/api/v1/users/me/addresses`, `/api/v1/locations/*`, and `/api/v1/orders*` (`202 Accepted` + `GET /orders/{orderId}` from Task 8).
- Produces:
  - `OrderStatus`: `"PENDING_VALIDATION" | "PENDING_STOCK" | "PENDING_PAYMENT_SESSION" | "PENDING_PAYMENT" | "CONFIRMED" | "CANCELLED"`.
  - `createOrder` (`_silentToast: true` on `202 Accepted`), `getOrder(orderId, signal?)`, `listOrders`, `cancelOrder`, `listShippingAddresses`, `createShippingAddress`.
  - Routes in `App.tsx`: `/gio-hang/dat-hang`, `/thanh-toan/ket-qua`, `/don-hang`, `/don-hang/:orderId`.

- [ ] **Step 1: Write failing Vitest tests for silent `202` toast, stepped Polling (`PENDING_VALIDATION` / `PENDING_STOCK` / `PENDING_PAYMENT_SESSION`), `*_TIMEOUT` fallback banner, F5 `sessionStorage` resume, and `/thanh-toan/ket-qua` convergence polling**
  - `api.test.ts`: `createOrder` sends `Idempotency-Key`, unwraps `202`, and suppresses premature `toast.success`.
  - `order-preview-page.test.tsx`:
    - Displays step copy for `PENDING_VALIDATION` (*"Đang kiểm tra thông tin và giá sản phẩm..."*), `PENDING_STOCK` (*"Đang giữ chỗ tồn kho..."*), and `PENDING_PAYMENT_SESSION` (*"Đang khởi tạo liên kết thanh toán MoMo..."*), polling at `300ms -> 500ms -> 1000ms -> 2000ms`.
    - `CONFIRMED` (`COD`) -> clears `sessionStorage`, refreshes cart, navigates to `/don-hang/${orderId}?placed=1`; `PENDING_PAYMENT` with `payUrl` (`MOMO`) -> clears `sessionStorage` and redirects to `payUrl`.
    - `CANCELLED` with `PRODUCT_SNAPSHOT_MISMATCH` -> renders per-SKU `rejectionDetails` banner, locks stale submit, and navigates to `/gio-hang` on click; `INVENTORY_INSUFFICIENT` / `PAYMENT_INITIATION_FAILED` -> displays `rejectionDetails` error alert and rotates `Idempotency-Key`.
    - `CANCELLED` with `PROCESSING_TIMEOUT`, `STOCK_RESERVATION_TIMEOUT`, or `PAYMENT_INITIATION_TIMEOUT` (empty `rejectionDetails`) -> renders fallback notice *"Hệ thống đang gián đoạn tạm thời nên chưa thể xác nhận đơn hàng. Giỏ hàng của bạn vẫn được giữ nguyên, vui lòng thử lại sau ít phút."*, keeps cart intact, and rotates `Idempotency-Key` so the user can retry safely.
    - Resumes polling from `sessionStorage` on F5 reload and navigates to `/don-hang/${orderId}` if `30s` elapses while still `PENDING_*`.
  - `payment-result-page.test.tsx` & `orders-page.test.tsx`: polls `getOrder(orderId)` every `500ms` (up to `10s`) on `/thanh-toan/ket-qua` while `status === "PENDING_PAYMENT"` settles across RabbitMQ (`"Đang đồng bộ kết quả thanh toán..."`), renders `REFUND_REQUIRED` notice when applicable, auto-refreshes `OrderDetailPage` every `3s` while `PENDING_VALIDATION | PENDING_STOCK | PENDING_PAYMENT_SESSION`, and supports cancelling `PENDING_PAYMENT` orders.

- [ ] **Step 2: Run `npm --prefix ecommerce-fe test -- --run` to verify RED failure**

- [ ] **Step 3: Implement `api.ts`, `ShippingAddressDialog`, `OrderPreviewPage`, `PaymentResultPage`, `OrdersPage`, `OrderDetailPage`, `SiteHeader`, and routes**

- [ ] **Step 4: Run `go run ./scripts/verify-service -service ecommerce-fe` (`PASSED: ecommerce-fe`)**

- [ ] **Step 5: Commit (`feat(ecommerce-fe): add 202 adaptive polling, timeout fallback UX, and payment result convergence`)**

---

### Task 10: Event-Driven Multi-Threaded Concurrency Simulation Suite + Full Repository Verification

**Files:**
- Create: `scripts/concurrency-sim/{main,scenarios,sim_test}.go`, `scripts/run-concurrency-sim.ps1`

**Interfaces:**
- Produces: CLI `go run ./scripts/concurrency-sim -scenario <all|flash-sale-oversell|snapshot-mismatch-race|idempotency-storm|momo-callback-race> -base-url http://localhost:8000/api/v1` and `scripts/run-concurrency-sim.ps1` verifying `202 Accepted` intake latency, end-to-end settled latency, and SQL invariants across `order_db`, `inventory_db`, and `payment_db`.

- [ ] **Step 1: Write failing simulation harness unit tests (`scripts/concurrency-sim/sim_test.go`) covering `RunFlashSaleOversell`, `RunSnapshotMismatchRace`, `RunIdempotencyStorm`, and `RunMomoCallbackRace` (including forged `resultCode=0` & `resultCode=1006` rejection and `REFUND_REQUIRED` convergence)**
- [ ] **Step 2: Run `go test ./scripts/concurrency-sim/...` to verify RED failure**
- [ ] **Step 3: Implement `scripts/concurrency-sim` and `scripts/run-concurrency-sim.ps1`**
- [ ] **Step 4: Run full repository verification gates across all surfaces**
  - Run:
    - `go run ./scripts/check-boundaries`
    - `go test ./scripts/...`
    - `go run ./scripts/verify-service -service contracts`
    - `go run ./scripts/verify-service -service identity-service`
    - `go run ./scripts/verify-service -service catalog-service`
    - `go run ./scripts/verify-service -service inventory-service`
    - `go run ./scripts/verify-service -service cart-service`
    - `go run ./scripts/verify-service -service payment-service`
    - `go run ./scripts/verify-service -service order-service`
    - `go run ./scripts/verify-service -service bff-service`
    - `go run ./scripts/verify-service -service ecommerce-fe`
    - `go run ./scripts/verify-service -service admin-fe`
  - If local Docker stack is running, execute `powershell -ExecutionPolicy Bypass -File ./scripts/run-concurrency-sim.ps1 -Scenario all` with `PAYMENT_MOMO_MOCK_ENABLED=true` (stopping any temporary background local processes once verification completes).
  - Expected: Zero exit code on every gate and simulation scenario.
- [ ] **Step 5: Commit & Run Mandatory Independent Reviews (`code-reviewer` + `boundary-reviewer`)**
  - Run: `git add scripts/concurrency-sim scripts/run-concurrency-sim.ps1 && git commit -m "test(concurrency): add event-driven checkout, payment-service, and saga simulation suite"`
  - Dispatch independent read-only `code-reviewer` and `boundary-reviewer` subagents over `git diff main...HEAD` per `AGENTS.md` before final handoff.

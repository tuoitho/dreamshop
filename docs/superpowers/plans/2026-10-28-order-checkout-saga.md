# Order Checkout, Snapshot Consistency, `payment-service` (COD/MoMo) & Anti-Oversell Hybrid Saga Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an end-to-end multi-item Order & Checkout workflow with a dedicated Go `payment-service` (`payment_db`), shipping address book selection, immutable order snapshots, preview-to-order snapshot verification (`409 PRODUCT_SNAPSHOT_MISMATCH`), `COD` and `MOMO` (`captureWallet` v2 HMAC) payments, atomic SKU stock reservation, bidirectional Transactional Outbox/RabbitMQ Saga compensation, selective cart cleanup, and a multi-threaded concurrency simulation suite.

**Architecture:** `order-service` (Java 21, HTTP `:8083`, mgmt `:8093`, private gRPC `:50055`) orchestrates a Hybrid Saga: synchronous private gRPC checks `order.*` RBAC via `identity-service` (`:50054`), verifies product `version` + SKU snapshot fields via `catalog-service` (`:50051`), atomically reserves SKU stock in `inventory-service` (`:50052`), and—for `MOMO` orders—creates an idempotent 15-minute payment session in the new Go `payment-service` (`:8086` health, private gRPC `:50056`, owning `payment_db`). Public MoMo callbacks (`momo-redirect` & `momo-ipn`) route through `Kong -> bff-service -> payment-service (:50056)`, where constant-time `HmacSHA256` verification updates `payments` and emits `PaymentSucceeded`/`PaymentFailed` via `payment_db.outbox_events` (`wakeupCh` + `1s` sweep) to `order-service`, which settles `orders` (`CONFIRMED`/`CANCELLED` or `REFUND_REQUIRED` on late payment) and emits `OrderConfirmed`, `ReservationReleaseRequested`, `PaymentCancelRequested`, or `RefundRequested` via `order_db.outbox_events`.

**Tech Stack:** Protobuf / Buf, Java 21 / Spring Boot 4.1.0 / Spring Data JPA / Flyway / Spring AMQP / `grpc-netty-shaded` 1.65.1 / Testcontainers, Go 1.24 (`pgx/v5`, `sqlc`, `google.golang.org/grpc` v1.65.0, `github.com/rabbitmq/amqp091-go` v1.10.0), Kong 3.8 declarative config, React 19 / TypeScript 6 / Vite 8 / Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-28-order-checkout-saga-design.md`

## Global Constraints

- **Service boundaries (`AGENTS.md`, `scripts/internal/boundaries/boundaries.go`):** Zero cross-service code imports, zero foreign `<other>_db` strings (even in SQL/code comments — `payment-service` must never mention `order_db`, and `order-service` must never mention `payment_db`), and zero `:8000` literals (`localhost:8000`, `127.0.0.1:8000`, `kong:8000`, `host.docker.internal:8000`) anywhere in `.go`, `.java`, `.yml`, `.yaml`, `.properties`, or `.env` under `services/*`.
- **Contract compatibility (`docs/steering/api-contracts.md`, `ADR-013`):** Protobuf changes in `contracts/proto/**/*.proto` must be strictly additive (`lower_snake_case`) on top of the SKU baseline, preserve existing field numbers (`1..4` in `OrderItem`, `1..2` in `CreateOrderRequest`, `1..3` in `CreateOrderResponse`), add `contracts/proto/payment/v1/payment.proto`, pass `buf format --diff --exit-code` and `buf lint`, and commit synchronized `buf generate` outputs in `contracts/go` and `contracts/java/src/main/generated`.
- **Transaction & concurrency discipline (`services/order-service/AGENTS.md`, `ADR-002`, `ADR-003`, `ADR-005`, `ADR-013`):** Never hold a database transaction open across gRPC, MoMo HTTPS, or RabbitMQ network calls in **any** service:
  - `payment-service` splits `CreatePaymentSession` into short `TX #A` (`INSERT INTO payments ... status = 'INITIATING' ON CONFLICT (order_id) DO NOTHING` committed before MoMo HTTPS with `3s` dial / `10s` total timeout) and short `TX #B` (`SELECT ... FOR UPDATE` verifying `status == 'INITIATING'` before saving `PENDING` + `pay_url`).
  - `order-service` bounds `PaymentService.CreatePaymentSession` gRPC deadline to `12s` (far below the `2m` `PENDING_STOCK` sweep), locks `orders` with `SELECT ... FOR UPDATE` verifying expected source status (plus `@Version`) on **every** state transition (`PENDING_STOCK -> CONFIRMED | PENDING_PAYMENT | CANCELLED` and `PENDING_PAYMENT -> CONFIRMED | CANCELLED`), and aborts without returning `payUrl` if already cancelled.
  - Both `OutboxRelay` (Go in `payment-service`) and `OutboxRelayScheduler` (Java in `order-service`) claim short leases (`FOR UPDATE SKIP LOCKED`, `next_attempt_at += 15s`) and commit before awaiting `3s` RabbitMQ publisher confirms, with immediate post-commit wakeup (`wakeupCh` / `afterCommit`).
  - Both `inventory-service` (`stock_reservations`) and `payment-service` (`payments`) execute `INSERT ... ON CONFLICT (order_id) DO NOTHING` before `SELECT ... FOR UPDATE` for conflict-free idempotency and tombstones (`RELEASED` / `CANCELLED`), and `inventory-service` sorts `sku_id ASC` before conditional `UPDATE stock`.
- **REST & frontend conventions (`AGENTS.md`, `ecommerce-fe/src/lib/api.ts`):** Keep Kong to its existing 2 application upstreams (`identity-service`, `bff-service`); keep standard REST JSON envelopes (`camelCase`); add `Idempotency-Key` to Kong CORS headers; keep `Idempotency-Key` on `error.status === null` or `409 REQUEST_IN_PROGRESS` (auto-retrying after `1.5s`), rotate only after terminal HTTP errors or checkout input changes; poll `GET /api/v1/orders/{orderId}` (`500ms` interval, max `10s`) on `/thanh-toan/ket-qua` while `status === "PENDING_PAYMENT"`; never fire duplicate manual toasts in React components for mutations handled by `api.ts`; never commit live MoMo secrets.
- **Windows & CI compatibility (`services/cart-service/AGENTS.md`):** Keep `services/cart-service/cmd/server/main_test.go` Linux signal tests intact; run `go test ./services/cart-service/internal/...` on Windows and `go run ./scripts/verify-service -service <surface>` for every touched surface (including `payment-service`).

## Review Focus

1. **Duplicate `sku_id` aggregation, independent variant stocks & `sku_id ASC` lock ordering in `inventory-service` (`Task 4`):** Repeating the same `sku_id` across lines whose individual quantities fit `available` (`2 <= 3`) but sum exceeds `available` (`2 + 2 = 4 > 3`) must roll back cleanly with `INVENTORY_INSUFFICIENT` and zero `reserved` leak, while distinct SKUs of the same `product_id` draw independently and concurrent `(sku-b, sku-a)` vs `(sku-a, sku-b)` orders never deadlock.
2. **DB-free MoMo HTTPS split, Tombstone race & forged-failure callback DoS protection in `payment-service` (`Task 6`):** `CreatePaymentSession` must commit `TX #A` before calling MoMo HTTPS, return cached `pay_url` on duplicate `order_id` replay, block late `CreatePaymentSession` when a `CANCELLED` tombstone already exists (or arrives in-flight before `TX #B`), and `HandleMomoCallback` must verify constant-time `HmacSHA256` **before** any DB query so neither forged `resultCode=0` nor forged `resultCode=1006` can hijack or cancel another user's pending payment.
3. **Cross-service compensation & bidirectional Late-Payment `REFUND_REQUIRED` convergence (`Task 6` & `Task 7`):** When `CreatePaymentSession` times out or errors after `ReserveStock`, `order-service` cancels the order (`FAILED_TERMINAL`) and emits **both** `ReservationReleaseRequested` and `PaymentCancelRequested`; when valid MoMo `resultCode=0` races with 15-minute expiry or user cancellation in either arrival order, both `orders` and `payments` converge to `REFUND_REQUIRED` (preserving `orders.status = CANCELLED` per `ADR-005` with zero `OrderConfirmed` emitted).
4. **Selective cart cleanup on `OrderConfirmed` & cart preservation on unpaid/failed MoMo orders (`Task 5`):** Consuming `OrderConfirmed` deletes only the ordered `cart_item_ids` (leaving unselected cart items intact), replays of the same `event_id` are ignored via `processed_events`, and `PENDING_PAYMENT` / `CANCELLED` MoMo orders leave the cart untouched.
5. **Atomic `Local TX #1` idempotency, stale `OrderPreviewPage` lock & `/thanh-toan/ket-qua` convergence polling (`Task 7` & `Task 9`):** Pre-`TX #1` checks write zero rows on failure; `idempotency_records(IN_PROGRESS)` inserts atomically with `orders(PENDING_STOCK)` via `ON CONFLICT DO NOTHING`; frontend keeps `Idempotency-Key` on `status === null` and `409 REQUEST_IN_PROGRESS`, locks stale submit on `409 PRODUCT_SNAPSHOT_MISMATCH`, and polls `/api/v1/orders/{orderId}` (`500ms` up to `10s`) while `PENDING_PAYMENT` settles across RabbitMQ.

---

## File Structure

- **Contracts, DB Init & Architecture Docs (`Task 1`):**
  - Create: `contracts/proto/payment/v1/payment.proto`, `docs/adr/014-order-checkout-saga-and-payment-service.md`
  - Modify: `contracts/proto/{catalog,cart,inventory,order}/v1/*.proto`, `deployments/postgres/init/01-create-databases.sql`, `AGENTS.md`, `docs/steering/service-catalog.md`, `services/{order,inventory,cart,bff}-service/AGENTS.md`
  - Modify (generated): `contracts/go/{catalog,cart,inventory,order,payment}/v1/*`, `contracts/java/src/main/generated/com/ecommerce/contracts/{catalog,cart,inventory,order,payment}/v1/*`
- **Identity RBAC (`Task 2`):**
  - Create: `services/identity-service/src/main/resources/db/migration/V12__add_order_permissions.sql`
  - Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`, `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`
- **Catalog Product Version (`Task 3`):**
  - Create: `services/catalog-service/internal/store/migrations/000006_add_product_version.up.sql`
  - Modify: `services/catalog-service/internal/domain/{product,admin}.go`, `services/catalog-service/internal/store/{store,admin,store_test}.go`, `services/catalog-service/internal/grpc/{server,admin,server_test}.go`
- **Inventory Reservations, Tombstone & Event Consumer (`Task 4`):**
  - Create: `services/inventory-service/internal/store/migrations/000003_create_stock_reservations.up.sql`, `services/inventory-service/internal/store/{reservation_store,reservation_store_test}.go`, `services/inventory-service/internal/events/{consumer,consumer_test}.go`
  - Modify: `services/inventory-service/go.mod`, `services/inventory-service/internal/config/config.go`, `services/inventory-service/internal/domain/stock.go`, `services/inventory-service/internal/grpc/{server,server_test}.go`, `services/inventory-service/cmd/server/main.go`
- **Cart Version Enrichment & Event Cleanup (`Task 5`):**
  - Create: `services/cart-service/internal/store/migrations/000004_create_processed_events.up.sql`, `services/cart-service/internal/events/{consumer,consumer_test}.go`
  - Modify: `services/cart-service/go.mod`, `services/cart-service/internal/config/config.go`, `services/cart-service/internal/domain/cart.go`, `services/cart-service/internal/catalog/client.go`, `services/cart-service/internal/store/queries/cart.sql`, `services/cart-service/internal/store/db/*`, `services/cart-service/internal/store/{store,store_test}.go`, `services/cart-service/internal/grpc/{server,server_test}.go`, `services/cart-service/cmd/server/main.go`
- **`payment-service` Go Microservice (`Task 6`):**
  - Create:
    - `services/payment-service/{go.mod,Dockerfile,AGENTS.md}`
    - `services/payment-service/cmd/server/main.go`
    - `services/payment-service/internal/config/{config,config_test}.go`
    - `services/payment-service/internal/domain/payment.go`
    - `services/payment-service/internal/momo/{signer,signer_test,client,client_test}.go`
    - `services/payment-service/internal/store/migrations/000001_create_payments_and_outbox.up.sql`
    - `services/payment-service/internal/store/{store,store_test}.go`
    - `services/payment-service/internal/events/{relay,consumer,events_test}.go`
    - `services/payment-service/internal/grpc/{server,server_test}.go`
    - `services/payment-service/internal/http/health.go`
  - Modify: `go.work`, `scripts/verify-foundation/main.go`
- **`order-service` Saga Orchestrator, Payment Event Consumer, Outbox & gRPC (`Task 7`):**
  - Modify: `services/order-service/pom.xml`, `services/order-service/Dockerfile`, `services/order-service/src/main/resources/application.yml`, `services/order-service/src/main/java/com/ecommerce/order/OrderApplication.java`
  - Create:
    - `services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql`
    - `services/order-service/src/main/java/com/ecommerce/order/config/{OrderProperties,GrpcClientConfig,RabbitMqConfig}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/common/{TraceIdProvider,GrpcTraceIdInterceptor,DomainException}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/domain/{OrderStatus,PaymentMethod,PaymentStatus,ShippingPolicy,SnapshotMismatchDetail}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/persistence/{OrderEntity,OrderItemEntity,OrderRepository,IdempotencyRecordEntity,IdempotencyRecordRepository,OutboxEventEntity,OutboxEventRepository,ProcessedEventEntity,ProcessedEventRepository}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/integration/{AuthorizationGrpcAdapter,CatalogGrpcAdapter,InventoryGrpcAdapter,PaymentGrpcAdapter}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/application/{IdempotencyService,OrderTransactionStepService,OrderSagaOrchestrator,OrderQueryService,PaymentEventConsumer,OutboxRelayScheduler,OrderReconciliationScheduler}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/grpc/{OrderGrpcService,GrpcServerLifecycle,GrpcErrorMapper}.java`
  - Test: `services/order-service/src/test/java/com/ecommerce/order/{support/PostgresRabbitIntegrationTestSupport,persistence/OrderMigrationAndLockingTest,application/IdempotencyServiceTest,application/OrderSagaOrchestratorTest,application/PaymentEventConsumerTest,application/OrderReconciliationSchedulerTest,grpc/OrderGrpcIntegrationTest}.java`
- **BFF, Kong, Docker Compose, CI, `Makefile` & `start-local` (`Task 8`):**
  - Create: `services/bff-service/internal/order/{client,client_test}.go`, `services/bff-service/internal/payment/{client,client_test}.go`, `services/bff-service/internal/http/{orders,orders_test,payments,payments_test}.go`
  - Modify: `services/bff-service/internal/http/{cart,cart_test}.go`, `services/bff-service/internal/config/{config,config_test,kong_development_test,kong_production_test,compose_production_test}.go`, `services/bff-service/cmd/server/{main,main_test}.go`, `deployments/kong/{kong,kong-production}.yaml`, `deployments/docker/compose.production.yml`, `.github/workflows/ci.yml`, `Makefile`, `.env.example`, `.env.production.example`, `scripts/start-local/{main,main_test}.go`
- **Storefront Checkout, Payment Result Convergence & Order Pages (`Task 9`):**
  - Create: `ecommerce-fe/src/components/checkout/shipping-address-dialog.tsx`, `ecommerce-fe/src/pages/{payment-result-page,payment-result-page.test,orders-page,orders-page.test,order-detail-page}.tsx`
  - Modify: `ecommerce-fe/src/lib/{types,api,api.test}.ts`, `ecommerce-fe/src/pages/{order-preview-page,order-preview-page.test}.tsx`, `ecommerce-fe/src/components/layout/{site-header,site-header.test}.tsx`, `ecommerce-fe/src/{App,App.test}.tsx`
- **Concurrency & Race-Condition Simulation Suite (`Task 10`):**
  - Create: `scripts/concurrency-sim/{main,scenarios,sim_test}.go`, `scripts/run-concurrency-sim.ps1`

---

### Task 1: Shared Protobuf Contracts, Database Init & Architecture Documentation

**Files:**
- Modify: `contracts/proto/catalog/v1/catalog.proto`
- Modify: `contracts/proto/cart/v1/cart.proto`
- Modify: `contracts/proto/inventory/v1/inventory.proto`
- Create: `contracts/proto/payment/v1/payment.proto`
- Modify: `contracts/proto/order/v1/order.proto`
- Modify: `contracts/go/{catalog,cart,inventory,order,payment}/v1/*` (generated)
- Modify: `contracts/java/src/main/generated/com/ecommerce/contracts/{catalog,cart,inventory,order,payment}/v1/*` (generated)
- Modify: `deployments/postgres/init/01-create-databases.sql`
- Create: `docs/adr/014-order-checkout-saga-and-payment-service.md`
- Modify: `AGENTS.md`, `docs/steering/service-catalog.md`, `services/order-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, `services/bff-service/AGENTS.md`

**Interfaces:**
- Consumes: `ecommerce.common.v1.Money` (`common/v1/common.proto`)
- Produces:
  - `ecommerce.catalog.v1`: `GetProductResponse.version = 24` (`int64`), `ProductSummary.version = 19` (`int64`), `AdminProductSummary.version = 24` (`int64`)
  - `ecommerce.cart.v1`: `CartItem.product_version = 17` (`int64`)
  - `ecommerce.inventory.v1`:
    - `InventoryService.ReserveStock(ReserveStockRequest) returns (ReserveStockResponse)` (confirm/release are event-driven only)
    - `ReservationLine { 1: sku_id, 2: product_id, 3: quantity }`, `ReserveStockRequest { 1: order_id, 2: items, 3: expires_at }`, `ReserveStockResponse { 1: order_id, 2: status }`
  - `ecommerce.payment.v1` (`go_package = "ecommerce.local/contracts/go/payment/v1;paymentv1"`, `java_package = "com.ecommerce.contracts.payment.v1"`):
    - `PaymentService`: `CreatePaymentSession(CreatePaymentSessionRequest) returns (CreatePaymentSessionResponse)`, `HandleMomoCallback(HandleMomoCallbackRequest) returns (HandleMomoCallbackResponse)`
    - `CreatePaymentSessionRequest { 1: order_id, 2: order_code, 3: user_id, 4: amount (Money), 5: expires_at }`, `CreatePaymentSessionResponse { 1: order_id, 2: status, 3: pay_url }`
    - `HandleMomoCallbackRequest { 1: partner_code, 2: order_id, 3: request_id, 4: amount (int64), 5: order_info, 6: order_type, 7: trans_id, 8: result_code (int32), 9: message, 10: pay_type, 11: response_time (int64), 12: extra_data, 13: signature, 14: source }`, `HandleMomoCallbackResponse { 1: order_id, 2: payment_status, 3: redirect_result }`
  - `ecommerce.order.v1`:
    - `OrderService` (4 order RPCs only — no MoMo callback RPC): `CreateOrder`, `GetOrder`, `ListOrders`, `CancelOrder`
    - `ShippingAddressSnapshot` (`1..8`), `OrderItem` (`1..4` unchanged + `5..15` per Spec 5.2.e), `CreateOrderRequest` (`1..2` unchanged + `3: idempotency_key, 4: shipping_address, 5: payment_method, 6: note`), `OrderDetail` (`1..17` per Spec 5.2.e), `CreateOrderResponse` (`1..3` unchanged + `4: order, 5: pay_url`)
    - `GetOrderRequest { 1: user_id, 2: order_id }`, `GetOrderResponse { 1: order }`
    - `ListOrdersRequest { 1: user_id, 2: status, 3: page, 4: page_size }`, `ListOrdersResponse { 1: items (repeated OrderDetail), 2: page, 3: page_size, 4: total_items (int64), 5: total_pages }`
    - `CancelOrderRequest { 1: user_id, 2: order_id, 3: reason }`, `CancelOrderResponse { 1: order }`
  - Async RabbitMQ JSON Envelope (`ecommerce.events` durable topic exchange, shared across Tasks 4, 5, 6, 7):
    ```json
    {
      "eventId": "uuid",
      "eventType": "OrderConfirmed | ReservationReleaseRequested | PaymentCancelRequested | RefundRequested | PaymentSucceeded | PaymentFailed",
      "eventVersion": 1,
      "occurredAt": "RFC3339",
      "aggregateId": "<orderId>",
      "correlationId": "<traceId>",
      "data": {
        "orderId": "uuid",
        "orderCode": "ORD-20261105-ABC123",
        "userId": "uuid",
        "reason": "string",
        "momoOrderId": "uuid",
        "momoTransId": "string",
        "amountMinor": 150000,
        "currency": "VND",
        "resultCode": 0,
        "message": "string",
        "items": [{ "cartItemId": "uuid", "productId": "dt-1", "skuId": "dt-1-v1", "quantity": 2 }]
      }
    }
    ```
    Routing keys: `order.confirmed`, `order.reservation.release_requested`, `order.payment.cancel_requested`, `order.payment.refund_requested`, `payment.succeeded`, `payment.failed`.

- [ ] **Step 1: Add `payment.proto`, update `catalog.proto`, `cart.proto`, `inventory.proto`, `order.proto`, and add `payment_db` to `01-create-databases.sql`**
  - Keep every existing field number and name untouched (`OrderItem` `1..4`, `CreateOrderRequest` `1..2`, `CreateOrderResponse` `1..3`).
  - Append `CREATE DATABASE payment_db;` to `deployments/postgres/init/01-create-databases.sql`.

- [ ] **Step 2: Format, lint, and regenerate Go & Java contract modules**
  - Run: `buf format -w; buf generate`
  - Stage generated files so `git ls-files --others` check in `verify-service` sees tracked outputs: `git add contracts`
  - Run: `go run ./scripts/verify-service -service contracts`
  - Expected: `PASSED: contracts` (`exit 0`).

- [ ] **Step 3: Record `ADR-014` and align steering/service docs**
  - Create `docs/adr/014-order-checkout-saga-and-payment-service.md` following `docs/adr/TEMPLATE.md` documenting: extraction of `payment-service` (Go 1.24, HTTP `:8086`, private gRPC `127.0.0.1:50056`, `payment_db`, extending `ADR-001`); `order-service` private gRPC `:50055`; Hybrid Saga (`Catalog` SKU snapshot `version` check + `ReserveStock` `1.5s` + `CreatePaymentSession` `12s` with DB-free `3s/10s` MoMo HTTPS split + `Kong -> bff -> payment-service` callback ingress + bidirectional Outbox with post-commit wakeup + Tombstones on `stock_reservations` & `payments` + centralized `OrderReconciliationScheduler` + `REFUND_REQUIRED` late-payment convergence per `ADR-005`).
  - Update `AGENTS.md` (Repository Map & Local Port Topology adding `payment-service` `:8086` / `127.0.0.1:50056` and `order-service` `127.0.0.1:50055`), `docs/steering/service-catalog.md`, `services/order-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, and `services/bff-service/AGENTS.md`.

- [ ] **Step 4: Verify boundary check and downstream compilation**
  - Run: `go run ./scripts/check-boundaries && go build ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/... ./services/bff-service/... && mvn --batch-mode -pl contracts/java -am test-compile`
  - Expected: Exit code `0`.

- [ ] **Step 5: Commit**
  - Run: `git add contracts deployments/postgres/init/01-create-databases.sql docs AGENTS.md services/order-service/AGENTS.md services/inventory-service/AGENTS.md services/cart-service/AGENTS.md services/bff-service/AGENTS.md && git commit -m "feat(contracts): add payment-service, order checkout, and snapshot version contracts"`

---

### Task 2: `identity-service` Order RBAC Permissions Migration

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V12__add_order_permissions.sql`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`

**Interfaces:**
- Produces: System permissions `order.create`, `order.view`, `order.cancel` (`type = 'PERMISSION'`, `system_item = true`) assigned to `CUSTOMER` in `auth_item_children`.

- [ ] **Step 1: Write failing migration and RBAC inheritance tests**
  - In `IdentityDatabaseMigrationTest#createsAccountTablesAndSeedsCustomerRole`:
    - Add `"order.cancel"`, `"order.create"`, `"order.view"` alphabetically between `"inventory.view"` and `"product.create"` in `containsExactly(...)`.
    - Assert `auth_item_children` for `parent_name = 'CUSTOMER' AND child_name LIKE 'order.%'` contains `["order.cancel", "order.create", "order.view"]`, and `parent_name = 'ADMIN'` does not contain them.
  - In `RbacPersistenceTest`, add `grantsOrderPermissionsToCustomerAndSuperAdminButNotAdmin()` asserting `repository.findAllowedPermissions(USER_ID, List.of("order.create", "order.view", "order.cancel"))` returns all 3 for `CUSTOMER` and `SUPER_ADMIN`, and `[]` for `ADMIN`.

- [ ] **Step 2: Run Identity tests to verify RED failure**
  - Run: `mvn -f services/identity-service/pom.xml test -Dtest=IdentityDatabaseMigrationTest,RbacPersistenceTest`
  - Expected: FAIL (`order.cancel`, `order.create`, `order.view` missing).

- [ ] **Step 3: Create `V12__add_order_permissions.sql`**
  - Follow `V11__add_cart_permissions.sql` to upsert `order.create`, `order.view`, and `order.cancel` into `auth_items` (`system_item = true`) and insert `('CUSTOMER', 'order.create')`, `('CUSTOMER', 'order.view')`, `('CUSTOMER', 'order.cancel')` into `auth_item_children` with `on conflict do nothing`.

- [ ] **Step 4: Run `identity-service` verification gate**
  - Run: `go run ./scripts/verify-service -service identity-service`
  - Expected: `PASSED: identity-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/identity-service && git commit -m "feat(identity): seed order RBAC permissions for customers"`

---

### Task 3: `catalog-service` Monotonic Product `version`

**Files:**
- Create: `services/catalog-service/internal/store/migrations/000006_add_product_version.up.sql`
- Modify: `services/catalog-service/internal/domain/product.go`
- Modify: `services/catalog-service/internal/domain/admin.go`
- Modify: `services/catalog-service/internal/store/store.go`
- Modify: `services/catalog-service/internal/store/admin.go`
- Modify: `services/catalog-service/internal/store/store_test.go`
- Modify: `services/catalog-service/internal/grpc/server.go`
- Modify: `services/catalog-service/internal/grpc/admin.go`
- Modify: `services/catalog-service/internal/grpc/server_test.go`

**Interfaces:**
- Consumes: `catalogv1` `version` fields from Task 1.
- Produces:
  - `domain.Product.Version int64`, `domain.Summary.Version int64`, `domain.AdminSummary.Version int64` (initialized to `1`, incremented `version = version + 1` on `UpdateProduct` and `SoftDelete`).
  - `CatalogService.GetProduct`, `GetAdminProduct`, `CreateProduct`, `UpdateProduct`, `SearchProducts`, `ListAdminProducts` populate `version`.

- [ ] **Step 1: Write failing store & gRPC tests for `version` lifecycle**
  - In `services/catalog-service/internal/store/store_test.go`, add `TestProductVersionIncrementsOnUpdateAndSoftDelete`: create product -> assert `Version == 1`; call `UpdateProduct` -> assert `Version == 2`; call `SoftDelete(id, true)` -> assert `Version == 3`; call `SoftDelete(id, false)` -> assert `Version == 4`; assert `Search` and `ListAdmin` return `Version == 4`.
  - In `services/catalog-service/internal/grpc/server_test.go`, assert `GetProductResponse.GetVersion()` and `SearchProductsResponse.Items[0].GetVersion()` map `domain.Product.Version`.

- [ ] **Step 2: Run `catalog-service` tests to verify RED failure**
  - Run: `go test ./services/catalog-service/...`
  - Expected: FAIL (`Version` field undefined / stays `0`).

- [ ] **Step 3: Add `000006_add_product_version.up.sql` and wire `version` across domain, store, and gRPC**
  - Migration SQL: `ALTER TABLE products ADD COLUMN version bigint NOT NULL DEFAULT 1;`
  - Add `Version int64` to `domain.Product`, `domain.Summary`, `domain.AdminSummary` (default `1` in in-memory seed/fixtures if `0`).
  - Include `version` in `productColumns`, `Search` `SELECT`, and `ListAdmin` `SELECT`; increment `version = version + 1` inside `SoftDelete` (`store.go`) and `UpdateProduct` (`admin.go`).
  - Map `Version` in `internal/grpc/server.go` (`productToProto`, `summaryToProto`) and `internal/grpc/admin.go` (`adminSummaryToProto`).

- [ ] **Step 4: Run `catalog-service` verification gate**
  - Run: `go run ./scripts/verify-service -service catalog-service`
  - Expected: `PASSED: catalog-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/catalog-service && git commit -m "feat(catalog): track monotonic product version on commercial updates"`

---

### Task 4: `inventory-service` Atomic Reservations, Tombstone & RabbitMQ Event Consumer

**Files:**
- Modify: `services/inventory-service/go.mod`
- Create: `services/inventory-service/internal/store/migrations/000003_create_stock_reservations.up.sql`
- Modify: `services/inventory-service/internal/config/config.go`
- Modify: `services/inventory-service/internal/domain/stock.go`
- Create: `services/inventory-service/internal/store/reservation_store.go`
- Create: `services/inventory-service/internal/store/reservation_store_test.go`
- Modify: `services/inventory-service/internal/grpc/server.go`
- Modify: `services/inventory-service/internal/grpc/server_test.go`
- Create: `services/inventory-service/internal/events/consumer.go`
- Create: `services/inventory-service/internal/events/consumer_test.go`
- Modify: `services/inventory-service/cmd/server/main.go`

**Interfaces:**
- Consumes: `inventoryv1.InventoryService.ReserveStock` & RabbitMQ envelope from Task 1 (`OrderConfirmed`, `ReservationReleaseRequested`).
- Produces:
  - `domain.ReservationLine { SkuID string; ProductID string; Quantity int32 }`, `domain.InsufficientStockError { SkuID string; ProductID string; Available int32; Requested int32 }`, `domain.ErrReservationAlreadyReleased`.
  - `Store.ReserveStock(ctx, orderID string, items []domain.ReservationLine, expiresAt time.Time) (string, error)`
  - `Store.ConfirmReservation(ctx, orderID string) (string, error)`
  - `Store.ReleaseReservation(ctx, orderID, reason string) (string, error)`
  - gRPC `ReserveStock` returns `codes.FailedPrecondition` with `errdetails.ErrorInfo{Reason: "INVENTORY_INSUFFICIENT", Domain: "ecommerce.inventory.v1", Metadata: {"sku_id": ..., "product_id": ..., "available": ..., "requested": ...}}` or `Reason: "RESERVATION_ALREADY_RELEASED"`.
  - Config env addition: `RABBITMQ_URL` (default `""`).

- [ ] **Step 1: Write failing reservation, duplicate-SKU aggregation, deadlock-free ordering, conflict-free tombstone, and event consumer tests**
  - In `reservation_store_test.go`:
    - `TestReserveStockAggregatesDuplicateSkuIDsAndKeepsDistinctVariantsIndependent`: seed `p-1-v1` (`quantity=3, reserved=0`) and `p-1-v2` (`quantity=3, reserved=0`); duplicate `p-1-v1` lines `2 + 2` fail with `InsufficientStockError{SkuID: "p-1-v1", Available: 3, Requested: 4}` and `Reserved == 0`; distinct `p-1-v1` (`2`) + `p-1-v2` (`2`) succeed independently.
    - `TestReserveStockIdempotentReplay`: calling `ReserveStock("ord-2", ...)` twice reserves `2` total (not `4`) and returns `"ACTIVE"`.
    - `TestReleaseBeforeReserveWritesTombstoneBlockingLateReserve`: `ReleaseReservation("ord-late", "TIMEOUT")` before `ReserveStock` writes `RELEASED` tombstone; subsequent `ReserveStock("ord-late", ...)` fails with `domain.ErrReservationAlreadyReleased` and `reserved == 0`.
    - `TestConcurrentLateReserveAndReleaseNeverRaisesUniqueViolation`: 20 concurrent iterations of `ReserveStock` vs `ReleaseReservation` produce zero `23505` errors and exact stock invariants.
    - `TestConfirmReservationDeductsQuantityAndReservedIdempotently`: `ReserveStock` (`qty=5, reserved=2`) -> `ConfirmReservation` -> `quantity == 3, reserved == 0, status == "CONFIRMED"`, idempotent on replay.
    - `TestConcurrentCrossOrderReservationsDoNotDeadlock`: 20 goroutines reserving `(sku-a, sku-b)` and `(sku-b, sku-a)` concurrently complete with zero deadlocks.
  - In `server_test.go` and `consumer_test.go`: assert `ReserveStock` gRPC `ErrorInfo` metadata and consumer routing of `order.confirmed` / `order.reservation.release_requested`.

- [ ] **Step 2: Run `inventory-service` tests to verify RED failure**
  - Run: `go test ./services/inventory-service/...`
  - Expected: FAIL.

- [ ] **Step 3: Add `amqp091-go`, migration `000003`, `reservation_store.go`, `ReserveStock` gRPC handler, and RabbitMQ consumer**
  - Add `github.com/rabbitmq/amqp091-go v1.10.0` to `services/inventory-service/go.mod`.
  - Create `000003_create_stock_reservations.up.sql` per Spec Section 5.4.
  - Implement `ReserveStock`, `ReleaseReservation`, and `ConfirmReservation` using `INSERT INTO stock_reservations ... ON CONFLICT (order_id) DO NOTHING` first, `SELECT ... FOR UPDATE`, and `ORDER BY sku_id ASC`.
  - Implement `internal/events/consumer.go` (`ecommerce.events`, DLX `ecommerce.events.dlx`, DLQ `inventory.order-events.dlq`, queue `inventory.order-events` bound to `order.confirmed` and `order.reservation.release_requested`, max 5 attempts) and start it in `cmd/server/main.go` when `cfg.RabbitMQURL != ""`.

- [ ] **Step 4: Run `inventory-service` verification gate**
  - Run: `go run ./scripts/verify-service -service inventory-service`
  - Expected: `PASSED: inventory-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/inventory-service go.work.sum && git commit -m "feat(inventory): implement atomic stock reservations, tombstones, and order event consumer"`

---

### Task 5: `cart-service` `product_version` Enrichment & `OrderConfirmed` Cleanup Consumer

**Files:**
- Modify: `services/cart-service/go.mod`
- Create: `services/cart-service/internal/store/migrations/000004_create_processed_events.up.sql`
- Modify: `services/cart-service/internal/store/queries/cart.sql`
- Modify: `services/cart-service/internal/store/db/*`
- Modify: `services/cart-service/internal/config/config.go`
- Modify: `services/cart-service/internal/domain/cart.go`
- Modify: `services/cart-service/internal/catalog/client.go`
- Modify: `services/cart-service/internal/store/{store,store_test}.go`
- Modify: `services/cart-service/internal/grpc/{server,server_test}.go`
- Create: `services/cart-service/internal/events/{consumer,consumer_test}.go`
- Modify: `services/cart-service/cmd/server/main.go`

**Interfaces:**
- Consumes: `catalogv1.GetProductResponse.version` (Task 3), `cartv1.CartItem.product_version` & `OrderConfirmed` RabbitMQ event (Task 1).
- Produces:
  - `CartItem.ProductVersion` populated during `GetCart` / mutation enrichment.
  - `Store.RemoveOrderedItems(ctx context.Context, eventID, userID string, items []domain.OrderedItemRef) error` where `OrderedItemRef { CartItemID string; ProductID string; SkuID string }`.
  - Config env addition: `RABBITMQ_URL` (default `""`).

- [ ] **Step 1: Write failing tests for `product_version` enrichment and selective idempotent cart cleanup**
  - In `services/cart-service/internal/grpc/server_test.go`, assert `GetCart` maps `Product.Version = 7` into `CartItem.GetProductVersion() == 7`.
  - In `services/cart-service/internal/store/store_test.go` and `internal/events/consumer_test.go`, add `TestHandleOrderConfirmedRemovesOnlyOrderedItemsIdempotently` verifying ordered `itemA` (by `cartItemId`) and `itemB` (fallback by `skuId`) are deleted, unselected `itemC` stays intact, and replaying `"evt-1"` is ignored via `processed_events`.

- [ ] **Step 2: Run `cart-service` internal tests to verify RED failure**
  - Run: `go test ./services/cart-service/internal/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement migration `000004`, `product_version` mapping, `RemoveOrderedItems`, and RabbitMQ consumer**
  - Add `github.com/rabbitmq/amqp091-go v1.10.0` to `services/cart-service/go.mod`.
  - Create `000004_create_processed_events.up.sql` per Spec Section 5.5, map `Version int64` through `internal/catalog/client.go` -> `domain.EnrichedCartItem` -> `cartv1.CartItem.ProductVersion`, and implement transactional `Store.RemoveOrderedItems`.
  - Implement `internal/events/consumer.go` (`cart.order-events` bound to `order.confirmed` with DLX/DLQ) and start it inside `buildProductionService` in `cmd/server/main.go` only when `cfg.RabbitMQURL != ""`.

- [ ] **Step 4: Run `cart-service` verification gate**
  - Run: `go run ./scripts/verify-service -service cart-service`
  - Expected: `PASSED: cart-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/cart-service go.work.sum && git commit -m "feat(cart): expose product version and clean up confirmed order items"`

---

### Task 6: `payment-service` (New Go 1.24 Microservice) — `payment_db`, MoMo v2 HMAC, Outbox Relay & Compensation Consumer

**Files:**
- Modify: `go.work`
- Modify: `scripts/verify-foundation/main.go`
- Create: `services/payment-service/go.mod`
- Create: `services/payment-service/Dockerfile`
- Create: `services/payment-service/AGENTS.md`
- Create: `services/payment-service/internal/config/{config,config_test}.go`
- Create: `services/payment-service/internal/domain/payment.go`
- Create: `services/payment-service/internal/momo/{signer,signer_test,client,client_test}.go`
- Create: `services/payment-service/internal/store/migrations/000001_create_payments_and_outbox.up.sql`
- Create: `services/payment-service/internal/store/{store,store_test}.go`
- Create: `services/payment-service/internal/events/{relay,consumer,events_test}.go`
- Create: `services/payment-service/internal/grpc/{server,server_test}.go`
- Create: `services/payment-service/internal/http/health.go`
- Create: `services/payment-service/cmd/server/main.go`

**Interfaces:**
- Consumes: `paymentv1.PaymentServiceServer` & RabbitMQ envelope from Task 1 (`order.payment.cancel_requested`, `order.payment.refund_requested`).
- Produces:
  - Go module `ecommerce.local/payment-service` listening on HTTP `HTTP_ADDR` (default `":8086"`, `/healthz`) and private gRPC `GRPC_ADDR` (default `"127.0.0.1:50056"`).
  - Config env (MUST NOT contain `:8000` or any foreign `*_db` token anywhere in `services/payment-service/`):
    - `DATABASE_URL` (`postgres://ecommerce:change-me-local-only@127.0.0.1:5432/payment_db?sslmode=disable`), `RABBITMQ_URL` (`""` default), `OUTBOX_POLL_INTERVAL` (`1s`), `MOMO_PARTNER_CODE` (`"MOMO"`), `MOMO_ACCESS_KEY` (`""`), `MOMO_SECRET_KEY` (`""`), `MOMO_API_ENDPOINT` (`"https://test-payment.momo.vn/v2/gateway/api/create"`), `MOMO_CONNECT_TIMEOUT` (`3s`), `MOMO_READ_TIMEOUT` (`10s`), `MOMO_REDIRECT_URL` (`"http://localhost:8081/api/v1/payments/momo-redirect"`), `MOMO_IPN_URL` (`"http://localhost:8081/api/v1/payments/momo-ipn"`), `PAYMENT_MOMO_MOCK_ENABLED` (`false`).
  - `momo.SignCreateRequest(...) string` and `momo.VerifyCallbackSignature(..., secret string) bool` using `crypto/hmac` + `hmac.Equal` over exact MoMo v2 raw canonical strings from Spec Section 4.3 Pillar 7.
  - Store & gRPC methods:
    - `CreatePaymentSession`: `TX #A` (`INSERT INTO payments ... status = 'INITIATING' ON CONFLICT (order_id) DO NOTHING`) -> MoMo HTTPS outside DB TX -> `TX #B` (`SELECT ... FOR UPDATE` checking `status == 'INITIATING'`).
    - `HandleMomoCallback`: verifies `HmacSHA256` first (`INVALID_MOMO_SIGNATURE`), checks `amount == amount_minor` (`PAYMENT_AMOUNT_MISMATCH`), updates `payments` (`SUCCEEDED` / `FAILED` / `REFUND_REQUIRED`) + inserts `payment_db.outbox_events` (`payment.succeeded` / `payment.failed`) in 1 local TX, and calls `relay.Notify()`.
    - Consumer on `payment.order-events` (`order.payment.cancel_requested` -> Tombstone `CANCELLED` `momo_request_id = 'tombstone:' || order_id` or `INITIATING|PENDING -> CANCELLED` / `SUCCEEDED -> REFUND_REQUIRED`; `order.payment.refund_requested` -> `REFUND_REQUIRED`).

- [ ] **Step 1: Scaffold `services/payment-service/go.mod`, register in `go.work` & `verify-foundation`, and write failing unit, Postgres store, gRPC, and Outbox tests**
  - Add `./services/payment-service` to `go.work` and `"services/payment-service/go.mod"` to `requiredFoundationFiles` in `scripts/verify-foundation/main.go`.
  - Write failing tests:
    1. `signer_test.go` & `client_test.go`: exact MoMo v2 request HMAC signature, constant-time `VerifyCallbackSignature` rejecting tampered `resultCode=0`, tampered `resultCode=1006`, or tampered `amount`, `httptest.Server` timeout enforcement (`DialTimeout=3s`, `Timeout=10s`), and `PAYMENT_MOMO_MOCK_ENABLED=true` generating a locally signed callback URL.
    2. `store_test.go`:
       - `TestCreatePaymentSessionTwoPhaseCommitReplayAndTombstone`: `BeginCreateSession` (`TX #A`) -> `CompleteCreateSession` (`TX #B`) saves `PENDING` + `pay_url`; replaying `BeginCreateSession` for the same `order_id` returns cached `pay_url` immediately; calling `CancelPayment("ord-tomb", "TIMEOUT")` **before** `BeginCreateSession` plants `CANCELLED` tombstone and rejects late `BeginCreateSession` with `ErrPaymentAlreadyCancelled`; calling `CancelPayment` **between** `TX #A` and `TX #B` causes `CompleteCreateSession` to keep `CANCELLED` and return `ErrPaymentAlreadyCancelled` without storing `pay_url`.
       - `TestHandleCallbackIdempotencyAndLatePaymentRefundRequired`: concurrent `HandleMomoCallback(resultCode=0)` calls produce `SUCCEEDED` and **1** `outbox_events` row (`payment.succeeded`); late `HandleMomoCallback(resultCode=0)` on a `CANCELLED` payment transitions `CANCELLED -> REFUND_REQUIRED` and writes `payment.succeeded` outbox row so `order-service` syncs `REFUND_REQUIRED`; subsequent `resultCode=1006` callback never downgrades `SUCCEEDED` or `REFUND_REQUIRED`.
    3. `server_test.go` & `events_test.go`: forged signature (`resultCode=0` and `resultCode=1006`) rejected with `codes.InvalidArgument` (`INVALID_MOMO_SIGNATURE`) **before** touching the store; `OutboxRelay` lease-claim (`FOR UPDATE SKIP LOCKED`, `next_attempt_at += 15s`) + `Notify()` wakeup + `payment.order-events` consumer handling `PaymentCancelRequested` and `RefundRequested`.

- [ ] **Step 2: Run `payment-service` tests to verify RED failure**
  - Run: `go test ./services/payment-service/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement `payment-service` migration `000001`, MoMo v2 client/signer, store, Outbox relay, RabbitMQ consumer, gRPC/HTTP servers, `Dockerfile`, and `AGENTS.md`**
  - Create `000001_create_payments_and_outbox.up.sql` per Spec Section 5.6 (ensure zero mention of `order_db` or `:8000` in any file or comment).
  - Implement `store.go`, `momo/signer.go`, `momo/client.go`, `events/relay.go` (`wakeupCh` buffered size 1 + `1s` ticker + short claim TX committed before `3s` AMQP publisher confirm + short `MarkPublished` TX), `events/consumer.go` (`payment.order-events` with DLX `ecommerce.events.dlx` / DLQ `payment.order-events.dlq`), `grpc/server.go` (`errdetails.ErrorInfo` with `Domain: "ecommerce.payment.v1"`), `cmd/server/main.go`, `Dockerfile` (`EXPOSE 8086 50056`), and `services/payment-service/AGENTS.md`.

- [ ] **Step 4: Run `payment-service` verification gate**
  - Run: `go run ./scripts/verify-service -service payment-service`
  - Expected: `PASSED: payment-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add go.work go.work.sum scripts/verify-foundation/main.go services/payment-service && git commit -m "feat(payment): create Go payment-service with MoMo v2 HMAC, tombstones, and outbox relay"`

---

### Task 7: `order-service` Persistence, Snapshot Diff, Saga Orchestrator, Payment Event Consumer & Reconciler

**Files:**
- Modify: `services/order-service/pom.xml`
- Modify: `services/order-service/Dockerfile`
- Modify: `services/order-service/src/main/resources/application.yml`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/OrderApplication.java`
- Create: `services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql`
- Create: `services/order-service/src/main/java/com/ecommerce/order/config/{OrderProperties,GrpcClientConfig,RabbitMqConfig}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/common/{TraceIdProvider,GrpcTraceIdInterceptor,DomainException}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/domain/{OrderStatus,PaymentMethod,PaymentStatus,ShippingPolicy,SnapshotMismatchDetail}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/persistence/{OrderEntity,OrderItemEntity,OrderRepository,IdempotencyRecordEntity,IdempotencyRecordRepository,OutboxEventEntity,OutboxEventRepository,ProcessedEventEntity,ProcessedEventRepository}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/integration/{AuthorizationGrpcAdapter,CatalogGrpcAdapter,InventoryGrpcAdapter,PaymentGrpcAdapter}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/application/{IdempotencyService,OrderTransactionStepService,OrderSagaOrchestrator,OrderQueryService,PaymentEventConsumer,OutboxRelayScheduler,OrderReconciliationScheduler}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/grpc/{OrderGrpcService,GrpcServerLifecycle,GrpcErrorMapper}.java`
- Test: `services/order-service/src/test/java/com/ecommerce/order/{support/PostgresRabbitIntegrationTestSupport,persistence/OrderMigrationAndLockingTest,application/IdempotencyServiceTest,application/OrderSagaOrchestratorTest,application/PaymentEventConsumerTest,application/OrderReconciliationSchedulerTest,grpc/OrderGrpcIntegrationTest}.java`

**Interfaces:**
- Consumes: `identity.v1.AuthorizationService` (`:50054`), `catalog.v1.CatalogService` (`:50051`), `inventory.v1.InventoryService` (`:50052`), `payment.v1.PaymentService` (`:50056`), `order.v1.OrderService`, and RabbitMQ events (`payment.succeeded`, `payment.failed`) from Tasks 1 & 6.
- Produces:
  - Private gRPC server on `${ORDER_GRPC_HOST:127.0.0.1}:${ORDER_GRPC_PORT:50055}` implementing all 4 `OrderService` RPCs (`CreateOrder`, `GetOrder`, `ListOrders`, `CancelOrder`).
  - Shipping fee server invariant (`ShippingPolicy`): `subtotalMinor >= 299_000L ? 0L : 25_000L`, `totalMinor = subtotalMinor + shippingFeeMinor`, currency `"VND"`.
  - Rich gRPC error codes (`com.google.rpc.ErrorInfo` with `domain = "ecommerce.order.v1"`):
    - `InvalidArgument`: `INVALID_PARAMETER`
    - `PermissionDenied`: `ORDER_PERMISSION_DENIED`
    - `NotFound`: `ORDER_NOT_FOUND`
    - `FailedPrecondition`: `PRODUCT_SNAPSHOT_MISMATCH` (`Metadata` `<skuId>` -> `"[UNAVAILABLE|VARIANT_UNAVAILABLE|PRICE_CHANGED|INFO_UPDATED|VERSION_CHANGED] ..."`), `INVENTORY_INSUFFICIENT`, `ORDER_NOT_CANCELLABLE`, `PAYMENT_GATEWAY_REJECTED`
    - `Aborted` / `AlreadyExists`: `REQUEST_IN_PROGRESS`, `IDEMPOTENCY_KEY_REUSED`
    - `Unavailable`: `AUTHORIZATION_UNAVAILABLE`, `CATALOG_UNAVAILABLE`, `INVENTORY_UNAVAILABLE`, `PAYMENT_GATEWAY_UNAVAILABLE`
  - Environment config in `application.yml` (MUST NOT contain `:8000`, `MOMO_*` secrets, or `payment_db`):
    - `ORDER_DB_URL` (`jdbc:postgresql://localhost:5432/order_db`), `ORDER_DB_USERNAME`, `ORDER_DB_PASSWORD`
    - `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USERNAME`, `RABBITMQ_PASSWORD`
    - `ORDER_GRPC_HOST` (`127.0.0.1`), `ORDER_GRPC_PORT` (`50055`), `IDENTITY_GRPC_ADDR` (`127.0.0.1:50054`, `500ms`), `CATALOG_GRPC_ADDR` (`127.0.0.1:50051`, `2s`), `INVENTORY_GRPC_ADDR` (`127.0.0.1:50052`, `1500ms`), `PAYMENT_GRPC_ADDR` (`127.0.0.1:50056`, `12s`), `ORDER_PAYMENT_TTL` (`15m`).

- [ ] **Step 1: Add Spring Boot 4.1 dependencies to `services/order-service/pom.xml` and write failing unit & Testcontainers tests**
  - Add dependencies matching `identity-service/pom.xml`: `spring-boot-starter-data-jpa`, `spring-boot-starter-flyway`, `flyway-database-postgresql`, `postgresql`, `spring-boot-starter-amqp`, `io.grpc:grpc-netty-shaded:1.65.1`, plus Testcontainers (`postgresql`, `rabbitmq`).
  - Write failing tests covering:
    1. `IdempotencyServiceTest`: pre-existing `IN_PROGRESS` / `COMPLETED` / `FAILED_TERMINAL` / `IDEMPOTENCY_KEY_REUSED` replay, and concurrent duplicate `INSERT ... ON CONFLICT (user_id, idempotency_key) DO NOTHING` inside `Local TX #1` without `23505`.
    2. `OrderSagaOrchestratorTest`:
       - Read-only pre-`TX #1` checks (validation, RBAC, per-SKU snapshot diff `UNAVAILABLE` / `VARIANT_UNAVAILABLE` / `PRICE_CHANGED` / `INFO_UPDATED` / `VERSION_CHANGED`) write zero rows on failure.
       - `COD` happy path (`PENDING_STOCK -> CONFIRMED`, `UNPAID`, `OrderConfirmed` outbox, zero DB TX held during gRPC).
       - `INVENTORY_INSUFFICIENT` marks `CANCELLED` + `FAILED_TERMINAL` with zero compensation outbox; uncertain `ReserveStock` timeout marks `CANCELLED` + `FAILED_TERMINAL` + `ReservationReleaseRequested`.
       - `MOMO` happy path calls `PaymentGrpcAdapter.createPaymentSession` (`12s` deadline) and transitions `PENDING_STOCK -> PENDING_PAYMENT` (`pay_url`, `COMPLETED`) under `SELECT ... FOR UPDATE`; if reconciler already cancelled the order in flight, `markPendingPayment` aborts without overwriting `CANCELLED` or returning `payUrl`.
       - `MOMO` `CreatePaymentSession` timeout/failure marks `CANCELLED` + `FAILED_TERMINAL` and writes **both** `ReservationReleaseRequested` and `PaymentCancelRequested` into `outbox_events`.
    3. `PaymentEventConsumerTest`:
       - `PaymentSucceeded` on `PENDING_PAYMENT` transitions `CONFIRMED` (`PAID`, `payment_trans_id`) + `OrderConfirmed` outbox; duplicate `eventId` is ignored via `processed_events`.
       - Late `PaymentSucceeded` on `CANCELLED` keeps `status = CANCELLED`, sets `paymentStatus = REFUND_REQUIRED`, records `paymentTransId`, and writes `RefundRequested` (`order.payment.refund_requested`) into `outbox_events` (zero `OrderConfirmed`).
       - `PaymentFailed` on `PENDING_PAYMENT` transitions `CANCELLED` (`FAILED`) + `ReservationReleaseRequested`, and is a no-op if already `CONFIRMED`.
       - `CancelOrder` on `PENDING_PAYMENT` emits both `ReservationReleaseRequested` and `PaymentCancelRequested`; on `CONFIRMED` throws `ORDER_NOT_CANCELLABLE`.
    4. `OrderReconciliationSchedulerTest`: sweeps `PENDING_STOCK > 2m` (`CANCELLED` + `IN_PROGRESS -> FAILED_TERMINAL` + `ReservationReleaseRequested` + `PaymentCancelRequested` when `MOMO`) and `PENDING_PAYMENT` with `payment_expires_at < now()` (`CANCELLED` + `ReservationReleaseRequested` + `PaymentCancelRequested`).
    5. `OrderGrpcIntegrationTest`: end-to-end gRPC + RabbitMQ round-trip (`CreateOrder` -> simulated `payment.succeeded` message -> `OrderConfirmed` published via post-commit `OutboxRelayScheduler` wakeup without holding a DB TX during broker confirm).

- [ ] **Step 2: Run `order-service` tests to verify RED failure**
  - Run: `mvn -f services/order-service/pom.xml test`
  - Expected: FAIL.

- [ ] **Step 3: Implement `V1` Flyway schema, JPA entities (`@Version` + `FOR UPDATE`), gRPC adapters, `OrderSagaOrchestrator`, `PaymentEventConsumer`, `OutboxRelayScheduler` (post-commit wakeup + lease-claim), `OrderReconciliationScheduler`, and gRPC server**
  - Create `V1__create_orders_idempotency_and_outbox.sql` per Spec Section 5.7 (`orders`, `order_items`, `idempotency_records`, `outbox_events` with `routing_key`, `processed_events` + partial/reconcile indexes).
  - Bind durable queue `order.payment-events` (with DLX `ecommerce.events.dlx` / DLQ `order.payment-events.dlq`) to `ecommerce.events` on `payment.succeeded` and `payment.failed`.
  - Register `TransactionSynchronization.afterCommit` in `OrderTransactionStepService` to trigger `OutboxRelayScheduler` asynchronously right after commit alongside its `1000ms` scheduled sweep.
  - Update `services/order-service/Dockerfile` to `EXPOSE 8083 8093 50055`.

- [ ] **Step 4: Run `order-service` verification gate**
  - Run: `go run ./scripts/verify-service -service order-service`
  - Expected: `PASSED: order-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/order-service && git commit -m "feat(order): implement hybrid checkout saga, payment event consumer, reconciler, and outbox"`

---

### Task 8: `bff-service`, Kong Gateway, Docker Compose, CI, `Makefile` & `start-local` Wiring

**Files:**
- Create: `services/bff-service/internal/order/{client,client_test}.go`
- Create: `services/bff-service/internal/payment/{client,client_test}.go`
- Create: `services/bff-service/internal/http/{orders,orders_test,payments,payments_test}.go`
- Modify: `services/bff-service/internal/http/{cart,cart_test}.go`
- Modify: `services/bff-service/internal/config/{config,config_test,kong_development_test,kong_production_test,compose_production_test}.go`
- Modify: `services/bff-service/cmd/server/{main,main_test}.go`
- Modify: `deployments/kong/kong.yaml`
- Modify: `deployments/kong/kong-production.yaml`
- Modify: `deployments/docker/compose.production.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `Makefile`
- Modify: `.env.example`, `.env.production.example`
- Modify: `scripts/start-local/{main,main_test}.go`

**Interfaces:**
- Consumes: `cartv1.CartItem.product_version` (Task 5), `paymentv1.PaymentServiceClient` (Task 6), `orderv1.OrderServiceClient` (Task 7).
- Produces:
  - `cartItem.ProductVersion int64` (`json:"productVersion"`) on `/api/v1/cart*` responses.
  - BFF config env: `BFF_ORDER_GRPC_ADDR` (default `"127.0.0.1:50055"`), `BFF_ORDER_GRPC_TIMEOUT` (`"15s"` to accommodate `12s` `CreatePaymentSession` deadline), `BFF_PAYMENT_GRPC_ADDR` (default `"127.0.0.1:50056"`), `BFF_PAYMENT_GRPC_TIMEOUT` (`"5s"`), `BFF_STOREFRONT_BASE_URL` (default `"http://localhost:5173"`).
  - Protected REST routes (`X-User-Id` required) -> `OrderService`:
    - `POST /api/v1/orders` (`Idempotency-Key` header required -> `201 Created` `{ data: { order: OrderDetailJSON, payUrl?: string }, message: "Đặt hàng thành công" }`)
    - `GET /api/v1/orders`, `GET /api/v1/orders/{orderId}`, `POST /api/v1/orders/{orderId}/cancel`
  - Public MoMo callback routes (`request-transformer` strips `X-User-Id` & `X-User-Roles`) -> `PaymentService`:
    - `GET /api/v1/payments/momo-redirect` -> `PaymentService.HandleMomoCallback(source = "REDIRECT")`; returns `400 INVALID_MOMO_SIGNATURE` on invalid signature/amount, or `302 Found` to `${BFF_STOREFRONT_BASE_URL}/thanh-toan/ket-qua?orderId=<encoded>&result=<redirect_result>`.
    - `POST /api/v1/payments/momo-ipn` -> `PaymentService.HandleMomoCallback(source = "IPN")`; returns `400` on invalid signature/amount, or `204 No Content`.

- [ ] **Step 1: Write failing BFF HTTP/client, Kong route, Compose manifest, and `start-local` tests**
  - Assert `productVersion` in `cart_test.go`; order endpoints & `PRODUCT_SNAPSHOT_MISMATCH` `errors` map in `orders_test.go`; `momo-redirect` (`302` vs `400`) and `momo-ipn` (`204` vs `400`) routed to `PaymentService` in `payments_test.go`.
  - Update `kong_development_test.go` and `kong_production_test.go` (`orders-api` with `caller-context`, `payments-momo-public` with `request-transformer`, `Idempotency-Key` in dev CORS, and no extra Kong upstream services).
  - Update `compose_production_test.go` to assert `payment-service` (`HTTP_ADDR: ":8086"`, `GRPC_ADDR: "0.0.0.0:50056"`, `postgres` & `rabbitmq` healthy deps), `order-service` (`payment-service` healthy dep + `PAYMENT_GRPC_ADDR: "payment-service:50056"`), `inventory-service` & `cart-service` (`RABBITMQ_URL`), and `bff-service` (`BFF_ORDER_GRPC_ADDR: "order-service:50055"`, `BFF_PAYMENT_GRPC_ADDR: "payment-service:50056"`).
  - Update `scripts/start-local/main_test.go` for `.env` loading and `payment-service` (`make payment`), `order-service`, `inventory-service`, `cart-service`, and `bff-service` defaults.

- [ ] **Step 2: Run BFF and scripts tests to verify RED failure**
  - Run: `go test ./services/bff-service/... ./scripts/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement BFF clients/handlers, Kong configs, `compose.production.yml`, `.github/workflows/ci.yml`, `Makefile`, `.env*.example`, and `start-local`**
  - Wire `internal/order/client.go`, `internal/payment/client.go`, `internal/http/orders.go`, `internal/http/payments.go`, and `cart.go` `productVersion`.
  - Update `deployments/kong/kong.yaml`, `deployments/kong/kong-production.yaml`, `deployments/docker/compose.production.yml`, `.github/workflows/ci.yml` (add `services/payment-service/go.sum`, `payment_db` Go test step, and `./services/payment-service/...` build step), `Makefile` (`payment` target), `.env.example`, `.env.production.example`, and `scripts/start-local/main.go`.

- [ ] **Step 4: Run `bff-service` and `scripts` verification gates**
  - Run: `go run ./scripts/verify-service -service bff-service && go test ./scripts/...`
  - Expected: `PASSED: bff-service` and `ok` on all `scripts` packages (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/bff-service deployments .github/workflows/ci.yml Makefile .env.example .env.production.example scripts && git commit -m "feat(bff,deploy): wire order-service and payment-service routes, compose, CI, and local launcher"`

---

### Task 9: `ecommerce-fe` Checkout Address Book, Payment Selector, Snapshot Mismatch UX & Payment Result Convergence

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Modify: `ecommerce-fe/src/lib/api.ts`
- Modify: `ecommerce-fe/src/lib/api.test.ts`
- Create: `ecommerce-fe/src/components/checkout/shipping-address-dialog.tsx`
- Modify: `ecommerce-fe/src/pages/order-preview-page.tsx`
- Modify: `ecommerce-fe/src/pages/order-preview-page.test.tsx`
- Create: `ecommerce-fe/src/pages/payment-result-page.tsx`
- Create: `ecommerce-fe/src/pages/payment-result-page.test.tsx`
- Create: `ecommerce-fe/src/pages/orders-page.tsx`
- Create: `ecommerce-fe/src/pages/orders-page.test.tsx`
- Create: `ecommerce-fe/src/pages/order-detail-page.tsx`
- Modify: `ecommerce-fe/src/components/layout/site-header.tsx`
- Modify: `ecommerce-fe/src/components/layout/site-header.test.tsx`
- Modify: `ecommerce-fe/src/App.tsx`
- Modify: `ecommerce-fe/src/App.test.tsx`

**Interfaces:**
- Consumes: `/api/v1/users/me/addresses`, `/api/v1/locations/*`, and `/api/v1/orders*` (`bff-service` from Task 8).
- Produces:
  - Types in `types.ts`: `CartItem.productVersion: number`, `ShippingAddress`, `CreateShippingAddressPayload`, `ShippingAddressSnapshot`, `PaymentMethod` (`"COD" | "MOMO"`), `OrderStatus`, `PaymentStatus`, `OrderItemSnapshot`, `OrderDetail`, `CreateOrderPayload`, `CreateOrderResult`, `OrderListResult`.
  - API functions in `api.ts`: `listShippingAddresses`, `createShippingAddress`, `createOrder(payload, idempotencyKey)`, `listOrders`, `getOrder`, `cancelOrder`.
  - Routes in `App.tsx` (wrapped in `<RequireAuth>`): `/gio-hang/dat-hang`, `/thanh-toan/ket-qua`, `/don-hang`, `/don-hang/:orderId`.

- [ ] **Step 1: Write failing Vitest tests for API client, `OrderPreviewPage`, `PaymentResultPage` (including `PENDING_PAYMENT` convergence polling & `REFUND_REQUIRED`), `OrdersPage`, and `SiteHeader`**
  - `api.test.ts`: `createOrder` sends `Idempotency-Key` header and preserves `errors` map on `409 PRODUCT_SNAPSHOT_MISMATCH`.
  - `order-preview-page.test.tsx`: address book selection/creation + profile fallback, SKU snapshot capture, `COD` vs `MOMO` navigation, keeping `Idempotency-Key` on `status === null` and `409 REQUEST_IN_PROGRESS` (auto-retrying after `1.5s`), rotating after terminal errors or input changes, and locking submit with banner + `"Quay lại giỏ hàng để cập nhật"` on `409 PRODUCT_SNAPSHOT_MISMATCH`.
  - `payment-result-page.test.tsx`:
    - `pollsPendingPaymentOrderUntilConfirmedAndRefreshesCart`: when first `getOrder` returns `status: "PENDING_PAYMENT"` during RabbitMQ propagation from `payment-service` to `order-service`, renders `"Đang đồng bộ kết quả thanh toán..."`, polls every `500ms` (up to `10s`), renders confirmed state once `status === "CONFIRMED"`, and calls `retryLoad()`.
    - Renders clear refund notice when `status === "CANCELLED"` and `paymentStatus === "REFUND_REQUIRED"`.
  - `orders-page.test.tsx` & `site-header.test.tsx`: paginated order history, snapshot detail view, cancelling `PENDING_PAYMENT` orders, and `"Đơn mua"` header menu item.

- [ ] **Step 2: Run frontend tests to verify RED failure**
  - Run: `npm --prefix ecommerce-fe test -- --run`
  - Expected: FAIL.

- [ ] **Step 3: Implement API client, `ShippingAddressDialog`, `OrderPreviewPage`, `PaymentResultPage`, `OrdersPage`, `OrderDetailPage`, `SiteHeader`, and routes**
  - Suppress duplicate manual toasts around mutations handled by `api.ts` (suppress `REQUEST_IN_PROGRESS` toast in `api.ts` and suppress mutation toast when `MOMO` immediately redirects to `payUrl`).

- [ ] **Step 4: Run `ecommerce-fe` verification gate**
  - Run: `go run ./scripts/verify-service -service ecommerce-fe`
  - Expected: `PASSED: ecommerce-fe` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add ecommerce-fe && git commit -m "feat(ecommerce-fe): build checkout address selector, COD/MoMo flow, snapshot mismatch UX, and payment result convergence"`

---

### Task 10: Multi-Threaded Concurrency & Race-Condition Simulation Suite + Final Repository Verification

**Files:**
- Create: `scripts/concurrency-sim/main.go`
- Create: `scripts/concurrency-sim/scenarios.go`
- Create: `scripts/concurrency-sim/sim_test.go`
- Create: `scripts/run-concurrency-sim.ps1`

**Interfaces:**
- Produces:
  - CLI `go run ./scripts/concurrency-sim -scenario <all|flash-sale-oversell|snapshot-mismatch-race|idempotency-storm|momo-callback-race> -base-url http://localhost:8000/api/v1` and PowerShell wrapper `scripts/run-concurrency-sim.ps1`.
  - Structured JSON/table summary reporting goroutines, p50/p95 latency, status/error-code histogram, and invariant assertions across `order-service`, `inventory-service`, and `payment-service` (`exit 0` iff all invariants hold).

- [ ] **Step 1: Write failing simulation harness unit tests (`scripts/concurrency-sim/sim_test.go`)**
  - Test all 4 scenario runners (`RunFlashSaleOversell`, `RunSnapshotMismatchRace`, `RunIdempotencyStorm`, `RunMomoCallbackRace`) against stateful concurrent `httptest.Server` fakes verifying start-gate synchronization, oversell detection (`5` `CONFIRMED` + `45` `409 INVENTORY_INSUFFICIENT`), snapshot price-change barrier (`409 PRODUCT_SNAPSHOT_MISMATCH`), single `orderId` + `payUrl` under `20` duplicate keys + `409 IDEMPOTENCY_KEY_REUSED` on `5` mutated payloads, and `400` on forged signatures (`resultCode=0` and `resultCode=1006`) + consistent terminal convergence (`CONFIRMED`/`PAID` or `CANCELLED`/`REFUND_REQUIRED`) under concurrent redirect + IPN + cancel bursts.

- [ ] **Step 2: Run simulation unit tests to verify RED failure**
  - Run: `go test ./scripts/concurrency-sim/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement `scripts/concurrency-sim` and `scripts/run-concurrency-sim.ps1`**
  - Implement synchronized barrier runners (`close(startGate)` + `sync.WaitGroup`) and bounded eventual-consistency polling (`5s`) for Outbox/RabbitMQ settlement across `payment-service`, `order-service`, `inventory-service`, and `cart-service`.

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
  - Run: `git add scripts/concurrency-sim scripts/run-concurrency-sim.ps1 && git commit -m "test(concurrency): add multi-threaded checkout, payment-service, and saga simulation suite"`
  - Dispatch independent read-only `code-reviewer` and `boundary-reviewer` subagents over `git diff main...HEAD` per `AGENTS.md` before final handoff.

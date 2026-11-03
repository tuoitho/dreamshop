# Order Checkout, Snapshot Consistency, MoMo Payment & Anti-Oversell Saga Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an end-to-end multi-item Order & Checkout workflow supporting shipping address book selection, immutable order snapshots, preview-to-order snapshot verification (`409 PRODUCT_SNAPSHOT_MISMATCH`), `COD` and `MOMO` (`captureWallet` v2 HMAC) payments, atomic stock reservation with Transactional Outbox/RabbitMQ Saga compensation, selective cart cleanup, and a multi-threaded concurrency simulation suite.

**Architecture:** `order-service` (`:8083`, mgmt `:8093`, private gRPC `:50055`) orchestrates a Hybrid Saga: synchronous private gRPC checks `order.*` RBAC via `identity-service` (`:50054`), verifies product `version` + snapshot fields via `catalog-service` (`:50051`), and atomically reserves stock in `inventory-service` (`:50052`) before either confirming `COD` orders or creating a 15-minute `MOMO` payment session (`PENDING_PAYMENT`). Asynchronous state convergence (`OrderConfirmed` and `ReservationReleaseRequested`) flows through `order_db.outbox_events` -> RabbitMQ `ecommerce.events` -> idempotent consumers in `inventory-service` (physical stock deduction or `RELEASED` tombstone/release) and `cart-service` (selective ordered-item deletion), backed by `OrderReconciliationScheduler` (Layer 1) for `PENDING_STOCK > 2m` and `PENDING_PAYMENT > 15m` recovery.

**Tech Stack:** Protobuf / Buf, Java 21 / Spring Boot 4.1.0 / Spring Data JPA / Flyway / Spring AMQP / `grpc-netty-shaded` 1.65.1 / Testcontainers, Go 1.24 (`pgx/v5`, `sqlc`, `google.golang.org/grpc` v1.65.0, `github.com/rabbitmq/amqp091-go` v1.10.0), Kong 3.8 declarative config, React 19 / TypeScript 6 / Vite 8 / Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-28-order-checkout-saga-design.md`

## Global Constraints

- **Service boundaries (`AGENTS.md`, `scripts/internal/boundaries/boundaries.go`):** Zero cross-service code imports, zero foreign `<other>_db` strings (even in SQL/code comments) inside `services/<name>-service/`, and zero `:8000` literals (`localhost:8000`, `127.0.0.1:8000`, `kong:8000`, `host.docker.internal:8000`) anywhere in `.go`, `.java`, `.yml`, `.yaml`, `.properties`, or `.env` under `services/*`.
- **Contract compatibility (`docs/steering/api-contracts.md`, `ADR-013`):** Protobuf changes in `contracts/proto/**/*.proto` must be strictly additive (`lower_snake_case`) on top of the SKU baseline, preserve existing field numbers (`1..4` in `OrderItem`, `1..2` in `CreateOrderRequest`, `1..3` in `CreateOrderResponse`), pass `buf format --diff --exit-code` and `buf lint`, and commit synchronized `buf generate` outputs in `contracts/go` and `contracts/java/src/main/generated`.
- **Transaction & concurrency discipline (`services/order-service/AGENTS.md`, `ADR-002`, `ADR-003`, `ADR-005`, `ADR-013`):** Never hold a database transaction open across gRPC, MoMo HTTPS, or RabbitMQ network calls (`OutboxRelayScheduler` claims a short lease `next_attempt_at += 15s` with `FOR UPDATE SKIP LOCKED` and commits before awaiting RabbitMQ publisher confirms); enforce hard timeouts on `MomoGatewayClient` (`connectTimeout = 3s`, `readTimeout = 10s`, well below the `2m` `PENDING_STOCK` sweep); lock `orders` rows with `SELECT ... FOR UPDATE` and verify the expected source status (plus `@Version`) on **every** state transition in both `OrderSagaOrchestrator` (`PENDING_STOCK -> CONFIRMED | PENDING_PAYMENT | CANCELLED`) and post-creation handlers (`PENDING_PAYMENT -> CONFIRMED | CANCELLED`), aborting without returning `payUrl` if the order was already cancelled; in `inventory-service`, execute `INSERT INTO stock_reservations ... ON CONFLICT (order_id) DO NOTHING` before `SELECT ... FOR UPDATE`, and aggregate reservation quantities by `sku_id` sorted `ORDER BY sku_id ASC` before conditional `UPDATE stock`.
- **REST & frontend conventions (`AGENTS.md`, `ecommerce-fe/src/lib/api.ts`):** Keep `{ "data": ..., "message": "..." }` and `{ "code": "...", "message": "...", "errors": {...}, "traceId": "..." }` envelopes (`camelCase`); add `Idempotency-Key` to Kong CORS headers; on the frontend, keep the same `Idempotency-Key` when no HTTP response was received (`error.status === null`) OR on `409 REQUEST_IN_PROGRESS` (showing `"Đơn hàng đang được xử lý..."` and auto-retrying after `1.5s`), and rotate to a fresh key only after a terminal HTTP error response (`503`, other `409`s, `400`, etc.) or checkout input change; do not fire duplicate manual `toast.success` / `toast.error` in React components for mutations handled by `api.ts` interceptors; never commit raw MoMo secret keys into tracked files.
- **Windows & CI test compatibility (`services/cart-service/AGENTS.md`):** Keep `services/cart-service/cmd/server/main_test.go` Linux signal tests intact by wiring optional RabbitMQ startup inside `buildProductionService`; run `go test ./services/cart-service/internal/...` on Windows and `make verify SERVICE=<surface>` for every touched surface.

## Review Focus

1. **Duplicate `sku_id` aggregation, independent variant stocks & cross-order lock ordering in `inventory-service` (`Task 4`):** An order repeating the same `sku_id` across lines whose individual quantities fit `available` (`2 <= 3`) but combined sum exceeds `available` (`2 + 2 = 4 > 3`) must roll back cleanly with `INVENTORY_INSUFFICIENT` and zero `reserved` leak, while distinct SKUs of the same `product_id` draw from independent `stock(sku_id)` rows, and concurrent orders with reversed SKU lists `(sku-2, sku-1)` vs `(sku-1, sku-2)` must lock rows in `sku_id ASC` order without deadlocking.
2. **Conflict-free `INSERT ... ON CONFLICT DO NOTHING` tombstone & single-catch uncertain gRPC compensation (`Task 4` & `Task 6`):** Both `ReserveStock` and `ReservationReleaseRequested` must execute `INSERT INTO stock_reservations ... ON CONFLICT (order_id) DO NOTHING` before `SELECT ... FOR UPDATE` so concurrent delayed `ReserveStock` and `ReservationReleaseRequested` never raise `23505` unique violations and a pre-existing `RELEASED` tombstone rejects delayed `ReserveStock` with `RESERVATION_ALREADY_RELEASED`; in `order-service`, only `INVENTORY_INSUFFICIENT` skips Outbox compensation, while any timeout/network/unexpected error cancels the order and writes `ReservationReleaseRequested` into `outbox_events`.
3. **MoMo timeout/reconciler oversell guard, callback HMAC forgery, and late payment (`Task 6` & `Task 7`):** `MomoGatewayClient` enforces hard `3s` connect / `10s` read timeouts and `markPendingPayment` locks the order with `SELECT ... FOR UPDATE` verifying `status == PENDING_STOCK` (aborting without returning `payUrl` if `OrderReconciliationScheduler` already cancelled it); any `momo-redirect` or `momo-ipn` call with an invalid `HmacSHA256` signature or `amount != total_amount_minor` fails with `400 INVALID_MOMO_SIGNATURE`, and a valid `resultCode=0` arriving after `CANCELLED` keeps `status = CANCELLED`, sets `payment_status = REFUND_REQUIRED`, and emits no `OrderConfirmed` event.
4. **Selective cart cleanup on `OrderConfirmed` & cart preservation on unpaid MoMo orders (`Task 5`):** Consuming `OrderConfirmed` deletes only the ordered `cart_item_ids` (leaving unselected cart items intact), replays of the same `event_id` are ignored via `processed_events`, and `PENDING_PAYMENT` / `CANCELLED` MoMo orders leave the customer's cart untouched.
5. **Atomic `Local TX #1` idempotency insert (`ON CONFLICT DO NOTHING`), `REQUEST_IN_PROGRESS` polling & stale `OrderPreviewPage` lock (`Task 6` & `Task 8`):** All pre-creation checks (validation, RBAC, snapshot diff) are strictly read-only and write nothing on failure; `idempotency_records(IN_PROGRESS)` is inserted inside `Local TX #1` alongside `orders(PENDING_STOCK)` via `INSERT ... ON CONFLICT (user_id, idempotency_key) DO NOTHING` (if `inserted == 0`, rollback `TX #1` without creating an order and replay `findRecord` outside `TX #1`); `OrderReconciliationScheduler` updates paired `idempotency_records` (`IN_PROGRESS -> FAILED_TERMINAL`) when cancelling stuck `PENDING_STOCK > 2m` orders; frontend keeps `Idempotency-Key` on `error.status === null` and `409 REQUEST_IN_PROGRESS` (auto-retrying until terminal outcome), rotates after terminal HTTP errors (`503`, `409 INVENTORY_INSUFFICIENT`, etc.), and locks stale submit on `409 PRODUCT_SNAPSHOT_MISMATCH` until returning to `/gio-hang`.

---

## File Structure

- **Contracts & Architecture Docs (`Task 1`):**
  - Modify: `contracts/proto/catalog/v1/catalog.proto`, `contracts/proto/cart/v1/cart.proto`, `contracts/proto/inventory/v1/inventory.proto`, `contracts/proto/order/v1/order.proto`
  - Modify (generated): `contracts/go/{catalog,cart,inventory,order}/v1/*`, `contracts/java/src/main/generated/com/ecommerce/contracts/{catalog,cart,inventory,order}/v1/*`
  - Create: `docs/adr/014-order-checkout-saga-and-momo-payment.md`
  - Modify: `AGENTS.md`, `docs/steering/service-catalog.md`, `services/order-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, `services/bff-service/AGENTS.md`
- **Identity RBAC (`Task 2`):**
  - Create: `services/identity-service/src/main/resources/db/migration/V12__add_order_permissions.sql`
  - Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`, `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`
- **Catalog Product Version (`Task 3`):**
  - Create: `services/catalog-service/internal/store/migrations/000006_add_product_version.up.sql`
  - Modify: `services/catalog-service/internal/domain/product.go`, `services/catalog-service/internal/domain/admin.go`, `services/catalog-service/internal/store/store.go`, `services/catalog-service/internal/store/admin.go`, `services/catalog-service/internal/grpc/server.go`, `services/catalog-service/internal/grpc/admin.go` (+ adjacent `*_test.go`)
- **Inventory Reservations, Tombstone & Event Consumer (`Task 4`):**
  - Create: `services/inventory-service/internal/store/migrations/000003_create_stock_reservations.up.sql`, `services/inventory-service/internal/store/reservation_store.go`, `services/inventory-service/internal/store/reservation_store_test.go`, `services/inventory-service/internal/events/consumer.go`, `services/inventory-service/internal/events/consumer_test.go`
  - Modify: `services/inventory-service/go.mod`, `services/inventory-service/internal/config/config.go`, `services/inventory-service/internal/domain/stock.go`, `services/inventory-service/internal/grpc/server.go`, `services/inventory-service/internal/grpc/server_test.go`, `services/inventory-service/cmd/server/main.go`
- **Cart Version Enrichment & Event Cleanup (`Task 5`):**
  - Create: `services/cart-service/internal/store/migrations/000004_create_processed_events.up.sql`, `services/cart-service/internal/events/consumer.go`, `services/cart-service/internal/events/consumer_test.go`
  - Modify: `services/cart-service/go.mod`, `services/cart-service/internal/config/config.go`, `services/cart-service/internal/domain/cart.go`, `services/cart-service/internal/catalog/client.go`, `services/cart-service/internal/store/queries/cart.sql`, `services/cart-service/internal/store/db/*`, `services/cart-service/internal/store/store.go`, `services/cart-service/internal/store/store_test.go`, `services/cart-service/internal/grpc/server.go`, `services/cart-service/internal/grpc/server_test.go`, `services/cart-service/cmd/server/main.go`
- **Order Saga Orchestrator, Persistence, MoMo & gRPC (`Task 6`):**
  - Modify: `services/order-service/pom.xml`, `services/order-service/Dockerfile`, `services/order-service/src/main/resources/application.yml`, `services/order-service/src/main/java/com/ecommerce/order/OrderApplication.java`
  - Create:
    - `services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql`
    - `services/order-service/src/main/java/com/ecommerce/order/config/{OrderProperties,GrpcClientConfig,RabbitMqConfig}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/common/{TraceIdProvider,GrpcTraceIdInterceptor,DomainException}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/domain/{OrderStatus,PaymentMethod,PaymentStatus,ShippingPolicy,SnapshotMismatchDetail}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/persistence/{OrderEntity,OrderItemEntity,OrderRepository,IdempotencyRecordEntity,IdempotencyRecordRepository,OutboxEventEntity,OutboxEventRepository}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/integration/{AuthorizationGrpcAdapter,CatalogGrpcAdapter,InventoryGrpcAdapter,MomoGatewayClient,MomoSignatureVerifier}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/application/{IdempotencyService,OrderTransactionStepService,OrderSagaOrchestrator,OrderQueryService,MomoCallbackService,OutboxRelayScheduler,OrderReconciliationScheduler}.java`
    - `services/order-service/src/main/java/com/ecommerce/order/grpc/{OrderGrpcService,GrpcServerLifecycle,GrpcErrorMapper}.java`
  - Test: `services/order-service/src/test/java/com/ecommerce/order/{support/PostgresRabbitIntegrationTestSupport,persistence/OrderMigrationAndLockingTest,integration/MomoSignatureVerifierTest,application/IdempotencyServiceTest,application/OrderSagaOrchestratorTest,application/MomoCallbackLifecycleTest,application/OrderReconciliationSchedulerTest,grpc/OrderGrpcIntegrationTest}.java`
- **BFF, Kong, Docker Compose & Local Launcher (`Task 7`):**
  - Create: `services/bff-service/internal/order/client.go`, `services/bff-service/internal/order/client_test.go`, `services/bff-service/internal/http/orders.go`, `services/bff-service/internal/http/orders_test.go`
  - Modify: `services/bff-service/internal/http/cart.go`, `services/bff-service/internal/http/cart_test.go`, `services/bff-service/internal/config/config.go`, `services/bff-service/internal/config/config_test.go`, `services/bff-service/cmd/server/main.go`, `services/bff-service/cmd/server/main_test.go`, `deployments/kong/kong.yaml`, `deployments/kong/kong-production.yaml`, `deployments/docker/compose.production.yml`, `.env.example`, `.env.production.example`, `services/bff-service/internal/config/kong_development_test.go`, `services/bff-service/internal/config/kong_production_test.go`, `services/bff-service/internal/config/compose_production_test.go`, `scripts/start-local/main.go`, `scripts/start-local/main_test.go`
- **Storefront Checkout, Payment Result & Order History UI (`Task 8`):**
  - Create: `ecommerce-fe/src/components/checkout/shipping-address-dialog.tsx`, `ecommerce-fe/src/pages/payment-result-page.tsx`, `ecommerce-fe/src/pages/orders-page.tsx`, `ecommerce-fe/src/pages/order-detail-page.tsx`, `ecommerce-fe/src/pages/payment-result-page.test.tsx`, `ecommerce-fe/src/pages/orders-page.test.tsx`
  - Modify: `ecommerce-fe/src/lib/types.ts`, `ecommerce-fe/src/lib/api.ts`, `ecommerce-fe/src/lib/api.test.ts`, `ecommerce-fe/src/pages/order-preview-page.tsx`, `ecommerce-fe/src/pages/order-preview-page.test.tsx`, `ecommerce-fe/src/components/layout/site-header.tsx`, `ecommerce-fe/src/components/layout/site-header.test.tsx`, `ecommerce-fe/src/App.tsx`, `ecommerce-fe/src/App.test.tsx`
- **Concurrency & Race-Condition Simulation Suite (`Task 9`):**
  - Create: `scripts/concurrency-sim/main.go`, `scripts/concurrency-sim/scenarios.go`, `scripts/concurrency-sim/sim_test.go`, `scripts/run-concurrency-sim.ps1`

---

### Task 1: Shared Protobuf Contracts & Architecture Documentation

**Files:**
- Modify: `contracts/proto/catalog/v1/catalog.proto`
- Modify: `contracts/proto/cart/v1/cart.proto`
- Modify: `contracts/proto/inventory/v1/inventory.proto`
- Modify: `contracts/proto/order/v1/order.proto`
- Modify: `contracts/go/{catalog,cart,inventory,order}/v1/*` (generated)
- Modify: `contracts/java/src/main/generated/com/ecommerce/contracts/{catalog,cart,inventory,order}/v1/*` (generated)
- Create: `docs/adr/014-order-checkout-saga-and-momo-payment.md`
- Modify: `AGENTS.md`, `docs/steering/service-catalog.md`, `services/order-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, `services/bff-service/AGENTS.md`

**Interfaces:**
- Consumes: `ecommerce.common.v1.Money` (`common/v1/common.proto`)
- Produces:
  - `ecommerce.catalog.v1`: `GetProductResponse.version = 24` (`int64`), `ProductSummary.version = 19` (`int64`), `AdminProductSummary.version = 24` (`int64`)
  - `ecommerce.cart.v1`: `CartItem.product_version = 17` (`int64`)
  - `ecommerce.inventory.v1`:
    - `InventoryService.ReserveStock(ReserveStockRequest) returns (ReserveStockResponse)` (confirm/release are event-driven only; do not add unused `ConfirmReservation` or `ReleaseReservation` RPCs)
    - Messages: `ReservationLine { 1: sku_id, 2: product_id, 3: quantity }`, `ReserveStockRequest { 1: order_id, 2: items, 3: expires_at }`, `ReserveStockResponse { 1: order_id, 2: status }`
  - `ecommerce.order.v1`:
    - `OrderService`: `CreateOrder`, `GetOrder`, `ListOrders`, `CancelOrder`, `HandleMomoCallback`
    - `ShippingAddressSnapshot` (`1..8`), `OrderItem` (`1..4` unchanged + `5..15` per Spec 5.2.d), `CreateOrderRequest` (`1..2` unchanged + `3: idempotency_key, 4: shipping_address, 5: payment_method, 6: note`), `OrderDetail` (`1..17` per Spec 5.2.d), `CreateOrderResponse` (`1..3` unchanged + `4: order, 5: pay_url`)
    - `GetOrderRequest { 1: user_id, 2: order_id }`, `GetOrderResponse { 1: order }`
    - `ListOrdersRequest { 1: user_id, 2: status, 3: page, 4: page_size }`, `ListOrdersResponse { 1: items (repeated OrderDetail), 2: page, 3: page_size, 4: total_items (int64), 5: total_pages }`
    - `CancelOrderRequest { 1: user_id, 2: order_id, 3: reason }`, `CancelOrderResponse { 1: order }`
    - `HandleMomoCallbackRequest { 1: partner_code, 2: order_id, 3: request_id, 4: amount (int64), 5: order_info, 6: order_type, 7: trans_id (int64), 8: result_code (int32), 9: message, 10: pay_type, 11: response_time (int64), 12: extra_data, 13: signature }`, `HandleMomoCallbackResponse { 1: order_id, 2: status, 3: payment_status, 4: result_code }`
  - Async RabbitMQ JSON Envelope (`ecommerce.events` topic exchange, shared across Tasks 4, 5, 6):
    ```json
    {
      "eventId": "uuid",
      "eventType": "OrderConfirmed | ReservationReleaseRequested",
      "eventVersion": 1,
      "occurredAt": "RFC3339",
      "aggregateId": "<orderId>",
      "correlationId": "<traceId>",
      "data": {
        "orderId": "uuid",
        "userId": "uuid",
        "reason": "string",
        "items": [{ "cartItemId": "uuid", "productId": "dt-1", "skuId": "dt-1-v1", "quantity": 2 }]
      }
    }
    ```
    Routing keys: `order.confirmed`, `order.reservation.release_requested`.

- [ ] **Step 1: Add additive fields, RPCs, and messages to `catalog.proto`, `cart.proto`, `inventory.proto`, and `order.proto`**
  - Keep every existing field number and name untouched (`OrderItem` `1..4`, `CreateOrderRequest` `1..2`, `CreateOrderResponse` `1..3`).

- [ ] **Step 2: Format, lint, and regenerate Go & Java contract modules**
  - Run: `buf format -w; buf generate`
  - Stage generated files so `git ls-files --others` check in `verify-service` sees tracked outputs: `git add contracts`
  - Run: `go run ./scripts/verify-service -service contracts`
  - Expected: `PASSED: contracts` (`exit 0`).

- [ ] **Step 3: Record `ADR-014` and align steering/service docs**
  - Create `docs/adr/014-order-checkout-saga-and-momo-payment.md` following `docs/adr/TEMPLATE.md` documenting: `order-service` private gRPC port `50055`; Hybrid Saga (`Catalog` per-SKU snapshot `version` check + single fail-closed SKU-level `ReserveStock` `1.5s` + Outbox/RabbitMQ `OrderConfirmed`/`ReservationReleaseRequested` + `INSERT ... ON CONFLICT (order_id) DO NOTHING` Tombstone in `inventory-service` without extra confirm/release RPCs, outbox, or `processed_events`); `PENDING_PAYMENT` 15-minute MoMo v2 `captureWallet` flow (`3s/10s` client timeout, `SELECT ... FOR UPDATE` source-status guard on all `PENDING_STOCK -> *` and `PENDING_PAYMENT -> *` transitions, two-way `HmacSHA256`); `Idempotency-Key` final-outcome replay (`COMPLETED` / `FAILED_TERMINAL`) + frontend rotation rule + Layer 1 `OrderReconciliationScheduler` (`IN_PROGRESS -> FAILED_TERMINAL`); and `OutboxRelayScheduler` lease-claim pattern (`FOR UPDATE SKIP LOCKED` short claim TX committed before `3s` RabbitMQ publisher confirm + short mark-published TX so no DB transaction spans network/broker I/O).
  - Update `AGENTS.md` (Local Port Topology `order-service` gRPC `127.0.0.1:50055` and internal gRPC description), `docs/steering/service-catalog.md`, `services/order-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, and `services/bff-service/AGENTS.md`.

- [ ] **Step 4: Verify boundary check and downstream compilation**
  - Run: `go run ./scripts/check-boundaries && go build ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/... ./services/bff-service/... && mvn --batch-mode -pl contracts/java -am test-compile`
  - Expected: Exit code `0`.

- [ ] **Step 5: Commit**
  - Run: `git add contracts docs AGENTS.md services/order-service/AGENTS.md services/inventory-service/AGENTS.md services/cart-service/AGENTS.md services/bff-service/AGENTS.md && git commit -m "feat(contracts): add order checkout, reservation, and snapshot version contracts"`

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
    - Add `"order.cancel"`, `"order.create"`, `"order.view"` alphabetically between `"inventory.view"` and `"product.create"` in the `containsExactly(...)` permission assertion.
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
  - `domain.Product.Version int64`, `domain.Summary.Version int64`, `domain.AdminSummary.Version int64` (initialized to `1` on insert, incremented `version = version + 1` on `UpdateProduct` and `SoftDelete`).
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
  - Config env addition (optional when empty so standalone tests remain lightweight): `RABBITMQ_URL` (default `""`).

- [ ] **Step 1: Write failing reservation, duplicate-SKU aggregation, independent variant stock, deadlock-free ordering, conflict-free tombstone, and event consumer tests**
  - In `reservation_store_test.go`:
    - `TestReserveStockAggregatesDuplicateSkuIDsAndKeepsDistinctVariantsIndependent`: seed `p-1-v1` (`quantity=3, reserved=0`) and `p-1-v2` (`quantity=3, reserved=0`); call `ReserveStock("ord-1", [{SkuID: "p-1-v1", ProductID: "p-1", Quantity: 2}, {SkuID: "p-1-v1", ProductID: "p-1", Quantity: 2}], ...)` -> assert `InsufficientStockError{SkuID: "p-1-v1", ProductID: "p-1", Available: 3, Requested: 4}` and `Get("p-1-v1").Reserved == 0`; then call `ReserveStock("ord-1b", [{SkuID: "p-1-v1", ProductID: "p-1", Quantity: 2}, {SkuID: "p-1-v2", ProductID: "p-1", Quantity: 2}], ...)` -> succeeds with `Reserved == 2` on each SKU independently.
    - `TestReserveStockIdempotentReplay`: calling `ReserveStock("ord-2", [{SkuID: "p-1-v1", ProductID: "p-1", Quantity: 2}], ...)` twice reserves `2` total (not `4`) and returns `"ACTIVE"`.
    - `TestReleaseBeforeReserveWritesTombstoneBlockingLateReserve`: call `ReleaseReservation("ord-late", "TIMEOUT")` before any `ReserveStock` -> assert row `status = 'RELEASED'` exists; subsequent `ReserveStock("ord-late", ...)` fails with `domain.ErrReservationAlreadyReleased` and `reserved == 0`.
    - `TestConcurrentLateReserveAndReleaseNeverRaisesUniqueViolation`: run `ReserveStock("ord-race-i", ...)` and `ReleaseReservation("ord-race-i", "TIMEOUT")` concurrently across 20 iterations -> assert zero `23505` unique-constraint errors and exact stock invariant (`status == 'RELEASED'` => `reserved == 0`; `status == 'ACTIVE'` => `reserved == qty`).
    - `TestConfirmReservationDeductsQuantityAndReservedIdempotently`: `ReserveStock` (`qty=5, reserved=2`) -> `ConfirmReservation("ord-conf-1")` -> `quantity == 3, reserved == 0, status == "CONFIRMED"`; re-running `ConfirmReservation("ord-conf-1")` is a no-op and does not double-deduct.
    - `TestConcurrentCrossOrderReservationsDoNotDeadlock`: 20 goroutines reserving `(sku-a, sku-b)` and `(sku-b, sku-a)` concurrently against `quantity=10` complete with zero deadlocks and exact `reserved` sum.
  - In `server_test.go` and `consumer_test.go`: assert `ReserveStock` gRPC `ErrorInfo` metadata (`sku_id`, `product_id`, `available`, `requested`) and consumer routing of `order.confirmed` / `order.reservation.release_requested` JSON payloads.

- [ ] **Step 2: Run `inventory-service` tests to verify RED failure**
  - Run: `go test ./services/inventory-service/...`
  - Expected: FAIL (missing `ReserveStock` store/gRPC/consumer implementations).

- [ ] **Step 3: Add `amqp091-go`, migration `000003`, `reservation_store.go`, `ReserveStock` gRPC handler, and RabbitMQ consumer**
  - Add `github.com/rabbitmq/amqp091-go v1.10.0` to `services/inventory-service/go.mod`.
  - Create `000003_create_stock_reservations.up.sql` per Spec Section 5.4 (`stock_reservations` and `stock_reservation_items(order_id, sku_id)` storing `(order_id, sku_id, product_id, quantity)` — no `processed_events` or `outbox_events`).
  - In `ReserveStock`: collapse duplicate `sku_id` quantities into a map, sort `sku_id` keys `ASC`, execute `INSERT INTO stock_reservations (order_id, status, expires_at) VALUES ($1, 'ACTIVE', $2) ON CONFLICT (order_id) DO NOTHING` **first**; if `RowsAffected() == 0`, query `SELECT status FROM stock_reservations WHERE order_id = $1 FOR UPDATE` (`ACTIVE`/`CONFIRMED` -> idempotent return, `RELEASED` -> return `domain.ErrReservationAlreadyReleased`); if `RowsAffected() == 1`, execute conditional `UPDATE stock SET reserved = reserved + $2, updated_at = now() WHERE sku_id = $1 AND (quantity - reserved) >= $2` and insert `stock_reservation_items (order_id, sku_id, product_id, quantity)` per sorted SKU (on `0` rows affected, query current `(quantity - reserved)` for that `sku_id`, roll back, and return `*domain.InsufficientStockError`).
  - In `ReleaseReservation`: execute `INSERT INTO stock_reservations (order_id, status, expires_at, release_reason) VALUES ($1, 'RELEASED', now(), $2) ON CONFLICT (order_id) DO NOTHING` **first** (if `RowsAffected() == 1`, tombstone is written cleanly with zero `23505` race); if `RowsAffected() == 0`, query `SELECT status FROM stock_reservations WHERE order_id = $1 FOR UPDATE` and when `status == 'ACTIVE'`, decrement `reserved` on `stock` sorted by `sku_id ASC` and set `status = 'RELEASED', release_reason = $2, updated_at = now()`.
  - In `ConfirmReservation`: query `SELECT status FROM stock_reservations WHERE order_id = $1 FOR UPDATE`; when `status == 'ACTIVE'`, decrement both `quantity` and `reserved` on `stock` sorted by `sku_id ASC` and set `status = 'CONFIRMED', updated_at = now()` (when already `CONFIRMED`, return idempotently by `order_id`).
  - Implement `internal/events/consumer.go` (declares durable topic exchange `ecommerce.events`, DLX `ecommerce.events.dlx`, DLQ `inventory.order-events.dlq`, queue `inventory.order-events` bound to `order.confirmed` and `order.reservation.release_requested`, retries up to delivery count `5` before `Nack(false, false)` to DLQ). Start it in `cmd/server/main.go` when `cfg.RabbitMQURL != ""`.

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
- Modify: `services/cart-service/internal/store/db/*` (generated via `sqlc` or synchronized Go query methods)
- Modify: `services/cart-service/internal/config/config.go`
- Modify: `services/cart-service/internal/domain/cart.go`
- Modify: `services/cart-service/internal/catalog/client.go`
- Modify: `services/cart-service/internal/store/store.go`
- Modify: `services/cart-service/internal/store/store_test.go`
- Modify: `services/cart-service/internal/grpc/server.go`
- Modify: `services/cart-service/internal/grpc/server_test.go`
- Create: `services/cart-service/internal/events/consumer.go`
- Create: `services/cart-service/internal/events/consumer_test.go`
- Modify: `services/cart-service/cmd/server/main.go`

**Interfaces:**
- Consumes: `catalogv1.GetProductResponse.version` (Task 3), `cartv1.CartItem.product_version` & `OrderConfirmed` RabbitMQ event (Task 1).
- Produces:
  - `CartItem.ProductVersion` populated during `GetCart` / mutation enrichment.
  - `Store.RemoveOrderedItems(ctx context.Context, eventID, userID string, items []domain.OrderedItemRef) error` where `OrderedItemRef { CartItemID string; ProductID string; SkuID string }`.
  - Config env addition: `RABBITMQ_URL` (default `""`).

- [ ] **Step 1: Write failing tests for `product_version` enrichment and selective idempotent cart cleanup**
  - In `services/cart-service/internal/grpc/server_test.go`, assert `GetCart` maps `Product.Version = 7` into `CartItem.GetProductVersion() == 7`.
  - In `services/cart-service/internal/store/store_test.go` and `internal/events/consumer_test.go`, add `TestHandleOrderConfirmedRemovesOnlyOrderedItemsIdempotently`:
    - Seed user cart with 3 SKU items (`itemA`, `itemB`, `itemC`); consume `OrderConfirmed` (`eventId = "evt-1"`) referencing `itemA` (by `cartItemId`) and `itemB` (fallback by `skuId` when `cartItemId == ""`).
    - Assert `itemA` and `itemB` are deleted, `itemC` remains with its original quantity, and replaying `"evt-1"` (even after re-adding `itemA`) is ignored via `processed_events`.

- [ ] **Step 2: Run `cart-service` internal tests to verify RED failure**
  - Run: `go test ./services/cart-service/internal/...`
  - Expected: FAIL (`ProductVersion` and `RemoveOrderedItems` missing).

- [ ] **Step 3: Implement migration `000004`, `product_version` mapping, `RemoveOrderedItems`, and RabbitMQ consumer**
  - Add `github.com/rabbitmq/amqp091-go v1.10.0` to `services/cart-service/go.mod`.
  - Create `000004_create_processed_events.up.sql` per Spec Section 5.5.
  - Propagate `Version int64` through `internal/catalog/client.go` -> `domain.EnrichedCartItem` -> `cartv1.CartItem.ProductVersion`.
  - Implement transactional `Store.RemoveOrderedItems`: `INSERT INTO processed_events (event_id, event_type) VALUES ($1, 'OrderConfirmed') ON CONFLICT (event_id) DO NOTHING`; if inserted, delete matching rows in `cart_items` scoped to `user_id` (`id = $cartItemID` when valid UUID, else `sku_id = $2`).
  - Implement `internal/events/consumer.go` binding durable queue `cart.order-events` (with DLX `ecommerce.events.dlx` / DLQ `cart.order-events.dlq`) to `ecommerce.events` on routing key `order.confirmed`. Start it inside `buildProductionService` in `cmd/server/main.go` only when `cfg.RabbitMQURL != ""`, stopping it in the returned `cleanup()` closure so `cmd/server/main_test.go` remains untouched and fast.

- [ ] **Step 4: Run `cart-service` verification gate**
  - Run: `go run ./scripts/verify-service -service cart-service`
  - Expected: `PASSED: cart-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/cart-service go.work.sum && git commit -m "feat(cart): expose product version and clean up confirmed order items"`

---

### Task 6: `order-service` Persistence, Snapshot Diff, MoMo v2 HMAC, Outbox & Saga Orchestrator

**Files:**
- Modify: `services/order-service/pom.xml`
- Modify: `services/order-service/Dockerfile`
- Modify: `services/order-service/src/main/resources/application.yml`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/OrderApplication.java`
- Create: `services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql`
- Create: `services/order-service/src/main/java/com/ecommerce/order/config/{OrderProperties,GrpcClientConfig,RabbitMqConfig}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/common/{TraceIdProvider,GrpcTraceIdInterceptor,DomainException}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/domain/{OrderStatus,PaymentMethod,PaymentStatus,ShippingPolicy,SnapshotMismatchDetail}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/persistence/{OrderEntity,OrderItemEntity,OrderRepository,IdempotencyRecordEntity,IdempotencyRecordRepository,OutboxEventEntity,OutboxEventRepository}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/integration/{AuthorizationGrpcAdapter,CatalogGrpcAdapter,InventoryGrpcAdapter,MomoGatewayClient,MomoSignatureVerifier}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/application/{IdempotencyService,OrderTransactionStepService,OrderSagaOrchestrator,OrderQueryService,MomoCallbackService,OutboxRelayScheduler,OrderReconciliationScheduler}.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/grpc/{OrderGrpcService,GrpcServerLifecycle,GrpcErrorMapper}.java`
- Test: `services/order-service/src/test/java/com/ecommerce/order/{support/PostgresRabbitIntegrationTestSupport,persistence/OrderMigrationAndLockingTest,integration/MomoSignatureVerifierTest,application/IdempotencyServiceTest,application/OrderSagaOrchestratorTest,application/MomoCallbackLifecycleTest,application/OrderReconciliationSchedulerTest,grpc/OrderGrpcIntegrationTest}.java`

**Interfaces:**
- Consumes: `identity.v1.AuthorizationService`, `catalog.v1.CatalogService`, `inventory.v1.InventoryService`, `order.v1.OrderService` from Task 1; RBAC permissions `order.create`, `order.view`, `order.cancel` from Task 2.
- Produces:
  - Private gRPC server on `${ORDER_GRPC_HOST:127.0.0.1}:${ORDER_GRPC_PORT:50055}` implementing all 5 `OrderService` RPCs.
  - Shipping fee server invariant (`ShippingPolicy`): `subtotalMinor >= 299_000L ? 0L : 25_000L`, `totalMinor = subtotalMinor + shippingFeeMinor`, currency `"VND"`.
  - Rich gRPC error codes (`com.google.rpc.ErrorInfo` with `domain = "ecommerce.order.v1"`):
    - `InvalidArgument`: `INVALID_PARAMETER`, `INVALID_MOMO_SIGNATURE`
    - `PermissionDenied`: `ORDER_PERMISSION_DENIED`
    - `NotFound`: `ORDER_NOT_FOUND`
    - `FailedPrecondition`:
      - `PRODUCT_SNAPSHOT_MISMATCH` (`Metadata` maps `<skuId>` -> Vietnamese change description prefixed with `[UNAVAILABLE|VARIANT_UNAVAILABLE|PRICE_CHANGED|INFO_UPDATED|VERSION_CHANGED]`, e.g., `"[PRICE_CHANGED] Giá phân loại vừa thay đổi từ 29.990.000 ₫ sang 31.990.000 ₫"`)
      - `INVENTORY_INSUFFICIENT` (`Metadata` forwards `sku_id`, `product_id`, `available`, `requested`)
      - `ORDER_NOT_CANCELLABLE` (when `CancelOrder` is called on `status != PENDING_PAYMENT`)
    - `Aborted` / `AlreadyExists` -> mapped to `409` in BFF via `ErrorInfo`: `REQUEST_IN_PROGRESS`, `IDEMPOTENCY_KEY_REUSED`
    - `Unavailable`: `AUTHORIZATION_UNAVAILABLE`, `CATALOG_UNAVAILABLE`, `INVENTORY_UNAVAILABLE`, `PAYMENT_GATEWAY_UNAVAILABLE`
  - Environment config in `application.yml` (MUST NOT contain `:8000` or any foreign `*_db` name):
    - `ORDER_DB_URL` (`jdbc:postgresql://localhost:5432/order_db`), `ORDER_DB_USERNAME`, `ORDER_DB_PASSWORD`
    - `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USERNAME`, `RABBITMQ_PASSWORD`
    - `ORDER_GRPC_HOST` (`127.0.0.1`), `ORDER_GRPC_PORT` (`50055`), `IDENTITY_GRPC_ADDR` (`127.0.0.1:50054`, `500ms`), `CATALOG_GRPC_ADDR` (`127.0.0.1:50051`, `2s`), `INVENTORY_GRPC_ADDR` (`127.0.0.1:50052`, `1500ms`)
    - `MOMO_PARTNER_CODE` (`MOMO`), `MOMO_ACCESS_KEY` (`""`), `MOMO_SECRET_KEY` (`""`), `MOMO_API_ENDPOINT` (`https://test-payment.momo.vn/v2/gateway/api/create`), `MOMO_CONNECT_TIMEOUT` (`3s`), `MOMO_READ_TIMEOUT` (`10s`), `MOMO_REDIRECT_URL` (`http://localhost:8081/api/v1/payments/momo-redirect`), `MOMO_IPN_URL` (`http://localhost:8081/api/v1/payments/momo-ipn`), `MOMO_PAYMENT_TTL` (`15m`), `ORDER_MOMO_MOCK_ENABLED` (`false`).

- [ ] **Step 1: Add Spring Boot 4.1 dependencies to `services/order-service/pom.xml` and write failing unit & Testcontainers tests**
  - Add dependencies matching `identity-service/pom.xml` & Spring Boot 4.1 BOM: `spring-boot-starter-data-jpa`, `spring-boot-starter-flyway`, `flyway-database-postgresql`, `postgresql` (`runtime`), `spring-boot-starter-amqp`, `io.grpc:grpc-netty-shaded:1.65.1`, plus test dependencies `spring-boot-testcontainers`, `testcontainers-junit-jupiter`, `testcontainers-postgresql`, `testcontainers-rabbitmq`.
  - Write failing tests covering:
    1. `MomoSignatureVerifierTest`: exact v2 `captureWallet` request signature generation and constant-time callback verification (`MessageDigest.isEqual`), rejecting tampered `resultCode` or `amount`.
    2. `IdempotencyServiceTest`:
       - Pre-existing key fast-path & replay: `IN_PROGRESS` throws `REQUEST_IN_PROGRESS`; `COMPLETED` replays cached `CreateOrderResponse`; `FAILED_TERMINAL` replays the exact stored terminal `DomainException` (code, message, metadata) without attempting a duplicate `orders` insert; mutated quantity/address/paymentMethod throws `IDEMPOTENCY_KEY_REUSED`.
       - Concurrent duplicate `INSERT` race: two threads calling `createPendingOrderAtomically` simultaneously with the same `(userId, idempotencyKey)` execute `INSERT INTO idempotency_records ... ON CONFLICT (user_id, idempotency_key) DO NOTHING` -> winner (`inserted == 1`) inserts `orders(PENDING_STOCK)` + `order_items` in `Local TX #1`, while loser (`inserted == 0`) rolls back `Local TX #1` without `23505`/`500` and replays `findRecord(userId, key)` (`REQUEST_IN_PROGRESS` / `COMPLETED` / `FAILED_TERMINAL` / `IDEMPOTENCY_KEY_REUSED`).
    3. `OrderSagaOrchestratorTest`:
       - Validation, RBAC failure, and per-SKU snapshot diff cases (`deleted` -> `UNAVAILABLE`, missing `sku_id` in `product.variants` -> `VARIANT_UNAVAILABLE`, variant `price` -> `PRICE_CHANGED`, product `name` or variant `label` -> `INFO_UPDATED`, `version` -> `VERSION_CHANGED`) run strictly **before** `Local TX #1` and write **zero** rows to `idempotency_records` or `orders`.
       - `COD` happy path transitions `PENDING_STOCK -> CONFIRMED` (`paymentStatus = UNPAID`) under `SELECT ... FOR UPDATE`, calculates server-side shipping (`>= 299000 -> 0`, `< 299000 -> 25000`), marks `idempotency_records = COMPLETED`, and inserts `OrderConfirmed` in `outbox_events` with zero active DB transaction during gRPC calls.
       - Clean `INVENTORY_INSUFFICIENT` marks `CANCELLED` + `idempotency_records = FAILED_TERMINAL` without inserting `ReservationReleaseRequested`.
       - Uncertain gRPC timeout/network exception in the single `catch (Exception ex)` block (single fail-closed gRPC call with `1500ms` deadline, no synchronous retry loop) marks `CANCELLED` + `idempotency_records = FAILED_TERMINAL` AND inserts `ReservationReleaseRequested` into `outbox_events`, throwing `INVENTORY_UNAVAILABLE`.
       - `MOMO` gateway failure/timeout after `ReserveStock` marks `CANCELLED` + `idempotency_records = FAILED_TERMINAL` AND inserts `ReservationReleaseRequested`.
       - `abortsMomoCheckoutWithoutReturningPayUrlWhenOrderAlreadyCancelledByReconciler`: if `OrderReconciliationScheduler` already transitioned `PENDING_STOCK -> CANCELLED` while `MomoGatewayClient.createPayment` was in flight, `markPendingPayment` (`SELECT ... FOR UPDATE` checking `status == PENDING_STOCK`) aborts without overwriting `CANCELLED` and never returns `payUrl`.
    4. `MomoCallbackLifecycleTest`:
       - Concurrent `HandleMomoCallback(resultCode=0)` + `CancelOrder` + duplicate IPN serialized via `SELECT ... FOR UPDATE` produce exactly 1 terminal state and 1 outbox event.
       - Late `HandleMomoCallback(resultCode=0)` on an already `CANCELLED` order keeps `status = CANCELLED`, sets `paymentStatus = REFUND_REQUIRED`, records `momoTransId`, and emits no `OrderConfirmed`.
       - `CancelOrder` on `CONFIRMED` throws `ORDER_NOT_CANCELLABLE`.
    5. `OrderReconciliationSchedulerTest`: sweeps `PENDING_STOCK` older than `2m` (`FOR UPDATE SKIP LOCKED`) -> transitions order to `CANCELLED`, updates its paired `idempotency_records` (`WHERE status = 'IN_PROGRESS'` -> `FAILED_TERMINAL` with error payload) in the **same transaction**, and inserts `ReservationReleaseRequested`; sweeps `PENDING_PAYMENT` with `payment_expires_at < now()` -> transitions order to `CANCELLED` and inserts `ReservationReleaseRequested` (leaving its already-`COMPLETED` idempotency record intact; no separate orphan `IN_PROGRESS` sweep needed because `idempotency_records` is only ever inserted atomically with `orders` in `Local TX #1`).
    6. `OrderGrpcIntegrationTest`: end-to-end gRPC round-trip (`CreateOrder`, `GetOrder`, `ListOrders`, `CancelOrder`, `HandleMomoCallback`) + `OutboxRelayScheduler` publishing to Testcontainers RabbitMQ with publisher confirms without holding a DB transaction open during broker confirm.

- [ ] **Step 2: Run `order-service` tests to verify RED failure**
  - Run: `mvn -f services/order-service/pom.xml test`
  - Expected: FAIL (classes and migration `V1` not implemented yet).

- [ ] **Step 3: Implement `V1` Flyway schema, JPA entities with `@Version` + `FOR UPDATE`, adapters, MoMo v2 client, Saga orchestrator, schedulers, and gRPC server**
  - Create `V1__create_orders_idempotency_and_outbox.sql` per Spec Section 5.6 (`orders`, `order_items` including `sku_id VARCHAR(128) NOT NULL`, `idempotency_records`, `outbox_events` + `ix_outbox_unpublished` partial index on `(next_attempt_at, created_at) WHERE published_at IS NULL` + `ix_orders_reconcile` index on `(status, created_at, payment_expires_at)`).
  - Implement `Local TX #1` inside `OrderTransactionStepService` strictly **after** all read-only steps (input validation, `AuthorizationGrpcAdapter`, read-only `findRecord(userId, key)` check, and `CatalogGrpcAdapter` snapshot diff):
    - Execute `int inserted = idempotencyRepository.insertIfAbsent(userId, key, fingerprint, orderId, expiresAt)` (`INSERT INTO idempotency_records ... ON CONFLICT (user_id, idempotency_key) DO NOTHING`).
    - If `inserted == 0`, mark `Local TX #1` rollback-only / return empty without inserting `orders`, then outside `Local TX #1` call `idempotencyService.replayExisting(userId, key, fingerprint)` (`REQUEST_IN_PROGRESS` / `COMPLETED` / `FAILED_TERMINAL` / `IDEMPOTENCY_KEY_REUSED`).
    - If `inserted == 1`, persist `orders(PENDING_STOCK)` + `order_items` in the same `Local TX #1` and commit.
  - Use short `TransactionTemplate` / `REQUIRES_NEW` step methods so `OrderSagaOrchestrator` never holds a JDBC connection across `AuthorizationGrpcAdapter`, `CatalogGrpcAdapter`, `InventoryGrpcAdapter`, or `MomoGatewayClient`. Every transition method (`markConfirmedCod`, `markPendingPayment`, `markCancelledOnInsufficientStock`, `markCancelledWithCompensation`, `confirmMomoPayment`, `cancelPendingPayment`) locks the row via `OrderRepository.findByIdForUpdate` (`SELECT ... FOR UPDATE`) and validates the expected source `OrderStatus` before mutating; if `markPendingPayment` finds `status != PENDING_STOCK`, it throws `PAYMENT_GATEWAY_UNAVAILABLE` without mutating the row or returning `payUrl`.
  - Configure `MomoGatewayClient` with `java.net.http.HttpClient` `connectTimeout = 3s` (`MOMO_CONNECT_TIMEOUT`) and per-request `timeout = 10s` (`MOMO_READ_TIMEOUT`), strictly bounded far below the `2m` `PENDING_STOCK` reconciliation threshold.
  - Implement `MomoSignatureVerifier` with exact raw strings from Spec Pillar 6:
    - Request raw string: `"accessKey=" + accessKey + "&amount=" + amount + "&extraData=" + extraData + "&ipnUrl=" + ipnUrl + "&orderId=" + momoOrderId + "&orderInfo=" + orderInfo + "&partnerCode=" + partnerCode + "&redirectUrl=" + redirectUrl + "&requestId=" + requestId + "&requestType=captureWallet"`
    - Callback raw string: `"accessKey=" + accessKey + "&amount=" + amount + "&extraData=" + extraData + "&message=" + message + "&orderId=" + momoOrderId + "&orderInfo=" + orderInfo + "&orderType=" + orderType + "&partnerCode=" + partnerCode + "&payType=" + payType + "&requestId=" + requestId + "&responseTime=" + responseTime + "&resultCode=" + resultCode + "&transId=" + transId`
    - When `ORDER_MOMO_MOCK_ENABLED=true`, generate deterministic mock `momoOrderId`, `requestId`, and `payUrl` (`${redirectUrl}?partnerCode=...&signature=<validHmac>`) without calling external HTTPS so CI and load simulations can test full MoMo flows offline.
  - Configure RabbitMQ `CachingConnectionFactory` with `ConfirmType.CORRELATED` (`3s` confirm timeout) and implement `OutboxRelayScheduler` using two short transactions (`claimBatch` with `FOR UPDATE SKIP LOCKED` setting `next_attempt_at = now() + 15s` committed before broker publish, followed by `markPublished` or exponential-backoff `recordFailure` after broker confirm) so zero DB connections are held across RabbitMQ network I/O; gate `@EnableScheduling` by property so slice tests can trigger schedulers deterministically.
  - Update `services/order-service/Dockerfile` to `EXPOSE 8083 8093 50055`.

- [ ] **Step 4: Run `order-service` verification gate**
  - Run: `go run ./scripts/verify-service -service order-service`
  - Expected: `PASSED: order-service` (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/order-service && git commit -m "feat(order): implement checkout saga orchestrator, snapshot verification, MoMo v2 HMAC, and outbox"`

---

### Task 7: `bff-service`, Kong Gateway, Docker Compose & `start-local` Wiring

**Files:**
- Create: `services/bff-service/internal/order/client.go`
- Create: `services/bff-service/internal/order/client_test.go`
- Create: `services/bff-service/internal/http/orders.go`
- Create: `services/bff-service/internal/http/orders_test.go`
- Modify: `services/bff-service/internal/http/cart.go`
- Modify: `services/bff-service/internal/http/cart_test.go`
- Modify: `services/bff-service/internal/config/config.go`
- Modify: `services/bff-service/internal/config/config_test.go`
- Modify: `services/bff-service/cmd/server/main.go`
- Modify: `services/bff-service/cmd/server/main_test.go`
- Modify: `deployments/kong/kong.yaml`
- Modify: `deployments/kong/kong-production.yaml`
- Modify: `deployments/docker/compose.production.yml`
- Modify: `.env.example`, `.env.production.example`
- Modify: `services/bff-service/internal/config/kong_development_test.go`
- Modify: `services/bff-service/internal/config/kong_production_test.go`
- Modify: `services/bff-service/internal/config/compose_production_test.go`
- Modify: `scripts/start-local/main.go`
- Modify: `scripts/start-local/main_test.go`

**Interfaces:**
- Consumes: `cartv1.CartItem.product_version` (Task 5), `orderv1.OrderServiceClient` & `ErrorInfo` codes (Task 6).
- Produces:
  - `cartItem.ProductVersion int64` (`json:"productVersion"`) on `/api/v1/cart*` responses in `services/bff-service/internal/http/cart.go`.
  - BFF config env: `BFF_ORDER_GRPC_ADDR` (default `"127.0.0.1:50055"`), `BFF_ORDER_GRPC_TIMEOUT` (default `"5s"`), `BFF_STOREFRONT_BASE_URL` (default `"http://localhost:5173"`).
  - Protected REST routes (require `X-User-Id` injected by Kong `caller-context`):
    - `POST /api/v1/orders` (requires `Idempotency-Key` header `1..128` chars -> `201 Created` `{ data: { order: OrderDetailJSON, payUrl?: string }, message: "Đặt hàng thành công" }`)
    - `GET /api/v1/orders` (`page`, `pageSize`, `status` -> `200 OK` paginated `{ items, page, pageSize, totalItems, totalPages }`)
    - `GET /api/v1/orders/{orderId}` (`200 OK` `{ data: OrderDetailJSON, message: "..." }`)
    - `POST /api/v1/orders/{orderId}/cancel` (`{ reason?: string }` -> `200 OK` `{ data: OrderDetailJSON, message: "Đã hủy đơn hàng" }`)
  - Public MoMo callback routes (no `X-User-Id` required):
    - `GET /api/v1/payments/momo-redirect`: parses query params -> `HandleMomoCallback`; returns `400 INVALID_MOMO_SIGNATURE` on signature/amount failure, or `302 Found` to `${BFF_STOREFRONT_BASE_URL}/thanh-toan/ket-qua?orderId=<encoded>&result=success|failed` (`success` iff `resultCode == 0 && status == "CONFIRMED"`).
    - `POST /api/v1/payments/momo-ipn`: parses strict JSON body -> `HandleMomoCallback`; returns `400 INVALID_MOMO_SIGNATURE` on invalid signature/amount, or `204 No Content` on success.

- [ ] **Step 1: Write failing BFF HTTP/client, Kong route, Compose manifest, and `start-local` `.env` tests**
  - In `cart_test.go`, assert `GET /api/v1/cart` serializes `productVersion` from `cartv1.CartItem.ProductVersion`.
  - In `orders_test.go`:
    - `TestCreateOrderRequiresIdempotencyKeyAndForwardsSnapshotMismatchDetails` (`409 PRODUCT_SNAPSHOT_MISMATCH` with `errors` map populated from `ErrorInfo.Metadata`).
    - `TestListGetAndCancelOrderRoutes` (`200`, `403 ORDER_PERMISSION_DENIED`, `404 ORDER_NOT_FOUND`, `409 ORDER_NOT_CANCELLABLE`).
    - `TestMomoRedirectValidSignatureRedirectsToStorefrontResultPage` (`302` to `http://localhost:5173/thanh-toan/ket-qua?orderId=...&result=success` and `result=failed` on `resultCode=1006`).
    - `TestMomoRedirectAndIpnInvalidSignatureReturn400` (`400 INVALID_MOMO_SIGNATURE`).
  - Update `kong_development_test.go` and `kong_production_test.go` `expectedRoutes` to require `orders-api` (`caller-context`) and `payments-momo-public` (`request-transformer` stripping `X-User-Id` & `X-User-Roles`), plus `Idempotency-Key` in dev CORS allowed headers.
  - Update `compose_production_test.go` to assert `order-service` healthy deps (`postgres`, `rabbitmq`, `identity-service`, `catalog-service`, `inventory-service`), `ORDER_GRPC_HOST: "0.0.0.0"`, `ORDER_GRPC_PORT: "50055"`, `inventory-service` & `cart-service` `RABBITMQ_URL` + `rabbitmq` healthy dep, and `bff-service` `BFF_ORDER_GRPC_ADDR: "order-service:50055"` + `order-service` healthy dep.
  - In `scripts/start-local/main_test.go`, add `TestLoadsRootDotEnvWithoutOverridingExplicitProcessEnv` and assert `order-service`, `inventory-service`, `cart-service`, and `bff-service` receive RabbitMQ, gRPC, and MoMo defaults.

- [ ] **Step 2: Run BFF and scripts tests to verify RED failure**
  - Run: `go test ./services/bff-service/... ./scripts/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement `cart.go` `productVersion` mapping, `internal/order/client.go`, `internal/http/orders.go`, Kong & Compose updates, `.env.example`, and `start-local` `.env` loader**
  - Add `ProductVersion int64` (`json:"productVersion"`) to `cartItem` and `newCartData` in `services/bff-service/internal/http/cart.go`.
  - Map gRPC `ErrorInfo` codes (`PRODUCT_SNAPSHOT_MISMATCH`, `INVENTORY_INSUFFICIENT`, `REQUEST_IN_PROGRESS`, `IDEMPOTENCY_KEY_REUSED`, `ORDER_NOT_CANCELLABLE` -> `409`; `INVALID_MOMO_SIGNATURE`, `INVALID_PARAMETER` -> `400`; `ORDER_PERMISSION_DENIED` -> `403`; `ORDER_NOT_FOUND` -> `404`; `*_UNAVAILABLE` -> `503`) and attach `ErrorInfo.Metadata` as `errors` in the REST error envelope.
  - Update `deployments/kong/kong.yaml` (`orders-api` `/api/v1/orders`, `payments-momo-public` `/api/v1/payments/momo-redirect` & `/api/v1/payments/momo-ipn`, and `Idempotency-Key` under `cors.config.headers`), `deployments/kong/kong-production.yaml` (anchored `~/api/v1/payments/momo-redirect$` & `~/api/v1/payments/momo-ipn$` with `methods: ["GET", "POST"]`, `orders-api` with `methods: ["GET", "POST"]`), and `deployments/docker/compose.production.yml`.
  - Update `.env.example` and `.env.production.example` with placeholder `MOMO_PARTNER_CODE`, `MOMO_ACCESS_KEY`, `MOMO_SECRET_KEY`, `MOMO_API_ENDPOINT`, `MOMO_REDIRECT_URL`, `MOMO_IPN_URL`, `BFF_STOREFRONT_BASE_URL` (no live secrets in tracked files; `.env` and `.env.production` are already git-ignored).
  - In `scripts/start-local/main.go`, load `<root>/.env` key-values via `setDefaultEnv` before service switch so explicit shell env vars still win, and wire `RABBITMQ_URL`, `ORDER_GRPC_*`, `BFF_ORDER_GRPC_*`, `BFF_STOREFRONT_BASE_URL`, and `MOMO_REDIRECT_URL=http://localhost:8000/api/v1/payments/momo-redirect` / `MOMO_IPN_URL=http://localhost:8000/api/v1/payments/momo-ipn`.

- [ ] **Step 4: Run `bff-service` and `scripts` verification gates**
  - Run: `go run ./scripts/verify-service -service bff-service && go test ./scripts/...`
  - Expected: `PASSED: bff-service` and `ok` on all `scripts` packages (`exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add services/bff-service deployments .env.example .env.production.example scripts && git commit -m "feat(bff,kong): expose order and MoMo callback endpoints and wire runtime topology"`

---

### Task 8: `ecommerce-fe` Checkout Address Book, Payment Selector, Snapshot Mismatch UX & Order Pages

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
- Consumes: `/api/v1/users/me/addresses` (`identity-service`), `/api/v1/locations/provinces` & `/wards`, and `/api/v1/orders*` (`bff-service` from Task 7).
- Produces:
  - Types in `types.ts`: `CartItem.productVersion: number`, `ShippingAddress`, `CreateShippingAddressPayload`, `ShippingAddressSnapshot`, `PaymentMethod` (`"COD" | "MOMO"`), `OrderStatus`, `PaymentStatus`, `OrderItemSnapshot`, `OrderDetail`, `CreateOrderPayload`, `CreateOrderResult`, `OrderListResult`.
  - API functions in `api.ts`:
    - `listShippingAddresses(): Promise<ShippingAddress[]>` (`GET /users/me/addresses` -> `data.data`)
    - `createShippingAddress(payload: CreateShippingAddressPayload): Promise<ShippingAddress>` (`POST /users/me/addresses` -> `data.data`)
    - `createOrder(payload: CreateOrderPayload, idempotencyKey: string): Promise<CreateOrderResult>` (`POST /orders` with `headers: { "Idempotency-Key": idempotencyKey }`)
    - `listOrders(params?: { page?: number; pageSize?: number; status?: string }): Promise<OrderListResult>` (`GET /orders`)
    - `getOrder(orderId: string): Promise<OrderDetail>` (`GET /orders/${encodeURIComponent(orderId)}`)
    - `cancelOrder(orderId: string, reason?: string): Promise<OrderDetail>` (`POST /orders/${encodeURIComponent(orderId)}/cancel`)
  - Routes in `App.tsx` (wrapped in `<RequireAuth>`): `/gio-hang/dat-hang`, `/thanh-toan/ket-qua`, `/don-hang`, `/don-hang/:orderId`.

- [ ] **Step 1: Write failing Vitest tests for API client, `OrderPreviewPage`, `PaymentResultPage`, `OrdersPage`, and `SiteHeader`**
  - `api.test.ts`: `createOrder` sends `Idempotency-Key` header and preserves `errors` map on `409 PRODUCT_SNAPSHOT_MISMATCH`; `listShippingAddresses` unwraps `ApiResponse<ShippingAddress[]>`.
  - `order-preview-page.test.tsx`:
    - Loads `/users/me/addresses`, auto-selects `isDefault: true` address (or pre-populates shipping snapshot from `CurrentUser` when the address book is empty and `phone`, `provinceCode`, `wardCode`, `addressLine` are complete), resolves province/ward names, lets user switch or add a new shipping address via `ShippingAddressDialog`, and selects `COD` or `MOMO`.
    - Captures initial item snapshots (`skuId`, `productVersion`, `unitPrice`, `name`, `variantLabel`) on mount; `COD` submit refreshes cart (`retryLoad()`) and navigates to `/don-hang/${orderId}?placed=1`; `MOMO` submit redirects via `redirectToExternalUrl(payUrl)` helper.
    - Keeps the same `Idempotency-Key` in two cases: (a) network drop / browser timeout where no HTTP response was received (`ApiError` with `status === null`), and (b) `409 REQUEST_IN_PROGRESS` (displays `"Đơn hàng đang được xử lý..."`, keeps submit locked without rotating the key, and automatically retries `createOrder` after `1.5s` until receiving the final `201` or terminal error).
    - Rotates to a fresh `Idempotency-Key` after any **terminal** HTTP error response (`503`, `409 INVENTORY_INSUFFICIENT`, etc.) or when payment method, shipping address, or note changes.
    - On `409 PRODUCT_SNAPSHOT_MISMATCH`: renders prominent alert banner listing each per-item message from `err.errors`, disables `"Đặt hàng"`, and clicking `"Quay lại giỏ hàng để cập nhật"` calls `retryLoad()` and navigates to `/gio-hang`.
  - `payment-result-page.test.tsx` & `orders-page.test.tsx`: renders MoMo success/failure states, calls `retryLoad()` on confirmed MoMo order, lists paginated orders with status filter tabs, renders immutable snapshot details on `OrderDetailPage`, and allows cancelling `PENDING_PAYMENT` orders.
  - `site-header.test.tsx`: renders `"Đơn mua"` link to `/don-hang` in the authenticated user menu (hidden when `cartForbidden` is true for admin-only accounts).

- [ ] **Step 2: Run frontend tests to verify RED failure**
  - Run: `npm --prefix ecommerce-fe test -- --run`
  - Expected: FAIL.

- [ ] **Step 3: Implement API client, `ShippingAddressDialog`, `OrderPreviewPage`, `PaymentResultPage`, `OrdersPage`, `OrderDetailPage`, `SiteHeader`, and routes**
  - Keep components modular and styled consistently with existing storefront cards/badges (`ring-1 ring-foreground/10`, `rounded-2xl`, `SearchableSelect` for province/ward).
  - Do not invoke manual `toast.success` / `toast.error` around `createShippingAddress`, `createOrder`, or `cancelOrder` (handled centrally by `api.ts`). Suppress error toast in `api.ts` when `code === "REQUEST_IN_PROGRESS"` (since `OrderPreviewPage` displays `"Đơn hàng đang được xử lý..."` and auto-polls to completion) and suppress mutation success toast if `MOMO` immediately redirects away.

- [ ] **Step 4: Run `ecommerce-fe` verification gate**
  - Run: `go run ./scripts/verify-service -service ecommerce-fe`
  - Expected: `PASSED: ecommerce-fe` (`vitest`, `eslint`, and `tsc + vite build` all `exit 0`).

- [ ] **Step 5: Commit**
  - Run: `git add ecommerce-fe && git commit -m "feat(ecommerce-fe): build checkout address selector, COD/MoMo payment flow, snapshot mismatch UX, and order pages"`

---

### Task 9: Multi-Threaded Concurrency & Race-Condition Simulation Suite + Final Verification

**Files:**
- Create: `scripts/concurrency-sim/main.go`
- Create: `scripts/concurrency-sim/scenarios.go`
- Create: `scripts/concurrency-sim/sim_test.go`
- Create: `scripts/run-concurrency-sim.ps1`

**Interfaces:**
- Produces:
  - CLI `go run ./scripts/concurrency-sim -scenario <all|flash-sale-oversell|snapshot-mismatch-race|idempotency-storm|momo-callback-race> -base-url http://localhost:8000/api/v1` (plus `-momo-secret` / `.env` auto-load for Scenario 4 HMAC signing) and PowerShell wrapper `scripts/run-concurrency-sim.ps1`.
  - Structured JSON/table summary reporting total goroutines, p50/p95 latency, status/error-code histogram, and pass/fail invariant assertions (`exit 0` iff all invariants hold).

- [ ] **Step 1: Write failing simulation harness unit tests (`scripts/concurrency-sim/sim_test.go`)**
  - Using stateful concurrent `httptest.Server` fakes with mutex-guarded invariants, test all 4 scenario runners (`RunFlashSaleOversell`, `RunSnapshotMismatchRace`, `RunIdempotencyStorm`, `RunMomoCallbackRace`) asserting:
    1. Start-gate barrier (`close(startGate)`) releases all goroutines simultaneously.
    2. `RunFlashSaleOversell` fails when fake oversells (`6` `201`s for stock `5`) and passes when exact `5` `CONFIRMED` + `45` `409 INVENTORY_INSUFFICIENT` occur.
    3. `RunSnapshotMismatchRace` fails if any order post-admin-update commits at `v2` price with `v1` expectation, and passes when pre-update orders lock `v1` price while post-update requests get `409 PRODUCT_SNAPSHOT_MISMATCH`.
    4. `RunIdempotencyStorm` verifies 1 unique `orderId` across `20` identical-key requests and `409 IDEMPOTENCY_KEY_REUSED` on all `5` mutated-payload requests.
    5. `RunMomoCallbackRace` verifies `400` on all `5` forged signatures and single terminal state across concurrent `momo-redirect`, `momo-ipn`, and `cancel` calls.

- [ ] **Step 2: Run simulation unit tests to verify RED failure**
  - Run: `go test ./scripts/concurrency-sim/...`
  - Expected: FAIL (package not implemented yet).

- [ ] **Step 3: Implement `scripts/concurrency-sim` and `scripts/run-concurrency-sim.ps1`**
  - Implement self-bootstrapping live runner in `scenarios.go` + `main.go` (registers/logs in test customer accounts, seeds/sets stock via admin token when supplied or creates test product, executes synchronized goroutines via `close(startGate)` + `sync.WaitGroup`, polls eventual consistency window up to `5s` for RabbitMQ/Outbox settlement, and verifies stock/order invariants).
  - Add `scripts/run-concurrency-sim.ps1` accepting `-Scenario`, `-BaseUrl`, and `-AdminEmail`/`-AdminPassword` parameters.

- [ ] **Step 4: Run full repository verification gates across all touched surfaces**
  - Configure local `.env` (git-ignored) with the dev MoMo sandbox credentials if running live stack tests (`ORDER_MOMO_MOCK_ENABLED=true` for high-concurrency Scenario 4 bursts so `test-payment.momo.vn` is not rate-limited).
  - Run:
    - `go run ./scripts/check-boundaries`
    - `go test ./scripts/...`
    - `go run ./scripts/verify-service -service contracts`
    - `go run ./scripts/verify-service -service identity-service`
    - `go run ./scripts/verify-service -service catalog-service`
    - `go run ./scripts/verify-service -service inventory-service`
    - `go run ./scripts/verify-service -service cart-service`
    - `go run ./scripts/verify-service -service order-service`
    - `go run ./scripts/verify-service -service bff-service`
    - `go run ./scripts/verify-service -service ecommerce-fe`
    - `go run ./scripts/verify-service -service admin-fe`
  - If local Docker stack is running, execute `powershell -ExecutionPolicy Bypass -File ./scripts/run-concurrency-sim.ps1 -Scenario all` against the live stack (stopping any background local processes started solely for verification once complete).
  - Expected: Zero exit code on every gate and simulation scenario.

- [ ] **Step 5: Commit & Run Mandatory Independent Reviews (`code-reviewer` + `boundary-reviewer`)**
  - Run: `git add scripts/concurrency-sim scripts/run-concurrency-sim.ps1 && git commit -m "test(concurrency): add multi-threaded checkout and saga race-condition simulation suite"`
  - Dispatch independent read-only `code-reviewer` and `boundary-reviewer` subagents over `git diff main...HEAD` per `AGENTS.md` before final handoff.

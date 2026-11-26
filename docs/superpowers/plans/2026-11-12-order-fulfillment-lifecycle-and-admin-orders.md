# Vòng Đời Giao Hàng Đơn Hàng & Trang Quản Trị Đơn Hàng – Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Đơn COD tự xác nhận sau 1 phút, vòng đời đơn mở rộng tới Đã giao/Giao thất bại kèm lịch sử, giỏ xóa ngay khi đặt COD, hủy sau xác nhận hoàn kho, và admin có trang quản lý đơn.

**Architecture:** `order-service` sở hữu máy trạng thái (`OrderTransitions`), lịch sử (`order_status_history`), lịch tự xác nhận COD và 3 RPC admin. Tác dụng phụ sang service khác đi qua outbox: `OrderPlaced` (cart xóa giỏ), `StockRestockRequested` (inventory hoàn kho). BFF thêm `/api/v1/admin/orders`, Kong thêm route; hai frontend hiển thị trạng thái mới.

**Tech Stack:** Java 21/Spring Boot/Flyway/JPA (order, identity), Go 1.24/pgx/amqp091 (inventory, cart, bff), protobuf + buf, React/Vite/TypeScript/Vitest (admin-fe, ecommerce-fe).

**Spec:** `docs/superpowers/specs/2026-11-12-order-fulfillment-lifecycle-and-admin-orders-design.md`

## Global Constraints

- Mọi thay đổi contract (proto, REST, event) là additive; field proto mới đánh số từ 18 trong `OrderDetail`.
- Tên trạng thái: `PENDING_CONFIRMATION`, `PREPARING`, `SHIPPING`, `DELIVERED`, `DELIVERY_FAILED` (giữ nguyên `PENDING_STOCK`, `PENDING_PAYMENT`, `CONFIRMED`, `CANCELLED`).
- Permission: `order.admin.view` (đọc), `order.admin.update` (cập nhật), gán `ADMIN`.
- Event: `OrderPlaced` / `order.placed`; `StockRestockRequested` / `order.stock.restock_requested`; envelope ADR-002 (`eventId, eventType, eventVersion=1, occurredAt, aggregateId, correlationId, data`).
- Mã lỗi: `ORDER_INVALID_TRANSITION` (FAILED_PRECONDITION→409), `ORDER_STATUS_CONFLICT` (ABORTED→409), `ORDER_REASON_REQUIRED` (INVALID_ARGUMENT→400), `ORDER_NOT_CANCELLABLE` (409).
- Độ dài: `reason` ≤ 128, `shippingCarrier` ≤ 100, `trackingNumber` ≤ 100; vượt → `INVALID_PARAMETER`.
- Config: `order.cod.auto-confirm-delay` mặc định `1m` (`ORDER_COD_AUTO_CONFIRM_DELAY`), `order.cod.auto-confirm-interval-ms` mặc định `5000` (`ORDER_COD_AUTO_CONFIRM_INTERVAL_MS`).
- Không giữ DB transaction qua lời gọi mạng; mọi thay đổi `orders.status` ghi lịch sử và outbox trong cùng transaction.
- REST envelope: `{ data, message }`; lỗi `{ code, message, errors?, traceId }`; JSON `camelCase`.
- Không import chéo giữa `admin-fe` và `ecommerce-fe`; nhãn trạng thái được định nghĩa riêng ở mỗi app.
- Triển khai/khởi động: inventory và cart (consumer) lên trước order-service.

## Review Focus

1. **Hai admin cùng cập nhật một đơn** → người thứ hai nhận 409 `ORDER_STATUS_CONFLICT`, không ghi đè. Test ở Task 9.
2. **Event hoàn kho tới trước event xác nhận** (reservation còn `ACTIVE`) → release, không trừ kho; confirm đến sau là no-op. Test ở Task 3.
3. **Khách hủy đúng lúc scheduler tự xác nhận** → chỉ một bên thắng nhờ khóa dòng; không phát cả `OrderConfirmed` lẫn `ReservationReleaseRequested` cho cùng đơn. Test ở Task 7 (scheduler bỏ qua đơn đã `CANCELLED`).
4. **Replay `Idempotency-Key` của đơn COD sau khi đã tự xác nhận** → trả lại response gốc (`PENDING_CONFIRMATION`), không lỗi, không tạo đơn mới. Test ở Task 7.
5. **Hủy đơn MoMo đã `PAID` sau xác nhận** → `REFUND_REQUIRED` + `RefundRequested` + `StockRestockRequested`. Test ở Task 8.

---

### Task 1: ADR-015 và contract proto

**Files:**
- Create: `docs/adr/015-order-fulfillment-lifecycle.md` (theo `docs/adr/TEMPLATE.md`)
- Modify: `contracts/proto/order/v1/order.proto`
- Regenerate: `contracts/go/order/v1/*`, `contracts/java/...` (qua `buf generate`, không sửa tay)

**Interfaces:**
- Produces: message `OrderStatusChange`; `OrderDetail` fields 18–21 (`status_history`, `auto_confirm_at`, `shipping_carrier`, `tracking_number`); RPC `AdminListOrders`, `AdminGetOrder`, `AdminUpdateOrderStatus` với request/response đúng như spec §9.1.

- [ ] **Step 1:** Làm theo skill `contract-change`. Viết ADR-015: bối cảnh (ADR-005 chỉ có `PENDING_STOCK → CONFIRMED | CANCELLED`), quyết định (5 trạng thái mới, bảng chuyển §4.2, COD trì hoãn `OrderConfirmed`, `OrderPlaced`, `StockRestockRequested`, `RESTOCKED`), hệ quả (thứ tự triển khai consumer trước).
- [ ] **Step 2:** Sửa `order.proto` theo spec §9.1.
- [ ] **Step 3:** Chạy `buf format -w`, `buf lint`, `buf breaking --against '.git#branch=main'`. Expected: exit 0, không có breaking change.
- [ ] **Step 4:** Chạy `buf generate`, rồi `make verify SERVICE=contracts`. Expected: exit 0.
- [ ] **Step 5:** Commit `feat(contracts): add admin order RPCs and order status history fields`.

### Task 2: Identity – permission admin đơn hàng

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V13__add_admin_order_permissions.sql`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`

- [ ] **Step 1: Test fail.** Trong `IdentityDatabaseMigrationTest`: thêm `order.admin.update`, `order.admin.view` vào danh sách `containsExactly` tất cả permission (giữ thứ tự alphabet); thêm assertion ADMIN có đúng `order.admin.update`, `order.admin.view` trong các con `order.%`; CUSTOMER vẫn chỉ `order.cancel`, `order.create`, `order.view`. Sửa assertion hiện có nếu nó khẳng định ADMIN không có `order.*`.
- [ ] **Step 2:** `cd services/identity-service; mvn test -Dtest=IdentityDatabaseMigrationTest`. Expected: FAIL vì thiếu permission.
- [ ] **Step 3:** Viết `V13` theo mẫu `V12`: insert 2 `auth_items` (`PERMISSION`, `system_item = true`, mô tả "View all orders in the admin console" / "Update order fulfillment status in the admin console"), `on conflict (name) do update`; insert `('ADMIN', 'order.admin.view')`, `('ADMIN', 'order.admin.update')` vào `auth_item_children`, `on conflict do nothing`.
- [ ] **Step 4:** Chạy lại test. Expected: PASS. Rồi `mvn -pl services/identity-service -am test`. Expected: exit 0.
- [ ] **Step 5:** Commit `feat(identity): add admin order permissions`.

### Task 3: Inventory – hoàn kho theo đơn

**Files:**
- Create: `services/inventory-service/internal/store/migrations/000004_add_restocked_reservation_status.up.sql`
- Modify: `services/inventory-service/internal/store/reservation_store.go`
- Modify: `services/inventory-service/internal/events/consumer.go`
- Test: `services/inventory-service/internal/store/reservation_store_test.go`, `services/inventory-service/internal/events/consumer_test.go`
- Modify: `services/inventory-service/AGENTS.md` (mục `internal/events/` và migrations)

**Interfaces:**
- Produces: `func (s *Store) RestockReservation(ctx context.Context, orderID, reason string) (string, error)`; `ReservationHandler` thêm method cùng chữ ký; hằng `RoutingKeyStockRestock = "order.stock.restock_requested"`, `reservationStatusRestocked = "RESTOCKED"`.

- [ ] **Step 1: Test fail** trong `reservation_store_test.go` (cùng fixture DB với các test hiện có):
  - `TestRestockConfirmedReservationReturnsQuantity`: seed SKU `quantity=10`, reserve 3, confirm (→ quantity 7, reserved 0), restock → status `"RESTOCKED"`, quantity 10, reserved 0; gọi lại restock → `"RESTOCKED"`, quantity vẫn 10.
  - `TestRestockActiveReservationReleasesInsteadOfDeducting`: reserve 3 (reserved 3), restock → `"RELEASED"`, quantity 10, reserved 0; sau đó `ConfirmReservation` → trả `"RELEASED"`, quantity vẫn 10.
  - `TestConfirmAfterRestockIsNoop`: reserve, confirm, restock, confirm lại → `"RESTOCKED"`, quantity 10.
  - `TestRestockWithoutReservationWritesTombstone`: restock order lạ → `"RELEASED"`; `ReserveStock` sau đó → `domain.ErrReservationAlreadyReleased`.
- [ ] **Step 2:** `go test ./services/inventory-service/internal/store/... -run Restock`. Expected: FAIL (method chưa có).
- [ ] **Step 3:** Migration `000004`: drop CHECK hiện có của `stock_reservations.status` (tên mặc định `stock_reservations_status_check`) và tạo lại với `('ACTIVE','CONFIRMED','RELEASED','RESTOCKED')`. Implement `RestockReservation` theo mẫu `ReleaseReservation`: insert tombstone `RELEASED` nếu chưa có; khóa `FOR UPDATE`; `ACTIVE` → `reserved -= q`, `RELEASED`; `CONFIRMED` → `quantity += q`, `RESTOCKED`, `release_reason = reason`; trạng thái khác → no-op trả trạng thái hiện tại. Cập nhật stock theo `sku_id ASC` (dùng `loadReservationItemsTx`).
- [ ] **Step 4:** Chạy lại. Expected: PASS.
- [ ] **Step 5: Test fail** trong `consumer_test.go`: `TestDispatchMessageRoutesStockRestockRequested` – fake handler ghi lại lời gọi; body `{"eventId":"e1","eventType":"StockRestockRequested","data":{"orderId":"o-1","reason":"ADMIN_CANCELLED"}}` với routing key `order.stock.restock_requested` → handler nhận `RestockReservation("o-1","ADMIN_CANCELLED")`, không gọi Confirm/Release. Cập nhật mọi fake `ReservationHandler` khác trong test cho đủ method.
- [ ] **Step 6:** Chạy, thấy FAIL; thêm case vào `DispatchMessage`, thêm `RestockReservation` vào interface, thêm routing key vào vòng `QueueBind` trong `declareTopology`. Chạy lại: PASS.
- [ ] **Step 7:** `make verify SERVICE=inventory-service`. Expected: exit 0. Cập nhật `AGENTS.md`. Commit `feat(inventory): restock committed reservations on StockRestockRequested`.

### Task 4: Cart – xóa giỏ khi nhận `OrderPlaced`

**Files:**
- Modify: `services/cart-service/internal/events/consumer.go`
- Test: `services/cart-service/internal/events/consumer_test.go`
- Modify: `services/cart-service/AGENTS.md` (mục `internal/events/`)

**Interfaces:**
- Produces: hằng `RoutingKeyOrderPlaced = "order.placed"`.

- [ ] **Step 1: Test fail:** `TestDispatchMessageHandlesOrderPlaced` – body `OrderPlaced` có `eventId`, `data.userId`, 2 items, routing key `order.placed` → `RemoveOrderedItems` được gọi với đúng `eventId`, `userId`, 2 ref. Thêm `TestDispatchMessageIgnoresUnrelatedRoutingKey` (routing key `order.payment.cancel_requested`, eventType khác) → không gọi handler, trả nil.
- [ ] **Step 2:** `go test ./services/cart-service/internal/events/... -run DispatchMessage`. Expected: test `OrderPlaced` FAIL.
- [ ] **Step 3:** Mở rộng điều kiện lọc trong `DispatchMessage` để nhận `order.placed`/`OrderPlaced`; thêm routing key vào phần bind queue trong topology.
- [ ] **Step 4:** Chạy lại: PASS. `make verify SERVICE=cart-service`. Expected: exit 0.
- [ ] **Step 5:** Cập nhật `AGENTS.md`. Commit `feat(cart): clear ordered items on OrderPlaced`.

### Task 5: Order – schema, trạng thái và bảng chuyển

**Files:**
- Create: `services/order-service/src/main/resources/db/migration/V2__add_fulfillment_statuses_and_status_history.sql`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/domain/OrderStatus.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/domain/ActorType.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/domain/OrderTransitions.java`
- Modify: `services/order-service/src/main/java/com/ecommerce/order/persistence/OrderEntity.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/persistence/OrderStatusHistoryEntity.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/persistence/OrderStatusHistoryRepository.java`
- Test: `services/order-service/src/test/java/com/ecommerce/order/domain/OrderTransitionsTest.java`, `.../persistence/OrderMigrationAndLockingTest.java`

**Interfaces:**
- Produces:
  - `enum OrderStatus { PENDING_STOCK, PENDING_PAYMENT, PENDING_CONFIRMATION, CONFIRMED, PREPARING, SHIPPING, DELIVERED, DELIVERY_FAILED, CANCELLED }`
  - `enum ActorType { SYSTEM, CUSTOMER, ADMIN }`
  - `final class OrderTransitions`: `static List<OrderStatus> adminTargets(OrderStatus from)`, `static boolean customerCanCancel(OrderStatus from)`, `static boolean requiresReason(OrderStatus target)`.
  - `OrderEntity` thêm `Instant autoConfirmAt`, `String shippingCarrier` (default `""`), `String trackingNumber` (default `""`).
  - `OrderStatusHistoryEntity` (`id, orderId, fromStatus (nullable OrderStatus), toStatus, actorType, actorId (nullable UUID), reason, shippingCarrier, trackingNumber, createdAt`; các String default `""`).
  - `OrderStatusHistoryRepository extends JpaRepository<OrderStatusHistoryEntity, UUID>`: `List<OrderStatusHistoryEntity> findByOrderIdOrderByCreatedAtAsc(UUID orderId)`.

- [ ] **Step 1: Test fail** `OrderTransitionsTest` (JUnit thuần):
  - `adminTargets`: `PENDING_CONFIRMATION→[CANCELLED]`, `CONFIRMED→[PREPARING, CANCELLED]`, `PREPARING→[SHIPPING, CANCELLED]`, `SHIPPING→[DELIVERED, DELIVERY_FAILED]`, `DELIVERY_FAILED→[SHIPPING, CANCELLED]`; `PENDING_STOCK`, `PENDING_PAYMENT`, `DELIVERED`, `CANCELLED` → rỗng (dùng `@EnumSource` cho nhóm rỗng).
  - `customerCanCancel`: true đúng với `PENDING_PAYMENT`, `PENDING_CONFIRMATION`, `CONFIRMED`; false với mọi trạng thái khác.
  - `requiresReason`: true với `CANCELLED`, `DELIVERY_FAILED`; false với còn lại.
- [ ] **Step 2:** `mvn test -Dtest=OrderTransitionsTest` (trong `services/order-service`). Expected: FAIL biên dịch.
- [ ] **Step 3:** Implement `OrderStatus`, `ActorType`, `OrderTransitions` (bảng `EnumMap` bất biến).
- [ ] **Step 4:** Chạy lại: PASS.
- [ ] **Step 5: Test fail** trong `OrderMigrationAndLockingTest`: lưu `OrderEntity` với `status = DELIVERED`, `autoConfirmAt`, `shippingCarrier="GHN"`, `trackingNumber="T1"`; lưu 1 `OrderStatusHistoryEntity` và đọc lại bằng `findByOrderIdOrderByCreatedAtAsc`. Cập nhật `TRUNCATE` trong các test integration để gồm `order_status_history`.
- [ ] **Step 6:** Chạy, thấy FAIL; viết `V2`: drop/thêm lại CHECK `orders_status_check` với 9 trạng thái; `ALTER TABLE orders ADD COLUMN auto_confirm_at TIMESTAMPTZ, ADD COLUMN shipping_carrier VARCHAR(100) NOT NULL DEFAULT '', ADD COLUMN tracking_number VARCHAR(100) NOT NULL DEFAULT ''`; index một phần `ix_orders_pending_confirmation_due ON orders (auto_confirm_at) WHERE status = 'PENDING_CONFIRMATION'`; index `ix_orders_status_created_at ON orders (status, created_at DESC)`; bảng `order_status_history` theo spec §7 (reason `VARCHAR(128)`, các text NOT NULL DEFAULT '' trừ `from_status`, `actor_id`), CHECK `actor_type IN ('SYSTEM','CUSTOMER','ADMIN')`, index `(order_id, created_at)`. Thêm field entity và entity/repo lịch sử.
- [ ] **Step 7:** `mvn test -Dtest=OrderTransitionsTest,OrderMigrationAndLockingTest`. Expected: PASS. Commit `feat(order): add fulfillment statuses, transitions and status history schema`.

### Task 6: Order – ghi lịch sử ở mọi bước chuyển và trả về qua proto

**Files:**
- Modify: `services/order-service/src/main/java/com/ecommerce/order/application/OrderTransactionStepService.java`
- Modify: `.../application/OrderReconciliationScheduler.java`, `.../application/PaymentEventConsumer.java`
- Modify: `.../application/OrderProtoMapper.java`, `.../application/OrderQueryService.java`
- Test: `.../application/OrderReconciliationSchedulerTest.java`, `.../application/PaymentEventConsumerTest.java`, `.../application/OrderSagaOrchestratorTest.java`

**Interfaces:**
- Consumes: Task 5.
- Produces:
  - `OrderTransactionStepService.recordStatusChange(OrderEntity order, OrderStatus fromStatus, ActorType actorType, UUID actorId, String reason, Instant at)` – lưu một dòng lịch sử với `toStatus = order.getStatus()`, carrier/tracking lấy từ order; `reason` null → `""`.
  - `OrderProtoMapper.toOrderDetail(OrderEntity order, List<OrderStatusHistoryEntity> history)` – như bản cũ cộng `status_history`, `auto_confirm_at` (ISO hoặc `""`), `shipping_carrier`, `tracking_number`. Bản một tham số giữ nguyên (không có lịch sử) và cũng điền 3 field đơn.

- [ ] **Step 1: Test fail:** thêm assertion lịch sử vào test hiện có, mỗi test truy vấn `SELECT from_status, to_status, actor_type FROM order_status_history WHERE order_id = ? ORDER BY created_at`:
  - `OrderSagaOrchestratorTest`: luồng MoMo thành công → `[(null, PENDING_STOCK, CUSTOMER), (PENDING_STOCK, PENDING_PAYMENT, SYSTEM)]`; luồng tồn kho không đủ → dòng cuối `(PENDING_STOCK, CANCELLED, SYSTEM)`.
  - `OrderReconciliationSchedulerTest`: hai đơn bị hủy mỗi đơn có dòng `(PENDING_STOCK|PENDING_PAYMENT, CANCELLED, SYSTEM)` kèm reason `PENDING_STOCK_TIMEOUT`/`PAYMENT_EXPIRED`.
  - `PaymentEventConsumerTest`: `PaymentSucceeded` → `(PENDING_PAYMENT, CONFIRMED, SYSTEM)`; `PaymentFailed` → `(PENDING_PAYMENT, CANCELLED, SYSTEM)`.
- [ ] **Step 2:** Chạy 3 lớp test. Expected: FAIL (không có dòng lịch sử).
- [ ] **Step 3:** Implement `recordStatusChange` và gọi nó ngay sau mỗi `setStatus` trong: `beginOrderTx1` (from null, CUSTOMER, actorId = userId), `markPendingPayment`, `markCancelledOnSagaFailure`, 2 nhánh reconciliation, 2 nhánh `PaymentEventConsumer` đổi `status` (nhánh `REFUND_REQUIRED` không đổi status nên không ghi).
- [ ] **Step 4:** `OrderQueryService.getOrder` dùng bản mapper có lịch sử (`findByOrderIdOrderByCreatedAtAsc`); `listOrders` giữ bản không lịch sử. Thêm vào test `getOrder` hiện có (hoặc test mới trong `PaymentEventConsumerTest`) assertion `getStatusHistoryCount() > 0`.
- [ ] **Step 5:** Chạy lại các lớp trên. Expected: PASS. Commit `feat(order): record status history for every transition`.

### Task 7: Order – COD chờ xác nhận, `OrderPlaced` và tự xác nhận

**Files:**
- Modify: `.../application/OrderTransactionStepService.java`, `.../application/OrderSagaOrchestrator.java`
- Create: `.../application/CodAutoConfirmScheduler.java`
- Modify: `.../config/OrderProperties.java`, `.../config/RabbitMqConfig.java`, `src/main/resources/application.yml`
- Modify: `src/test/java/com/ecommerce/order/support/PostgresRabbitIntegrationTestSupport.java`
- Test: `.../application/OrderSagaOrchestratorTest.java`, Create `.../application/CodAutoConfirmSchedulerTest.java`

**Interfaces:**
- Produces:
  - `RabbitMqConfig.RK_ORDER_PLACED = "order.placed"`, `RK_STOCK_RESTOCK_REQUESTED = "order.stock.restock_requested"`.
  - `OrderProperties` thêm `Cod cod` với `record Cod(Duration autoConfirmDelay, long autoConfirmIntervalMs)`.
  - `OrderTransactionStepService.markPendingConfirmation(UUID orderId, UUID userId, String idempotencyKey) : CreateOrderResponse` (thay `confirmCodOrder`, xóa bản cũ).
  - `OrderTransactionStepService.insertOrderPlacedOutbox(OrderEntity order, Instant now)` – cùng `data` với `insertOrderConfirmedOutbox`; hai method dùng chung một helper riêng tư.
  - `CodAutoConfirmScheduler.confirmDueOrders() : void` (`@Scheduled(fixedDelayString = "${order.cod.auto-confirm-interval-ms:5000}")`).

- [ ] **Step 1: Test fail** (sửa test `codHappyPath...` và đổi tên thành `codHappyPathWaitsForConfirmationAndPublishesOrderPlaced`): response status `PENDING_CONFIRMATION`, `auto_confirm_at` ≈ now + 60s (±5s); outbox của đơn có đúng 1 `order.placed`, 0 `order.confirmed`; lịch sử có `(PENDING_STOCK, PENDING_CONFIRMATION, SYSTEM)`. Thêm `codReplayAfterAutoConfirmReturnsOriginalResponse`: tạo COD, gọi `CodAutoConfirmScheduler.confirmDueOrders()` sau khi đặt `auto_confirm_at` về quá khứ bằng SQL, gọi lại `createOrder` cùng key → response status `PENDING_CONFIRMATION`, cùng `orderId`, số đơn trong DB vẫn 1.
- [ ] **Step 2: Test fail** `CodAutoConfirmSchedulerTest` (extends `PostgresRabbitIntegrationTestSupport`, seed qua repository như `OrderReconciliationSchedulerTest`):
  - đơn COD `PENDING_CONFIRMATION`, `auto_confirm_at = now - 1s` → sau `confirmDueOrders()`: `CONFIRMED`, `paymentStatus = UNPAID`, 1 outbox `order.confirmed`, lịch sử `(PENDING_CONFIRMATION, CONFIRMED, SYSTEM)`;
  - đơn `auto_confirm_at = now + 60s` → giữ nguyên, không outbox;
  - đơn `CANCELLED` có `auto_confirm_at` quá khứ → giữ nguyên, không outbox.
- [ ] **Step 3:** Chạy 2 lớp test. Expected: FAIL.
- [ ] **Step 4:** Implement: properties + yml (`order.cod.auto-confirm-delay: ${ORDER_COD_AUTO_CONFIRM_DELAY:1m}`, `auto-confirm-interval-ms: ${ORDER_COD_AUTO_CONFIRM_INTERVAL_MS:5000}`); test support đặt `order.cod.auto-confirm-interval-ms=600000`; `markPendingConfirmation` (status, `autoConfirmAt = now + delay`, `OrderPlaced`, lịch sử, idempotency completed, wakeup outbox); orchestrator gọi nó cho COD; scheduler theo mẫu `OrderReconciliationScheduler` (`SELECT id ... WHERE status = 'PENDING_CONFIRMATION' AND auto_confirm_at <= ? ORDER BY auto_confirm_at LIMIT 50 FOR UPDATE SKIP LOCKED`, kiểm tra lại status sau khi load, `CONFIRMED`, lịch sử SYSTEM, `insertOrderConfirmedOutbox`, wakeup sau commit).
- [ ] **Step 5:** Chạy lại: PASS. Commit `feat(order): auto-confirm COD orders after a configurable delay`.

### Task 8: Order – hủy đơn mở rộng và tác dụng phụ

**Files:**
- Create: `.../application/OrderLifecycleService.java`
- Modify: `.../application/OrderSagaOrchestrator.java` (`cancelOrder` ủy quyền), `.../application/OrderTransactionStepService.java` (xóa `cancelPendingPaymentOrder`)
- Test: `.../application/PaymentEventConsumerTest.java` (sửa test hủy hiện có), Create `.../application/OrderLifecycleServiceTest.java`

**Interfaces:**
- Produces:
  - `OrderLifecycleService.cancelByCustomer(UUID userId, UUID orderId, String reason) : OrderDetail` (`@Transactional`; kiểm tra chủ sở hữu, `CANCELLED` trả lại nguyên trạng, `!customerCanCancel` → `ORDER_NOT_CANCELLABLE`; reason rỗng → `USER_CANCELLED`; trả detail kèm lịch sử).
  - `OrderLifecycleService.applyCancellation(OrderEntity order, ActorType actor, UUID actorId, String reason, Instant now) : void` (package-private, dùng chung với Task 9) – đặt `CANCELLED`, `cancelReason`, xóa `payUrl`, ghi lịch sử, và outbox theo trạng thái trước:
    - `PENDING_PAYMENT`: `ReservationReleaseRequested` + `PaymentCancelRequested`.
    - `PENDING_CONFIRMATION`: `ReservationReleaseRequested`.
    - `CONFIRMED`, `PREPARING`, `DELIVERY_FAILED`: `StockRestockRequested`; nếu `paymentStatus == PAID` → `REFUND_REQUIRED` + `RefundRequested`.

- [ ] **Step 1: Test fail.** Đổi tên và sửa `cancelOrderEmitsReleaseAndPaymentCancelOnPendingPaymentAndRejectsConfirmed` thành `cancelOrderEmitsReleaseAndPaymentCancelOnPendingPaymentAndRejectsShipping` (đơn `SHIPPING` → `ORDER_NOT_CANCELLABLE`). Trong `OrderLifecycleServiceTest` (seed bằng helper như `PaymentEventConsumerTest.seedOrder`):
  - `PENDING_CONFIRMATION` COD → routing keys đúng `[order.reservation.release_requested]`, payment `UNPAID`, lịch sử `(PENDING_CONFIRMATION, CANCELLED, CUSTOMER)`.
  - `CONFIRMED` COD → `[order.stock.restock_requested]`.
  - `CONFIRMED` MoMo `PAID` → `[order.payment.refund_requested, order.stock.restock_requested]`, payment `REFUND_REQUIRED`.
  - Đơn của user khác → `ORDER_NOT_FOUND`.
  - `CANCELLED` → trả về, không thêm outbox.
- [ ] **Step 2:** Chạy. Expected: FAIL.
- [ ] **Step 3:** Implement `OrderLifecycleService`; `OrderSagaOrchestrator.cancelOrder` giữ kiểm tra `order.cancel` rồi gọi `cancelByCustomer`; xóa `cancelPendingPaymentOrder`.
- [ ] **Step 4:** Chạy lại: PASS. Commit `feat(order): allow cancelling confirmed orders with restock and refund compensation`.

### Task 9: Order – RPC admin

**Files:**
- Modify: `.../application/OrderLifecycleService.java`
- Create: `.../application/OrderAdminQueryService.java`
- Modify: `.../persistence/OrderRepository.java`, `.../grpc/OrderGrpcService.java`
- Test: Create `.../application/OrderAdminServiceTest.java`; Modify `.../grpc/OrderGrpcIntegrationTest.java`

**Interfaces:**
- Consumes: Task 5, 6, 8.
- Produces:
  - `OrderAdminQueryService.listOrders(String rawUserId, String rawStatus, String query, int page, int pageSize) : AdminListOrdersResponse` – quyền `order.admin.view`; page mặc định 1, pageSize mặc định 20 tối đa 100; `status_counts` đếm mọi trạng thái theo cùng `query` (không theo `status`), trạng thái không có đơn vẫn có key với giá trị 0; danh sách không kèm lịch sử.
  - `OrderAdminQueryService.getOrder(String rawUserId, String rawOrderId) : AdminGetOrderResponse` – kèm lịch sử và `allowed_next_statuses = OrderTransitions.adminTargets(status)`.
  - `OrderLifecycleService.updateStatusByAdmin(String rawUserId, String rawOrderId, String rawExpectedStatus, String rawTargetStatus, String reason, String shippingCarrier, String trackingNumber) : AdminUpdateOrderStatusResponse` – quyền `order.admin.update` (kiểm tra trước transaction); validate độ dài/UUID/enum → `INVALID_PARAMETER`; khóa `findByIdForUpdate`; `status != expected` → `ORDER_STATUS_CONFLICT` (ABORTED); target không thuộc `adminTargets` → `ORDER_INVALID_TRANSITION`; `requiresReason(target)` và reason trống → `ORDER_REASON_REQUIRED`; target `CANCELLED` → `applyCancellation(... ADMIN ...)`; target `SHIPPING` → ghi carrier/tracking (trim) lên order; target `DELIVERED` và COD → `PAID`; ghi lịch sử ADMIN.
  - `OrderRepository`: `Page<OrderEntity> findAllByOrderByCreatedAtDesc(Pageable)`, `findByStatusOrderByCreatedAtDesc(OrderStatus, Pageable)`, `findByOrderCodeStartingWithIgnoreCaseOrderByCreatedAtDesc(String, Pageable)`, `findByStatusAndOrderCodeStartingWithIgnoreCaseOrderByCreatedAtDesc(OrderStatus, String, Pageable)`, và `@Query` đếm theo status (`SELECT o.status, COUNT(o) ... WHERE (:prefix = '' OR UPPER(o.orderCode) LIKE CONCAT(:prefix, '%')) GROUP BY o.status`).

- [ ] **Step 1: Test fail** `OrderAdminServiceTest` (mock `AuthorizationGrpcAdapter` cho `order.admin.view`/`order.admin.update`):
  - thiếu quyền → `ORDER_PERMISSION_DENIED` cho cả list/get/update.
  - list: 3 đơn (2 `CONFIRMED`, 1 `SHIPPING`), `status=CONFIRMED` → 2 item, `status_counts["CONFIRMED"]=2`, `["SHIPPING"]=1`, `["DELIVERED"]=0`; `query="ord-2026"` khớp tiền tố không phân biệt hoa thường.
  - get: đơn `CONFIRMED` → `allowed_next_statuses = [PREPARING, CANCELLED]`.
  - update `CONFIRMED→PREPARING→SHIPPING(carrier "GHN", tracking "T1")→DELIVERED` cho COD: cuối cùng `DELIVERED`, `PAID`, carrier/tracking lưu, 4 dòng lịch sử ADMIN theo thứ tự, `allowed_next_statuses` rỗng.
  - `expected_status` cũ (đơn đã `PREPARING`, gửi expected `CONFIRMED`) → `ORDER_STATUS_CONFLICT`, không đổi DB.
  - `CONFIRMED → DELIVERED` → `ORDER_INVALID_TRANSITION`.
  - `SHIPPING → DELIVERY_FAILED` không lý do → `ORDER_REASON_REQUIRED`; reason 129 ký tự → `INVALID_PARAMETER`.
  - admin hủy `PREPARING` MoMo `PAID` → `REFUND_REQUIRED`, outbox restock + refund, lịch sử actor ADMIN.
- [ ] **Step 2:** Chạy. Expected: FAIL.
- [ ] **Step 3:** Implement các method trên và 3 override trong `OrderGrpcService` theo đúng mẫu try/catch hiện có.
- [ ] **Step 4:** Thêm vào `OrderGrpcIntegrationTest` một test gọi `AdminGetOrder` qua stub gRPC thật với quyền bị từ chối → `PERMISSION_DENIED` có `ErrorInfo.reason = ORDER_PERMISSION_DENIED`.
- [ ] **Step 5:** `mvn -pl services/order-service -am test` và `make verify SERVICE=order-service`. Expected: exit 0. Commit `feat(order): add admin order list, detail and status update RPCs`.

### Task 10: BFF và Kong

**Files:**
- Modify: `services/bff-service/internal/order/client.go`, `client_test.go`
- Modify: `services/bff-service/internal/http/orders.go`, `orders_test.go`
- Create: `services/bff-service/internal/http/admin_orders.go`, `admin_orders_test.go`
- Modify: `services/bff-service/cmd/server/main.go`
- Modify: `deployments/kong/kong.yaml`, `deployments/kong/kong-production.yaml`, `services/bff-service/internal/config/kong_development_test.go`, `kong_production_test.go`
- Modify: `services/bff-service/AGENTS.md`, `docs/steering/service-catalog.md` (nếu liệt kê route)

**Interfaces:**
- Consumes: Task 1 (generated Go types).
- Produces:
  - `Client` thêm `AdminListOrders(ctx, userID, statusFilter, query string, page, pageSize int32) (*orderv1.AdminListOrdersResponse, error)`, `AdminGetOrder(ctx, userID, orderID string) (*orderv1.AdminGetOrderResponse, error)`, `AdminUpdateOrderStatus(ctx, req *orderv1.AdminUpdateOrderStatusRequest) (*orderv1.AdminUpdateOrderStatusResponse, error)`.
  - `orderDetailJSON` thêm `StatusHistory []orderStatusChangeJSON` (`statusHistory`, luôn là mảng), `AutoConfirmAt` (`autoConfirmAt,omitempty`), `ShippingCarrier`, `TrackingNumber` (omitempty). `orderStatusChangeJSON{fromStatus,toStatus,actorType,reason?,shippingCarrier?,trackingNumber?,createdAt}`.
  - `writeOrderError` tách thành hàm package `writeOrderServiceError(w, r, err)` dùng chung; thêm `ORDER_INVALID_TRANSITION`, `ORDER_STATUS_CONFLICT` vào nhóm 409, `ORDER_REASON_REQUIRED` → 400 với chính mã đó.
  - `NewAdminOrdersHandler(client orderclient.Client) *AdminOrdersHandler`, routes `GET /api/v1/admin/orders`, `GET /api/v1/admin/orders/{orderId}`, `POST /api/v1/admin/orders/{orderId}/status` (đều `correlateTrace(RequireCaller(...))`).

- [ ] **Step 1: Test fail** `admin_orders_test.go` (fake client; cập nhật `fakeOrderClient` hiện có và mọi fake khác cho đủ interface):
  - list: query `status=SHIPPING&q=ORD&page=2&pageSize=30` chuyển đúng xuống client; response `data` có `items`, `page`, `pageSize`, `totalItems`, `totalPages`, `statusCounts`; `pageSize=101` → 400 `INVALID_PARAMETER`.
  - get: `data.order.statusHistory` có phần tử, `data.allowedNextStatuses` đúng.
  - update: body JSON chuyển đủ 5 field + `UserId` = caller; thiếu `targetStatus` → 400; response 200 với `message` `"Cập nhật trạng thái đơn hàng thành công"`.
  - map lỗi: `ORDER_STATUS_CONFLICT`→409, `ORDER_INVALID_TRANSITION`→409, `ORDER_REASON_REQUIRED`→400, `ORDER_PERMISSION_DENIED`→403 `FORBIDDEN`, `AUTHORIZATION_UNAVAILABLE`→503.
  - Trong `orders_test.go`: chi tiết đơn khách có `statusHistory: []` khi proto rỗng và `autoConfirmAt` khi có.
- [ ] **Step 2:** `go test ./services/bff-service/internal/...`. Expected: FAIL.
- [ ] **Step 3:** Implement client, JSON, handler, wiring trong `main.go`.
- [ ] **Step 4: Test fail** Kong: thêm route `admin-orders-api` (path `/api/v1/admin/orders`, prod methods `["GET", "POST"]`) vào expected list của 2 test Kong; chạy → FAIL; thêm route vào 2 file YAML theo mẫu `admin-inventory-api`; chạy lại → PASS.
- [ ] **Step 5:** `make verify SERVICE=bff-service` và `make check-boundaries`. Expected: exit 0. Cập nhật docs. Commit `feat(bff): expose admin order endpoints and status history`.

### Task 11: admin-fe – trang Đơn hàng

**Files:**
- Modify: `admin-fe/src/lib/types.ts`, `admin-fe/src/lib/api.ts`, `admin-fe/src/lib/permissions.ts`
- Create: `admin-fe/src/lib/order-status.ts`
- Create: `admin-fe/src/pages/orders-page.tsx`, `admin-fe/src/pages/order-detail-sheet.tsx`, `admin-fe/src/pages/order-status-dialog.tsx`
- Modify: `admin-fe/src/App.tsx`, `admin-fe/src/components/layout/admin-shell.tsx`
- Test: Create `admin-fe/src/pages/orders-page.test.tsx`, `admin-fe/src/pages/order-status-dialog.test.tsx`; Modify `admin-fe/src/lib/api.test.ts`
- Modify: `admin-fe/AGENTS.md` (mục Ownership)

**Interfaces:**
- Consumes: Task 10 REST.
- Produces:
  - Types: `OrderStatus` (9 giá trị), `PaymentStatus`, `PaymentMethod`, `AdminOrderStatusChange`, `AdminOrderItem`, `AdminOrder`, `AdminOrderPage` (`statusCounts: Record<OrderStatus, number>`), `AdminOrderView { order: AdminOrder; allowedNextStatuses: OrderStatus[] }`, `ListAdminOrdersParams { status?: OrderStatus; q?: string; page?: number; pageSize?: number }`, `UpdateOrderStatusPayload { expectedStatus; targetStatus; reason?; shippingCarrier?; trackingNumber? }`.
  - API: `listAdminOrders(params, signal?)`, `getAdminOrder(orderId, signal?)`, `updateAdminOrderStatus(orderId, payload)`.
  - `order-status.ts`: `ORDER_STATUS_LABELS`, `PAYMENT_STATUS_LABELS`, `ORDER_STATUS_TABS` (thứ tự: Tất cả, Chờ thanh toán, Chờ xác nhận, Đã xác nhận, Đang chuẩn bị, Đang giao, Đã giao, Giao thất bại, Đã hủy), `orderStatusBadgeVariant(status)`.

- [ ] **Step 1: Test fail** `orders-page.test.tsx` (mẫu `inventory-page.test.tsx`, `makeAuth`):
  - render với `order.admin.view`: gọi `listAdminOrders` với `{ page: 1 }`; hiển thị tab kèm số từ `statusCounts`; click tab "Đang giao" → gọi lại với `status: "SHIPPING", page: 1`.
  - nhập mã đơn vào ô tìm và submit → gọi với `q`.
  - click một dòng → `getAdminOrder` được gọi, sheet hiển thị timeline theo `statusHistory`.
  - không có `order.admin.update` → không có nút "Cập nhật trạng thái".
- [ ] **Step 2: Test fail** `order-status-dialog.test.tsx`:
  - chỉ hiển thị lựa chọn từ `allowedNextStatuses`.
  - chọn `SHIPPING` → hiện ô "Đơn vị vận chuyển" và "Mã vận đơn"; submit gửi đúng payload có `expectedStatus` = trạng thái hiện tại.
  - chọn `CANCELLED` không nhập lý do → nút xác nhận bị vô hiệu và có thông báo bắt buộc lý do; không gọi API.
  - API ném lỗi code `ORDER_STATUS_CONFLICT` → gọi callback tải lại đơn và hiển thị "Trạng thái đơn vừa thay đổi, vui lòng kiểm tra lại".
- [ ] **Step 3: Test fail** `api.test.ts`: 3 hàm gọi đúng method/path/params và trả `data.data`.
- [ ] **Step 4:** `npm --prefix admin-fe run test`. Expected: FAIL.
- [ ] **Step 5:** Implement types, api, `order-status.ts`, 3 component (theo mẫu `inventory-page.tsx`, `product-detail-sheet.tsx`, `stock-dialog.tsx`), route `orders` gate `order.admin.view`, mục menu "Đơn hàng" trong nhóm "Thương mại & Kho vận" + `ROUTE_TITLES`, thêm 2 permission vào `ADMIN_CONSOLE_PERMISSIONS`.
- [ ] **Step 6:** `npm --prefix admin-fe run test`, `lint`, `build`, `make verify SERVICE=admin-fe`. Expected: exit 0. Cập nhật `AGENTS.md`. Commit `feat(admin-fe): add order management page`.

### Task 12: ecommerce-fe – trạng thái mới, timeline, hủy, thông báo COD

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Create: `ecommerce-fe/src/lib/order-status.ts`, `ecommerce-fe/src/components/order/order-timeline.tsx`
- Modify: `ecommerce-fe/src/pages/orders-page.tsx`, `ecommerce-fe/src/pages/order-detail-page.tsx`
- Test: `ecommerce-fe/src/pages/orders-page.test.tsx`; Create `ecommerce-fe/src/pages/order-detail-page.test.tsx`

**Interfaces:**
- Produces: `OrderStatus` union 9 giá trị; `OrderStatusChange { fromStatus?: OrderStatus; toStatus: OrderStatus; actorType: "SYSTEM" | "CUSTOMER" | "ADMIN"; reason?; shippingCarrier?; trackingNumber?; createdAt }`; `OrderDetail` thêm `statusHistory?: OrderStatusChange[]`, `autoConfirmAt?`, `shippingCarrier?`, `trackingNumber?`. `order-status.ts`: `getOrderStatusMeta(status, paymentStatus) → { label, className }` (thay `getStatusMeta` và `formatOrderStatus` cục bộ), `CUSTOMER_CANCELLABLE_STATUSES`, `ORDER_STATUS_TABS`.

- [ ] **Step 1: Test fail** `orders-page.test.tsx`: có tab "Chờ xác nhận", "Đang giao", "Đã giao"; badge đơn `DELIVERED` hiện "Đã giao"; click "Chờ xác nhận" → `listOrders` với `status: "PENDING_CONFIRMATION"`.
- [ ] **Step 2: Test fail** `order-detail-page.test.tsx`:
  - đơn `PENDING_CONFIRMATION` với `?placed=1` → hiện "hệ thống sẽ tự xác nhận trong khoảng 1 phút" và nút "Làm mới"; có nút hủy.
  - đơn `CONFIRMED` → có nút hủy; đơn `SHIPPING` → không có nút hủy, hiện đơn vị vận chuyển + mã vận đơn.
  - `statusHistory` 3 phần tử → timeline hiển thị 3 mục theo thứ tự thời gian với nhãn tiếng Việt.
- [ ] **Step 3:** `npm --prefix ecommerce-fe run test`. Expected: FAIL.
- [ ] **Step 4:** Implement; `PaymentResultPage` giữ nguyên.
- [ ] **Step 5:** `npm --prefix ecommerce-fe run test`, `lint`, `build`, `make verify SERVICE=ecommerce-fe`. Expected: exit 0. Commit `feat(ecommerce-fe): show fulfillment statuses, timeline and COD confirmation notice`.

### Task 13: Kiểm chứng toàn hệ thống và review

- [ ] **Step 1:** Dừng stack đang chạy (`.\scripts\start-all.ps1 -Stop`), khởi động lại hạ tầng và mọi service theo thứ tự (`.\scripts\start-all.ps1`), bật `ecommerce-fe` và `admin-fe` ở background; kiểm tra health.
- [ ] **Step 2:** Kịch bản COD qua UI (Chrome DevTools MCP) hoặc `order-hybrid-manual-test.http`: đặt COD bằng `customer1@gmail.com` → giỏ không còn dòng đã đặt ngay sau khi đặt; trạng thái `PENDING_CONFIRMATION`; sau ≤ 70 giây `CONFIRMED` và tồn kho bị trừ (`psql` vào `inventory_db`: `quantity` giảm, reservation `CONFIRMED`).
- [ ] **Step 3:** Admin (tài khoản SUPER_ADMIN/ADMIN) trên `admin-fe`: Đang chuẩn bị → Đang giao (GHN/T1) → Đã giao; đơn hiện `PAID`, timeline 5+ dòng ở cả admin và storefront.
- [ ] **Step 4:** Đặt COD thứ hai, chờ `CONFIRMED`, khách hủy → reservation `RESTOCKED`, `quantity` trở lại giá trị trước khi đặt.
- [ ] **Step 5:** `make check-boundaries`, `buf lint`. Expected: exit 0.
- [ ] **Step 6:** Review độc lập: `code-reviewer` và `boundary-reviewer` trên diff `main...HEAD`, kèm spec, plan, danh sách lệnh đã chạy. Sửa finding blocking, chạy lại gate liên quan, re-review phạm vi đã sửa (tối đa 3 vòng).

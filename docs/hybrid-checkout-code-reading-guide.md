# Hướng Dẫn Đọc Code & Cài Đặt Luồng Hybrid Saga (`feat/checkout-hybrid`)

> **Thư mục mã nguồn:** Mọi đường dẫn bên dưới tính từ thư mục gốc repository.

---

## 1. File Cần Xem Đầu Tiên (Trọng Tâm & Bức Tranh Toàn Cảnh)

👉 **[`services/order-service/.../OrderSagaOrchestrator.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderSagaOrchestrator.java)**

### Lý do:
Đây là **trung tâm điều phối (Orchestrator)** của toàn bộ nghiệp vụ đặt hàng trong bản Hybrid. Chỉ cần đọc phương thức [`createOrder(CreateOrderRequest)`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderSagaOrchestrator.java#L72-L289), bạn sẽ nắm được toàn bộ quy trình từ đầu đến cuối:
1. **Kiểm tra quyền:** Gọi Identity Service kiểm tra quyền `order.create`.
2. **Kiểm tra Idempotency:** Tính fingerprint SHA-256 của request và kiểm tra xem đã có đơn tương tự chưa.
3. **Thẩm định Snapshot:** Gọi gRPC sang `catalog-service` kiểm tra giá, nhãn biến thể và `version`. Nếu lệch, ném `409 PRODUCT_SNAPSHOT_MISMATCH`.
4. **Local TX #1:** Ghi nguyên tử đơn hàng ở trạng thái `PENDING_STOCK` và bản ghi idempotency `IN_PROGRESS`.
5. **Giữ kho đồng bộ:** Gọi gRPC sang `inventory-service` (`ReserveStock`). Nếu thiếu hàng, hủy đơn `FAILED_TERMINAL` và ném `409 INVENTORY_INSUFFICIENT`.
6. **Rẽ nhánh thanh toán:**
   - Nếu là **`COD`**: Chuyển đơn sang `PENDING_CONFIRMATION`, đặt `auto_confirm_at = now + 1 phút`, ghi Outbox `OrderPlaced` (để giỏ hàng được dọn ngay), trả ngay `201 Created`. Khoảng 1 phút sau `CodAutoConfirmScheduler` mới chốt `CONFIRMED` + Outbox `OrderConfirmed`.
   - Nếu là **`MOMO`**: Gọi gRPC `payment-service` (`CreatePaymentSession`) lấy link `payUrl`, cập nhật `PENDING_PAYMENT`, trả ngay `201 Created` kèm `payUrl`.

Sau khi đơn được xác nhận, vòng đời giao hàng (Đang chuẩn bị → Đang giao → Đã giao / Giao thất bại) do admin điều khiển; xem **Bước 8** và [ADR-015](adr/015-order-fulfillment-lifecycle.md).

---

## 2. Lộ Trình Đọc Toàn Bộ Các File (Theo Đúng Trật Tự Luồng Chạy)

### Bước 1: Cửa ngõ tiếp nhận HTTP & Chuyển đổi sang gRPC (BFF)
- [`services/bff-service/internal/http/orders.go`](../services/bff-service/internal/http/orders.go)  
  *Tiếp nhận `POST /api/v1/orders`, đọc header `Idempotency-Key`, gọi gRPC `order-service`, chuyển đổi mã lỗi gRPC sang REST envelope chuẩn (`201 Created`, `409 Conflict`, v.v.).*
- [`services/bff-service/internal/order/client.go`](../services/bff-service/internal/order/client.go)  
  *Gói client gRPC gọi sang `order-service` cổng `:50055`.*

---

### Bước 2: Quản lý Transaction Cục bộ & Idempotency (`order-service`)
- [`services/order-service/.../OrderTransactionStepService.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderTransactionStepService.java)  
  *Tập hợp các method transaction ngắn (`@Transactional`), độc lập, tuyệt đối không chứa network call: `beginOrderTx1`, `markPendingConfirmation` (COD), `markPendingPayment` (MoMo), `markCancelledOnSagaFailure`. Đây cũng là nơi có `recordStatusChange` (ghi `order_status_history` cùng transaction với mọi lần đổi trạng thái), các helper ghi Outbox (`insertOrderPlacedOutbox`, `insertOrderConfirmedOutbox`, `insertCompensationOutbox`) và `registerOutboxWakeupAfterCommit` (đăng ký `TransactionSynchronization.afterCommit()` để chỉ đánh thức `OutboxRelayScheduler` sau khi DB commit xong, tránh thread nền đọc hụt dòng chưa commit hoặc chạy thừa khi transaction rollback).*
- [`services/order-service/.../IdempotencyService.java`](../services/order-service/src/main/java/com/ecommerce/order/application/IdempotencyService.java)  
  *Tính mã băm `SHA-256` của payload và kiểm tra trạng thái `IN_PROGRESS`, `COMPLETED`, `FAILED_TERMINAL`.*
- [`services/order-service/.../V1__create_orders_idempotency_and_outbox.sql`](../services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql)  
  *Schema database của `order_db` với các bảng `orders`, `order_items`, `idempotency_records`, `outbox_events`.*
- [`services/order-service/.../V2__add_fulfillment_statuses_and_status_history.sql`](../services/order-service/src/main/resources/db/migration/V2__add_fulfillment_statuses_and_status_history.sql)  
  *Mở rộng `CHECK` trạng thái thành 9 giá trị, thêm cột `auto_confirm_at`, `shipping_carrier`, `tracking_number` và bảng `order_status_history`.*

---

### Bước 3: Giữ kho nguyên tử & Chống bán âm (`inventory-service`)
- [`services/inventory-service/internal/grpc/server.go`](../services/inventory-service/internal/grpc/server.go) (tìm hàm `ReserveStock`)  
  *Xác thực danh sách SKU đầu vào, thời hạn giữ kho (`expires_at`), gọi store giữ kho.*
- [`services/inventory-service/internal/store/reservation_store.go`](../services/inventory-service/internal/store/reservation_store.go) (tìm hàm `ReserveStock`)  
  *Trọng tâm giữ kho:*
  - Sắp xếp SKU theo thứ tự `sku_id ASC` để triệt tiêu nguy cơ Deadlock khi nhiều đơn cùng mua các SKU chéo nhau.
  - Ghi bản ghi `stock_reservations` (`ACTIVE`).
  - Thực thi câu lệnh `UPDATE stock SET reserved = reserved + $2 WHERE sku_id = $1 AND (quantity - reserved) >= $2` bảo vệ bất biến tồn kho trực tiếp trong PostgreSQL.

---

### Bước 4: Khởi tạo phiên MoMo & Ký HMAC (`payment-service`)
- [`services/payment-service/internal/grpc/server.go`](../services/payment-service/internal/grpc/server.go) (hàm `CreatePaymentSession`)  
  *Nhận yêu cầu tạo phiên thanh toán từ `order-service`.*
- [`services/payment-service/internal/momo/client.go`](../services/payment-service/internal/momo/client.go) & [`signer.go`](../services/payment-service/internal/momo/signer.go)  
  *Ký `HmacSHA256` cho request gửi sang MoMo API v2 (`captureWallet`) để lấy `payUrl`.*
- [`services/payment-service/internal/store/store.go`](../services/payment-service/internal/store/store.go)  
  *Lưu bản ghi `payments` (`PENDING`) kèm `pay_url` trong `payment_db`.*

---

### Bước 5: Tiếp nhận Callback MoMo & Chốt đơn bất đồng bộ
- [`services/bff-service/internal/http/payments.go`](../services/bff-service/internal/http/payments.go)  
  *Tiếp nhận webhook `/api/v1/payments/momo-redirect` và `/api/v1/payments/momo-ipn`, xóa header người dùng giả mạo rồi chuyển tiếp sang gRPC `PaymentService.HandleMomoCallback`.*
- [`services/payment-service/internal/grpc/server.go`](../services/payment-service/internal/grpc/server.go) (hàm `HandleMomoCallback`)  
  *Xác thực chữ ký HMAC MoMo hằng thời gian (`hmac.Equal`), cập nhật `payments` thành `SUCCEEDED` hoặc `FAILED`, ghi sự kiện vào `payment_db.outbox_events`.*
- [`services/payment-service/internal/events/relay.go`](../services/payment-service/internal/events/relay.go)  
  *Tiến trình quét Outbox định kỳ đẩy `PaymentSucceeded` / `PaymentFailed` lên RabbitMQ.*
- [`services/order-service/.../PaymentEventConsumer.java`](../services/order-service/src/main/java/com/ecommerce/order/application/PaymentEventConsumer.java)  
  *Tiêu thụ sự kiện MoMo từ RabbitMQ: chuyển đơn sang `CONFIRMED` + `PAID` (+ Outbox `OrderConfirmed`) hoặc `CANCELLED` (+ Outbox `ReservationReleaseRequested`). Nếu thanh toán đến muộn khi đơn đã hủy -> chuyển `REFUND_REQUIRED`.*
- [`services/inventory-service/internal/events/consumer.go`](../services/inventory-service/internal/events/consumer.go) & [`reservation_store.go`](../services/inventory-service/internal/store/reservation_store.go)  
  *Lắng nghe RabbitMQ trên queue `inventory.order-events`:*
  - `OrderConfirmed` → `ConfirmReservation`: trừ kho vật lý (`quantity - X`, `reserved - X`), reservation `CONFIRMED`.
  - `ReservationReleaseRequested` → `ReleaseReservation`: nhả giữ chỗ (`reserved - X`), reservation `RELEASED`.
  - `StockRestockRequested` → `RestockReservation`: hoàn kho cho đơn bị hủy sau khi đã xác nhận (`quantity + X`), reservation `RESTOCKED`; nếu reservation còn `ACTIVE` thì nhả giữ chỗ thay vì cộng kho.
- [`services/cart-service/internal/events/consumer.go`](../services/cart-service/internal/events/consumer.go)  
  *Lắng nghe `OrderPlaced` (COD, ngay khi đặt) và `OrderConfirmed` (MoMo, khi thanh toán xong) để xóa các sản phẩm đã mua khỏi giỏ hàng. Khóa khử trùng là `order-cleanup:<orderId>` nên mỗi đơn chỉ dọn giỏ một lần.*

---

### Bước 6: Xử lý hết hạn & Đối soát tự động (Scheduler)
- [`services/order-service/.../OrderReconciliationScheduler.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderReconciliationScheduler.java)  
  *Quét các đơn bị kẹt `PENDING_STOCK` quá 2 phút (do lỗi mạng) để tự động hủy và nhả kho; quét các đơn MoMo `PENDING_PAYMENT` quá hạn 15 phút chưa thanh toán để tự động hết hạn và nhả kho.*
- [`services/order-service/.../CodAutoConfirmScheduler.java`](../services/order-service/src/main/java/com/ecommerce/order/application/CodAutoConfirmScheduler.java)  
  *Mỗi 5 giây quét đơn COD `PENDING_CONFIRMATION` có `auto_confirm_at <= now()` bằng `FOR UPDATE SKIP LOCKED`, chuyển `CONFIRMED`, ghi lịch sử và Outbox `OrderConfirmed`. Cấu hình: `ORDER_COD_AUTO_CONFIRM_DELAY` (mặc định `1m`), `ORDER_COD_AUTO_CONFIRM_INTERVAL_MS` (mặc định `5000`).*
- [`services/order-service/.../OutboxRelayScheduler.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OutboxRelayScheduler.java)  
  *Đẩy sự kiện Outbox từ `order_db` lên RabbitMQ với publisher confirm và `mandatory`: event chưa có queue nhận bị broker trả về và được giữ lại để gửi lại, không bị đánh dấu đã publish. Kết hợp 2 cơ chế kích hoạt: đánh thức tức thì (`triggerFlush`) dùng `AtomicBoolean.compareAndSet` (lock-free CAS) để gom nhiều yêu cầu flush đồng thời vào một lượt chạy trên thread nền, và quét định kỳ `scheduledSweep` mỗi `1000ms` (`FOR UPDATE SKIP LOCKED`) làm lưới an toàn khi app khởi động lại hoặc gặp lỗi tạm thời.*

---

### Bước 7: Trải nghiệm Giao diện Người dùng (Frontend)
- [`ecommerce-fe/src/pages/order-preview-page.tsx`](../ecommerce-fe/src/pages/order-preview-page.tsx)  
  *Trang xem trước và đặt hàng: sinh `Idempotency-Key`, gọi API tạo đơn, điều hướng sang `payUrl` MoMo hoặc trang chi tiết đơn.*
- [`ecommerce-fe/src/pages/payment-result-page.tsx`](../ecommerce-fe/src/pages/payment-result-page.tsx)  
  *Trang kết quả sau khi MoMo redirect về: kiểm tra trạng thái đơn, tự động poll nhẹ nếu đơn đang đợi RabbitMQ hội tụ.*
- [`ecommerce-fe/src/pages/orders-page.tsx`](../ecommerce-fe/src/pages/orders-page.tsx) & [`order-detail-page.tsx`](../ecommerce-fe/src/pages/order-detail-page.tsx)  
  *Danh sách và chi tiết đơn của khách: tab theo trạng thái, thông báo "hệ thống sẽ tự xác nhận trong khoảng 1 phút" khi vừa đặt COD, timeline lịch sử ([`components/order/order-timeline.tsx`](../ecommerce-fe/src/components/order/order-timeline.tsx)), đơn vị vận chuyển + mã vận đơn, nút hủy ở `PENDING_PAYMENT` / `PENDING_CONFIRMATION` / `CONFIRMED`. Nhãn trạng thái nằm ở [`lib/order-status.ts`](../ecommerce-fe/src/lib/order-status.ts).*

---

### Bước 8: Vòng đời giao hàng & Quản trị đơn hàng ([ADR-015](adr/015-order-fulfillment-lifecycle.md))
- [`services/order-service/.../domain/OrderTransitions.java`](../services/order-service/src/main/java/com/ecommerce/order/domain/OrderTransitions.java)  
  *Nguồn duy nhất của các bước hợp lệ: bước admin được phép từ mỗi trạng thái (`adminTargets`), trạng thái khách còn được hủy (`customerCanCancel`), bước bắt buộc nhập lý do (`requiresReason`).*
- [`services/order-service/.../OrderLifecycleService.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderLifecycleService.java)  
  *`cancelByCustomer` và `updateStatusByAdmin` (kiểm quyền `order.admin.update`, so `expectedStatus` để chặn ghi đè đồng thời → `409 ORDER_STATUS_CONFLICT`). `applyCancellation` chọn bồi hoàn theo trạng thái trước khi hủy: nhả giữ chỗ (`ReservationReleaseRequested`), hủy phiên MoMo (`PaymentCancelRequested`), hoặc hoàn kho (`StockRestockRequested`) kèm `RefundRequested` nếu đã `PAID`. Đơn COD chuyển `PAID` khi `DELIVERED`.*
- [`services/order-service/.../OrderAdminQueryService.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderAdminQueryService.java)  
  *Danh sách đơn cho admin (lọc trạng thái, tìm theo tiền tố mã đơn, `status_counts` cho các tab) và chi tiết đơn kèm lịch sử + `allowed_next_statuses`; kiểm quyền `order.admin.view`.*
- [`services/order-service/.../grpc/OrderGrpcService.java`](../services/order-service/src/main/java/com/ecommerce/order/grpc/OrderGrpcService.java)  
  *Các RPC `AdminListOrders`, `AdminGetOrder`, `AdminUpdateOrderStatus` (contract ở [`contracts/proto/order/v1/order.proto`](../contracts/proto/order/v1/order.proto)).*
- [`services/bff-service/internal/http/admin_orders.go`](../services/bff-service/internal/http/admin_orders.go)  
  *REST `GET /api/v1/admin/orders`, `GET /api/v1/admin/orders/{orderId}`, `POST /api/v1/admin/orders/{orderId}/status`; route Kong `admin-orders-api`.*
- [`services/identity-service/.../V13__add_admin_order_permissions.sql`](../services/identity-service/src/main/resources/db/migration/V13__add_admin_order_permissions.sql)  
  *Quyền `order.admin.view`, `order.admin.update` gán cho `ADMIN`.*
- [`admin-fe/src/pages/orders-page.tsx`](../admin-fe/src/pages/orders-page.tsx), [`order-detail-sheet.tsx`](../admin-fe/src/pages/order-detail-sheet.tsx), [`order-status-dialog.tsx`](../admin-fe/src/pages/order-status-dialog.tsx)  
  *Trang "Đơn hàng": tab trạng thái kèm số lượng, tìm mã đơn, chi tiết + timeline, dialog cập nhật chỉ hiện `allowedNextStatuses` (chọn "Đang giao" thì nhập đơn vị vận chuyển và mã vận đơn dạng văn bản tự do; "Hủy" / "Giao thất bại" bắt buộc lý do).*

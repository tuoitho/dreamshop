# Hướng Dẫn Đọc Code & Cài Đặt Luồng Hybrid Saga (`feat/checkout-hybrid`)

> **Thư mục mã nguồn:** Mọi đường dẫn bên dưới tính từ thư mục gốc repository.

---

## 1. File Cần Xem Đầu Tiên (Trọng Tâm & Bức Tranh Toàn Cảnh)

👉 **[`services/order-service/.../OrderSagaOrchestrator.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderSagaOrchestrator.java)**

### Lý do:
Đây là **trung tâm điều phối (Orchestrator)** của toàn bộ nghiệp vụ đặt hàng trong bản Hybrid. Chỉ cần đọc phương thức [`createOrder(CreateOrderRequest)`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderSagaOrchestrator.java#L69-L286), bạn sẽ nắm được toàn bộ quy trình từ đầu đến cuối:
1. **Kiểm tra quyền:** Gọi Identity Service kiểm tra quyền `order.create`.
2. **Kiểm tra Idempotency:** Tính fingerprint SHA-256 của request và kiểm tra xem đã có đơn tương tự chưa.
3. **Thẩm định Snapshot:** Gọi gRPC sang `catalog-service` kiểm tra giá, nhãn biến thể và `version`. Nếu lệch, ném `409 PRODUCT_SNAPSHOT_MISMATCH`.
4. **Local TX #1:** Ghi nguyên tử đơn hàng ở trạng thái `PENDING_STOCK` và bản ghi idempotency `IN_PROGRESS`.
5. **Giữ kho đồng bộ:** Gọi gRPC sang `inventory-service` (`ReserveStock`). Nếu thiếu hàng, hủy đơn `FAILED_TERMINAL` và ném `409 INVENTORY_INSUFFICIENT`.
6. **Rẽ nhánh thanh toán:**
   - Nếu là **`COD`**: Chốt đơn `CONFIRMED` + ghi Outbox `OrderConfirmed`, trả ngay `201 Created`.
   - Nếu là **`MOMO`**: Gọi gRPC `payment-service` (`CreatePaymentSession`) lấy link `payUrl`, cập nhật `PENDING_PAYMENT`, trả ngay `201 Created` kèm `payUrl`.

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
  *Tập hợp các method transaction ngắn (`@Transactional`), độc lập, tuyệt đối không chứa network call: `beginOrderTx1`, `confirmCodOrder`, `markPendingPayment`, `markCancelledOnSagaFailure`.*
- [`services/order-service/.../IdempotencyService.java`](../services/order-service/src/main/java/com/ecommerce/order/application/IdempotencyService.java)  
  *Tính mã băm `SHA-256` của payload và kiểm tra trạng thái `IN_PROGRESS`, `COMPLETED`, `FAILED_TERMINAL`.*
- [`services/order-service/.../V1__create_orders_idempotency_and_outbox.sql`](../services/order-service/src/main/resources/db/migration/V1__create_orders_idempotency_and_outbox.sql)  
  *Schema database của `order_db` với các bảng `orders`, `order_items`, `idempotency_records`, `outbox_events`.*

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
  *Tiêu thụ sự kiện MoMo từ RabbitMQ: chuyển đơn sang `CONFIRMED` (+ Outbox `OrderConfirmed`) hoặc `CANCELLED` (+ Outbox `ReservationReleaseRequested`). Nếu thanh toán đến muộn khi đơn đã hủy -> chuyển `REFUND_REQUIRED`.*
- [`services/inventory-service/internal/events/consumer.go`](../services/inventory-service/internal/events/consumer.go)  
  *Lắng nghe RabbitMQ: nhận `OrderConfirmed` để trừ kho vật lý (`quantity - X`), hoặc nhận `ReservationReleaseRequested` để hoàn trả tồn kho.*
- [`services/cart-service/internal/events/consumer.go`](../services/cart-service/internal/events/consumer.go)  
  *Lắng nghe `OrderConfirmed` để xóa các sản phẩm đã mua khỏi giỏ hàng.*

---

### Bước 6: Xử lý hết hạn & Đối soát tự động (Scheduler)
- [`services/order-service/.../OrderReconciliationScheduler.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OrderReconciliationScheduler.java)  
  *Quét các đơn bị kẹt `PENDING_STOCK` quá 2 phút (do lỗi mạng) để tự động hủy và nhả kho; quét các đơn MoMo `PENDING_PAYMENT` quá hạn 15 phút chưa thanh toán để tự động hết hạn và nhả kho.*
- [`services/order-service/.../OutboxRelayScheduler.java`](../services/order-service/src/main/java/com/ecommerce/order/application/OutboxRelayScheduler.java)  
  *Đẩy sự kiện Outbox từ `order_db` lên RabbitMQ.*

---

### Bước 7: Trải nghiệm Giao diện Người dùng (Frontend)
- [`ecommerce-fe/src/pages/order-preview-page.tsx`](../ecommerce-fe/src/pages/order-preview-page.tsx)  
  *Trang xem trước và đặt hàng: sinh `Idempotency-Key`, gọi API tạo đơn, điều hướng sang `payUrl` MoMo hoặc trang chi tiết đơn.*
- [`ecommerce-fe/src/pages/payment-result-page.tsx`](../ecommerce-fe/src/pages/payment-result-page.tsx)  
  *Trang kết quả sau khi MoMo redirect về: kiểm tra trạng thái đơn, tự động poll nhẹ nếu đơn đang đợi RabbitMQ hội tụ.*

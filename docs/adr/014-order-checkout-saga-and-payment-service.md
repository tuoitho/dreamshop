# ADR-014: Hybrid Order Checkout Saga và tách riêng `payment-service`

## Status

Accepted

## Context

Luồng đặt hàng đa biến thể SKU cần đồng thời giữ độ trễ phản hồi tức thì cho `COD`/`MOMO`, chống bán lố (oversell) dưới tải đồng thời, kiểm tra lệch bản chụp thương mại (`version`, giá/biến thể SKU) giữa bước xem trước và bước đặt hàng, xử lý cổng thanh toán ngoài MoMo v2 (`captureWallet` HMAC-SHA256) không giam kết nối database, và đồng bộ bất đồng bộ qua RabbitMQ mà vẫn tuân thủ ranh giới mỗi microservice một miền nghiệp vụ và một database riêng (`ADR-001`, `ADR-002`, `ADR-003`, `ADR-005`).

## Decision

1. **Tách riêng `payment-service` (Go 1.24, HTTP `:8086`, private gRPC `127.0.0.1:50056`, sở hữu độc quyền `payment_db`):**
   - Tách toàn bộ vòng đời phiên thanh toán MoMo, chữ ký `HmacSHA256` hằng thời gian (`hmac.Equal`), khóa bí mật `MOMO_*`, và đối soát callback (`momo-redirect`, `momo-ipn`) ra khỏi `order-service`.
   - `CreatePaymentSession` tách làm 2 giao dịch ngắn (`TX #A` ghi `INITIATING` bằng `INSERT ... ON CONFLICT (order_id) DO NOTHING` rồi commit trước khi gọi HTTPS MoMo với timeout `3s` connect / `10s` read; `TX #B` khóa `SELECT ... FOR UPDATE` kiểm tra `status == 'INITIATING'` mới lưu `PENDING` + `pay_url`), kết hợp cơ chế Tombstone `CANCELLED` chống race hủy sớm.
   - Luồng callback công khai đi qua `Kong -> bff-service -> payment-service (:50056)`, xác thực HMAC trước mọi truy vấn DB và phát `PaymentSucceeded` / `PaymentFailed` qua `payment_db.outbox_events` (`wakeupCh` + quét `1s`).
2. **Điều phối Hybrid Saga tại `order-service` (Java 21, HTTP `:8083`, mgmt `:8093`, private gRPC `127.0.0.1:50055`, `order_db`):**
   - Trước `Local TX #1`: kiểm tra RBAC `order.create` qua `identity-service` (`500ms`) và đối chiếu bản chụp SKU + `version` sản phẩm qua `catalog-service` (`2s`), trả `409 PRODUCT_SNAPSHOT_MISMATCH` mà không ghi rác vào DB.
   - `Local TX #1` chèn nguyên tử `idempotency_records(IN_PROGRESS)` (`ON CONFLICT DO NOTHING`) cùng `orders(PENDING_STOCK)` và `order_items`.
   - Gọi gRPC nội bộ giữ chỗ nguyên tử theo thứ tự khóa `ORDER BY sku_id ASC` tại `inventory-service` (`ReserveStock`, `1.5s`, hỗ trợ Tombstone `RELEASED`), và nếu chọn `MOMO` thì gọi `PaymentService.CreatePaymentSession` (`12s`). Mọi bước chuyển trạng thái trên `orders` đều khóa `SELECT ... FOR UPDATE` kèm `@Version`, không giữ DB transaction mở qua mạng, và kích hoạt `OutboxRelayScheduler` ngay sau commit (`afterCommit`).
   - Tập trung bộ quét đối soát quá hạn (`PENDING_STOCK > 2m` và `PENDING_PAYMENT` hết hạn `15m`) tại `OrderReconciliationScheduler`, phát `ReservationReleaseRequested` / `PaymentCancelRequested`, và hội tụ thanh toán muộn sau hủy về `CANCELLED` + `REFUND_REQUIRED` theo `ADR-005` (kèm phát `RefundRequested`).

## Consequences

- Được: UX đặt hàng `201 Created` trả ngay kết quả hoặc `payUrl`, chống oversell ở tầng PostgreSQL row lock mà không cần khóa phân tán Redis, cô lập hoàn toàn khóa bí mật và I/O MoMo trong `payment-service`, không rò rỉ tồn kho hay mở lại đơn đã hủy khi mạng chập chờn.
- Mất: Vận hành thêm một Go microservice (`payment-service` + `payment_db`) và hai chiều Transactional Outbox giữa `order-service` và `payment-service`.
- Trade-off: Luồng đặt hàng `MOMO` phụ thuộc đồng bộ vào bước khởi tạo phiên thanh toán (giới hạn cứng `12s`, không mở DB TX trong lúc chờ); các bước chốt kho thực sự, nhả kho và dọn giỏ hàng tại `cart-service` hội tụ bất đồng bộ qua RabbitMQ.
→ Không được nhúng khóa `MOMO_*`/logic cổng thanh toán vào `order-service`, không mở DB transaction qua lệnh gọi gRPC/HTTPS/RabbitMQ, và không truy cập chéo `order_db`/`payment_db`/`inventory_db` nếu không có ADR mới thay thế.

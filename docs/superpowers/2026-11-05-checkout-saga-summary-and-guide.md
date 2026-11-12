# Tổng hợp Kiến trúc Đặt hàng & Thanh toán (Order Checkout Saga) và Hướng dẫn Vận hành / Mô phỏng

**Ngày cập nhật:** 2026-11-05  
**Phạm vi:** `contracts`, `identity-service`, `catalog-service`, `inventory-service`, `cart-service`, `payment-service`, `order-service`, `bff-service`, `ecommerce-fe`, `deployments`, `scripts/concurrency-sim`

> **Cập nhật sau tài liệu này:** bản Hybrid đã được hợp nhất vào `main` và mở rộng vòng đời giao hàng theo [ADR-015](../adr/015-order-fulfillment-lifecycle.md): COD chờ `PENDING_CONFIRMATION` rồi tự xác nhận sau 1 phút, thêm `PREPARING` / `SHIPPING` / `DELIVERED` / `DELIVERY_FAILED`, hủy sau xác nhận hoàn kho qua `StockRestockRequested`, giỏ hàng dọn khi nhận `OrderPlaced`, và trang quản trị đơn hàng trong `admin-fe`. Các bảng so sánh bên dưới phản ánh thời điểm 2026-11-05.

---

## 1. Tổng quan Hai Biến thể Kiến trúc Đã Triển khai

Hệ thống đặt hàng và thanh toán (`COD` & `MoMo Wallet`) được triển khai đầy đủ trên hai nhánh Git độc lập để phục vụ vận hành thực tế và đối chiếu thực nghiệm dưới tải cao:

| Thuộc tính | Nhánh 1: Hybrid Saga (`feat/checkout-hybrid`) | Nhánh 2: Pure Event-Driven Saga (`feat/checkout-event-driven`) |
| --- | --- | --- |
| **Thư mục Worktree** | `.worktrees/checkout-hybrid` | `.worktrees/checkout-event-driven` |
| **ADR & Tài liệu thiết kế** | [ADR-014](../adr/014-order-checkout-saga-and-payment-service.md), [Spec Hybrid](specs/2026-10-28-order-checkout-saga-design.md) | [Spec Event-Driven](specs/2026-11-03-event-driven-order-checkout-saga-design.md) (ADR của nhánh này không có trên `main`) |
| **Phản hồi `POST /api/v1/orders`** | `201 Created` — trả kết quả chốt đơn hoặc `payUrl` ngay trong 1 vòng HTTP; trả `409 Conflict` trực tiếp nếu hết hàng hoặc lệch giá | `202 Accepted` — ghi nhận đơn ở trạng thái `PENDING_VALIDATION` + ghi Transactional Outbox rồi phản hồi ngay |
| **Kiểm tra Snapshot Sản phẩm (`Catalog`)** | Đồng bộ trong luồng đặt hàng tại `order-service` qua gRPC `CatalogService.GetProductsBySkus` | Bất đồng bộ tại worker `OrderValidationConsumer` của `order-service` sau khi đơn đã được tiếp nhận |
| **Giữ chỗ Tồn kho (`Inventory`)** | Đồng bộ qua private gRPC `InventoryService.ReserveStock` | Bất đồng bộ qua RabbitMQ (`order.stock.reserve_requested` → `inventory.stock.reserved` / `inventory.stock.rejected`) kèm Transactional Outbox tại `inventory-service` |
| **Khởi tạo Phiên MoMo (`Payment`)** | Đồng bộ qua private gRPC `PaymentService.CreatePaymentSession` | Bất đồng bộ qua RabbitMQ (`order.payment.session_requested` → `payment.session.created` / `payment.session.failed`) kèm 2-phase session persistence chống gọi trùng cổng MoMo |
| **Xử lý MoMo Redirect & IPN** | BFF → gRPC `PaymentService.HandleMomoCallback` (xác thực HMAC-SHA256 hằng số thời gian) → Outbox `payment.succeeded` / `payment.failed` | Giống bản Hybrid: BFF → gRPC `PaymentService.HandleMomoCallback` → Outbox `payment.succeeded` / `payment.failed` |
| **Máy trạng thái `OrderStatus`** | `PENDING_PAYMENT`, `CONFIRMED`, `CANCELLED` | `PENDING_VALIDATION`, `PENDING_STOCK`, `PENDING_PAYMENT_SESSION`, `PENDING_PAYMENT`, `CONFIRMED`, `CANCELLED` (hỗ trợ lọc nhóm `status=PROCESSING`) |
| **Xử lý Thanh toán Trễ (Late MoMo IPN)** | Đơn đã `CANCELLED` nhận `payment.succeeded` → giữ `CANCELLED`, chuyển `paymentStatus = REFUND_REQUIRED` và phát `order.payment.refund_requested` | Tương đương bản Hybrid (`CANCELLED` + `REFUND_REQUIRED` + phát `order.payment.refund_requested`) |
| **Trải nghiệm Storefront (`ecommerce-fe`)** | Chuyển trang kết quả hoặc redirect sang `payUrl` tức thì; hiển thị lỗi `409` ngay tại trang xác nhận đơn | Lưu phiên chờ vào `sessionStorage` theo `userId`, chạy vòng Polling hội tụ, hiển thị lỗi chi tiết từng SKU qua `rejectionDetails` |
| **Đặc trưng Hiệu năng dưới Đỉnh tải** | Độ trễ HTTP phụ thuộc tổng thời gian gRPC `Catalog` + `Inventory` + cổng `MoMo`; trải nghiệm người dùng đơn giản nhất ở tải thường | Độ trễ tiếp nhận `202_P50/P95` rất thấp và ổn định khi Flash Sale vì tách toàn bộ I/O giữ kho & cổng MoMo sang hàng đợi; đánh đổi bằng thời gian hội tụ `SETTLE_P50/P95` |

---

## 2. Kết quả Kiểm chứng Tự động & Đánh giá Độc lập

Cả hai nhánh đều đã vượt qua toàn bộ các cổng kiểm tra chất lượng (`make verify`, `check-boundaries`, `buf lint`, `buf breaking`) và hai vòng đánh giá độc lập (`code-reviewer` & `boundary-reviewer`):

| Hạng mục Kiểm chứng | `feat/checkout-hybrid` | `feat/checkout-event-driven` | Ghi chú Kỹ thuật |
| --- | --- | --- | --- |
| `go run ./scripts/check-boundaries` | `PASS` (exit `0`) | `PASS` (exit `0`) | Không có import chéo giữa các service, không truy cập chéo DB, không gọi nội bộ qua Kong |
| `contracts` (`buf lint`, `buf breaking`, `mvn test-compile`) | `PASS` (exit `0`) | `PASS` (exit `0`) | Tương thích ngược hoàn toàn với nhánh `main` |
| `identity-service` (`verify-service`) | `PASS` | `PASS` | Bổ sung quyền `order:create`, `order:read_own`, `order:cancel_own` (Flyway `V10`) |
| `catalog-service` (`verify-service`) | `PASS` | `PASS` | Bổ sung `version` chống race condition cập nhật giá/biến thể (migration `000006`) |
| `inventory-service` (`verify-service`) | `PASS` | `PASS` | Giữ kho theo thứ tự `sku` tăng dần chống deadlock (`SELECT ... FOR UPDATE`), giải phóng/chốt kho idempotent |
| `cart-service` (`verify-service`) | `PASS` | `PASS` | Lắng nghe `order.confirmed` để dọn các dòng SKU đã đặt theo cơ chế idempotent |
| `payment-service` (`verify-service`) | `PASS` | `PASS` | Dịch vụ Go mới (`:8086` / gRPC `:50056`), xác thực HMAC-SHA256 MoMo v2, Outbox Relay & Consumer |
| `order-service` (`verify-service` + Testcontainers) | `PASS` | `PASS` | Idempotency (`SHA-256` request hash), Saga Coordinator, Outbox Publisher, Sweeper quá hạn 15 phút |
| `bff-service` (`verify-service`) | `PASS` | `PASS` | Adapter HTTP ↔ gRPC cho `/api/v1/orders*` và `/api/v1/payments/momo-*` (giới hạn IPN body `64 KiB`) |
| `ecommerce-fe` (`test`, `lint`, `build`) | `PASS` | `PASS` | Luồng Checkout, chọn địa chỉ, phương thức `COD`/`MOMO`, trang kết quả thanh toán và lịch sử đơn hàng |
| `scripts/...` (`concurrency-sim` unit suite) | `PASS` | `PASS` | Kiểm chứng 4 kịch bản tranh chấp đa luồng (Flash Sale, đổi giá đồng thời, bão Idempotency, đua MoMo IPN/Hủy đơn) |
| Cấu hình Docker Compose (`foundation` & `production`) | `PASS` | `PASS` | Đầy đủ route Kong, `payment_db` và biến môi trường dịch vụ |
| Đánh giá độc lập (`code-reviewer` & `boundary-reviewer`) | `APPROVE` / `PASS` | `APPROVE` / `PASS` | Không còn lỗi tồn đọng về logic nghiệp vụ, bảo mật hay ranh giới microservice |

---

## 3. Sơ đồ Cổng Dịch vụ Local (Canonical Port Topology)

| Dịch vụ / Hạ tầng | Cổng HTTP Local | Cổng gRPC Nội bộ | Endpoint Kiểm tra Sức khỏe (Health Check) |
| --- | --- | --- | --- |
| `identity-service` | `8080` (mgmt `8090`) | `127.0.0.1:50054` | `http://localhost:8090/actuator/health/readiness` |
| `bff-service` | `8081` | - | `http://localhost:8081/healthz` |
| `inventory-service` | `8082` | `127.0.0.1:50052` | `http://localhost:8082/healthz` |
| `order-service` | `8083` (mgmt `8093`) | `127.0.0.1:50055` | `http://localhost:8093/actuator/health/readiness` |
| `cart-service` | `8084` | `127.0.0.1:50053` | `http://localhost:8084/healthz` |
| `catalog-service` | `8085` | `127.0.0.1:50051` | `http://localhost:8085/healthz` |
| `payment-service` | `8086` | `127.0.0.1:50056` | `http://localhost:8086/healthz` |
| `ecommerce-fe` | `5173` | - | `http://localhost:5173` |
| `admin-fe` | `5174` | - | `http://localhost:5174` |
| `Kong Gateway` | `8000` (admin `8001`) | - | `http://localhost:8001/status` |
| `PostgreSQL` | `5432` | - | Container `ecommerce-postgres` |
| `RabbitMQ` | `5672` (UI `15672`) | - | `http://localhost:15672` (`ecommerce` / `change-me-local-only`) |
| `MinIO` | `9000` (UI `9001`) | - | `http://localhost:9000/minio/health/live` |

---

## 4. Quy trình Reset Database & RabbitMQ Khi Đổi Nhánh Demo

Khi dùng chung một bộ Docker volume local (`postgres_data`, `rabbitmq_data`) và chuyển đổi qua lại giữa `.worktrees/checkout-hybrid` và `.worktrees/checkout-event-driven`, có **5 điểm khác biệt về schema/queue** cần làm sạch trước khi bật service của nhánh còn lại:

1. **`order_db`:** Bản Event-Driven có thêm cột `rejection_details` trong `V1__create_orders_idempotency_and_outbox.sql` → khác Flyway checksum so với bản Hybrid.
2. **`payment_db`:** Bản Event-Driven có thêm cột quản lý 2-phase session trong migration `000001_create_payments_and_outbox.up.sql`.
3. **`inventory_db` (migration `000003`):** Bản Hybrid dùng `000003_create_stock_reservations`, bản Event-Driven dùng `000003_create_stock_reservations_and_outbox` (có thêm bảng `outbox_events`).
4. **`cart_db` (migration `000004`):** Bảng `processed_events` khác cột `event_type` giữa hai nhánh.
5. **RabbitMQ Saga Queues:** Các queue cùng tên (như `payment.order-events`, `inventory.order-events`) có khác biệt về `x-dead-letter-routing-key` hoặc routing key binding giữa hai nhánh.

> **Dữ liệu được giữ nguyên 100%:** Toàn bộ `identity_db` (tài khoản, phân quyền RBAC, sổ địa chỉ), `catalog_db` (danh mục sản phẩm, biến thể, ảnh MinIO), giỏ hàng & danh sách yêu thích (`carts`, `cart_items`, `favorites` trong `cart_db`), và số lượng tồn kho gốc (`stock` trong `inventory_db`, chỉ đưa `reserved` về `0`) đều **không bị mất**.

### Khối lệnh PowerShell Reset Nhanh Khi Đổi Nhánh

Trước tiên hãy **dừng các tiến trình ứng dụng đang chạy** (`order-service`, `payment-service`, `inventory-service`, `cart-service`), giữ nguyên Docker hạ tầng đang bật, sau đó chạy khối lệnh PowerShell sau:

```powershell
# 1. Ngắt kết nối cũ và tạo mới sạch order_db & payment_db
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c @"
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname IN ('order_db', 'payment_db') AND pid <> pg_backend_pid();
"@
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS order_db;"
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE order_db;"
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS payment_db;"
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE payment_db;"

# 2. Reset riêng migration 000003 của inventory_db (giữ nguyên bảng tồn kho stock, đưa reserved = 0)
docker exec -i ecommerce-postgres psql -U ecommerce -d inventory_db -v ON_ERROR_STOP=1 -c @"
DROP TABLE IF EXISTS outbox_events, stock_reservations, processed_events CASCADE;
DO `$`$
BEGIN
    IF to_regclass('public.stock') IS NOT NULL THEN
        UPDATE stock SET reserved = 0;
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        -- 000004 (ADR-015) alters the stock_reservations CHECK, so it must re-run with 000003.
        DELETE FROM schema_migrations WHERE version LIKE '000003_%' OR version LIKE '000004_%';
    END IF;
END `$`$;
"@

# 3. Reset riêng migration 000004 của cart_db (giữ nguyên carts, cart_items, favorites)
docker exec -i ecommerce-postgres psql -U ecommerce -d cart_db -v ON_ERROR_STOP=1 -c @"
DROP TABLE IF EXISTS processed_events CASCADE;
DO `$`$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        DELETE FROM schema_migrations WHERE version LIKE '000004_%';
    END IF;
END `$`$;
"@

# 4. Xóa các queue Saga trên RabbitMQ để nhánh mới tự khai báo lại queue & binding chuẩn
$sagaQueues = @(
    "inventory.order-events", "inventory.order-events.dlq",
    "cart.order-events", "cart.order-events.dlq",
    "payment.order-events", "payment.order-events.dlq",
    "payment.session-events", "payment.session-events.dlq",
    "order.payment-events", "order.payment-events.dlq",
    "order.validation-events", "order.validation-events.dlq",
    "order.saga-events", "order.saga-events.dlq"
)
foreach ($q in $sagaQueues) {
    docker exec ecommerce-rabbitmq rabbitmqctl delete_queue $q 2>$null | Out-Null
}
Write-Host "Reset DB & RabbitMQ Saga state hoàn tất!" -ForegroundColor Green
```

---

## 5. Hướng dẫn Khởi chạy Full-Stack Local

Đứng tại thư mục của nhánh muốn chạy (`E:\microservice\.worktrees\checkout-hybrid` hoặc `E:\microservice\.worktrees\checkout-event-driven`):

### Bước 1: Chuẩn bị khóa JWT và bật hạ tầng Docker

```powershell
# Tạo cặp khóa RSA local cho identity-service (chỉ cần chạy lần đầu trên mỗi worktree)
go run ./scripts/generate-identity-dev-keys

# Khởi động PostgreSQL, Redis, RabbitMQ, MinIO và cập nhật cấu hình Kong Gateway
make infra-up

# Nếu volume PostgreSQL được tạo từ trước khi có payment_db, đảm bảo payment_db đã tồn tại:
docker exec -i ecommerce-postgres psql -U ecommerce -d postgres -c "SELECT 'CREATE DATABASE payment_db' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'payment_db')\gexec"
```

### Bước 2: Khởi động các Service theo đúng thứ tự phụ thuộc

Mở các cửa sổ terminal riêng tại cùng thư mục worktree và bật lần lượt từ tầng Provider đến Consumer:

1. **Tầng Định danh & Phân quyền (bắt buộc chạy đầu tiên và đợi cổng `:8090` báo `UP`):**
   ```powershell
   make identity
   ```
2. **Tầng Dịch vụ Miền & Thanh toán (có thể bật song song sau khi `identity-service` sẵn sàng):**
   ```powershell
   make catalog
   make inventory
   make cart
   make payment
   make order
   ```
   > **Lưu ý về MoMo Local:** Lệnh `make payment` trên cả hai nhánh đã cấu hình sẵn chế độ giả lập an toàn cho môi trường local (`PAYMENT_MOMO_MOCK_ENABLED=true`, `MOMO_PARTNER_CODE=MOMO_LOCAL`, `MOMO_ACCESS_KEY=local-access-key`, `MOMO_SECRET_KEY=local-secret-key-change-me`). Nếu muốn gọi trực tiếp MoMo Sandbox thật trên Internet, đặt `PAYMENT_MOMO_MOCK_ENABLED=false` cùng bộ khóa Sandbox trong file `.env`.
3. **Tầng BFF & Giao diện Người dùng:**
   ```powershell
   make bff
   make fe
   # Tuỳ chọn nếu cần vào trang quản trị sản phẩm / tồn kho:
   make admin-fe
   ```

### Bước 3: Kiểm tra nhanh trạng thái sẵn sàng của toàn bộ Stack

```powershell
@(
    "http://localhost:8090/actuator/health/readiness",
    "http://localhost:8085/healthz",
    "http://localhost:8082/healthz",
    "http://localhost:8084/healthz",
    "http://localhost:8086/healthz",
    "http://localhost:8093/actuator/health/readiness",
    "http://localhost:8081/healthz"
) | ForEach-Object {
    try {
        $resp = Invoke-RestMethod -Uri $_ -TimeoutSec 3
        Write-Host "[OK] $_" -ForegroundColor Green
    } catch {
        Write-Host "[FAIL] $_ -> $($_.Exception.Message)" -ForegroundColor Red
    }
}
```

---

## 6. Hướng dẫn Chạy Mô phỏng Đa luồng (`scripts/run-concurrency-sim.ps1`)

Script thực thi `scripts/run-concurrency-sim.ps1` trên cả hai nhánh đã được cấu hình sẵn thông tin tài khoản local mặc định, không cần truyền thủ công bất kỳ tham số đăng nhập nào:

- **Tài khoản Quản trị (`SUPER_ADMIN`):** `admin@example.com` / `AdminPassword123!` (được `identity-service` tự động khởi tạo khi bật local).
- **Tài khoản Khách hàng (`CUSTOMER`):** `customer1@gmail.com` / `123456123456` (script tự động gọi `POST /api/v1/auth/register` theo cơ chế idempotent nếu tài khoản chưa có trong `identity_db`).
- **Thông tin chữ ký HMAC MoMo:** Tự động khớp với cấu hình mặc định của `make payment` (`MOMO_LOCAL` / `local-access-key` / `local-secret-key-change-me`) hoặc ưu tiên đọc từ `.env` / biến môi trường nếu được khai báo.

### Các lệnh thực thi

```powershell
# 1. Chạy toàn bộ 4 kịch bản tranh chấp + đối soát bất biến SQL trực tiếp trên PostgreSQL
pwsh ./scripts/run-concurrency-sim.ps1 -VerifySql

# (Hoặc nếu dùng Windows PowerShell 5.1 mặc định của Windows):
powershell -ExecutionPolicy Bypass -File ./scripts/run-concurrency-sim.ps1 -VerifySql

# 2. Chạy riêng lẻ từng kịch bản mô phỏng
pwsh ./scripts/run-concurrency-sim.ps1 -Scenario flash-sale-oversell -VerifySql
pwsh ./scripts/run-concurrency-sim.ps1 -Scenario snapshot-mismatch-race -VerifySql
pwsh ./scripts/run-concurrency-sim.ps1 -Scenario idempotency-storm -VerifySql
pwsh ./scripts/run-concurrency-sim.ps1 -Scenario momo-callback-race -VerifySql

# 3. Xuất báo cáo định dạng JSON để lưu trữ / vẽ biểu đồ so sánh giữa 2 nhánh
pwsh ./scripts/run-concurrency-sim.ps1 -VerifySql -Json > benchmark-result.json
```

### Ý nghĩa 4 kịch bản & các chỉ số đo lường

1. **`flash-sale-oversell` (50 luồng mua đồng thời tranh 5 sản phẩm tồn kho):**
   - Kiểm chứng: Đúng `5` đơn thành công (`CONFIRMED`), `45` đơn bị từ chối sạch sẽ (`OUT_OF_STOCK` / `INVENTORY_INSUFFICIENT`), tồn kho khả dụng về `0` và không bao giờ âm.
2. **`snapshot-mismatch-race` (20 luồng đặt hàng đồng thời với thao tác Admin tăng giá sản phẩm):**
   - Kiểm chứng: Không có đơn hàng nào bị tính sai giá so với `productVersion` mà khách đã xác nhận; các request dùng snapshot cũ bị chặn bởi `PRODUCT_SNAPSHOT_MISMATCH`.
3. **`idempotency-storm` (20 luồng gửi trùng `Idempotency-Key` cùng payload + 5 luồng gửi trùng key nhưng đổi số lượng):**
   - Kiểm chứng: Chỉ tạo duy nhất `1` đơn hàng và `1` phiên thanh toán trong DB; `5` luồng sửa payload bị từ chối với mã `IDEMPOTENCY_KEY_REUSED`.
4. **`momo-callback-race` (10 Redirect hợp lệ + 10 IPN hợp lệ + 5 IPN giả mạo chữ ký + 5 yêu cầu hủy đơn chạy đồng thời):**
   - Kiểm chứng: Cả `5/5` callback giả mạo bị chặn (`INVALID_MOMO_SIGNATURE` - HTTP `400`); trạng thái cuối cùng hội tụ nhất quán về `(CONFIRMED, PAID)` hoặc `(CANCELLED, REFUND_REQUIRED)` nếu hủy đơn thắng trước khi tiền về, không bao giờ rơi vào trạng thái mâu thuẫn `(CANCELLED, PAID)` hay `(CONFIRMED, UNPAID)`.
5. **Đối chiếu độ trễ giữa hai kiến trúc:**
   - Trên **`feat/checkout-hybrid`**: Bảng kết quả hiển thị `P50` và `P95` của toàn bộ vòng đời HTTP `POST /api/v1/orders`.
   - Trên **`feat/checkout-event-driven`**: Bảng kết quả tách rõ `202_P50` / `202_P95` (độ trễ tiếp nhận đơn vào hàng đợi) và `SETTLE_P50` / `SETTLE_P95` (tổng thời gian từ lúc gửi đơn đến khi Saga hội tụ xong qua RabbitMQ).
   - Cờ **`-VerifySql`**: Kiểm tra trực tiếp trên `ecommerce-postgres` đảm bảo không có dòng `stock` nào có `quantity < 0` hoặc `reserved < 0`, không có `outbox_events` nào bị kẹt chưa phát, và không tồn tại đơn hàng vi phạm bất biến trạng thái thanh toán.

---

## 7. Hướng dẫn Chọn Một Nhánh Để Merge Vào `main`

Vì cả hai nhánh cùng cài đặt chung luồng Đặt hàng nhưng khác nhau về hợp đồng HTTP/gRPC/Event nội bộ, **khi hợp nhất vào `main` chỉ chọn một trong hai nhánh làm kiến trúc chính thức**:

- **Chọn `feat/checkout-hybrid`** nếu ưu tiên độ phức tạp vận hành thấp, nhận ngay kết quả đặt hàng / `payUrl` trong một request HTTP duy nhất và dễ debug theo luồng đồng bộ.
- **Chọn `feat/checkout-event-driven`** nếu ưu tiên khả năng hấp thụ đỉnh tải đột biến (Flash Sale), cách ly hoàn toàn luồng nhận đơn khỏi độ trễ của dịch vụ tồn kho và cổng thanh toán bên thứ ba.

### Các bước thực hiện Merge và Lưu trữ Nhánh Đối chiếu

```powershell
# Đứng tại thư mục gốc E:\microservice (nhánh main)
Set-Location E:\microservice
git checkout main

# Giữ lại tag đối chiếu cho cả 2 kiến trúc trước khi merge
git tag archive/checkout-hybrid feat/checkout-hybrid
git tag archive/checkout-event-driven feat/checkout-event-driven

# Chọn 1 trong 2 lệnh dưới đây tùy kiến trúc chính thức bạn quyết định đưa vào main:
# Phương án A — Chọn Hybrid Saga:
git merge --no-ff feat/checkout-hybrid -m "feat(checkout): merge hybrid order checkout saga and payment-service"

# Phương án B — Chọn Pure Event-Driven Saga:
# git merge --no-ff feat/checkout-event-driven -m "feat(checkout): merge event-driven order checkout saga and payment-service"

# Kiểm tra lại ranh giới kiến trúc trên main sau khi merge
go run ./scripts/check-boundaries
```

Sau khi đã merge nhánh chính thức và không còn nhu cầu chạy song song hai thư mục worktree, có thể gỡ worktree bằng:

```powershell
git worktree remove .worktrees/checkout-hybrid
git worktree remove .worktrees/checkout-event-driven
```
*(Hai tag `archive/checkout-hybrid` và `archive/checkout-event-driven` vẫn lưu giữ toàn bộ lịch sử code để có thể checkout lại bất kỳ lúc nào).*

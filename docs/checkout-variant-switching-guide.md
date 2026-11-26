# Chuyển đổi giữa hai biến thể Checkout (Hybrid ↔ Event-Driven)

Hai cách xử lý order nằm ở hai thư mục (git worktree) riêng, dùng chung hạ tầng
Docker (PostgreSQL, RabbitMQ, ...) và chung port local. Tại một thời điểm chỉ
chạy được **một** biến thể.

| Biến thể | Nhánh | Thư mục gốc |
| --- | --- | --- |
| Hybrid | `main` (đã merge `feat/checkout-hybrid`) | `E:\microservice` |
| Event-Driven | `feat/checkout-event-driven` | `E:\microservice\.worktrees\checkout-event-driven` |

"Chuyển nhánh" nghĩa là chạy service từ thư mục kia, không cần `git checkout`.

## Quy trình chuyển (PowerShell)

```powershell
# 0. Tắt TẤT CẢ service đang chạy
#    - bật bằng script:  .\scripts\start-all.ps1 -Stop   (chạy trong thư mục đã bật)
#    - bật bằng make:    Ctrl+C từng terminal

# 1. Vào thư mục gốc của biến thể muốn test
cd E:\microservice                                     # Hybrid
# cd E:\microservice\.worktrees\checkout-event-driven  # Event-Driven

# 2. Reset state checkout (Docker postgres + rabbitmq phải đang chạy)
.\scripts\reset-checkout-state.ps1

# 3. Cài lại Java contracts của biến thể này vào ~/.m2
mvn -q -f contracts/java/pom.xml install -DskipTests

# 4. Bật toàn bộ service theo thứ tự, chờ từng health check
.\scripts\start-all.ps1
```

Bước 3 cần thiết vì hai biến thể cài cùng artifact `com.ecommerce:contracts:0.0.1-SNAPSHOT`
vào chung `~/.m2`. Bản cài sau cùng thắng; nếu là của biến thể kia, order-service
không biên dịch được (`cannot find symbol ReserveStockRequest` /
`CreatePaymentSessionRequest` khi chạy Hybrid).

Thứ tự là bắt buộc:

- **Bước 0 trước bước 2:** service còn chạy sẽ kết nối lại vào database đang bị
  xóa/tạo lại và làm hỏng state.
- **Bước 0 trước bước 4:** `start-all.ps1` bỏ qua service đã trả lời health check
  (`already UP`). Nếu service của biến thể cũ còn chạy, bạn sẽ test nhầm code cũ.

## Vì sao phải reset

Hai biến thể có schema và topology RabbitMQ khác nhau:

- `order_db`: migration `V1` khác nội dung → Flyway báo
  `Migration checksum mismatch for migration version 1` và order-service không khởi động.
- `inventory_db`: migration `000003` khác nhau (Event-Driven có thêm bảng outbox).
- `cart_db`: migration `000004` (`processed_events`) chỉ có ở một biến thể.
- RabbitMQ: Event-Driven khai báo thêm queue/binding (`order.validation-events`,
  `payment.session-events`, ...).

`reset-checkout-state.ps1` xóa và tạo lại `order_db`, `payment_db`; xóa bảng saga
trong `inventory_db`, `cart_db` cùng dòng migration tương ứng; đưa `stock.reserved`
về `0`; xóa các queue saga. Service tự tạo lại schema và queue khi khởi động.

**Giữ nguyên:** `identity_db` (tài khoản, địa chỉ), `catalog_db` (sản phẩm),
dữ liệu giỏ hàng trong `cart_db`, số lượng tồn kho trong bảng `stock`.
**Mất:** toàn bộ đơn hàng và phiên thanh toán.

## `start-all.ps1`

| Lệnh | Tác dụng |
| --- | --- |
| `.\scripts\start-all.ps1` | Bật identity → catalog → inventory → cart → payment → order → bff |
| `.\scripts\start-all.ps1 -Stop` | Tắt các process do script này bật (đọc từ `tmp\logs\pids.txt`) |

- Mỗi service chạy ẩn bằng `go run ./scripts/start-local -service <tên>`
  (tương đương `make <tên>`), dùng code của thư mục chứa script.
- Identity bật đầu tiên vì Catalog, Inventory, Cart, Order gọi authorization gRPC
  của nó; BFF bật cuối vì gọi tất cả service còn lại.
- Log: `tmp\logs\<service>.log` và `tmp\logs\<service>.log.err`.
- Một service không lên sau 3 phút hoặc bị crash → script dừng và in đường dẫn log.
- `-Stop` không tắt được service bật bằng `make` trong terminal riêng.
- Không bật frontend; dùng `make fe` / `make admin-fe` nếu cần.

Health check dùng để chờ:

| Service | URL |
| --- | --- |
| identity-service | `http://localhost:8090/actuator/health` |
| catalog-service | `http://localhost:8085/healthz` |
| inventory-service | `http://localhost:8082/healthz` |
| cart-service | `http://localhost:8084/healthz` |
| payment-service | `http://localhost:8086/healthz` |
| order-service | `http://localhost:8093/actuator/health` |
| bff-service | `http://localhost:8081/healthz` |

## Chuẩn bị một lần cho mỗi thư mục

- Hai script `start-all.ps1` và `reset-checkout-state.ps1` phải có trong
  `scripts\` của thư mục đó.
- Khóa JWT của Identity: `deployments\secrets\identity\private.pem` và `public.pem`
  (tạo bằng `make identity-keys` nếu thiếu).
- File `.env` ở thư mục gốc được `start-local` đọc làm giá trị mặc định. Worktree
  không có `.env` thì copy từ `E:\microservice\.env` nếu cần cùng cấu hình.
- `IDENTITY_JWT_ISSUER` trong `.env` phải là `http://localhost:8080` (khớp `issuer`
  của Kong local trong `deployments/kong/kong.yaml`). Giá trị
  `http://identity-service:8080` chỉ dành cho production Compose (đã khai báo sẵn
  trong `compose.production.yml`); dùng nó khi chạy local làm mọi route có xác thực
  trả `401 Invalid or expired access token`.
- Chạy service trên **một** môi trường duy nhất (Windows hoặc WSL). Kong gọi
  `host.docker.internal`, tức Windows host; nếu hai bộ service chạy song song ở
  Windows và WSL thì request có thể rơi vào bộ không mong muốn.
- Nếu `.env` có `IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL` thì phải có cả
  `IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD`; script chạy ẩn nên không nhập mật khẩu
  tương tác được và identity-service sẽ lỗi.

## Xử lý lỗi thường gặp

| Triệu chứng | Nguyên nhân / cách xử lý |
| --- | --- |
| `<service>: FAILED` | Đọc `tmp\logs\<service>.log.err` trong thư mục đang chạy. |
| order-service lỗi Flyway checksum | Chưa reset sau khi đổi biến thể → làm lại từ bước 0. |
| order-service `BUILD FAILURE` / `cannot find symbol` trong `com.ecommerce.contracts` | `~/.m2` đang giữ contracts của biến thể kia → chạy bước 3. |
| `502 ORDER_UNAVAILABLE` trên Hybrid | Request rơi vào BFF Event-Driven (mã lỗi này chỉ có ở Event-Driven) → còn bộ service cũ chạy song song, làm lại bước 0. |
| `503 AUTHENTICATION_UNAVAILABLE` | Kong không lấy được JWKS từ identity (thường do gọi khi identity chưa lên). Kong cache kết quả lỗi 300 giây → chờ hoặc `docker restart ecommerce-kong`. |
| `401 Invalid or expired access token` với token mới | `iss` của token khác `issuer` của Kong → kiểm tra `IDENTITY_JWT_ISSUER` trong `.env`, restart identity. |
| Tất cả `already UP` ngay sau khi đổi thư mục | Service của biến thể cũ chưa tắt → làm lại bước 0. |
| Port bị chiếm nhưng không rõ process | `Get-NetTCPConnection -LocalPort <port> -State Listen` rồi `taskkill /T /F /PID <pid>`. |

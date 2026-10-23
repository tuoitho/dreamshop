# E-commerce Microservices Platform

Monorepo chứa frontend React và 6 microservice độc lập cho hệ thống thương mại
điện tử: Identity, BFF, Catalog, Inventory, Cart và Order. Hệ thống hiện hỗ trợ
luồng xác thực, hồ sơ người dùng, duyệt sản phẩm và giỏ hàng; luồng checkout và
vòng đời đơn hàng vẫn đang ở giai đoạn nền tảng.

## Prerequisites

- Java 21 LTS
- Apache Maven 3.9+
- Go 1.24+
- Docker Desktop (với Docker Compose v2)
- Buf CLI 1.70+

## Quick start

1. Sao chép cấu hình môi trường mẫu:
   ```bash
   cp .env.example .env
   ```
2. Sinh mã từ các Protobuf contract:
   ```bash
   make proto
   ```
3. Khởi chạy hạ tầng nền tảng local:
   ```bash
   make infra-up
   ```
4. Chạy service cần phát triển theo hướng dẫn local:
   ```bash
   make identity
   ```
5. Dừng hạ tầng khi hoàn tất:
   ```bash
   make infra-down
   ```

Chi tiết onboarding, local development, build/test và operations nằm trong
[`docs/instructions/README.md`](docs/instructions/README.md). Quy tắc kiến trúc
canonical nằm trong [`docs/steering/README.md`](docs/steering/README.md), còn
workflow PR nằm trong [`docs/rules/README.md`](docs/rules/README.md).

## Service map

- **identity-service** (Java/Spring Boot): đăng ký, đăng nhập, JWT/JWKS,
  refresh session, hồ sơ, danh mục địa giới Việt Nam, and private RBAC
  authorization at `127.0.0.1:50054`.
- **bff-service** (Go): REST adapter cho storefront và cart; gọi Catalog và
  Cart trực tiếp qua gRPC.
- **catalog-service** (Go): chi tiết sản phẩm, tìm kiếm/lọc/phân trang,
  soft-delete, PostgreSQL và gRPC.
- **inventory-service** (Go): tồn kho khả dụng từ quantity/reserved,
  PostgreSQL và gRPC.
- **cart-service** (Go): lưu giỏ hàng, CRUD line item và tổng hợp thông tin
  sản phẩm/tồn kho từ Catalog và Inventory qua gRPC; calls Identity directly
  for authorization.
- **order-service** (Java/Spring Boot): application skeleton và health check;
  nghiệp vụ đặt hàng chưa được triển khai.
- **ecommerce-fe** (React/TypeScript): storefront, auth/profile, tìm kiếm,
  chi tiết sản phẩm, giỏ hàng và xem trước đơn hàng.

## Kong routes

Browser chỉ gọi API qua Kong API Gateway tại cổng `8000`:

- `/api/v1/auth` → Identity; register/login/refresh/logout và JWKS.
- `/api/v1/users` và `/api/v1/locations` → Identity; được bảo vệ bởi
  `caller-context` trừ các route auth công khai.
- `/api/v1/storefront` → BFF; tìm kiếm và chi tiết sản phẩm.
- `/api/v1/cart` → BFF; giỏ hàng có xác thực.
- `/api/v1/admin` → BFF; soft-delete/restore sản phẩm trong cấu hình local.

## Local development

For the authorization dependency, roll out provider-first: start Identity,
then Cart, then BFF. Identity authorization is private gRPC at
`127.0.0.1:50054` and is not exposed through Kong.

- Chạy kiểm thử đơn vị Go:
  ```bash
  go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...
  ```
- Chạy kiểm thử đơn vị Java:
  ```bash
  mvn test
  ```

## Identity session and profile demo

Chạy infrastructure trong Docker và Identity trực tiếp trên Windows. Kong gọi Identity qua `host.docker.internal:8080`:

```bash
make identity-keys
make infra-up
make identity
```

Các public endpoint qua Kong là:

- `POST http://localhost:8000/api/v1/auth/register`
- `POST http://localhost:8000/api/v1/auth/login`
- `POST http://localhost:8000/api/v1/auth/refresh`
- `POST http://localhost:8000/api/v1/auth/logout`
- `GET http://localhost:8000/api/v1/auth/.well-known/jwks.json`
- `GET/PATCH http://localhost:8000/api/v1/users/me`

Ví dụ body giả lập cho register:

```json
{"email":"customer@example.test","password":"local-only-example-password","fullName":"Example Customer"}
```

Ví dụ body giả lập cho login:

```json
{"email":"customer@example.test","password":"local-only-example-password"}
```

Identity dùng HTTP port `8080` và private authorization gRPC `127.0.0.1:50054`
khi chạy local trực tiếp. Cart gọi RPC này với timeout `500ms`; nếu Identity
authorization unavailable, add-item trả `503` và không mutate Cart, còn bị từ
chối permission trả `403`. Access token mặc định có TTL 15 phút; refresh token opaque sống 30 ngày, chỉ được gửi trong cookie `HttpOnly`, `SameSite=Lax`, path `/api/v1/auth`. Local development đặt `IDENTITY_SESSION_COOKIE_SECURE=false`; môi trường HTTPS phải bật cờ này. Refresh token được rotate tại mỗi lần refresh. Dùng lại token đã rotate sẽ revoke toàn bộ session family, kể cả concurrent refresh không có grace interval. PEM được tạo chỉ dành cho local development, nằm ở `deployments/secrets/identity/`, và tuyệt đối không được commit hoặc tái sử dụng cho môi trường khác.

Frontend tại `http://localhost:5173` hỗ trợ đăng ký, đăng nhập, khôi phục
session, cập nhật hồ sơ, tìm kiếm/xem sản phẩm và thao tác giỏ hàng. Trang
`/gio-hang/dat-hang` chỉ xem trước các line item đã chọn; chưa tạo Order.

## Verification

Tiện ích foundation kiểm tra cấu hình hạ tầng và các điều kiện runtime cụ thể.
Build/test toàn repository được chọn theo phạm vi trong hướng dẫn developer và
được CI chạy bằng các lệnh native của từng stack.

Chạy lệnh kiểm tra:
```bash
make verify
```

## Security boundary

- Các dịch vụ nội bộ (`catalog-service`, `inventory-service`, `cart-service`,
  `order-service`) không có public Kong route; direct host port chỉ phục vụ
  local development và diagnostics.
- Kong API Gateway là cổng duy nhất cho traffic bên ngoài đi vào hệ thống.
- Các secret và mật khẩu local chỉ nằm trong tệp `.env` (không được commit lên Git).

## Current limitations

- Order chưa triển khai create-order, persistence, idempotency hoặc
  transactional outbox; frontend order preview chưa submit checkout.
- Inventory mới hỗ trợ đọc tồn khả dụng; chưa có reserve/release, reservation
  TTL, RabbitMQ consumer hoặc event publication.
- Redis và RabbitMQ được provision cho kiến trúc mục tiêu nhưng chưa được
  application code sử dụng.
- OpenAPI hiện chưa bao phủ toàn bộ REST surface; protobuf là contract đã được
  sinh mã cho các gRPC service.
- Route admin local chưa gắn plugin xác thực và không phù hợp để expose ngoài
  môi trường phát triển.
- Không sử dụng `docker compose down -v` trong luồng thông thường vì thao tác
  đó xóa dữ liệu volume local.

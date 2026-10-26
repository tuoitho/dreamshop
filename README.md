# E-commerce Microservices Platform

Monorepo chứa frontend React và 6 microservice độc lập cho hệ thống thương mại
điện tử: Identity, BFF, Catalog, Inventory, Cart và Order. Hệ thống hiện hỗ trợ
luồng xác thực, quản lý RBAC, hồ sơ & sổ địa chỉ người dùng, duyệt/quản trị sản
phẩm và giỏ hàng; luồng checkout và vòng đời đơn hàng vẫn đang ở giai đoạn nền
tảng.

## Prerequisites

- Java 21 LTS và Apache Maven 3.9+
- Go 1.24+
- Node.js 22.12+ (kèm npm)
- Docker Desktop (với Docker Compose v2+)
- Buf CLI 1.70+
- GNU Make

## Quick start

1. Sinh cặp khóa RSA local cho Identity và sinh mã từ các Protobuf contract:
   ```bash
   make identity-keys
   make proto
   ```
2. Khởi chạy hạ tầng nền tảng local (PostgreSQL, Redis, RabbitMQ, Kong):
   ```bash
   make infra-up
   ```
3. Chạy các service cần phát triển trên những terminal riêng (theo thứ tự
   provider-first: `make identity` → `make catalog`, `make inventory` →
   `make cart` → `make bff` → `make fe`):
   ```bash
   make identity
   ```
4. Dừng hạ tầng khi hoàn tất (giữ nguyên dữ liệu trong named volumes):
   ```bash
   make infra-down
   ```

Chi tiết thiết lập & chạy local nằm tại [`local-development.md`](docs/instructions/local-development.md),
kiểm thử tại [`build-and-test.md`](docs/instructions/build-and-test.md), vận hành &
triển khai container full-stack tại [`operations.md`](docs/instructions/operations.md) /
[`production-deployment.md`](docs/instructions/production-deployment.md), và quy
trình PR/CI tại [`contributing.md`](docs/instructions/contributing.md). Quy tắc
kiến trúc canonical nằm trong [`docs/steering/`](docs/steering/).

## Service map

- **identity-service** (Java/Spring Boot, HTTP `8080`, Actuator `8090`, private
  gRPC `127.0.0.1:50054`): đăng ký, đăng nhập, JWT/JWKS, refresh session, hồ sơ,
  sổ địa chỉ giao hàng, danh mục địa giới Việt Nam, quản trị user/RBAC và đánh
  giá phân quyền nội bộ qua gRPC.
- **bff-service** (Go, local HTTP `8081` / container `8080`): REST adapter cho
  storefront, cart và quản trị sản phẩm; gọi Catalog và Cart trực tiếp qua gRPC.
- **catalog-service** (Go, local HTTP `8085` / standalone `8081`, gRPC
  `127.0.0.1:50051`): chi tiết sản phẩm, tìm kiếm/lọc/phân trang,
  soft-delete/restore có kiểm tra quyền `product.delete` qua Identity gRPC,
  PostgreSQL và gRPC.
- **inventory-service** (Go, HTTP `8082`, gRPC `127.0.0.1:50052`): tồn kho khả
  dụng từ `quantity`/`reserved`, PostgreSQL và gRPC.
- **cart-service** (Go, HTTP `8084`, gRPC `127.0.0.1:50053`): lưu giỏ hàng, CRUD
  line item, tổng hợp thông tin sản phẩm/tồn kho từ Catalog & Inventory qua gRPC,
  và kiểm tra quyền `cart.item.add` qua Identity gRPC khi thêm sản phẩm.
- **order-service** (Java/Spring Boot, HTTP `8083`, Actuator `8093`):
  application skeleton và health check; nghiệp vụ đặt hàng chưa được triển khai.
- **ecommerce-fe** (React/TypeScript, dev `5173` / container Nginx `8080`):
  storefront, auth/profile, tìm kiếm, chi tiết sản phẩm (kèm thao tác ẩn/khôi
  phục cho admin), giỏ hàng và xem trước đơn hàng.

## Kong routes

Browser chỉ gọi API qua Kong API Gateway tại cổng `8000` (local) hoặc cùng
origin qua Kong (`compose.production.yml`):

- `/api/v1/auth` → Identity; public auth (`register`, `login`, `refresh`,
  `logout`) và `/.well-known/jwks.json` (strip sẵn header `X-User-*` giả mạo).
- `/api/v1/users`, `/api/v1/locations`, `/api/v1/admin/users`,
  `/api/v1/admin/rbac` → Identity; được bảo vệ bởi plugin `caller-context`.
- `/api/v1/storefront/products` → BFF; tìm kiếm và chi tiết sản phẩm công khai.
- `/api/v1/cart` → BFF; giỏ hàng có xác thực (`caller-context`).
- `/api/v1/admin/products` → BFF; soft-delete/restore sản phẩm có xác thực
  (`caller-context`) và kiểm tra quyền RBAC `product.delete`.

## Local development

Khởi động các phụ thuộc phân quyền theo thứ tự provider-first: `Identity` →
`Catalog`, `Inventory`, `Cart` → `BFF`. Identity authorization chạy gRPC riêng
tư tại `127.0.0.1:50054` và không được expose qua Kong.

- Chạy kiểm thử đơn vị Go (trên Windows PowerShell dùng `cart-service/internal/...`
  do test `cmd/server` dùng tín hiệu Unix; trên Linux/CI chạy `./services/cart-service/...`):
  ```powershell
  go test ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/internal/...
  ```
- Chạy kiểm thử đơn vị Java (yêu cầu Docker Desktop đang bật cho Testcontainers):
  ```powershell
  mvn test
  ```

## Identity session and profile demo

Chạy infrastructure trong Docker và Identity trực tiếp trên Windows. Kong gọi
Identity qua `host.docker.internal:8080`:

```bash
make identity-keys
make infra-up
make identity
```

Các endpoint tiêu biểu qua Kong:

- `POST http://localhost:8000/api/v1/auth/register`
- `POST http://localhost:8000/api/v1/auth/login`
- `POST http://localhost:8000/api/v1/auth/refresh`
- `POST http://localhost:8000/api/v1/auth/logout`
- `GET http://localhost:8000/api/v1/auth/.well-known/jwks.json`
- `GET/PATCH http://localhost:8000/api/v1/users/me`
- `GET/POST/PATCH/DELETE http://localhost:8000/api/v1/users/me/addresses`
- `GET http://localhost:8000/api/v1/locations/provinces`

Ví dụ body giả lập cho register:

```json
{"email":"customer@example.test","password":"local-only-example-password","fullName":"Example Customer"}
```

Ví dụ body giả lập cho login:

```json
{"email":"customer@example.test","password":"local-only-example-password"}
```

Identity dùng HTTP port `8080` (Actuator health tách riêng ở `8090`) và private
authorization gRPC `127.0.0.1:50054` khi chạy local trực tiếp. Catalog và Cart
gọi RPC này với timeout `500ms`; nếu Identity authorization unavailable, thao
tác soft-delete sản phẩm và add-item giỏ hàng trả `503` mà không thay đổi trạng
thái, còn bị từ chối quyền trả `403`. Access token mặc định có TTL 15 phút;
refresh token opaque sống 30 ngày, chỉ gửi trong cookie `HttpOnly`,
`SameSite=Lax`, path `/api/v1/auth`. Local development đặt
`IDENTITY_SESSION_COOKIE_SECURE=false`; môi trường HTTPS (`compose.production.yml`)
mặc định bật `true`. Refresh token được rotate mỗi lần refresh; dùng lại token
đã rotate sẽ revoke toàn bộ session family. Khóa PEM trong
`deployments/secrets/identity/` chỉ dành cho local development và tuyệt đối
không được commit hoặc tái sử dụng cho môi trường khác.

Frontend tại `http://localhost:5173` hỗ trợ đăng ký, đăng nhập, khôi phục
session, cập nhật hồ sơ, tìm kiếm/xem sản phẩm, quản trị ẩn/khôi phục sản phẩm
và thao tác giỏ hàng. Trang `/gio-hang/dat-hang` chỉ xem trước các line item đã
chọn; chưa tạo Order.

## Verification

Kiểm tra cấu hình nền tảng, Buf lint và các route công khai qua Kong:

```bash
make verify
```

Kiểm tra theo từng surface (kèm `check-boundaries`):

```bash
make verify SERVICE=bff-service
```

## Security boundary

- Các dịch vụ nội bộ (`catalog-service`, `inventory-service`, `cart-service`,
  `order-service`) không có public Kong route; trong `compose.production.yml`,
  chỉ duy nhất Kong mở host port ra ngoài trên mạng nội bộ `ecommerce-prod-net`.
- Kong API Gateway là cổng duy nhất cho traffic bên ngoài đi vào hệ thống.
- Các biến secret không được commit lên Git (`.env` cho override hạ tầng local,
  `.env.production` và thư mục RSA keys bên ngoài repo cho môi trường
  production).

## Current limitations

- Order chưa triển khai create-order, persistence, idempotency hoặc
  transactional outbox; frontend order preview chưa submit checkout.
- Inventory mới hỗ trợ đọc tồn khả dụng; chưa có reserve/release, reservation
  TTL, RabbitMQ consumer hoặc event publication.
- Redis và RabbitMQ được provision cho kiến trúc mục tiêu nhưng chưa được
  application code sử dụng.
- Không sử dụng `docker compose down -v` trong luồng thông thường vì cờ `-v`
  sẽ xóa toàn bộ dữ liệu trong named volumes.

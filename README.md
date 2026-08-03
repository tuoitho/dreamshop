# E-commerce Microservices Platform Foundation (Phase 0)

Monorepo chứa các microservice cho hệ thống thương mại điện tử với 5 service skeletons (Identity, Order, BFF, Catalog, Inventory), cơ sở hạ tầng Docker, Kong API Gateway và hợp đồng gRPC Protobuf được sinh mã tự động.

## Prerequisites

- Java 21 LTS
- Apache Maven 3.9+
- Go 1.24+
- Docker Desktop (với Docker Compose v2)
- Buf CLI 1.70+
- PowerShell 7+ (`pwsh`)

## Quick start

1. Sao chép cấu hình môi trường mẫu:
   ```bash
   cp .env.example .env
   ```
2. Sinh mã từ các Protobuf contract:
   ```bash
   make proto
   ```
3. Khởi chạy toàn bộ hệ thống container local:
   ```bash
   make up
   ```
4. Chạy kiểm thử tự động toàn nền tảng:
   ```bash
   make verify
   ```
5. Dừng hệ thống container khi hoàn tất:
   ```bash
   make down
   ```

## Service map

Hệ thống bao gồm 5 dịch vụ microservices:
- **identity-service** (Java/Spring Boot): Quản lý danh tính người dùng và xác thực.
- **order-service** (Java/Spring Boot): Xử lý và lưu trữ đơn hàng.
- **bff-service** (Go): Backend-for-Frontend tổng hợp API cho client.
- **catalog-service** (Go): Quản lý danh mục sản phẩm.
- **inventory-service** (Go): Quản lý tồn kho sản phẩm.

## Public routes

Các API công khai duy nhất được định tuyến qua Kong API Gateway (cổng `8000`):
- `/api/v1/auth` -> Định tuyến tới `identity-service` (ví dụ: `/api/v1/auth/actuator/health`)
- `/api/v1/users` -> Định tuyến tới `identity-service`
- `/api/v1/storefront` -> Định tuyến tới `bff-service` (ví dụ: `/api/v1/storefront/healthz`)

## Local development

- Chạy kiểm thử đơn vị Go:
  ```bash
  go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
  ```
- Chạy kiểm thử đơn vị Java:
  ```bash
  mvn test
  ```

## Identity A: register, login and JWKS

Khởi tạo RSA key pair local (không commit PEM), chạy Compose, rồi xác minh register/login/JWKS qua Kong:

```bash
make identity-keys
make up
make verify-identity
```

Các public endpoint qua Kong là:

- `POST http://localhost:8000/api/v1/auth/register`
- `POST http://localhost:8000/api/v1/auth/login`
- `GET http://localhost:8000/api/v1/auth/.well-known/jwks.json`

Ví dụ body giả lập cho register:

```json
{"email":"customer@example.test","password":"local-only-example-password","full_name":"Example Customer"}
```

Ví dụ body giả lập cho login:

```json
{"email":"customer@example.test","password":"local-only-example-password"}
```

Identity dùng port `8090` khi chạy local trực tiếp; trong container service lắng nghe port `8080` và không public host port. Access token mặc định có TTL 15 phút. PEM được tạo chỉ dành cho local development, nằm ở `deployments/secrets/identity/`, và tuyệt đối không được commit hoặc tái sử dụng cho môi trường khác.

Slice này chưa có refresh token, logout, profile endpoint, hoặc Kong JWT enforcement.

## Verification

Tiện ích kiểm thử nền tảng kiểm tra tự động tất cả các tệp hợp đồng, cấu hình Docker Compose, linting Protobuf, tính sẵn sàng của hệ thống API qua Kong, và sự tồn tại của hướng dẫn vận hành trong README.md.

Chạy lệnh kiểm tra:
```bash
make verify
```

## Security boundary

- Tất cả các dịch vụ nội bộ (`catalog-service`, `inventory-service`, `order-service`) không được public ra bên ngoài và chỉ truy cập được trong mạng Docker `ecommerce-net`.
- Kong API Gateway là cổng duy nhất cho traffic bên ngoài đi vào hệ thống.
- Các secret và mật khẩu local chỉ nằm trong tệp `.env` (không được commit lên Git).

## Known phase-0 limitations

- Phase 0 tập trung xây dựng nền tảng khung (skeletons), chưa triển khai logic nghiệp vụ chi tiết (checkout, giao dịch DB, RabbitMQ consumer).
- Kong JWT plugin chưa kích hoạt ở Phase 0 do Identity service chưa phát hành JWT token chính thức.
- Không sử dụng `docker compose down -v` mặc định để tránh mất dữ liệu volume local trừ khi cần xóa sạch hoàn toàn.

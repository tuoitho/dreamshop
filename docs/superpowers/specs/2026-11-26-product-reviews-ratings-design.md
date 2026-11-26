# Design Spec: Product Reviews & Ratings (Đánh giá sản phẩm)

- **Date:** 2026-11-26
- **Branch:** `feat/product-reviews-ratings`
- **ADR:** [`docs/adr/016-product-reviews-and-ratings.md`](../../adr/016-product-reviews-and-ratings.md)
- **Mode Lock (E2E):** **Mode 2: Shape-Locked Mock E2E** (`make verify SERVICE=e2e`, khóa bằng `contracts/http-snapshots/*.json`).

## 1. Intent & Scope

Cho phép khách hàng có đơn hàng đã giao thành công (`DELIVERED`) đánh giá sản phẩm đã mua (chọn số sao 1–5, viết nhận xét, đính kèm tối đa 5 ảnh qua MinIO presigned PUT, sửa/xóa đánh giá của chính mình), hiển thị tổng hợp điểm sao & phân bố sao trên trang chi tiết sản phẩm, và cho phép quản trị viên tìm kiếm, ẩn/hiện, và gửi phản hồi chính thức trên Admin Console.

### Non-Goals
- Không tách microservice `review-service` riêng.
- Không backfill đánh giá từ đơn cũ trước khi có `OrderDelivered` bằng cross-DB query.
- Không hỗ trợ video trong đánh giá (chỉ `image/* <= 5 MB`, tối đa 5 ảnh).

## 2. Approaches Considered

1. **Đặt trong `catalog-service` + `OrderDelivered` RabbitMQ read-model (Đã chọn — ADR-016):**
   - Cập nhật `products.rating` / `review_count` trong cùng transaction PostgreSQL.
   - Tái dùng MinIO presigned PUT + sweeper (ADR-012) và Authorization gRPC client hiện có.
   - Ràng buộc sạch với `order-service` thông qua sự kiện `order.delivered` (outbox).
2. **Tạo `review-service` độc lập:**
   - Phải dựng thêm DB, port, Docker service, MinIO sweeper thứ hai và event đồng bộ ngược `rating`/`review_count` về `catalog_db`. Quá cồng kềnh cho quy mô hiện tại.
3. **Đồng bộ kiểm tra đơn `DELIVERED` bằng gRPC `catalog-service -> order-service`:**
   - Tạo phụ thuộc vòng (`order-service` đang gọi `catalog-service` khi checkout) — vi phạm kiến trúc acyclic. Loại bỏ.

## 3. Surface Map & Dependency Order

Thứ tự triển khai: `contracts` → `identity-service` → `order-service` → `catalog-service` → `bff-service` (+ Kong) → `ecommerce-fe` & `admin-fe` → `e2e`.

| Surface | Classification | Owner / Paths | Gate |
|---|---|---|---|
| `contracts` | CONTRACT (`additive`) | `contracts/proto/catalog/v1/catalog.proto`, generated `contracts/go/`, `contracts/java/` | `make verify SERVICE=contracts` |
| `identity-service` | PROVIDER (RBAC) | `V14__add_review_permissions.sql`, RBAC tests | `make verify SERVICE=identity-service` |
| `order-service` | PROVIDER (Event) | `RabbitMqConfig`, `OrderTransactionStepService`, `OrderLifecycleService`, tests | `make verify SERVICE=order-service` |
| `catalog-service` | PROVIDER (Domain/gRPC/Consumer) | Migration `000007`, `internal/domain`, `internal/store`, `internal/events`, `internal/grpc`, `internal/config`, `cmd/server`, `scripts/start-local`, `compose.production.yml` | `make verify SERVICE=catalog-service` |
| `bff-service` + Kong | CONSUMER / PUBLIC REST | `internal/catalog`, `internal/http/reviews.go`, `internal/http/admin_reviews.go`, `internal/http/contract_snapshot_test.go`, `deployments/kong/*.yaml`, `contracts/http-snapshots/*.json` | `make verify SERVICE=bff-service` |
| `ecommerce-fe` | FRONTEND | `src/lib/types.ts`, `src/lib/api.ts`, `src/lib/error-messages.ts`, `src/components/product-reviews-section.tsx`, `src/pages/product-detail-page.tsx`, `src/pages/order-detail-page.tsx`, tests | `make verify SERVICE=ecommerce-fe` |
| `admin-fe` | FRONTEND | `src/lib/types.ts`, `src/lib/api.ts`, `src/lib/permissions.ts`, `src/components/layout/admin-shell.tsx`, `src/pages/reviews-page.tsx`, `src/App.tsx`, tests | `make verify SERVICE=admin-fe` |
| `e2e` | MODE 2 E2E | `e2e/tests/mode2/`, golden snapshot lock | `make verify SERVICE=e2e` |

## 4. Contract Design (`additive`)

### Event Contract (`order-service` → `catalog-service`)
- Exchange: `ecommerce.events` (topic), Routing key: `order.delivered`, Event type: `OrderDelivered` (v1).
- Queue: `catalog.order-events` (DLQ: `catalog.order-events.dlq`).
- Payload `data`: `{ orderId, orderCode, userId, recipientName, deliveredAt, items: [{ productId, skuId, variantLabel, quantity }] }`.

### Protobuf RPCs (additive on `ecommerce.catalog.v1.CatalogService`)
- `ListProductReviews(ListProductReviewsRequest) returns (ListProductReviewsResponse)`
- `GetReviewEligibility(GetReviewEligibilityRequest) returns (GetReviewEligibilityResponse)`
- `CreateProductReview(CreateProductReviewRequest) returns (CreateProductReviewResponse)`
- `UpdateProductReview(UpdateProductReviewRequest) returns (UpdateProductReviewResponse)`
- `DeleteProductReview(DeleteProductReviewRequest) returns (DeleteProductReviewResponse)`
- `CreateReviewMediaUpload(CreateReviewMediaUploadRequest) returns (CreateReviewMediaUploadResponse)`
- `ListAdminReviews(ListAdminReviewsRequest) returns (ListAdminReviewsResponse)`
- `SetReviewVisibility(SetReviewVisibilityRequest) returns (SetReviewVisibilityResponse)`
- `ReplyProductReview(ReplyProductReviewRequest) returns (ReplyProductReviewResponse)`

### Public REST Endpoints (via Kong → BFF)
- Public (`storefront-api`): `GET /api/v1/storefront/products/{id}/reviews?rating=&hasImages=&sort=&page=&pageSize=`
- Authenticated Customer (`reviews-api`, `caller-context`):
  - `GET /api/v1/reviews/eligibility?productId={id}`
  - `POST /api/v1/reviews`
  - `PUT /api/v1/reviews/{id}`
  - `DELETE /api/v1/reviews/{id}`
  - `POST /api/v1/reviews/media/presign`
- Authenticated Admin (`admin-reviews-api`, `caller-context`):
  - `GET /api/v1/admin/reviews?q=&productId=&rating=&status=&replyStatus=&page=&pageSize=`
  - `PATCH /api/v1/admin/reviews/{id}/visibility`
  - `PUT /api/v1/admin/reviews/{id}/reply`

## 5. Acceptance Criteria & Verification Matrix

| AC | Mô tả | Verification Level |
|---|---|---|
| AC-1 | Khi admin chuyển đơn sang `DELIVERED`, `order-service` phát outbox event `OrderDelivered` (`order.delivered`) cùng transaction. | `unit/slice` (`order-service`) |
| AC-2 | `catalog-service` tiêu thụ `OrderDelivered` idempotent, ghi `review_eligibilities`, cho phép tạo tối đa 1 đánh giá trên mỗi `(user_id, order_id, product_id)` và từ chối người chưa mua (`REVIEW_NOT_ELIGIBLE`) hoặc tạo trùng (`REVIEW_ALREADY_EXISTS`). | `unit/slice` (`catalog-service`) |
| AC-3 | Tạo/sửa/xóa/ẩn/hiện đánh giá tính lại `products.rating` và `products.review_count` nguyên tử trong cùng transaction; đánh giá ẩn không tính vào điểm công khai. | `unit/slice` (`catalog-service`) |
| AC-4 | Upload ảnh đánh giá dùng MinIO presigned PUT (`image/* <= 5 MB`, tối đa 5 ảnh), chuyển trạng thái `product_media_assets` sang `ATTACHED`/`ORPHANED` đúng ADR-012. | `unit/slice` (`catalog-service`, `bff-service`) + `mock-e2e` |
| AC-5 | RBAC kiểm tra `review.create`, `review.admin.view`, `review.admin.moderate` qua Identity gRPC; thiếu quyền trả `403`. | `unit/slice` (`identity-service`, `catalog-service`, `bff-service`) |
| AC-6 | Storefront hiển thị tổng hợp sao (histogram 5→1★), lọc/sắp xếp, form tạo/sửa/xóa đánh giá + ảnh khi đủ điều kiện, và nút "Đánh giá" từ trang chi tiết đơn `DELIVERED`. | `unit/slice` (`ecommerce-fe`) + `mock-e2e` |
| AC-7 | Admin Console có trang `/reviews` cho phép lọc, tìm kiếm, ẩn/hiện đánh giá (có lý do), và gửi/cập nhật phản hồi chính thức; khóa RBAC bằng `review.admin.view` / `review.admin.moderate`. | `unit/slice` (`admin-fe`) + `mock-e2e` |
| AC-8 | Golden HTTP snapshots khóa 2 chiều cho cả `bff-storefront.json` và `bff-admin.json`; Kong local & production cấu hình khớp test. | `unit/slice` (`bff-service`) + `mock-e2e` (`e2e`) |

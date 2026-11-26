# ADR-016: Đánh giá sản phẩm xác thực đã giao hàng trong Catalog Service

**Status**: Accepted — 2026-11-26

## Context

`catalog_db.products` đã có sẵn hai cột `rating` và `review_count` được hiển thị trên Storefront và Admin Console, nhưng chưa có nghiệp vụ đánh giá thật từ người mua. Hệ thống vừa hoàn thiện vòng đời giao hàng (`DELIVERED`) và Transactional Outbox trên `order-service` (ADR-015), đồng thời `catalog-service` đã sở hữu kho media MinIO qua Presigned PUT và worker dọn file mồ côi (ADR-012). Cần quyết định ranh giới dịch vụ, cơ chế xác thực "đã mua hàng" không xuyên database, và cách cập nhật điểm trung bình nhất quán.

## Decision

- **Ranh giới sở hữu (`catalog-service` + `catalog_db`):** Đặt toàn bộ nghiệp vụ Product Reviews & Ratings (`product_reviews`, `product_review_images`, `review_eligibilities`, `processed_events`) trong `catalog-service`, tương tự quyết định đặt Favorites trong `cart-service` (ADR-010). Không tách thêm microservice `review-service` ở giai đoạn này để tránh phát sinh thêm database, hạ tầng MinIO lặp lại và độ trễ đồng bộ `products.rating` / `products.review_count`.
- **Cập nhật điểm đánh giá nguyên tử:** Mọi thao tác tạo, sửa, xóa, ẩn hoặc hiện đánh giá tính lại `products.rating` (làm tròn 1 chữ số thập phân) và `products.review_count` trên các đánh giá không bị ẩn (`hidden = false`) trong **cùng PostgreSQL transaction** trên `catalog_db`.
- **Xác thực mua hàng qua sự kiện (`OrderDelivered`):** Khi `order-service` chuyển đơn sang `DELIVERED`, dịch vụ ghi sự kiện `OrderDelivered` (`order.delivered`) vào `outbox_events` trong cùng transaction với `orders` và `order_status_history`. `catalog-service` chạy RabbitMQ consumer bền vững (queue `catalog.order-events`, DLQ `catalog.order-events.dlq`, khử trùng bằng `processed_events` và `ON CONFLICT DO NOTHING`) để đồng bộ read-model cục bộ `review_eligibilities(order_id, user_id, product_id, sku_id, variant_label, customer_name, delivered_at)`. Mỗi cặp `(user_id, order_id, product_id)` được tạo tối đa 1 đánh giá (`UNIQUE` constraint). `catalog-service` không truy cập `order_db` và không gọi đồng bộ HTTP/gRPC sang `order-service` khi đọc hoặc tạo đánh giá.
- **Tái sử dụng MinIO Presigned PUT (ADR-012):** Ảnh đánh giá của khách (`image/* <= 5 MB`, tối đa 5 ảnh/đánh giá, prefix `reviews/YYYY/MM/...`) đi qua `POST /api/v1/reviews/media/presign` → `CatalogService.CreateReviewMediaUpload`, tái dùng bảng `product_media_assets` và worker dọn file `PENDING`/`ORPHANED` hiện có của `catalog-service`.
- **Phân quyền RBAC (`identity-service`):** Thêm 3 quyền chuẩn `review.create` (gán `CUSTOMER`), `review.admin.view` và `review.admin.moderate` (gán `ADMIN`), kiểm tra qua Identity private gRPC như các RPC ghi hiện hành của `catalog-service`.
- **Public REST qua Kong + BFF:** Đọc đánh giá công khai dùng `GET /api/v1/storefront/products/{id}/reviews` (trên route `storefront-api` hiện có); thao tác khách hàng đi qua `/api/v1/reviews/**` và kiểm duyệt admin đi qua `/api/v1/admin/reviews/**`, cả hai đều gắn plugin `caller-context` trên Kong.

## Consequences

+ `products.rating` và `products.review_count` luôn nhất quán tức thì sau mọi thay đổi đánh giá mà không cần đồng bộ chéo service.
+ Tái sử dụng trọn vẹn MinIO presigned PUT + orphan sweeper (ADR-012) và Outbox/RabbitMQ pattern (ADR-002, ADR-015).
+ Không phát sinh service hay database mới; khi cần mở rộng độc lập vẫn có thể tách `product_reviews` + `review_eligibilities` sau cùng giao diện BFF/gRPC.
− `catalog-service` có thêm kết nối RabbitMQ consumer (`RABBITMQ_URL`) và bảng read-model `review_eligibilities` mang tính eventual consistency theo độ trễ outbox từ `order-service`.
→ Không truy cập `order_db` từ `catalog-service`, không stream ảnh đánh giá qua BFF/gRPC, không gửi email/profile query N+1 sang `identity-service` khi liệt kê đánh giá, và không tính đánh giá bị ẩn (`hidden = true`) vào `products.rating` hoặc `products.review_count`.

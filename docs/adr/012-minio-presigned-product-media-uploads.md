# ADR-012: Lưu trữ media sản phẩm trên MinIO qua Presigned URL và đối soát vòng đời file mồ côi

**Status**: Accepted — 2026-10-27

## Context
Quản trị danh mục sản phẩm cần tải lên ảnh (`jpeg/png/webp/avif`) và video ngắn (`mp4/webm`), trong khi truyền luồng nhị phân qua Kong → `bff-service` → `catalog-service` gRPC sẽ làm phình bộ nhớ tiến trình và chạm giới hạn frame gRPC. Đồng thời, các phiên tải dở dang (tắt form không lưu) hoặc thao tác chỉnh sửa sản phẩm gỡ bớt ảnh/video cũ sẽ để lại object mồ côi trên kho lưu trữ nếu không có cơ chế theo dõi trạng thái và thu hồi ngoài transaction cơ sở dữ liệu.

## Decision
Bổ sung kho lưu trữ đối tượng tương thích S3 bằng MinIO (bucket `product-media`) và giao `catalog-service` sở hữu toàn bộ metadata vòng đời media sản phẩm trong `catalog_db`:

- **Cấp quyền tải trực tiếp (Presigned PUT)**: `admin-fe` gọi `POST /api/v1/admin/products/media/presign` qua `bff-service` tới `CatalogService.CreateProductMediaUpload` (kiểm tra quyền `product.create` hoặc `product.update` qua Identity private gRPC). `catalog-service` xác thực MIME/dung lượng (`image/* <= 5 MB`, `video/* <= 30 MB`), ghi dòng `product_media_assets` ở trạng thái `PENDING` (`expires_at = now() + 24h`) và trả Presigned `PUT` URL TTL `5 phút` để trình duyệt đẩy thẳng file lên MinIO.
- **Định tuyến biên**: ở local development, trình duyệt gọi trực tiếp cổng MinIO `:9000` (bật CORS cho `:5173,:5174`); ở production Compose, Kong định tuyến `/product-media` (`GET`, `HEAD`, `PUT`, `preserve_host: false`) vào `http://minio:9000` để không mở host port nội bộ và giữ nguyên chữ ký SigV4 `Host: minio:9000`. Bucket chỉ mở `s3:GetObject` công khai.
- **Đối soát nguyên tử & dọn rác trễ**: giữ bảng `product_images` (chỉ chứa ảnh để tương thích `ProductCard` và `cart-service`) và bổ sung bảng `product_videos` cho video chi tiết sản phẩm. Trong cùng PostgreSQL transaction của `CreateProduct`/`UpdateProduct`, các URL xuất hiện trong sản phẩm chuyển sang `ATTACHED` (`expires_at = NULL`), còn các media từng `ATTACHED` vào sản phẩm nhưng bị gỡ khỏi danh sách chuyển sang `ORPHANED` với grace period `24h`. Thao tác `SoftDeleteProduct` giữ nguyên `ATTACHED` để phục vụ khôi phục. Worker nền trong `catalog-service` quét định kỳ (`FOR UPDATE SKIP LOCKED`) các bản ghi `PENDING`/`ORPHANED` quá hạn, gọi `RemoveObject` idempotent trên MinIO rồi xóa dòng metadata.

## Consequences
+ Tách luồng nhị phân khỏi BFF và gRPC, giữ nguyên hiệu năng đọc của `product_images`/`product_videos`, tránh mất file khi hủy form chỉnh sửa và không giữ DB lock khi xóa object trên S3.
− `catalog-service` vận hành thêm kết nối S3 client, bảng `product_media_assets` và worker dọn rác định kỳ; production Kong thêm một upstream tĩnh `/product-media`.
→ Không stream file media qua BFF/gRPC, không gọi xóa S3 đồng bộ bên trong SQL transaction của `Catalog`, và không trộn URL video vào mảng `images` dùng cho thẻ ảnh thu nhỏ.

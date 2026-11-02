# Thiết Kế Kỹ Thuật: Định Danh SKU, Giá Bán & Quản Lý Tồn Kho Theo Từng Phân Loại Sản Phẩm

## 1. Bối Cảnh & Mục Đích (Intent and Why)

Trong mô hình dữ liệu trước đây của hệ thống, phân loại sản phẩm (`ProductVariant`) chỉ đóng vai trò nhãn hiển thị phụ trợ dưới cấp sản phẩm (`product_id`), kéo theo ba giới hạn nghiệp vụ lớn:

1. **Bên `catalog-service`**: Bảng `product_variants` chỉ lưu `(product_id, position, label, color_hex)` mà không có mã định danh `sku_id` bền vững hay cột giá riêng. Khi quản trị viên chỉnh sửa thông tin sản phẩm, `Store.Update` xóa trắng toàn bộ dòng trong `product_variants` rồi chèn lại từ đầu. Mọi phân loại (ví dụ bản `12GB/256GB` và bản `16GB/1TB`) buộc phải dùng chung một mức giá duy nhất trên bảng `products`.
2. **Bên `inventory-service`**: Bảng `stock` đặt khóa chính là `product_id (quantity, reserved, updated_at)`. Các phân loại khác màu sắc hoặc dung lượng của cùng một sản phẩm rút chung từ một quỹ tồn kho tổng, khiến quản trị viên không thể kiểm kê hay nhập hàng riêng cho từng SKU, còn khách hàng trên trang chi tiết không biết phân loại nào còn hàng, phân loại nào đã hết.
3. **Bên `cart-service`, `bff-service` và luồng Đặt hàng**: Giỏ hàng tách dòng theo `(cart_id, product_id, variant_label)` nhưng lại đối chiếu khả dụng trên tổng kho của `product_id` và lấy đơn giá chung của sản phẩm; thiết kế Đặt hàng (`2026-10-28-order-checkout-saga-design.md`) phải gom nhóm cộng dồn số lượng các biến thể về `product_id` trước khi giữ chỗ.

Tài liệu này đặc tả kiến trúc tái cấu trúc toàn diện từng phân loại thành một thực thể **SKU (`sku_id`) độc lập về cả Giá bán lẫn Tồn kho** xuyên suốt `contracts`, `catalog-service`, `inventory-service`, `cart-service`, `bff-service`, `ecommerce-fe`, `admin-fe`, loại bỏ triệt để các hợp đồng/route/logic cũ theo `product_id` và `variant_label`, đồng thời đồng bộ hóa tài liệu thiết kế & kế hoạch triển khai `order-service` sang mô hình SKU.

---

## 2. Mục Tiêu, Phi Mục Tiêu & Phản Mục Tiêu (Goals, Non-Goals, Anti-Goals)

### 2.1. Mục tiêu (Goals)
- **Định danh `sku_id` bền vững & Giá bán riêng trên từng phân loại (`catalog-service`)**:
  - Mỗi dòng trong `product_variants` sở hữu một mã `sku_id` duy nhất toàn cục, `price_amount_minor` và `original_price_amount_minor` riêng.
  - Chỉnh sửa sản phẩm (`UpdateProduct`) đối soát (*upsert*) biến thể theo `sku_id`, bảo toàn tuyệt đối `sku_id` của các phân loại giữ lại để không làm đứt liên kết tồn kho và giỏ hàng.
  - Tự động suy dẫn và lưu giá đại diện trên bảng `products` (`price_amount_minor` và `original_price_amount_minor` lấy từ phân loại có giá bán thấp nhất, kèm `max_price_amount_minor` là giá bán cao nhất trong các phân loại) ngay trong cùng transaction ghi danh mục để giữ nguyên tốc độ lọc, tìm kiếm và sắp xếp theo giá mà không cần `JOIN` lúc đọc.
- **Tồn kho vật lý & giữ chỗ thuần cấp `sku_id` (`inventory-service`)**:
  - Chuyển khóa chính của bảng `stock` sang `sku_id` (kèm cột `product_id` có đánh chỉ mục), thay thế toàn bộ RPC tồn kho theo `product_id` bằng bộ RPC thuần `sku_id` (`GetAvailability`, `BatchGetAvailability`, `GetStock`, `BatchGetStock`, `ListStocks`, `SetStock`, `AdjustStock`, `DeleteStocks`).
  - Chặn xóa biến thể đang có lượng giữ chỗ đơn hàng (`reserved > 0` trả `409 VARIANT_HAS_RESERVED_STOCK`), và dọn sạch dòng `stock` có `reserved = 0` khi quản trị viên gỡ biến thể khỏi sản phẩm.
- **Trải nghiệm Storefront (`ecommerce-fe`) & Giỏ hàng (`cart-service`) chuẩn thương mại điện tử**:
  - Thẻ sản phẩm ngoài danh sách (`ProductCard`, `Search`, `Favorites`) hiển thị mức giá thấp nhất (kèm tiền tố *"Từ ..."* khi `maxPrice > price`) và trạng thái kho gộp từ tổng tồn khả dụng của các phân loại đang hoạt động (chỉ báo *"Tạm hết hàng"* khi mọi phân loại đều bằng `0`).
  - Trang chi tiết sản phẩm (`ProductDetailPage`) tự động chọn sẵn phân loại đầu tiên còn hàng; mọi nút phân loại vẫn cho phép bấm chuyển tự do (phân loại hết hàng hiển thị trạng thái mờ nhẹ kèm nhãn hết hàng) để khách xem chính xác giá bán, giá gốc, `%` giảm giá và số lượng khả dụng của riêng phân loại đó.
  - Giỏ hàng (`cart-service`) định danh dòng mua thuần theo `(cart_id, sku_id)`, làm giàu đơn giá, giá gốc, nhãn biến thể mới nhất và số tồn khả dụng trực tiếp từ SKU tương ứng.
- **Trải nghiệm Quản trị (`admin-fe`) theo cấp độ SKU**:
  - Trong `ProductDrawer`, quản trị viên cấu hình danh sách biến thể với Nhãn, Màu sắc, Giá bán, Giá gốc (kèm Tồn kho ban đầu cho biến thể mới tạo khi có quyền `inventory.update`).
  - Trên `InventoryPage`, `StockDialog` và bảng cảnh báo kho ở `OverviewPage`, quản trị viên theo dõi và điều chỉnh tồn kho trực tiếp trên từng dòng SKU (`Sản phẩm · Phân loại · Mã SKU`).
- **Đồng bộ hóa đặc tả & kế hoạch Đặt hàng (`order-service`)**:
  - Cập nhật `docs/superpowers/specs/2026-10-28-order-checkout-saga-design.md` và `docs/superpowers/plans/2026-10-28-order-checkout-saga.md` để việc thẩm định snapshot giá và giữ chỗ tồn kho (`ReserveStock`, `stock_reservation_items`, `order_items`) vận hành trực tiếp theo `sku_id` (`ORDER BY sku_id ASC`).

### 2.2. Phi mục tiêu (Non-Goals)
- Không triển khai mã nguồn runtime của `order-service` trong phạm vi đợt refactor này (`order-service` hiện là Spring Boot skeleton và sẽ được triển khai theo kế hoạch đặt hàng sau khi tài liệu được đồng bộ sang `sku_id`).
- Không xây dựng ma trận thuộc tính đa chiều động (ví dụ tách bảng trục thuộc tính Màu sắc × Dung lượng nhiều tầng); giữ mô hình danh sách `1..20` biến thể phẳng (`skuId`, `label`, `colorHex`, `price`, `originalPrice`).
- Không tách bộ sưu tập ảnh/video riêng cho từng biến thể (`images` và `videos` tiếp tục thuộc cấp sản phẩm).

### 2.3. Phản mục tiêu (Anti-Goals)
- **Không giữ mã thừa hoặc nhánh tương thích ngược cũ**: Xóa triệt để các RPC/trường Protobuf cũ theo `product_id` / `variant_label`, xóa trường nhập giá cấp sản phẩm trên `CreateProductRequest` / `UpdateProductRequest`, xóa cột `variant_label` trên bảng `cart_items`, xóa các route REST `/api/v1/admin/inventory/{productId}` cũ, và gỡ bỏ bước `buf breaking` khỏi `scripts/verify-service` và `.github/workflows/ci.yml`.
- **Không vi phạm ranh giới dịch vụ (`make check-boundaries`)**: Không truy vấn hay `JOIN` chéo giữa `catalog_db`, `inventory_db`, `cart_db`; không gọi vòng qua Kong (`:8000`) từ nội bộ backend.
- **Không xóa trắng chèn lại `product_variants` khi sửa sản phẩm**: Tuyệt đối không sinh lại `sku_id` mới cho phân loại cũ khi quản trị viên chỉnh sửa sản phẩm.

---

## 3. Ràng Buộc Kiến Trúc & Hợp Đồng (Architectural Constraints)

- **Tuân thủ các ADR hiện hành**:
  - [ADR-001](../../adr/001-service-boundaries.md), [ADR-009](../../adr/009-cart-service.md), [ADR-010](../../adr/010-product-favorites-in-cart-service.md): `catalog-service` là nguồn sự thật duy nhất về danh mục, danh sách biến thể (`sku_id`) và giá bán; `inventory-service` là nguồn sự thật duy nhất về số lượng tồn vật lý (`quantity`) và giữ chỗ (`reserved`); `cart-service` chỉ lưu định danh dòng giỏ hàng (`product_id`, `sku_id`, `quantity`) và làm giàu qua gRPC riêng tư.
  - [ADR-003](../../adr/003-redis-lock-vs-db-invariant.md): Bất biến `CHECK (quantity >= 0)` và `CHECK (reserved >= 0 AND reserved <= quantity)` được cưỡng chế trực tiếp tại bảng `stock` của PostgreSQL.
  - [ADR-006](../../adr/006-error-model-and-idempotency.md) & [ADR-011](../../adr/011-service-side-grpc-gateway-transcoding.md): Mọi lỗi nghiệp vụ qua gRPC đính kèm `google.rpc.ErrorInfo` ổn định; mọi phản hồi REST qua `bff-service` giữ nguyên chuẩn `{ "data": ..., "message": "..." }` (`camelCase`) và error envelope `{ "code", "message", "errors", "traceId" }`.
  - Ghi nhận [ADR-013](../../adr/013-variant-sku-pricing-and-inventory.md) cho quyết định tái cấu trúc thuần SKU tại Catalog, Inventory và Cart (lùi số hiệu ADR dự kiến của Order Checkout Saga sang `ADR-014`).
- **Tương thích cổng kiểm định (`make verify`)**:
  - `make verify SERVICE=contracts` chạy `make check-boundaries`, `buf format --diff --exit-code`, `buf lint`, `buf generate`, và kiểm tra đồng bộ mã sinh Go/Java (không còn chạy `buf breaking`). Job `contracts` trong `.github/workflows/ci.yml` được cập nhật tương ứng.
  - Trên Windows PowerShell, `cart-service` chạy kiểm thử gói `./services/cart-service/internal/...` (do `cmd/server` dùng tín hiệu Unix `syscall.Kill`).

---

## 4. Phương Án Kiến Trúc (Architecture Approach)

### Tái Cấu Trúc Thuần SKU (`Pure SKU Cutover`) + Giá Đại Diện Đồng Bộ Trên `products` + Đọc Tồn Kho Theo Lô (`BatchGetAvailability`)
- **Cơ chế**:
  - Bổ sung `sku_id`, `price_amount_minor`, `original_price_amount_minor` làm các cột bắt buộc trên `product_variants` (đặt `PRIMARY KEY (sku_id)`), đồng thời tự động tính và lưu giá đại diện (`price_amount_minor`, `original_price_amount_minor`, `max_price_amount_minor`) trên bảng `products` mỗi khi tạo/sửa sản phẩm. `ProductSummary` và `AdminProductSummary` trả kèm danh sách `sku_ids` đang hoạt động (nạp theo lô một lượt giống `loadImagesFor`).
  - Đổi khóa chính bảng `stock` sang `sku_id` (giữ cột `product_id` có index làm siêu dữ liệu liên kết sản phẩm), chuyển toàn bộ hợp đồng `InventoryService` sang thao tác trực tiếp theo `sku_id`.
  - Đổi khóa duy nhất bảng `cart_items` sang `(cart_id, sku_id)`, bỏ hẳn cột `variant_label` dư thừa trong `cart_db`.
  - `bff-service` và `cart-service` gọi `BatchGetAvailability` chỉ tốn **1 cuộc gọi gRPC duy nhất** cho toàn bộ trang tìm kiếm, trang chi tiết, danh sách yêu thích hoặc giỏ hàng, và chỉ cộng tổng trên đúng tập `sku_ids` đang hoạt động của sản phẩm.
- **Lợi ích**: Không còn gánh hai bộ RPC/client/route song song, lược đồ dữ liệu và Protobuf gọn sạch, truy vấn tìm kiếm/lọc giá trên `catalog-service` giữ nguyên tốc độ O(1) không cần `JOIN`, giảm từ tối đa 20 cuộc gọi gRPC rời rạc xuống 1 cuộc gọi lô trên mỗi trang danh sách, và không bao giờ cộng nhầm tồn kho của SKU đã gỡ.

---

## 5. Thiết Kế Chi Tiết Từng Phân Hệ (Surface-by-Surface Specification)

### 5.1. `contracts` — Hợp đồng Protobuf Thuần SKU

#### a) `contracts/proto/catalog/v1/catalog.proto`
- Định nghĩa `ProductVariant`:
  ```protobuf
  message ProductVariant {
    string sku_id = 1;
    string label = 2;
    string color_hex = 3;
    ecommerce.common.v1.Money price = 4;
    ecommerce.common.v1.Money original_price = 5;
  }
  ```
- Mở rộng các message đọc danh mục:
  - `GetProductResponse`: thêm `ecommerce.common.v1.Money max_price = 23;`
  - `ProductSummary`: thêm `ecommerce.common.v1.Money max_price = 17;` và `repeated string sku_ids = 18;`
  - `AdminProductSummary`: thêm `ecommerce.common.v1.Money max_price = 22;` và `repeated string sku_ids = 23;`
  - `InventoryCatalogItem`: thêm `repeated ProductVariant variants = 10;`
- Làm sạch `CreateProductRequest` và `UpdateProductRequest`: xóa hẳn hai trường `price` và `original_price` cấp sản phẩm (giá bán thuộc về từng phần tử trong `repeated ProductVariant variants`).

#### b) `contracts/proto/inventory/v1/inventory.proto`
- Thay thế toàn bộ hợp đồng cũ theo `product_id` bằng hợp đồng thuần `sku_id`:
  ```protobuf
  service InventoryService {
    rpc GetAvailability(GetAvailabilityRequest) returns (GetAvailabilityResponse);
    rpc BatchGetAvailability(BatchGetAvailabilityRequest) returns (BatchGetAvailabilityResponse);
    rpc GetStock(GetStockRequest) returns (GetStockResponse);
    rpc BatchGetStock(BatchGetStockRequest) returns (BatchGetStockResponse);
    rpc ListStocks(ListStocksRequest) returns (ListStocksResponse);
    rpc SetStock(SetStockRequest) returns (SetStockResponse);
    rpc AdjustStock(AdjustStockRequest) returns (AdjustStockResponse);
    rpc DeleteStocks(DeleteStocksRequest) returns (DeleteStocksResponse);
  }

  message AvailabilityItem {
    string sku_id = 1;
    string product_id = 2;
    int32 available_quantity = 3;
  }

  message GetAvailabilityRequest {
    string sku_id = 1;
  }

  message GetAvailabilityResponse {
    string sku_id = 1;
    string product_id = 2;
    int32 available_quantity = 3;
  }

  message BatchGetAvailabilityRequest {
    repeated string sku_ids = 1;
  }

  message BatchGetAvailabilityResponse {
    repeated AvailabilityItem items = 1;
  }

  message StockItem {
    string sku_id = 1;
    string product_id = 2;
    int32 quantity = 3;
    int32 reserved = 4;
    int32 available_quantity = 5;
    string updated_at = 6;
  }

  message GetStockRequest {
    string user_id = 1;
    string sku_id = 2;
  }

  message GetStockResponse {
    StockItem stock = 1;
  }

  message BatchGetStockRequest {
    string user_id = 1;
    repeated string sku_ids = 2;
  }

  message BatchGetStockResponse {
    repeated StockItem items = 1;
  }

  message ListStocksRequest {
    string user_id = 1;
  }

  message ListStocksResponse {
    repeated StockItem items = 1;
  }

  message SetStockRequest {
    string user_id = 1;
    string sku_id = 2;
    string product_id = 3;
    int32 quantity = 4;
  }

  message SetStockResponse {
    StockItem stock = 1;
  }

  message AdjustStockRequest {
    string user_id = 1;
    string sku_id = 2;
    string product_id = 3;
    int32 delta = 4;
  }

  message AdjustStockResponse {
    StockItem stock = 1;
  }

  message DeleteStocksRequest {
    string user_id = 1;
    repeated string sku_ids = 2;
    bool dry_run = 3;
  }

  message DeleteStocksResponse {}
  ```
- Mã lý do lỗi (`google.rpc.ErrorInfo`, domain `ecommerce.inventory.v1`):
  - `INVENTORY_PERMISSION_DENIED` (`codes.PermissionDenied`)
  - `AUTHORIZATION_UNAVAILABLE` (`codes.Unavailable`)
  - `STOCK_BELOW_RESERVED` (`codes.FailedPrecondition`)
  - `VARIANT_HAS_RESERVED_STOCK` (`codes.FailedPrecondition`, `Metadata = {"sku_id": "..."}` khi `DeleteStocks` gặp bất kỳ SKU nào đang có `reserved > 0`).

#### c) `contracts/proto/cart/v1/cart.proto`
- `AddItemRequest`: xóa `variant_label`, chỉ nhận `{ string user_id = 1; string product_id = 2; string sku_id = 3; int32 quantity = 4; }`.
- `CartItem`: bổ sung `string sku_id` và `ecommerce.common.v1.Money original_price` (`variant_label` và `variant_color_hex` chỉ còn là trường hiển thị được làm giàu từ Catalog).
- `FavoriteVariant`: `{ string sku_id = 1; string label = 2; string color_hex = 3; ecommerce.common.v1.Money price = 4; ecommerce.common.v1.Money original_price = 5; }`.
- `FavoriteItem`: thêm `ecommerce.common.v1.Money max_price = 19;`.

#### d) `contracts/proto/order/v1/order.proto`
- `OrderItem`: `{ string product_id = 1; string sku_id = 2; int32 quantity = 3; ecommerce.common.v1.Money unit_price = 4; }`.

---

### 5.2. `catalog-service` — Định danh SKU, Giá bán riêng & Bảo toàn Biến thể

- **Migration `services/catalog-service/internal/store/migrations/000005_add_variant_sku_and_pricing.up.sql`**:
  ```sql
  ALTER TABLE products
      ADD COLUMN max_price_amount_minor bigint NOT NULL DEFAULT 0;

  UPDATE products
  SET max_price_amount_minor = price_amount_minor
  WHERE max_price_amount_minor = 0;

  ALTER TABLE product_variants
      ADD COLUMN sku_id text,
      ADD COLUMN price_amount_minor bigint,
      ADD COLUMN original_price_amount_minor bigint;

  UPDATE product_variants pv
  SET sku_id = pv.product_id || '-v' || (pv.position + 1)::text,
      price_amount_minor = p.price_amount_minor,
      original_price_amount_minor = p.original_price_amount_minor
  FROM products p
  WHERE p.id = pv.product_id;

  ALTER TABLE product_variants
      ALTER COLUMN sku_id SET NOT NULL,
      ALTER COLUMN price_amount_minor SET NOT NULL,
      ALTER COLUMN original_price_amount_minor SET NOT NULL,
      DROP CONSTRAINT product_variants_pkey,
      ADD PRIMARY KEY (sku_id),
      ADD CONSTRAINT product_variants_price_positive CHECK (price_amount_minor > 0),
      ADD CONSTRAINT product_variants_original_price_valid CHECK (
          original_price_amount_minor >= price_amount_minor
      );

  CREATE INDEX product_variants_product_pos_idx
      ON product_variants (product_id, position);
  ```
- **Quy tắc định danh & kiểm tra nghiệp vụ (`internal/domain/admin.go`)**:
  - Định dạng `sku_id`: khớp biểu thức `^[a-z0-9][a-z0-9-]{1,95}$` (URL-safe, chữ thường và chữ số).
  - Trong `ValidateAndPrepareCreate` & `ValidateAndPrepareUpdate`:
    - Sản phẩm bắt buộc có từ `1` đến `20` biến thể; nhãn `label` sau khi `TrimSpace` dài `1..80` ký tự và **không được trùng nhau (không phân biệt hoa/thường)** trong cùng một sản phẩm; `sku_id` không được trùng lặp trong cùng payload.
    - Nếu biến thể có `sku_id == ""`, tự sinh `<product_id>-<8-ký-tự-hex-ngẫu-nhiên>` (cắt gọn tiền tố `product_id` nếu dài để tổng độ dài `<= 96`).
    - Mỗi biến thể bắt buộc cung cấp trực tiếp `PriceMinor > 0` (`variants.price`); nếu `OriginalPriceMinor == 0` thì chuẩn hóa `OriginalPriceMinor = PriceMinor`; nếu `OriginalPriceMinor < PriceMinor` thì trả lỗi validation `variants.original_price`.
    - **Đồng bộ giá đại diện cấp sản phẩm**: chọn biến thể `minVar` có `PriceMinor` nhỏ nhất (nếu bằng giá, ưu tiên biến thể đứng trước theo thứ tự hiển thị `position`):
      - `product.PriceMinor = minVar.PriceMinor`
      - `product.OriginalPriceMinor = minVar.OriginalPriceMinor`
      - `product.MaxPriceMinor = max(v.PriceMinor)` trên toàn bộ biến thể của sản phẩm.
- **Cập nhật nguyên tử bảo toàn `sku_id` (`internal/store/admin.go` & `store.go`)**:
  - Trong `Store.Create` và `Store.Update`: ghi thêm `max_price_amount_minor` trên bảng `products`.
  - Trong `Store.Update`:
    - Không chạy `DELETE FROM product_variants WHERE product_id = $1`.
    - Kiểm tra nếu bất kỳ `sku_id` nào trong payload đã tồn tại trong `product_variants` nhưng thuộc `product_id` khác thì từ chối ngay với lỗi xung đột (`domain.ErrSkuConflict` -> `codes.AlreadyExists` với `ErrorInfo.Reason = "SKU_ALREADY_EXISTS"` -> BFF ánh xạ `409 SKU_ALREADY_EXISTS`).
    - Xóa các biến thể cũ của sản phẩm không còn nằm trong danh sách `sku_id` mới (`DELETE FROM product_variants WHERE product_id = $1 AND NOT (sku_id = ANY($2))`), rồi `INSERT ... ON CONFLICT (sku_id) DO UPDATE SET position = EXCLUDED.position, label = EXCLUDED.label, color_hex = EXCLUDED.color_hex, price_amount_minor = EXCLUDED.price_amount_minor, original_price_amount_minor = EXCLUDED.original_price_amount_minor`.
  - Trong `Store.Search` và `Store.ListAdmin`: đọc `max_price_amount_minor` và nạp theo lô `loadSkuIDsFor(ctx, pageIDs)` (`SELECT product_id, sku_id FROM product_variants WHERE product_id = ANY($1) ORDER BY product_id, position`) để điền `SkuIDs`.
  - Trong `Store.ListInventoryItems`: mở rộng điều kiện tìm kiếm `query` khớp thêm `EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = p.id AND (pv.sku_id ILIKE pat OR pv.label ILIKE pat))`, và nạp theo lô toàn bộ `Variants` (`sku_id`, `label`, `color_hex`, `price`, `original_price`) cho các sản phẩm trả về.
- **Dữ liệu mẫu (`internal/store/seed_data.go` & `internal/store/seed.go`)**:
  - Gán `SkuID` tất định `<id>-v1`, `<id>-v2`... và mức giá bậc thang thực tế cho các biến thể dung lượng/cấu hình khác nhau (ví dụ `dt-1`: `12GB/256GB` giá `18.990.000₫`, `16GB/512GB` giá `20.990.000₫`, `16GB/1TB` giá `23.990.000₫`).

---

### 5.3. `inventory-service` — Tồn kho Thuần Cấp SKU

- **Migration `services/inventory-service/internal/store/migrations/000002_refactor_stock_to_sku.up.sql`**:
  ```sql
  ALTER TABLE stock
      ADD COLUMN sku_id text;

  UPDATE stock
  SET sku_id = product_id || '-v1'
  WHERE sku_id IS NULL;

  ALTER TABLE stock
      ALTER COLUMN sku_id SET NOT NULL,
      DROP CONSTRAINT stock_pkey,
      ADD PRIMARY KEY (sku_id);

  CREATE INDEX stock_product_id_idx ON stock (product_id);
  ```
- **Xử lý Lưu trữ & Nghiệp vụ (`internal/domain/stock.go`, `internal/store/store.go`, `internal/grpc/`)**:
  - `domain.Stock` gồm `SkuID`, `ProductID`, `Quantity`, `Reserved`, `UpdatedAt`.
  - **Các thao tác thuần SKU**:
    - `Get(ctx, skuID)` & `BatchGet(ctx, skuIDs)`: trả tồn kho theo đúng thứ tự `skuIDs`, tự điền bản ghi `Quantity = 0, Reserved = 0` cho các `sku_id` chưa có dòng trong `stock`.
    - `List(ctx)`: trả toàn bộ các dòng `stock` sắp xếp theo `updated_at DESC, sku_id ASC`.
    - `SetQuantity(ctx, skuID, productID, quantity)`: `INSERT INTO stock (sku_id, product_id, quantity, reserved, updated_at) VALUES ($1, $2, $3, 0, now()) ON CONFLICT (sku_id) DO UPDATE SET product_id = EXCLUDED.product_id, quantity = EXCLUDED.quantity, updated_at = now() WHERE stock.reserved <= EXCLUDED.quantity` (trả `ErrStockBelowReserved` nếu `quantity < reserved`).
    - `AdjustQuantity(ctx, skuID, productID, delta)`: cộng/trừ nguyên tử trên `sku_id` có kiểm tra `quantity + delta >= reserved`.
    - `DeleteUnreserved(ctx, skuIDs, dryRun)`: trong một PostgreSQL transaction, khóa các dòng hiện có `SELECT sku_id, reserved FROM stock WHERE sku_id = ANY($1) ORDER BY sku_id ASC FOR UPDATE`; nếu bất kỳ dòng nào có `reserved > 0`, rollback và trả `ErrVariantHasReservedStock{SkuID}` (ánh xạ ra `codes.FailedPrecondition` kèm `ErrorInfo.Reason = "VARIANT_HAS_RESERVED_STOCK"`, `Metadata = {"sku_id": skuID}`); nếu `dryRun == false`, chạy `DELETE FROM stock WHERE sku_id = ANY($1)` và commit.
  - **Phân quyền gRPC**:
    - `GetAvailability`, `BatchGetAvailability`: đọc khả dụng công khai nội bộ (không yêu cầu `user_id`, phục vụ storefront/cart).
    - `GetStock`, `BatchGetStock`, `ListStocks`: yêu cầu quyền `inventory.view`.
    - `SetStock`, `AdjustStock`: yêu cầu quyền `inventory.update`.
    - `DeleteStocks`: cho phép người gọi có quyền `product.update` **hoặc** `inventory.update`.
- **Dữ liệu mẫu (`internal/store/seed.go`)**:
  - Khởi tạo tồn kho mẫu theo từng `sku_id` (`dt-1-v1`, `dt-1-v2`, `dt-1-v3`, ...) với `ON CONFLICT (sku_id) DO NOTHING`, bao phủ đủ các trạng thái thực tế: phân loại còn nhiều, phân loại sắp hết (`<= 5`), phân loại hết hàng (`0`) nằm cạnh phân loại còn hàng (`dt-1-v3 = 0` trong khi `dt-1-v1`, `dt-1-v2` còn hàng), và sản phẩm hết sạch mọi phân loại (`dt-2`).

---

### 5.4. `cart-service` — Lưu Trữ & Làm Giàu Giỏ Hàng Thuần Theo `sku_id`

- **Migration `services/cart-service/internal/store/migrations/000003_refactor_cart_items_to_sku.up.sql`**:
  ```sql
  DELETE FROM cart_items;

  ALTER TABLE cart_items
      DROP CONSTRAINT cart_items_cart_id_product_id_variant_label_key,
      DROP COLUMN variant_label,
      ADD COLUMN sku_id text NOT NULL,
      ADD CONSTRAINT cart_items_sku_id_nonempty CHECK (btrim(sku_id) <> ''),
      ADD CONSTRAINT cart_items_cart_id_sku_id_key UNIQUE (cart_id, sku_id);
  ```
- **Lưu trữ & Upsert (`internal/store/` + `sqlc`)**:
  - `domain.Line` gồm `{ ID, ProductID, SkuID, Quantity }`.
  - `AddItem` thực hiện `INSERT INTO cart_items (cart_id, product_id, sku_id, quantity) VALUES (...) ON CONFLICT (cart_id, sku_id) DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity, updated_at = now()`.
- **Làm giàu Giỏ hàng & Yêu thích (`internal/grpc/server.go`, `internal/grpc/favorites.go`)**:
  - **Trong `getCart`**:
    1. Tải song song thông tin sản phẩm từ `CatalogService.GetProduct` cho các `product_id` duy nhất trong giỏ.
    2. Với mỗi dòng giỏ hàng, khớp biến thể trong `product.GetVariants()` theo `variant.GetSkuId() == line.SkuID`.
    3. Tập hợp danh sách `line.SkuID` hợp lệ (của các sản phẩm chưa xóa mềm và có biến thể khớp) và gọi **1 lần duy nhất** `InventoryService.BatchGetAvailability(ctx, skuIDs)`.
    4. Gán vào `CartItem`: `SkuId = line.SkuID`, `VariantLabel = matchedVariant.GetLabel()`, `VariantColorHex = matchedVariant.GetColorHex()`, `UnitPrice = matchedVariant.GetPrice()`, `OriginalPrice = matchedVariant.GetOriginalPrice()`, `AvailableQuantity` và `Status` tính riêng trên số tồn của đúng `sku_id` đó (khi `CatalogService` lỗi tạm thời ở chế độ `degraded`, `SkuId` vẫn được trả về đầy đủ để frontend hiển thị).
  - **Trong `getFavorites`**:
    1. Tải thông tin sản phẩm từ `CatalogService.GetProduct`.
    2. Thu thập toàn bộ `variant.GetSkuId()` của các sản phẩm đang hoạt động và gọi **1 lần duy nhất** `InventoryService.BatchGetAvailability(ctx, allSkuIDs)`.
    3. Cộng tổng `available_quantity` trên các biến thể đang hoạt động của từng sản phẩm để phân loại `FavoriteItem.Status` (`AVAILABLE`, `LIMITED`, `OUT_OF_STOCK`), đồng thời điền `MaxPrice` và đầy đủ `SkuId`, `Label`, `ColorHex`, `Price`, `OriginalPrice` cho từng `FavoriteVariant`.

---

### 5.5. `bff-service` — Hợp Đồng REST Cho Storefront & Admin

- **Storefront Products (`internal/http/products.go`)**:
  - `GET /api/v1/storefront/products`:
    - Bổ sung `maxPrice` vào `productSummary`.
    - Gom tất cả `sku_ids` của các sản phẩm trong trang kết quả, gọi 1 lần `inventory.BatchGetAvailability(ctx, allSkuIDs)`, cộng tổng `available_quantity` theo các `sku_ids` thuộc từng sản phẩm để gán `status` và `availableQuantity` (nếu sản phẩm không có `sku_ids` nào thì gán `UNAVAILABLE`).
  - `GET /api/v1/storefront/products/{id}`:
    - Bổ sung `maxPrice` trên `productDetail`.
    - Mở rộng từng phần tử trong `variants` thành:
      ```json
      {
        "skuId": "dt-1-v1",
        "label": "12GB/256GB",
        "colorHex": "#1c1c1e",
        "price": 18990000,
        "originalPrice": 21990000,
        "status": "LIMITED",
        "availableQuantity": 3
      }
      ```
    - Gọi 1 lần `inventory.BatchGetAvailability` cho danh sách biến thể của sản phẩm; tính `status` / `availableQuantity` riêng cho từng biến thể, đồng thời tính tổng `availableQuantity` cho trường tổng cấp sản phẩm.
- **Cart & Favorites (`internal/http/cart.go`, `internal/http/favorites.go`)**:
  - `POST /api/v1/cart/items`: nhận `{ "productId", "skuId", "quantity" }` (bắt buộc `skuId` không rỗng, bỏ hoàn toàn `variantLabel` trong request body). `GET /api/v1/cart` trả `skuId`, `variantLabel`, `variantColorHex`, `unitPrice`, `originalPrice`, `status`, `availableQuantity` trên từng `cartItem`.
  - `GET /api/v1/favorites`: trả thêm `maxPrice` trên `favoriteItem` và `skuId`, `label`, `colorHex`, `price`, `originalPrice` trên từng phần tử của `variants`.
- **Admin Products (`internal/http/admin_products.go`)**:
  - `GET /api/v1/admin/products`: trả thêm `maxPrice`; tính `stock` tổng của từng dòng sản phẩm bằng cách gọi `inventory.BatchGetStock(ctx, userID, allPageSkuIDs)` rồi cộng `quantity`, `reserved`, `availableQuantity` trên đúng các `sku_ids` đang hoạt động của từng sản phẩm.
  - `GET /api/v1/admin/products/{id}`: trả `maxPrice`, tổng `stock` cấp sản phẩm, và đính kèm `stock` (`quantity`, `reserved`, `availableQuantity`, `updatedAt`) trực tiếp vào từng biến thể trong mảng `variants` (lấy qua `BatchGetStock`).
  - `POST /api/v1/admin/products` & `PUT /api/v1/admin/products/{id}`: bỏ hoàn toàn trường `price` / `originalPrice` cấp sản phẩm trong payload đầu vào; mỗi phần tử trong `variants` bắt buộc có `{ skuId?, label, colorHex?, price, originalPrice? }` với `price > 0`.
  - Quy trình xóa biến thể an toàn trên `PUT /api/v1/admin/products/{id}`:
    1. Đọc sản phẩm hiện tại qua `catalog.GetAdminProduct(ctx, userID, id)` để xác định tập `removedSkuIDs` (các `sku_id` cũ không còn xuất hiện trong `input.Variants`).
    2. **Pha A (Pre-check chỉ đọc)**: nếu `len(removedSkuIDs) > 0` và `h.inventory != nil`, gọi `inventory.DeleteStocks(ctx, userID, removedSkuIDs, true)`. Nếu có SKU đang giữ chỗ (`reserved > 0`), từ chối ngay `409 VARIANT_HAS_RESERVED_STOCK` trước khi chạm vào `catalog-service`.
    3. **Pha B (Cập nhật Catalog)**: gọi `catalog.UpdateProduct`. Nếu dữ liệu danh mục không hợp lệ (`400`) hoặc trùng SKU chéo sản phẩm (`409 SKU_ALREADY_EXISTS`), trả lỗi ngay mà kho chưa hề bị xóa.
    4. **Pha C (Dọn kho không giữ chỗ)**: gọi `inventory.DeleteStocks(ctx, userID, removedSkuIDs, false)`. Nếu trong khe hở vài mili-giây vừa có đơn hàng mới giữ chỗ trên SKU đó (`VARIANT_HAS_RESERVED_STOCK`), BFF ghi log cảnh báo và bỏ qua việc xóa dòng `stock` đó để Saga của đơn hàng đang bay tất toán trọn vẹn.
- **Admin Inventory (`internal/http/admin_inventory.go`)**:
  - Xóa hẳn hai route cũ `/api/v1/admin/inventory/{productId}` và `/api/v1/admin/inventory/{productId}/adjust`.
  - `GET /api/v1/admin/inventory`:
    - Ghép nối từng biến thể (`ProductVariant`) trong `CatalogService.ListInventoryCatalogItems` với kết quả `InventoryService.ListStocks` theo khóa `sku_id`.
    - Khi có từ khóa `q`: nếu `q` không khớp các trường cấp sản phẩm (`id`, `name`, `brand`, `categoryName`), chỉ giữ các biến thể của sản phẩm đó có `skuId` hoặc `label` khớp `q` (không phân biệt hoa/thường).
    - Mỗi phần tử `adminInventoryItem` đại diện cho **1 SKU**: gồm `skuId`, `productId`, `name`, `variantLabel`, `variantColorHex`, `brand`, `categoryId`, `categoryName`, `price`, `imageUrl`, `emoji`, `imageHue`, `deleted`, `quantity`, `reserved`, `availableQuantity`, `stockStatus`, `updatedAt`.
    - `adminInventorySummary` gồm `totalTrackedProducts` (số sản phẩm duy nhất trong tập kết quả), `totalTrackedSkus` (tổng số dòng SKU), `inStockCount`, `lowStockCount`, `outOfStockCount`, `totalQuantity`, `totalReserved`, `totalAvailable`.
  - Endpoints cập nhật kho thuần SKU:
    - `PUT /api/v1/admin/inventory/skus/{skuId}` với body `{ "productId": "dt-1", "quantity": 25 }`: xác thực `(productId, skuId)` qua `CatalogService.GetProduct(productId)` (trả `404 PRODUCT_NOT_FOUND` hoặc `404 SKU_NOT_FOUND` nếu `skuId` không thuộc `product.variants`) rồi gọi `InventoryService.SetStock`.
    - `POST /api/v1/admin/inventory/skus/{skuId}/adjust` với body `{ "productId": "dt-1", "delta": -2 }`: xác thực `(productId, skuId)` qua `CatalogService.GetProduct(productId)` rồi gọi `InventoryService.AdjustStock`.

---

### 5.6. `ecommerce-fe` — Trải Nghiệm Chọn Phân Loại, Giá & Tồn Kho Riêng

- **`src/lib/types.ts` & `src/lib/product-availability.ts`**:
  - `ProductVariant`: `{ skuId: string; label: string; colorHex?: string; price: number; originalPrice?: number; status: ProductAvailabilityStatus; availableQuantity: number }`.
  - `ProductSummary` và `FavoriteItem`: thêm `maxPrice?: number`; `CartItem`: thêm `skuId: string`, `originalPrice?: number`.
  - Bổ sung hàm `resolveVariantAvailability(product, variant)` trong `product-availability.ts`: nếu `product.deleted` thì trả `UNAVAILABLE`; ngược lại tính `purchasable`, `maxQuantity`, `detailLabel`, `ctaLabel` trực tiếp trên `variant.status` và `variant.availableQuantity`.
- **`ProductCard` (`src/components/product-card.tsx`) & `FavoritesPage` (`src/pages/favorites-page.tsx`)**:
  - Khi `product.maxPrice && product.maxPrice > product.price`, hiển thị nhãn nhỏ `Từ` phía trước `formatVND(product.price)`.
  - Trên `FavoritesPage`, nút thêm nhanh vào giỏ cho sản phẩm 1 biến thể gọi `addItem(item.productId, item.variants[0].skuId, 1)`.
- **`ProductDetailPage` (`src/pages/product-detail-page.tsx`)**:
  - Khi tải xong sản phẩm, tự động chọn index của **biến thể đầu tiên có `purchasable === true`** (nếu tất cả biến thể đều hết hàng thì chọn `0`).
  - Các nút chọn biến thể luôn cho phép bấm chuyển đổi khi sản phẩm chưa ngừng kinh doanh (`!product.deleted && variants.length > 0`); biến thể nào `OUT_OF_STOCK` hiển thị viền đứt nét / độ mờ nhẹ kèm chữ nhỏ `(Hết hàng)`.
  - Khi đổi biến thể, khu vực giá chính, `%` giảm giá, huy hiệu trạng thái kho, giới hạn bộ chọn số lượng `1..variantAvailability.maxQuantity`, và nút **Thêm vào giỏ** / **Mua ngay** cập nhật tức thì theo đúng biến thể đang chọn; truyền `(product.id, selectedVariant.skuId, quantity)` vào `addItem`.
- **`CartProvider` (`src/context/cart-provider.tsx`), `CartPage` & `OrderPreviewPage`**:
  - `addItem(productId: string, skuId: string, quantity: number)` gửi `{ productId, skuId, quantity }` lên `POST /api/v1/cart/items`.
  - `CartPage` và `OrderPreviewPage` hiển thị giá gốc gạch ngang (`originalPrice > unitPrice` nếu có) và giới hạn tăng số lượng theo `item.availableQuantity` của đúng SKU.

---

### 5.7. `admin-fe` — Quản Trị Biến Thể Có Giá Riêng & Kiểm Kê Theo SKU

- **`ProductDrawer` (`src/pages/product-drawer.tsx`)**:
  - Bỏ hẳn 2 ô nhập `Giá bán / Giá gốc` đơn lẻ cấp sản phẩm và danh sách chip biến thể cũ; thay bằng **Bảng danh sách biến thể có cấu trúc**:
    - Mỗi dòng biến thể gồm: **Nhãn phân loại** (`label`), **Màu sắc** (`colorHex`), **Giá bán VND** (`price`), **Giá niêm yết gốc VND** (`originalPrice`, tùy chọn), và:
      - Với biến thể **mới thêm** (chưa có `skuId`, khi tài khoản có quyền `inventory.update`): ô nhập **Tồn ban đầu** (`initialStock`, mặc định `0`).
      - Với biến thể **đã tồn tại** (có `skuId`): hiển thị mã `skuId` dạng badge monospace + số tồn hiện tại (`Khả dụng: X · Giữ chỗ: Y`), bảo toàn nguyên vẹn `skuId` trong state khi chỉnh sửa nhãn/màu/giá. Nếu `reserved > 0`, vô hiệu hóa nút xóa dòng biến thể ngay trên giao diện kèm chú giải *"Đang có đơn giữ chỗ"*.
    - Hiển thị thanh tóm tắt tự động phía trên danh sách biến thể: *"Giá hiển thị đại diện: X ₫"* hoặc *"Từ X ₫ đến Y ₫"*.
  - Sau khi `createAdminProduct` hoặc `updateAdminProduct` thành công, với mọi biến thể mới có `initialStock > 0`, giao diện khớp biến thể vừa lưu theo `label` chuẩn hóa để lấy `skuId` do server cấp và gọi `setSkuStock(skuId, productId, initialStock)`. Nếu bất kỳ lệnh ghi kho ban đầu nào gặp lỗi mạng, giữ nguyên màn hình khôi phục `pendingStockRetry` (danh sách nhiều SKU cần thử lại) để thử lại mà không tạo trùng sản phẩm.
- **`ProductsPage` (`src/pages/products-page.tsx`) & `ProductDetailSheet` (`src/pages/product-detail-sheet.tsx`)**:
  - Cột Giá bán hiển thị `formatCurrencyVnd(price)` hoặc `Từ ...` khi `maxPrice > price`; cột Tồn kho hiển thị tổng tồn khả dụng gộp của các SKU.
  - Trong `ProductDetailSheet`, bảng chi tiết phân loại hiển thị rõ từng dòng `Mã SKU · Nhãn & Màu · Giá bán / Giá gốc · Tổng / Giữ chỗ / Khả dụng`.
- **`InventoryPage` (`src/pages/inventory-page.tsx`), `StockDialog` (`src/pages/stock-dialog.tsx`) & `OverviewPage` (`src/pages/overview-page.tsx`)**:
  - `InventoryPage` hiển thị danh sách theo từng dòng SKU (`skuId`, `name`, `variantLabel`, `variantColorHex`, `price`, `quantity`, `reserved`, `availableQuantity`); nút **Điều chỉnh** mở `StockDialog` cho SKU tương ứng.
  - `StockDialog` gọi `PUT /api/v1/admin/inventory/skus/{skuId}` và `POST /api/v1/admin/inventory/skus/{skuId}/adjust` kèm `productId`.
  - `OverviewPage` khử trùng lặp danh sách cảnh báo kho theo `skuId`, hiển thị tên sản phẩm + tên phân loại và mở `StockDialog` chỉnh kho ngay cho SKU đó.

---

### 5.8. Đồng Bộ Tài Liệu Thiết Kế & Kế Hoạch Đặt Hàng (`order-service`)

Cập nhật trực tiếp hai tệp `docs/superpowers/specs/2026-10-28-order-checkout-saga-design.md` và `docs/superpowers/plans/2026-10-28-order-checkout-saga.md`:
1. **`inventory.proto` & `inventory_db` trong đặc tả Đặt hàng**:
   - `ReservationLine`: `{ string sku_id = 1; string product_id = 2; int32 quantity = 3; }`.
   - `stock_reservation_items`: khóa chính là `(order_id, sku_id)` lưu `(order_id, sku_id, product_id, quantity)`.
   - `ReserveStock`: gom nhóm cộng dồn theo `sku_id` và **sắp xếp tăng dần `ORDER BY sku_id ASC`** trước khi chạy `UPDATE stock SET reserved = reserved + $2 WHERE sku_id = $1 AND (quantity - reserved) >= $2`. Metadata lỗi `INVENTORY_INSUFFICIENT` trả `{ "sku_id", "product_id", "available", "requested" }`.
   - Consumer `OrderConfirmed` và `ReservationReleaseRequested`: khấu trừ / hoàn trả `reserved` trên bảng `stock` theo `sku_id` đã sắp xếp `ORDER BY sku_id ASC`.
2. **Thẩm định Snapshot tại `order-service` & Lược đồ `order_items`**:
   - Bảng `order_items` bổ sung cột `sku_id VARCHAR(128) NOT NULL`.
   - Khi thẩm định snapshot trước lúc tạo đơn, `order-service` đối chiếu biến thể trong `CatalogService.GetProduct` theo `sku_id` (`VARIANT_UNAVAILABLE`, `PRICE_CHANGED` trên giá của đúng SKU, `INFO_UPDATED` trên nhãn biến thể hoặc tên sản phẩm).
3. **Số hiệu ADR**: Lùi ADR của Order Checkout Saga sang `docs/adr/014-order-checkout-saga-and-momo-payment.md` để không trùng với `ADR-013` của đợt refactor SKU này.

---

## 6. Thứ Tự Triển Khai (Implementation Order)

1. **`contracts` + `scripts/verify-service` + `.github/workflows/ci.yml` + `ADR-013`**: Gỡ bước `buf breaking`, cập nhật `.proto` thuần SKU, chạy `buf format`, `buf lint`, `buf generate`.
2. **`catalog-service`**: Migration `000005`, domain validation & suy dẫn giá, store upsert theo `sku_id`, seed data, gRPC mapping -> vượt qua `make verify SERVICE=catalog-service`.
3. **`inventory-service`**: Migration `000002`, store & gRPC thuần SKU, phân quyền `DeleteStocks`, seed data -> vượt qua `make verify SERVICE=inventory-service`.
4. **`cart-service`**: Migration `000003`, `sqlc`, làm giàu theo lô SKU cho giỏ hàng & danh sách yêu thích -> vượt qua `make verify SERVICE=cart-service`.
5. **`bff-service`**: Cập nhật handlers Storefront, Cart, Favorites, Admin Products (kèm pre-check & dọn kho khi xóa biến thể) và Admin Inventory thuần SKU -> vượt qua `make verify SERVICE=bff-service`.
6. **`ecommerce-fe` & `admin-fe`**: Cập nhật giao diện và unit/component tests trên cả hai frontend -> vượt qua `make verify SERVICE=ecommerce-fe` và `make verify SERVICE=admin-fe`.
7. **Đồng bộ tài liệu Order Saga & Tài liệu hệ thống**: Cập nhật spec/plan `order-service`, `docs/instructions/build-and-test.md`, `docs/steering/service-catalog.md`, kiểm tra biên dịch Java (`mvn -pl services/order-service -am test`) và `make check-boundaries`.

---

## 7. Các Quyết Định Chi Tiết Giao Cho Người Triển Khai (Builder Calls)

- **Quy ước khóa chính bảng `product_variants`**: Chuyển `PRIMARY KEY` của bảng `product_variants` từ `(product_id, position)` sang `(sku_id)` và tạo chỉ mục `CREATE INDEX product_variants_product_pos_idx ON product_variants (product_id, position)` để `INSERT ... ON CONFLICT (sku_id) DO UPDATE` hoán đổi thứ tự `position` mượt mà trong 1 bước.
- **Quy ước sinh `sku_id` tự động**: Dữ liệu mẫu và migration backfill dùng `<product_id>-v<position+1>` (ví dụ `dt-1-v1`); biến thể mới tạo từ giao diện quản trị dùng `<product_id>-<8-hex-random>` bằng `crypto/rand`.
- **Cơ chế Pre-check xóa biến thể không gây tác dụng phụ**: Dùng `bool dry_run = 3;` trên `DeleteStocksRequest`. Trước khi gọi `CatalogService.UpdateProduct`, `bff-service` gọi `DeleteStocks(dry_run = true)` để xác thực quyền (`product.update` hoặc `inventory.update`) và kiểm tra `reserved == 0` mà chưa xóa dữ liệu; chỉ sau khi `CatalogService.UpdateProduct` thành công thì `bff-service` mới gọi `DeleteStocks(dry_run = false)` để xóa thực sự các dòng `stock` có `reserved = 0`.

# Cart Feature Design (2026-08-12)

## Status

Approved (2026-08-12) pending written-spec review.

## Outcome

Người dùng có thể quản lý giỏ hàng server-side theo tài khoản của mình: thêm
sản phẩm (kèm phân loại), đổi số lượng, xóa dòng, chọn dòng để chuyển sang trang
đặt hàng (order preview). Mọi dòng hàng được hiển thị theo trạng thái khả dụng
thời điểm thực: còn hàng, chỉ còn số lượng ít hơn yêu cầu, hết hàng, hoặc sản
phẩm không còn khả dụng (bị xóa mềm / phân loại đã gỡ / không tồn tại) được gom
vào mục "Sản phẩm không khả dụng". Trang đặt hàng là preview có validation; nút
"Đặt hàng" là placeholder, chưa tạo đơn thật.

## Quyết định đã chốt

| Quyết định | Lựa chọn |
| --- | --- |
| Trạng thái giỏ hàng | Server-side, service mới `cart-service` (Go) |
| Yêu cầu đăng nhập | Bắt buộc; caller identity từ Kong `caller-context` headers |
| Order preview | Preview + validation; nút đặt hàng placeholder |
| Nguồn availability | Implement Inventory thật (`GetAvailability`) |
| Soft-delete sản phẩm | Catalog: cột `deleted_at` + field `deleted` additive |
| Trạng thái tích chọn | Client-side UI state |
| Phân loại sản phẩm | Tổng quát hóa `color` → `variant label`; `color_hex` optional |

## Phạm vi

### In scope

- Thêm `cart-service` (Go 1.24): gRPC CartService, DB `cart_db`, validation với
  Catalog + Inventory.
- Catalog: soft-delete (`deleted_at`, field `deleted`, RPC `SoftDeleteProduct`),
  đổi variant từ màu sang phân loại (`label` + `color_hex` optional), migration
  và seed data phân loại công nghệ.
- Inventory: implement stock + `GetAvailability` + seed.
- BFF: auth middleware, cart gRPC client, cart REST handlers, error mapping,
  config; route `/api/v1/cart` (protected) và `/api/v1/admin` (dev) qua Kong.
- Frontend: cart server-backed, selection client-side, cart page với trạng thái
  từng dòng + mục "Sản phẩm không khả dụng", order preview page, guard auth.
- ADR-009 ghi nhận service map mới; cập nhật steering docs + Makefile + init
  databases + service-catalog.

### Out of scope

- Tạo đơn hàng thật (Order service `CreateOrder` chưa implement; nút đặt hàng
  là placeholder).
- Thanh toán, khuyến mãi, mã giảm giá.
- Wishlist, so sánh sản phẩm.
- Guest cart / merge cart.
- Admin authentication (soft-delete dùng route dev tạm, chưa có auth admin).

## Kiến trúc

```text
Browser -> Kong (/api/v1/cart, protected) -> BFF (auth middleware, REST adapt)
   BFF -> cart-service (gRPC) -> catalog-service (gRPC: GetProduct, SoftDelete)
                            \-> inventory-service (gRPC: GetAvailability)
```

- Cart-service sở hữu persistence `cart_db`; không đọc DB của Catalog/Inventory.
- Catalog sở hữu product truth (tên, giá, phân loại, deleted). Inventory sở hữu
  stock/availability. Cart-service chỉ tổng hợp theo contract.
- Internal traffic qua gRPC trực tiếp, không qua Kong/BFF (theo ADR-001/ADR-008).
- `cart-service` không phải nguồn sự thật về giá; giá/variant lấy mới mỗi lần
  validate từ Catalog.

## Contracts

### `contracts/proto/cart/v1/cart.proto` (mới)

```proto
syntax = "proto3";
package ecommerce.cart.v1;
import "common/v1/common.proto";
option go_package = "ecommerce.local/contracts/go/cart/v1;cartv1";
option java_multiple_files = true;
option java_package = "com.ecommerce.contracts.cart.v1";

service CartService {
  rpc GetCart(GetCartRequest) returns (GetCartResponse);
  rpc AddItem(AddItemRequest) returns (AddItemResponse);
  rpc UpdateItemQuantity(UpdateItemQuantityRequest) returns (UpdateItemQuantityResponse);
  rpc RemoveItem(RemoveItemRequest) returns (RemoveItemResponse);
  rpc ClearCart(ClearCartRequest) returns (ClearCartResponse);
}

message GetCartRequest { string user_id = 1; }

message AddItemRequest {
  string user_id = 1;
  string product_id = 2;
  string variant_label = 3;
  int32 quantity = 4;
}

message UpdateItemQuantityRequest {
  string user_id = 1;
  string item_id = 2;
  int32 quantity = 3;
}

message RemoveItemRequest { string user_id = 1; string item_id = 2; }

message ClearCartRequest { string user_id = 1; }

message Cart {
  repeated CartItem items = 1;
  bool degraded = 2; // enrichment không đầy đủ (dependency down)
}

message GetCartResponse { Cart cart = 1; }
message AddItemResponse { Cart cart = 1; }
message UpdateItemQuantityResponse { Cart cart = 1; }
message RemoveItemResponse { Cart cart = 1; }
message ClearCartResponse { Cart cart = 1; }

message CartItem {
  string item_id = 1;
  string product_id = 2;
  string variant_label = 3;
  int32 quantity = 4;
  // enriched tại thời điểm validate
  string name = 5;
  string brand = 6;
  string category_name = 7;
  ecommerce.common.v1.Money unit_price = 8;
  string image_emoji = 9;
  repeated uint32 image_hue = 10;
  string image_url = 11;
  string variant_color_hex = 12;
  CartItemStatus status = 13;
  int32 available_quantity = 14;
}

enum CartItemStatus {
  CART_ITEM_STATUS_UNSPECIFIED = 0;
  CART_ITEM_STATUS_AVAILABLE = 1;
  CART_ITEM_STATUS_LIMITED = 2;        // 0 < available < quantity
  CART_ITEM_STATUS_OUT_OF_STOCK = 3;   // available == 0
  CART_ITEM_STATUS_UNAVAILABLE = 4;    // deleted / not found / variant removed / service degraded
}
```

Mỗi RPC trả về response đặt tên theo RPC đó (`GetCartResponse`,
`AddItemResponse`, `UpdateItemQuantityResponse`, `RemoveItemResponse`,
`ClearCartResponse`) và bọc `Cart` đã validate, để client luôn có trạng thái mới
nhất trong một round trip. Tên enum trên gRPC dùng prefix `CART_ITEM_STATUS_`
theo Buf STANDARD; BFF map sang chuỗi public REST không prefix: `AVAILABLE`,
`LIMITED`, `OUT_OF_STOCK`, `UNAVAILABLE`.

### `contracts/proto/catalog/v1/catalog.proto` (breaking, pre-production)

- `ProductVariant`: `color = 1` → `label = 1`; `color_hex = 2` giữ (optional).
- `GetProductResponse`: thêm `bool deleted = 21`.
- Thêm:
  ```proto
  rpc SoftDeleteProduct(SoftDeleteProductRequest) returns (SoftDeleteProductResponse);
  message SoftDeleteProductRequest { string id = 1; bool deleted = 2; }
  message SoftDeleteProductResponse { string id = 1; bool deleted = 2; }
  ```
- `SearchProducts`/`SearchProductsResponse` không đổi field, nhưng `store.Search`
  thêm điều kiện `deleted_at IS NULL`.

### `contracts/proto/inventory/v1/inventory.proto`

Không đổi. Implement server `GetAvailability` + bảng stock + seed.

### BFF REST

- `GET /api/v1/cart` → cart đã validate.
- `POST /api/v1/cart/items` body `{ productId, variantLabel, quantity }`.
- `PATCH /api/v1/cart/items/{itemId}` body `{ quantity }`.
- `DELETE /api/v1/cart/items/{itemId}`.
- `DELETE /api/v1/cart`.
- Envelope chuẩn; `data` = `{ items: [...] }` (+ `degraded`).
- `status` trong mỗi item là chuỗi public REST không prefix: `AVAILABLE`,
  `LIMITED`, `OUT_OF_STOCK`, `UNAVAILABLE` (BFF map từ `CART_ITEM_STATUS_*`).
- Error codes ổn định: `CART_ITEM_NOT_FOUND`, `INVALID_QUANTITY`,
  `INVALID_PRODUCT`, `CART_UNAVAILABLE`, `UNAUTHORIZED`.

### Kong

- Route `/api/v1/cart` → BFF, plugin `caller-context` (config giống
  identity protected route).
- Route `/api/v1/admin` → BFF (dev, tạm public) cho soft-delete demo.

## Data model

### `cart_db`

```sql
CREATE TABLE carts (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null unique,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

CREATE TABLE cart_items (
  id uuid primary key default gen_random_uuid(),
  cart_id uuid not null references carts(id) on delete cascade,
  product_id text not null,
  variant_label text not null default '',
  quantity int not null check (quantity > 0),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (cart_id, product_id, variant_label)
);
```

Cart tạo lazy khi thêm dòng đầu tiên.

### Catalog (migration mới)

- `ALTER TABLE products ADD COLUMN deleted_at timestamptz;`
- `product_variants`: đổi `color` → `label` (và cho phép `color_hex` null). Seed
  data: phân loại công nghệ (laptop "16GB/1TB", "32GB/1TB"; phone
  "128GB/256GB/512GB"; vài phân loại màu có `color_hex`).

### Inventory (migration mới)

```sql
CREATE TABLE stock (
  product_id text primary key,
  quantity int not null check (quantity >= 0),
  reserved int not null default 0 check (reserved >= 0),
  updated_at timestamptz not null default now()
);
```

`available_quantity = quantity - reserved`. Seed stock cho product hiện có; một
số sản phẩm hết hàng (`quantity = 0`) để demo trạng thái OUT_OF_STOCK.

## Validation logic (cart-service)

1. Lấy dòng hàng của `user_id`.
2. Gọi Catalog `GetProduct` cho từng `product_id` distinct bằng fan-out song
   song có bounded concurrency (mặc định 8) và per-call timeout (giống client
   hiện có); fail-fast → degraded.
3. Với product tìm thấy: đối chiếu `variant_label` trong `ProductVariant[]`; lấy
   `unit_price`/name/brand/category/image từ Catalog.
4. Gọi Inventory `GetAvailability` cho từng product.
5. Resolve status (proto `CartItemStatus`, prefix `CART_ITEM_STATUS_`; BFF map
   sang chuỗi REST không prefix):
   - product `deleted` / không tồn tại / `variant_label` không còn →
     `CART_ITEM_STATUS_UNAVAILABLE`
   - `available == 0` → `CART_ITEM_STATUS_OUT_OF_STOCK`
   - `0 < available < quantity` → `CART_ITEM_STATUS_LIMITED` (+
     `available_quantity`)
   - còn lại → `CART_ITEM_STATUS_AVAILABLE`
6. Dependency down (Catalog/Inventory): set `degraded = true`; các dòng chưa
   enrich được giữ bản ghi gốc với status `CART_ITEM_STATUS_UNAVAILABLE`
   (reason log nội bộ).
7. Không auto-clamp `quantity`; server chỉ báo cáo `available_quantity`. UI đề
   xuất "Đặt về N".

`AddItem` dùng upsert theo `(cart_id, product_id, variant_label)` (tự nhiên
idempotent). Mutation không cần `Idempotency-Key` ở MVP.

## BFF & Auth

- Auth middleware đọc `X-User-Id` / `X-User-Roles` (Kong `caller-context` inject).
  Route protected; header thiếu → `401 UNAUTHORIZED`.
- BFF giữ mỏng: chỉ adapt REST ↔ gRPC, map error, không implement domain rules.
- `internal/cart/client.go` mô phỏng `internal/catalog/client.go`; config mới
  `CART_GRPC_ADDR`, `CART_GRPC_TIMEOUT`.
- Map gRPC status → REST envelope:
  - `NotFound` → `404 CART_ITEM_NOT_FOUND`
  - `InvalidArgument` → `400 INVALID_QUANTITY` / `INVALID_PRODUCT`
  - `Unavailable` → `502 CART_UNAVAILABLE`

## Frontend

### Cart context (server-backed)

- `CartProvider` đọc `GET /api/v1/cart` khi mount (sau auth ready); mutation gọi
  API và cập nhật state từ response.
- Selection là client-side: `Map<itemId, selected>` (memory + localStorage riêng
  cho selection), không gửi lên server. `toggleAll`, `removeSelected` (gọi
  RemoveItem cho các itemId đang chọn).

### Routing

- `/gio-hang` và `/gio-hang/dat-hang` bọc `RequireAuth`.
- "Thêm vào giỏ"/"Mua ngay" khi chưa đăng nhập → redirect `/login`.

### Cart page

- Từng dòng: checkbox (khoá nếu `OUT_OF_STOCK`/`UNAVAILABLE`), ảnh, tên, phân
  loại (label + optional swatch `colorHex`), giá đơn vị/giá dòng từ server,
  bộ tăng/giảm số lượng.
- Badge trạng thái: `AVAILABLE` "Còn hàng"; `LIMITED` "Chỉ còn N" + nút "Đặt về
  N"; `OUT_OF_STOCK` "Hết hàng"; `UNAVAILABLE` gom vào mục "Sản phẩm không khả
  dụng" (tách khỏi danh sách chính, có nút xoá).
- Summary: đã chọn, tạm tính, phí ship (rule giữ như hiện tại: miễn phí ≥
  299.000đ, ngược lại 25.000đ), tổng cộng. Tổng chỉ tính các dòng AVAILABLE/
  LIMITED đã chọn.
- Nút "Tiếp tục" → `/gio-hang/dat-hang` (enabled khi có ≥1 dòng hợp lệ đã chọn).

### Order preview page

- Hiển thị các dòng đã chọn và hợp lệ (AVAILABLE/LIMITED): danh sách, số lượng,
  giá, tổng tạm tính, phí ship, tổng cộng.
- Nút "Đặt hàng" placeholder → toast "Chức năng đặt hàng chưa khả dụng".
- Cho phép quay lại giỏ.

### Product detail

- "Thêm vào giỏ" gọi `POST /api/v1/cart/items` (productId, variantLabel,
  quantity); thành công → toast dùng `message` từ envelope + cập nhật cart
  context.
- "Mua ngay" → thêm dòng vào cart rồi redirect `/gio-hang/dat-hang` với dòng
  vừa thêm được chọn.
- Nút admin demo (chỉ hiển thị khi `import.meta.env.DEV`): "Xóa mềm / Khôi
  phục" gọi `POST /api/v1/admin/products/{id}/delete` (`{ deleted: true|false }`)
  rồi reload sản phẩm, để dễ demo mục "Sản phẩm không khả dụng".

### Cập nhật demo data frontend

- `src/data/products.ts` là mirror demo của seed catalog: cập nhật `variants`
  theo label mới (đồng bộ với `seed_data.go`).
- Mọi tham chiếu `variant.color`/`colorHex` trong FE (types, product-card,
  product-detail, cart) đổi theo contract mới: `variant.label` (bắt buộc),
  `variant.colorHex` (optional).

## Testing

- **cart-service**: unit test domain (status resolution, variant match, upsert
  merge, clear); store integration (pgxpool) nếu cần; grpc handler test.
- **catalog**: store soft-delete filter, `SoftDeleteProduct` RPC, migration,
  seed variant mới; cập nhật test hiện có cho `label`.
- **inventory**: `GetAvailability` (available = quantity - reserved), seed, store
  migration.
- **bff**: auth middleware (missing header, malformed), cart handlers
  (table-driven: happy path, error mapping), config validation.
- **frontend**: API adapter cart (envelopes/error codes), cart page states
  (limited/out-of-stock/unavailable group), order preview, RequireAuth,
  add-to-cart redirect khi chưa login.
- **contracts**: `buf format --diff --exit-code`, `buf lint`, `buf generate`,
  diff generated output sạch; compile/test các producer/consumer Go/Java.

## Delivery / Infra

- Init DB: thêm `CREATE DATABASE cart_db;`.
- Env mới: `CART_GRPC_ADDR`, `CART_GRPC_TIMEOUT` (BFF); `DATABASE_URL`,
  `HTTP_ADDR`, `GRPC_ADDR` (cart-service).
- Makefile: target `cart` (start script), cập nhật `go test` command list.
- Kong: route `/api/v1/cart` + `/api/v1/admin`.
- Cập nhật `docs/steering/service-catalog.md`.
- ADR-009 ghi nhận `cart-service` thêm vào service map.
- Viết spec này vào git (commit).

## Rollout / Compatibility

- Catalog contract đổi `color`→`label` là breaking nhưng pre-production: cập nhật
  đồng bộ catalog-proto → generated → catalog-store/grpc → bff → fe trong một
  thay đổi.
- Thứ tự deploy (local): cart-service + inventory + catalog migration trước,
  sau đó bff, cuối cùng frontend.
- `deleted` field additive; consumers cũ bỏ qua field mới an toàn.

## Assumptions

- Cart gắn `user_id`; không có guest cart.
- Enrichment failure làm `degraded=true`; cart không chặn hoàn toàn khi
  dependency down.
- Phí ship rule giữ nguyên như FE hiện tại, tính ở FE.
- Soft-delete admin route chưa có auth (dev), đã ghi rõ trong deliverable.

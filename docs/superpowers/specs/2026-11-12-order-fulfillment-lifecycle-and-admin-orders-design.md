# Thiết Kế: Vòng Đời Giao Hàng Đơn Hàng, Tự Xác Nhận COD & Trang Quản Trị Đơn Hàng

## 1. Bối cảnh

Vòng đời đơn hàng hiện tại dừng ở `CONFIRMED`:

- `OrderStatus` chỉ có `PENDING_STOCK`, `PENDING_PAYMENT`, `CONFIRMED`, `CANCELLED`
  (`services/order-service/.../domain/OrderStatus.java`), được ràng buộc bằng `CHECK`
  trong `V1__create_orders_idempotency_and_outbox.sql`.
- Đơn COD được xác nhận ngay trong request tạo đơn (`OrderTransactionStepService.confirmCodOrder`)
  và giữ `paymentStatus = UNPAID` vĩnh viễn. Chỉ đơn MoMo mới lên `PAID` qua event
  `PaymentSucceeded`.
- Không có trạng thái chuẩn bị/giao hàng, không có lịch sử chuyển trạng thái, không có
  quyền hay giao diện cho quản trị viên xử lý đơn.
- `OrderConfirmed` làm `inventory-service` trừ hẳn tồn kho (reservation `ACTIVE → CONFIRMED`)
  và làm `cart-service` xóa các dòng giỏ đã đặt. Sau khi reservation đã `CONFIRMED`,
  `ReservationReleaseRequested` là no-op: hiện không có cách hoàn kho theo đơn.
- Khách chỉ hủy được đơn `PENDING_PAYMENT`.

## 2. Mục tiêu

1. Đơn COD sau khi giữ hàng thành công ở trạng thái **Chờ xác nhận** và được hệ thống tự
   xác nhận sau **1 phút**.
2. Đơn MoMo giữ nguyên luồng hiện tại: chuyển hướng sang cổng MoMo, chỉ xác nhận khi
   `PaymentSucceeded`, hủy khi hết hạn thanh toán. Không tự xác nhận theo thời gian.
3. Mở rộng vòng đời: Đang chuẩn bị → Đang giao → Đã giao / Giao thất bại.
4. Đơn COD chuyển `paymentStatus` sang `PAID` khi đơn chuyển sang **Đã giao**.
5. Giỏ hàng xóa các dòng đã đặt ngay khi đơn COD đặt thành công, không chờ tự xác nhận.
6. Ghi lịch sử (timeline) mọi lần chuyển trạng thái: thời điểm, người thực hiện, lý do,
   đơn vị vận chuyển, mã vận đơn.
7. Trang **Đơn hàng** trong `admin-fe`: tab theo trạng thái kèm số lượng, tìm theo mã đơn,
   xem chi tiết, cập nhật trạng thái chỉ theo bước hợp lệ.
8. Hủy đơn sau khi đã xác nhận phải hoàn lại tồn kho.

## 3. Ngoài phạm vi

- Khách gửi yêu cầu hủy chờ admin duyệt, trả hàng/hoàn tiền, khách bấm "Đã nhận hàng" và
  trạng thái `COMPLETED`.
- Thông báo real-time (socket/email) khi đổi trạng thái.
- Tích hợp đơn vị vận chuyển thật; đơn vị vận chuyển và mã vận đơn do admin nhập tay.
- Hoàn tiền MoMo thực tế (đã có cơ chế `REFUND_REQUIRED`/`RefundRequested`, giữ nguyên).
- Thời điểm xóa giỏ của đơn MoMo: vẫn xóa khi thanh toán thành công (`OrderConfirmed`), để
  khách không mất giỏ khi bỏ dở hoặc thanh toán thất bại.

## 4. Máy trạng thái

### 4.1. Trạng thái

Giữ nguyên ý nghĩa 4 trạng thái cũ, bổ sung 5 trạng thái:

| Trạng thái | Nhãn | Ghi chú |
| --- | --- | --- |
| `PENDING_STOCK` | Đang giữ hàng | Giữ nguyên |
| `PENDING_PAYMENT` | Chờ thanh toán | MoMo, giữ nguyên |
| `PENDING_CONFIRMATION` | Chờ xác nhận | **Mới.** COD sau khi giữ hàng thành công |
| `CONFIRMED` | Đã xác nhận | Giữ nguyên ý nghĩa: đơn được chấp nhận, tồn kho đã trừ |
| `PREPARING` | Đang chuẩn bị | **Mới** |
| `SHIPPING` | Đang giao | **Mới** |
| `DELIVERED` | Đã giao | **Mới.** Trạng thái cuối trong phạm vi này |
| `DELIVERY_FAILED` | Giao thất bại | **Mới** |
| `CANCELLED` | Đã hủy | Trạng thái cuối, giữ nguyên |

### 4.2. Bước chuyển hợp lệ

```text
PENDING_STOCK ──(COD giữ hàng OK)──────────────▶ PENDING_CONFIRMATION
PENDING_STOCK ──(MoMo tạo phiên OK)────────────▶ PENDING_PAYMENT          (giữ nguyên)
PENDING_PAYMENT ──(PaymentSucceeded)───────────▶ CONFIRMED                (giữ nguyên)
PENDING_CONFIRMATION ──(hệ thống, sau 1 phút)──▶ CONFIRMED
CONFIRMED ──(admin)────────────────────────────▶ PREPARING
PREPARING ──(admin)────────────────────────────▶ SHIPPING
SHIPPING ──(admin)─────────────────────────────▶ DELIVERED | DELIVERY_FAILED
DELIVERY_FAILED ──(admin, giao lại)────────────▶ SHIPPING

Hủy (→ CANCELLED):
  PENDING_PAYMENT        khách | hệ thống (hết hạn)           (giữ nguyên)
  PENDING_CONFIRMATION   khách | admin
  CONFIRMED              khách | admin
  PREPARING              admin
  DELIVERY_FAILED        admin
```

Bảng chuyển trạng thái nằm trong domain của `order-service` (lớp `OrderTransitions`) và là
nguồn duy nhất cho cả kiểm tra hợp lệ lẫn danh sách bước kế tiếp trả về cho admin.
Frontend không tự suy luận bước hợp lệ.

### 4.3. Tác dụng phụ theo bước chuyển

| Bước | Tồn kho | Thanh toán | Event phát ra |
| --- | --- | --- | --- |
| `PENDING_STOCK → PENDING_CONFIRMATION` | reservation vẫn `ACTIVE` | `UNPAID` | `OrderPlaced` |
| `PENDING_CONFIRMATION → CONFIRMED` | trừ hẳn | `UNPAID` | `OrderConfirmed` |
| `PENDING_CONFIRMATION → CANCELLED` | trả giữ chỗ | không đổi | `ReservationReleaseRequested` |
| `CONFIRMED / PREPARING / DELIVERY_FAILED → CANCELLED` | hoàn kho | `PAID → REFUND_REQUIRED` | `StockRestockRequested`, kèm `RefundRequested` nếu đã `PAID` |
| `SHIPPING → DELIVERED` | — | COD: `UNPAID → PAID` | — |
| Các bước admin còn lại | — | — | — |

- `OrderConfirmed` chỉ phát khi đơn thật sự `CONFIRMED`, nên hủy trong lúc chờ xác nhận
  dùng lại `ReservationReleaseRequested` hiện có.
- `OrderPlaced` để `cart-service` xóa các dòng đã đặt ngay khi đặt hàng thành công.
- Đơn MoMo đã `PAID` bị hủy chuyển `REFUND_REQUIRED` và phát `RefundRequested` hiện có,
  giống xử lý thanh toán muộn.

## 5. Tự xác nhận COD

- Thêm cột `orders.auto_confirm_at TIMESTAMPTZ NULL`. Khi COD vào `PENDING_CONFIRMATION`,
  đặt `auto_confirm_at = now + order.cod.auto-confirm-delay` (mặc định `PT1M`).
- Lịch chạy mới `CodAutoConfirmScheduler` (`@Scheduled`, `order.cod.auto-confirm-interval-ms`,
  mặc định 5000) theo mẫu `OrderReconciliationScheduler`:
  - `SELECT ... WHERE status = 'PENDING_CONFIRMATION' AND auto_confirm_at <= now()
    ORDER BY auto_confirm_at LIMIT 50 FOR UPDATE SKIP LOCKED`.
  - Mỗi đơn: `CONFIRMED`, ghi lịch sử (`SYSTEM`), ghi outbox `OrderConfirmed` trong cùng
    transaction; đánh thức outbox relay sau commit.
- Không giữ transaction qua lời gọi mạng (ADR-014). Nhiều instance an toàn nhờ `SKIP LOCKED`
  và kiểm tra lại trạng thái sau khi khóa.
- Độ trễ thực tế: 60–65 giây.
- Reservation COD vẫn được tạo với `expiresAt = now + payment-ttl` (15 phút). Inventory
  không tự hủy reservation theo `expires_at`, nên khoảng chờ 1 phút không làm mất giữ chỗ.

## 6. Hoàn kho cho đơn đã xác nhận (`inventory-service`)

- Event mới `StockRestockRequested`, routing key `order.stock.restock_requested`,
  `data = { orderId, reason }`. Bind thêm vào queue `inventory.order-events`.
- `RestockReservation(orderId, reason)`:
  - Reservation `CONFIRMED`: `quantity += q` cho từng dòng, chuyển reservation sang trạng
    thái mới `RESTOCKED`, ghi `release_reason`. Một transaction, khóa dòng stock theo thứ tự
    `sku_id` như `ConfirmReservation`.
  - Reservation `ACTIVE` (event xác nhận chưa tới): xử lý như release (`reserved -= q`,
    `RELEASED`) để không trừ kho cho đơn đã hủy.
  - `RESTOCKED` / `RELEASED`: no-op (lũy đẳng khi nhận lặp).
  - Không có reservation: ghi tombstone `RELEASED`, giống `ReleaseReservation`.
- Migration `000004`: mở rộng `CHECK` trạng thái reservation thêm `RESTOCKED`.
- `ConfirmReservation` gặp `RESTOCKED`: no-op, nên thứ tự event đảo ngược vẫn đúng.

## 7. Lịch sử trạng thái

Bảng mới `order_status_history` trong `order_db`:

| Cột | Kiểu | Ghi chú |
| --- | --- | --- |
| `id` | `UUID` PK | |
| `order_id` | `UUID` FK → `orders.id` | index `(order_id, created_at)` |
| `from_status` | `VARCHAR(32) NULL` | `NULL` cho bản ghi tạo đơn |
| `to_status` | `VARCHAR(32)` | |
| `actor_type` | `VARCHAR(16)` | `SYSTEM`, `CUSTOMER`, `ADMIN` |
| `actor_id` | `UUID NULL` | user id của khách/admin |
| `reason` | `VARCHAR(128) NULL` | Cùng giới hạn với `orders.cancel_reason`; request dài hơn 128 ký tự bị từ chối `INVALID_PARAMETER` |
| `shipping_carrier` | `VARCHAR(100) NULL` | |
| `tracking_number` | `VARCHAR(100) NULL` | |
| `created_at` | `TIMESTAMPTZ` | |

- Ghi trong cùng transaction với mọi thay đổi `orders.status`, kể cả các luồng hiện có
  (saga, thanh toán, hủy, đối soát quá hạn).
- Thêm vào `orders`: `shipping_carrier`, `tracking_number` (giá trị hiện hành để hiển thị),
  `auto_confirm_at`.
- Đơn tạo trước thay đổi này không có lịch sử; giao diện hiển thị timeline trống.

## 8. Phân quyền

- Migration Identity `V13__add_admin_order_permissions.sql`: thêm `order.admin.view` và
  `order.admin.update`, gán cho `ADMIN`. `SUPER_ADMIN` tự có mọi permission.
- Cập nhật `IdentityDatabaseMigrationTest` cho danh sách permission mới.
- `order-service` kiểm tra quyền qua `AuthorizationGrpcAdapter` hiện có:
  `order.admin.view` cho đọc, `order.admin.update` cho cập nhật trạng thái.
- Khách hủy đơn vẫn dùng `order.cancel`.
- `admin-fe`: thêm hai mã vào `ADMIN_CONSOLE_PERMISSIONS`; trang và menu gate bằng
  `order.admin.view`, nút cập nhật gate bằng `order.admin.update`.

## 9. Contract

Mọi thay đổi đều là **additive**, theo skill `contract-change`.

### 9.1. Protobuf `contracts/proto/order/v1/order.proto`

```proto
service OrderService {
  // 4 RPC hiện có giữ nguyên
  rpc AdminListOrders(AdminListOrdersRequest) returns (AdminListOrdersResponse);
  rpc AdminGetOrder(AdminGetOrderRequest) returns (AdminGetOrderResponse);
  rpc AdminUpdateOrderStatus(AdminUpdateOrderStatusRequest) returns (AdminUpdateOrderStatusResponse);
}

message OrderStatusChange {
  string from_status = 1;
  string to_status = 2;
  string actor_type = 3;
  string reason = 4;
  string shipping_carrier = 5;
  string tracking_number = 6;
  string created_at = 7;
}

message OrderDetail {
  // 1..17 giữ nguyên
  repeated OrderStatusChange status_history = 18;
  string auto_confirm_at = 19;
  string shipping_carrier = 20;
  string tracking_number = 21;
}

message AdminListOrdersRequest {
  string user_id = 1;      // admin gọi
  string status = 2;       // rỗng = tất cả
  string query = 3;        // mã đơn, khớp tiền tố, không phân biệt hoa thường
  int32 page = 4;
  int32 page_size = 5;
}

message AdminListOrdersResponse {
  repeated OrderDetail items = 1;
  int32 page = 2;
  int32 page_size = 3;
  int64 total_items = 4;
  int32 total_pages = 5;
  map<string, int64> status_counts = 6;  // đếm theo query, không theo status
}

message AdminGetOrderRequest {
  string user_id = 1;
  string order_id = 2;
}

message AdminGetOrderResponse {
  OrderDetail order = 1;
  repeated string allowed_next_statuses = 2;
}

message AdminUpdateOrderStatusRequest {
  string user_id = 1;
  string order_id = 2;
  string expected_status = 3;   // chống ghi đè khi hai admin cùng thao tác
  string target_status = 4;
  string reason = 5;
  string shipping_carrier = 6;
  string tracking_number = 7;
}

message AdminUpdateOrderStatusResponse {
  OrderDetail order = 1;
  repeated string allowed_next_statuses = 2;
}
```

`ListOrders` và `GetOrder` của khách nhận thêm các trường mới trong `OrderDetail`;
`ListOrders` chấp nhận lọc theo các trạng thái mới. Danh sách (`ListOrders`,
`AdminListOrders`) không nạp `status_history` để tránh truy vấn thừa; chỉ `GetOrder`,
`AdminGetOrder`, `AdminUpdateOrderStatus` và `CancelOrder` trả lịch sử.

### 9.2. Mã lỗi (ADR-006)

| Reason | gRPC | HTTP | Khi nào |
| --- | --- | --- | --- |
| `ORDER_INVALID_TRANSITION` | `FAILED_PRECONDITION` | 409 | Bước chuyển không có trong bảng |
| `ORDER_STATUS_CONFLICT` | `ABORTED` | 409 | `expected_status` khác trạng thái hiện tại |
| `ORDER_NOT_CANCELLABLE` | `FAILED_PRECONDITION` | 409 | Đã có, mở rộng điều kiện |
| `ORDER_REASON_REQUIRED` | `INVALID_ARGUMENT` | 400 | Hủy hoặc giao thất bại thiếu lý do |
| `ORDER_PERMISSION_DENIED` | `PERMISSION_DENIED` | 403 | Đã có |
| `ORDER_NOT_FOUND` | `NOT_FOUND` | 404 | Đã có |

### 9.3. REST (BFF)

| Method | Path | Quyền |
| --- | --- | --- |
| `GET` | `/api/v1/admin/orders?status=&q=&page=&pageSize=` | `order.admin.view` |
| `GET` | `/api/v1/admin/orders/{orderId}` | `order.admin.view` |
| `POST` | `/api/v1/admin/orders/{orderId}/status` | `order.admin.update` |

Body cập nhật:

```json
{
  "expectedStatus": "PREPARING",
  "targetStatus": "SHIPPING",
  "reason": "",
  "shippingCarrier": "GHN",
  "trackingNumber": "GHN123456"
}
```

- Danh sách trả `{ items, page, pageSize, totalItems, totalPages, statusCounts }` trong `data`.
- Chi tiết và cập nhật trả `{ order, allowedNextStatuses }` trong `data`; cập nhật có
  `message` "Cập nhật trạng thái đơn hàng thành công".
- `OrderDetail` JSON (cả khách và admin) thêm `statusHistory[]`, `autoConfirmAt`,
  `shippingCarrier`, `trackingNumber`.
- Kong: route mới `admin-orders-api` (`/api/v1/admin/orders`, plugin `caller-context`) trong
  `kong.yaml` và `kong-production.yaml`, kèm cập nhật `kong_development_test.go` và
  `kong_production_test.go`.

### 9.4. Event (`ecommerce.events`, envelope ADR-002)

| eventType | Routing key | `data` | Consumer |
| --- | --- | --- | --- |
| `OrderPlaced` | `order.placed` | Giống `OrderConfirmed`: `orderId, orderCode, userId, items[{cartItemId, productId, skuId, quantity}]` | `cart-service` |
| `StockRestockRequested` | `order.stock.restock_requested` | `orderId, reason` | `inventory-service` |

`cart-service` bind thêm `order.placed` và dùng chung handler `RemoveOrderedItems` (đã khử
trùng theo `eventId`, xóa lặp là vô hại). `OrderConfirmed` sau đó vẫn được xử lý như cũ.

## 10. Thay đổi theo surface

| Surface | Thay đổi |
| --- | --- |
| `contracts` | Proto 9.1, `buf lint`, `buf breaking`, `buf generate` |
| `identity-service` | Migration `V13`, test migration |
| `inventory-service` | Migration `000004`, `RestockReservation`, bind routing key mới, `ConfirmReservation` bỏ qua `RESTOCKED` |
| `cart-service` | Bind `order.placed` |
| `order-service` | Migration `V2` (enum `CHECK`, cột mới, bảng lịch sử, index), `OrderStatus` mở rộng, `OrderTransitions`, COD vào `PENDING_CONFIRMATION`, `CodAutoConfirmScheduler`, mở rộng hủy của khách, 3 RPC admin, ghi lịch sử ở mọi bước chuyển, event mới |
| `bff-service` | `admin_orders.go`, mở rộng client order, map JSON và lỗi mới, Kong config + test |
| `admin-fe` | Route `/orders`, menu "Đơn hàng", trang danh sách, sheet chi tiết + timeline, dialog cập nhật trạng thái, adapter và type |
| `ecommerce-fe` | Type `OrderStatus` mở rộng, tab và nhãn trạng thái mới, timeline + đơn vị vận chuyển trên trang chi tiết, nút hủy cho `PENDING_CONFIRMATION`/`CONFIRMED`, thông báo tự xác nhận khi vừa đặt COD |
| `docs/adr` | ADR-015 mở rộng máy trạng thái của ADR-005 |

## 11. Giao diện

### 11.1. Admin – trang Đơn hàng

- Menu "Đơn hàng" trong nhóm "Thương mại & Kho vận", route `/orders`.
- Hàng tab: Tất cả, Chờ thanh toán, Chờ xác nhận, Đã xác nhận, Đang chuẩn bị, Đang giao,
  Đã giao, Giao thất bại, Đã hủy, mỗi tab kèm `statusCounts`.
- Ô tìm theo mã đơn, bảng: mã đơn, người nhận + SĐT, tổng tiền, phương thức, trạng thái đơn,
  trạng thái thanh toán, ngày tạo; phân trang.
- Sheet chi tiết: sản phẩm, địa chỉ, tổng tiền, ghi chú, đơn vị vận chuyển + mã vận đơn,
  timeline.
- Dialog cập nhật: chỉ liệt kê `allowedNextStatuses`. Chọn `SHIPPING` hiện ô đơn vị vận
  chuyển và mã vận đơn; chọn `CANCELLED` hoặc `DELIVERY_FAILED` bắt buộc nhập lý do.
  Gặp `ORDER_STATUS_CONFLICT` thì tải lại đơn và báo trạng thái đã thay đổi.
- Theo mẫu `inventory-page.tsx`, `stock-dialog.tsx`, `product-detail-sheet.tsx`.

### 11.2. Storefront

- `/don-hang`: thêm tab cho trạng thái mới; nhãn và màu badge cho từng trạng thái.
- `/don-hang/:orderId`:
  - Vừa đặt COD: "Đơn hàng đang chờ xác nhận, hệ thống sẽ tự xác nhận trong khoảng 1 phút",
    kèm nút làm mới.
  - Timeline lịch sử trạng thái, đơn vị vận chuyển và mã vận đơn khi có.
  - Nút hủy hiển thị cho `PENDING_PAYMENT`, `PENDING_CONFIRMATION`, `CONFIRMED`; server vẫn
    là nơi quyết định.

## 12. Thứ tự triển khai

Provider trước consumer:

1. `contracts` (proto) → generate.
2. `identity-service` (permission).
3. `inventory-service` (consumer `order.stock.restock_requested`) và `cart-service`
   (consumer `order.placed`). Phải chạy trước `order-service`: routing key chưa có queue
   bind sẽ bị exchange bỏ.
4. `order-service`.
5. `bff-service` + Kong.
6. `admin-fe`, `ecommerce-fe`.

## 13. Kiểm thử

| Surface | Test |
| --- | --- |
| `order-service` | Unit `OrderTransitions` (mọi cặp hợp lệ/không hợp lệ); COD tạo đơn → `PENDING_CONFIRMATION` + outbox `OrderPlaced`, không có `OrderConfirmed`; scheduler xác nhận đơn đến hạn, bỏ qua đơn chưa đến hạn và đơn đã hủy; admin update: quyền, `expected_status` lệch, bước không hợp lệ, `DELIVERED` đặt COD `PAID`, hủy sau xác nhận phát `StockRestockRequested` (+ `RefundRequested` khi `PAID`); hủy của khách ở `PENDING_CONFIRMATION`/`CONFIRMED`; lịch sử ghi đủ ở mọi bước; migration (Testcontainers) |
| `inventory-service` | Restock reservation `CONFIRMED` cộng lại `quantity`; `ACTIVE` → release; lặp lại là no-op; confirm sau restock là no-op |
| `cart-service` | Consumer `order.placed` xóa dòng giỏ, lặp `eventId` không lỗi |
| `identity-service` | Migration test permission mới |
| `bff-service` | Handler admin: map request/response, lỗi 400/403/404/409/503; Kong dev/prod route test |
| `admin-fe` | Trang danh sách: tab + đếm, tìm, gate quyền; dialog: chỉ hiện bước hợp lệ, bắt buộc lý do, xử lý `ORDER_STATUS_CONFLICT` |
| `ecommerce-fe` | Nhãn/tab trạng thái mới, thông báo chờ xác nhận, timeline, điều kiện nút hủy |
| End-to-end | Chạy full stack: đặt COD → giỏ xóa ngay → sau ~1 phút `CONFIRMED` → admin chuẩn bị → giao → đã giao (`PAID`); hủy đơn `CONFIRMED` → tồn kho được hoàn |

Gate: `make verify SERVICE=<surface>` cho từng surface bị đổi, `buf lint`, `make check-boundaries`.

## 14. Rủi ro

- **Hoàn kho phụ thuộc thứ tự event:** `StockRestockRequested` có thể tới trước
  `OrderConfirmed` khi relay gửi lệch. Xử lý ở mục 6: restock trên `ACTIVE` thành release,
  confirm trên `RESTOCKED` là no-op.
- **Đơn COD hiện có** đang `CONFIRMED`: không cần backfill, admin xử lý tiếp như đơn mới.
- **Giỏ hàng khi hủy COD:** dòng giỏ đã xóa lúc đặt không được khôi phục khi khách hủy trong
  lúc chờ xác nhận; khách thêm lại sản phẩm nếu muốn.

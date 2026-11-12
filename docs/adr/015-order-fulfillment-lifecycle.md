# ADR-015: Vòng đời giao hàng đơn hàng, tự xác nhận COD và hoàn kho theo đơn

## Status

Accepted

## Context

[ADR-005](005-saga-compensation-and-reservation-lifecycle.md) và
[ADR-014](014-order-checkout-saga-and-payment-service.md) dừng vòng đời đơn ở
`CONFIRMED | CANCELLED`, với `PENDING_PAYMENT` cho MoMo. Cửa hàng cần xử lý đơn sau khi
xác nhận (chuẩn bị, giao, giao thất bại), cần một khoảng chờ trước khi chấp nhận đơn COD,
cần ghi nhận COD đã thu tiền khi giao xong, và cần cho phép hủy đơn đã xác nhận.

Khi `OrderConfirmed` được phát, Inventory trừ hẳn tồn kho (reservation `CONFIRMED`) và
`ReservationReleaseRequested` trở thành no-op, nên hủy sau xác nhận hiện làm mất tồn kho.
Cart chỉ xóa dòng giỏ khi nhận `OrderConfirmed`.

## Decision

- Mở rộng máy trạng thái của Order bằng các trạng thái mới, không đổi nghĩa trạng thái cũ:
  `PENDING_CONFIRMATION` (COD đã giữ hàng, chờ tự xác nhận), `PREPARING`, `SHIPPING`,
  `DELIVERED`, `DELIVERY_FAILED`. `CONFIRMED` vẫn có nghĩa "đơn được chấp nhận và tồn kho
  đã trừ"; `CANCELLED` và `DELIVERED` là trạng thái cuối.
- Bảng chuyển trạng thái nằm trong domain của `order-service` và là nguồn duy nhất cho
  kiểm tra hợp lệ và danh sách bước admin được phép:
  `PENDING_CONFIRMATION → CONFIRMED` (hệ thống), `CONFIRMED → PREPARING → SHIPPING`,
  `SHIPPING → DELIVERED | DELIVERY_FAILED`, `DELIVERY_FAILED → SHIPPING`; hủy được từ
  `PENDING_PAYMENT`, `PENDING_CONFIRMATION`, `CONFIRMED` (khách) và từ
  `PENDING_CONFIRMATION`, `CONFIRMED`, `PREPARING`, `DELIVERY_FAILED` (admin).
- COD vào `PENDING_CONFIRMATION` với `auto_confirm_at`; một job định kỳ trong
  `order-service` khóa đơn đến hạn bằng `FOR UPDATE SKIP LOCKED`, chuyển `CONFIRMED` và
  phát `OrderConfirmed`. Khi vào `PENDING_CONFIRMATION`, Order phát `OrderPlaced` để Cart
  xóa dòng giỏ ngay.
- Hủy sau xác nhận phát `StockRestockRequested` (`order.stock.restock_requested`).
  Inventory cộng lại `quantity` cho reservation `CONFIRMED` và đánh dấu `RESTOCKED`; gặp
  reservation `ACTIVE` thì release; `ConfirmReservation` trên `RESTOCKED` là no-op để thứ
  tự event đảo ngược vẫn đúng. Đơn đã `PAID` bị hủy chuyển `REFUND_REQUIRED` và phát
  `RefundRequested`.
- COD chuyển `paymentStatus` sang `PAID` khi đơn sang `DELIVERED`.
- Mọi thay đổi `orders.status` ghi một dòng `order_status_history` trong cùng transaction
  với outbox.
- Admin đọc và cập nhật đơn qua RPC riêng, ủy quyền bằng `order.admin.view` và
  `order.admin.update`; cập nhật mang `expected_status` để từ chối ghi đè đồng thời.

## Consequences

- Được: đơn có vòng đời giao hàng đầy đủ, lịch sử kiểm toán được, hủy sau xác nhận không
  làm mất tồn kho.
- Mất: thêm hai event, một trạng thái reservation, một job định kỳ và ma trận test lớn hơn.
- Ràng buộc triển khai: consumer của `order.placed` (Cart) và
  `order.stock.restock_requested` (Inventory) nên bind queue trước khi `order-service`
  phát các event này. Outbox của Order publish với `mandatory`; message không có queue
  nhận bị broker trả về và được giữ ở trạng thái chưa publish để gửi lại, nên triển khai
  sai thứ tự chỉ làm chậm event chứ không làm mất.
- Xem lại khi cần tích hợp đơn vị vận chuyển thật, trả hàng/hoàn tiền hoặc khách xác nhận
  đã nhận hàng (`COMPLETED`).

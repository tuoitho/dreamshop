# Hướng dẫn cấu hình & vận hành thanh toán MoMo (v2)

Tài liệu này mô tả luồng thanh toán MoMo trong hệ thống, sự khác biệt giữa chế độ **Mock local** và **MoMo Gateway thật (Sandbox / Production)**, cùng các biến môi trường cần thiết để cấu hình.

---

## 1. Kiến trúc & luồng xử lý tổng quan

Hệ thống tách phần giao tiếp cổng MoMo vào `payment-service` (Go, HTTP `:8086`, gRPC `127.0.0.1:50056`), còn hai endpoint callback công khai đi qua Kong (`:8000` ở local) $\rightarrow$ `bff-service` (`:8081` ở local):

1. **Khởi tạo phiên thanh toán (`CreatePaymentSession`)**:
   - Khi khách đặt hàng với `paymentMethod = "MOMO"`, `order-service` gọi gRPC `CreatePaymentSession` sang `payment-service`.
   - `payment-service` trả về `payUrl`.
   - Tại `ecommerce-fe`, trang đặt hàng (`order-preview-page.tsx`) chuyển hướng trình duyệt trên tab hiện tại bằng `window.location.assign(result.payUrl)`.
   - Nếu đơn vẫn đang ở trạng thái `PENDING_PAYMENT`, người dùng có thể bấm link `<a href={order.payUrl}>` ở trang danh sách đơn (`/don-hang`), chi tiết đơn (`/don-hang/:orderId`), hoặc trang kết quả thanh toán (`/thanh-toan/ket-qua`).

2. **Tiếp nhận Callback từ MoMo (`Redirect` và `IPN`)**:
   - **`GET /api/v1/payments/momo-redirect`** (`Source = "REDIRECT"`): MoMo chuyển hướng trình duyệt người dùng về Kong $\rightarrow$ `bff-service` $\rightarrow$ gRPC `HandleMomoCallback`. Sau khi xác thực xong, `bff-service` trả `HTTP 302 Found` về `${BFF_STOREFRONT_BASE_URL}/thanh-toan/ket-qua?orderId=...&result=success|failed` (hoặc `400` nếu chữ ký/số tiền không hợp lệ).
   - **`POST /api/v1/payments/momo-ipn`** (`Source = "IPN"`): Server MoMo gửi JSON server-to-server tới Kong $\rightarrow$ `bff-service` $\rightarrow$ gRPC `HandleMomoCallback`. `bff-service` trả `HTTP 204 No Content` (hoặc `400` nếu chữ ký/số tiền không hợp lệ).

3. **Chốt trạng thái & khử trùng (Idempotency)**:
   - Ở tầng `payment-service`, cả `REDIRECT` và `IPN` đều đi chung vào `HandleMomoCallback` $\rightarrow$ `RecordMomoCallback`.
   - Mọi callback đều phải qua bước kiểm tra chữ ký `HmacSHA256` hằng thời gian (`hmac.Equal`) và đối chiếu `amount_minor`.
   - Bảng `payments` được khóa dòng (`SELECT ... FOR UPDATE`). Callback hợp lệ đến trước sẽ cập nhật trạng thái (`SUCCEEDED` / `FAILED` / `REFUND_REQUIRED` nếu đơn đã hủy) và ghi sự kiện Outbox sang RabbitMQ để `order-service` cập nhật đơn hàng; callback đến sau được bỏ qua an toàn (`OutboxInserted = false`).
   - Sau khi được redirect về `/thanh-toan/ket-qua`, frontend tự động poll `GET /api/v1/orders/{orderId}` tối đa 20 lần (mỗi `500ms`) khi đơn còn ở `PENDING_PAYMENT` hoặc `PENDING_STOCK` để đồng bộ kết quả từ RabbitMQ.

---

## 2. Hai chế độ hoạt động (`PAYMENT_MOMO_MOCK_ENABLED`)

| Tiêu chí | `PAYMENT_MOMO_MOCK_ENABLED=true` (Mock local) | `PAYMENT_MOMO_MOCK_ENABLED=false` (Sandbox / Production) |
| --- | --- | --- |
| **Cách tạo `payUrl`** | `payment-service` không gọi API MoMo; tự ký callback URL trỏ thẳng tới `MOMO_REDIRECT_URL` với `resultCode=0`. | `payment-service` ký `HmacSHA256` (`requestType=captureWallet`) và `POST` sang `MOMO_API_ENDPOINT` để lấy `payUrl` thật từ MoMo. |
| **Giao diện thanh toán** | Chuyển qua `momo-redirect` $\rightarrow$ `302` thẳng về `/thanh-toan/ket-qua` (không mở trang QR/ví MoMo). | Trình duyệt mở trang thanh toán/QR thật của MoMo. |
| **Xử lý callback & RabbitMQ** | Chạy qua luồng verify chữ ký, cập nhật `payment_db`, Outbox/RabbitMQ và `order_db` thật. | Chạy đầy đủ cả `Redirect` (trình duyệt) lẫn `IPN` (server MoMo gọi về). |
| **Mặc định khi chạy** | Mặc định khi chạy `make payment` (`scripts/start-local`) và trong `.env.example`. | Mặc định trong `payment-service` (`config.Load`), `compose.production.yml` và `.env.production.example`. |

---

## 3. Bảng biến môi trường liên quan

### 3.1. `payment-service`

| Biến | Bắt buộc khi chạy thật | Mặc định (`config.go`) | Mặc định local (`scripts/start-local`) | Mô tả |
| --- | --- | --- | --- | --- |
| `PAYMENT_MOMO_MOCK_ENABLED` | Có (`false`) | `false` | `true` | Bật/tắt chế độ giả lập tạo `payUrl`. |
| `MOMO_PARTNER_CODE` | Có | `MOMO` | `MOMO_LOCAL` | Mã đối tác MoMo cấp. |
| `MOMO_ACCESS_KEY` | Có | `""` | `local-access-key` | Access key dùng ký request và verify callback. |
| `MOMO_SECRET_KEY` | Có | `""` | `local-secret-key-change-me` | Secret key HMAC-SHA256 của MoMo. |
| `MOMO_API_ENDPOINT` | Có | `https://test-payment.momo.vn/v2/gateway/api/create` | `https://test-payment.momo.vn/v2/gateway/api/create` | Endpoint tạo giao dịch MoMo v2 (Sandbox dùng `test-payment.momo.vn`, Production dùng `payment.momo.vn`). |
| `MOMO_REFUND_ENDPOINT` | Không | Suy ra từ `MOMO_API_ENDPOINT` (`/create` $\rightarrow$ `/refund`) | `https://test-payment.momo.vn/v2/gateway/api/refund` | Endpoint hoàn tiền MoMo v2 (`POST /v2/gateway/api/refund`). |
| `MOMO_REDIRECT_URL` | Có (`http(s)://`) | `https://shop.example.com/api/v1/payments/momo-redirect` | `http://localhost:8000/api/v1/payments/momo-redirect` | URL của Kong/BFF nhận browser redirect từ MoMo. |
| `MOMO_IPN_URL` | Có (`http(s)://`) | `https://shop.example.com/api/v1/payments/momo-ipn` | `http://localhost:8000/api/v1/payments/momo-ipn` | URL của Kong/BFF nhận webhook IPN từ server MoMo. |
| `MOMO_CONNECT_TIMEOUT` | Không | `3s` | Kế thừa default (`3s`) | Timeout kết nối TCP tới API MoMo. |
| `MOMO_READ_TIMEOUT` | Không | `10s` | Kế thừa default (`10s`) | Timeout đọc phản hồi HTTP từ API MoMo. |
| `PAYMENT_REFUND_MAX_AUTO_ATTEMPTS` | Không | `3` | `3` | Số lần tự động gọi hoàn tiền tối đa trước khi chuyển `REFUND_FAILED`. |
| `PAYMENT_REFUND_POLL_INTERVAL` | Không | `2s` | `2s` | Chu kỳ quét bảng `payment_refunds` của `RefundWorker`. |
| `PAYMENT_REFUND_RETRY_BACKOFF` | Không | `10s` | `10s` | Thời gian chờ cơ sở (`base * 2^(k-1)`) giữa các lần retry hoàn tiền tự động. |

> **Lưu ý khi thiếu hoặc sai cấu hình lúc `PAYMENT_MOMO_MOCK_ENABLED=false`:**
> - Nếu `MOMO_REDIRECT_URL` hoặc `MOMO_IPN_URL` không bắt đầu bằng `http://` hoặc `https://`, `payment-service` sẽ báo lỗi ngay khi khởi động.
> - Nếu `MOMO_ACCESS_KEY` hoặc `MOMO_SECRET_KEY` bị rỗng, service vẫn khởi động nhưng mọi yêu cầu tạo đơn MoMo sẽ lỗi `PAYMENT_GATEWAY_UNAVAILABLE` (`missing MoMo credentials`) và mọi callback sẽ bị từ chối `400 INVALID_MOMO_SIGNATURE`.
> - Nếu dùng key giả của local (`MOMO_LOCAL` / `local-access-key`) với `PAYMENT_MOMO_MOCK_ENABLED=false`, API MoMo thật sẽ từ chối tạo phiên (`PAYMENT_GATEWAY_REJECTED` hoặc `PAYMENT_GATEWAY_UNAVAILABLE`).

### 3.2. `bff-service` & `ecommerce-fe`

| Service | Biến | Local | Production (`compose.production.yml`) | Vai trò |
| --- | --- | --- | --- | --- |
| `bff-service` | `BFF_STOREFRONT_BASE_URL` | `http://localhost:5173` | `/` | Base URL dùng để `302` redirect trình duyệt về `/thanh-toan/ket-qua` sau khi `momo-redirect` xử lý xong. |
| `ecommerce-fe` | `VITE_API_BASE_URL` | `http://localhost:8000/api/v1` | `/api/v1` | Base URL gọi API qua Kong (`:8000` ở local, same-origin ở production). |

---

## 4. Hướng dẫn chạy MoMo Sandbox thật ở môi trường Local

1. Chuẩn bị bộ credential Sandbox từ MoMo (`MOMO_PARTNER_CODE`, `MOMO_ACCESS_KEY`, `MOMO_SECRET_KEY`).
2. Cấu hình trong file `.env` tại thư mục gốc (hoặc truyền biến môi trường cho `payment-service`):

```env
PAYMENT_MOMO_MOCK_ENABLED=false
MOMO_PARTNER_CODE=<partner_code_sandbox>
MOMO_ACCESS_KEY=<access_key_sandbox>
MOMO_SECRET_KEY=<secret_key_sandbox>
MOMO_API_ENDPOINT=https://test-payment.momo.vn/v2/gateway/api/create
MOMO_REDIRECT_URL=http://localhost:8000/api/v1/payments/momo-redirect
MOMO_IPN_URL=https://<public-tunnel-domain>/api/v1/payments/momo-ipn
```

3. **Lưu ý về `MOMO_REDIRECT_URL` và `MOMO_IPN_URL` khi test từ máy cá nhân**:
   - `MOMO_REDIRECT_URL` chạy trên trình duyệt của chính bạn nên có thể để `http://localhost:8000/api/v1/payments/momo-redirect` (Kong `:8000` $\rightarrow$ `bff-service` `:8081` $\rightarrow$ `302` về `http://localhost:5173/thanh-toan/ket-qua`).
   - `MOMO_IPN_URL` do **server MoMo** gọi từ bên ngoài internet, nên không thể dùng `localhost`. Muốn nhận thêm IPN server-to-server khi test local, cần mở tunnel HTTPS (ví dụ ngrok hoặc Cloudflare Tunnel) trỏ vào cổng Kong `8000` và điền URL public đó vào `MOMO_IPN_URL`. Nếu chưa mở tunnel cho IPN, luồng thanh toán vẫn hoàn tất được qua `MOMO_REDIRECT_URL` khi trình duyệt quay lại.
4. Không commit file `.env` hoặc credential thật lên Git.

---

## 5. Cấu hình cho môi trường Production

1. Sao chép `.env.production.example` thành `.env.production` và điền đầy đủ thông tin MoMo Production:
   - `PAYMENT_MOMO_MOCK_ENABLED=false`
   - `MOMO_API_ENDPOINT=https://payment.momo.vn/v2/gateway/api/create`
   - `MOMO_PARTNER_CODE`, `MOMO_ACCESS_KEY`, `MOMO_SECRET_KEY` thật
   - `MOMO_REDIRECT_URL=https://<domain-that>/api/v1/payments/momo-redirect`
   - `MOMO_IPN_URL=https://<domain-that>/api/v1/payments/momo-ipn`
   - `BFF_STOREFRONT_BASE_URL=/` (hoặc `https://<domain-that>`)
2. Đảm bảo domain công khai hỗ trợ HTTPS và reverse proxy phía trước trỏ đúng `/api/v1/payments/momo-redirect` và `/api/v1/payments/momo-ipn` qua Kong.

---

## 6. Hoàn tiền MoMo v2 (Tự động 3 lần + Thủ công Admin)

### 6.1. Luồng tự động (Auto-Retry)

- Khi đơn MoMo đã thanh toán bị hủy hoặc nhận callback thanh toán muộn sau khi hủy, `order-service` đặt `payment_status = REFUND_REQUIRED` và phát sự kiện `RefundRequested` (`order.payment.refund_requested`).
- `payment-service` nhận sự kiện, ghi bản ghi vào bảng `payment_refunds` (`status = PENDING`, `refund_order_id = REFUND<32 hex>`, `refund_request_id = REFREQ<24 hex>`, giữ nguyên `description` cố định).
- Background `RefundWorker` chạy mô hình 2-TX (không giữ transaction PostgreSQL khi gọi HTTPS) tối đa **3 lần** (`PAYMENT_REFUND_MAX_AUTO_ATTEMPTS=3`, backoff `10s`, `20s`):
  - **Thành công (`resultCode = 0`)**: cập nhật `payments.status = REFUNDED`, phát sự kiện Outbox `PaymentRefunded` (`payment.refunded`) để `order-service` chuyển `orders.payment_status = REFUNDED`.
  - **Thất bại tạm thời / mơ hồ (timeout, 5xx, crash, `resultCode` `7000` / `9000`)**: giữ nguyên `refund_order_id` và `refund_request_id` ở lần thử tiếp theo để đảm bảo idempotency phía MoMo.
  - **Bị MoMo từ chối rõ ràng (`resultCode != 0` ngoài `7000`/`9000`)**: đánh dấu xoay cặp `refund_order_id` + `refund_request_id` mới ở lần thử tiếp theo, vẫn giữ nguyên `description` cố định.
  - **Hết 3 lần tự động**: dừng retry, đặt `payments.status = REFUND_FAILED`, phát sự kiện Outbox `PaymentRefundFailed` (`payment.refund_failed`) để `order-service` chuyển `orders.payment_status = REFUND_FAILED` kèm `refund_last_error`.

### 6.2. Chữ ký HMAC-SHA256 & Payload `POST /v2/gateway/api/refund`

Chuỗi ký (`rawHash`) theo đúng thứ tự alphabet của MoMo v2 Refund:

```text
accessKey=$accessKey&amount=$amount&description=$description&orderId=$orderId&partnerCode=$partnerCode&requestId=$requestId&transId=$transId
```

- `orderId` là `refund_order_id` thuần alphanumeric và khác `momo_order_id` gốc.
- `transId` là số nguyên `int64` của giao dịch thanh toán gốc.
- Khi `PAYMENT_MOMO_MOCK_ENABLED=true`, `Client.Refund` trả về thành công giả lập (`mock-refund-...`) mà không gọi HTTP ra ngoài.

### 6.3. Hoàn tiền thủ công trên `admin-fe` & Backfill dữ liệu cũ

- Nút **"Hoàn tiền thủ công"** (`POST /api/v1/admin/orders/{orderId}/refund` $\rightarrow$ `OrderService.AdminRetryRefund` $\rightarrow$ `PaymentService.RetryRefund`) chỉ mở khi đơn có `status = CANCELLED`, `paymentMethod = MOMO`, `paymentStatus = REFUND_FAILED` và tài khoản có quyền `order.admin.update`.
- Kết quả trả về đồng bộ cho quản trị viên (cập nhật ngay `REFUNDED` cùng audit `ADMIN` hoặc lưu `refund_last_error` mới nhất nếu MoMo vẫn lỗi).
- **Backfill dữ liệu cũ (Option 1)**: migration `000002` (`payment_db`) và `V3` (`order_db`) chuyển các đơn `REFUND_REQUIRED` đã tồn tại từ trước sang `REFUND_FAILED` (không tự động chi tiền khi khởi động) để quản trị viên chủ động bấm **"Hoàn tiền thủ công"** trên `admin-fe`.
- **Thứ tự triển khai Production**: deploy `order-service` (migration `V3` và RabbitMQ binding cho `payment.refunded` / `payment.refund_failed`) **trước** `payment-service`, sau đó `bff-service`, rồi `admin-fe` và `ecommerce-fe`.

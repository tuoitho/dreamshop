# ADR-011: Hybrid routing với BFF và service-side grpc-gateway thay cho Kong gRPC transcoding

**Status**: Accepted — 2026-10-27 (Supersedes [ADR-008](008-hybrid-rest-bff-grpc-transcoding.md))

## Context
Plugin `grpc-gateway` tích hợp trên Kong ([ADR-008](008-hybrid-rest-bff-grpc-transcoding.md)) trả thẳng raw JSON của Protobuf (`snake_case`, không bọc `{ "data", "message" }`) và định dạng lỗi theo gRPC status thay vì chuẩn REST error envelope `{ "code", "message", "errors", "traceId" }` ([ADR-006](006-error-model-and-idempotency.md)). Viết thêm Lua plugin tại Kong để biến đổi payload gây cồng kềnh và khó bảo trì, còn chấp nhận lệch định dạng trong cùng một hệ thống sẽ làm phân mảnh contract phía client.

## Decision
Giữ mô hình hybrid routing với Kong là public ingress duy nhất cho các tác vụ hạ tầng biên (routing, TLS/CORS, rate limit, strip header giả mạo ở route public và ghi đè `X-User-Id`, `X-User-Roles` qua `caller-context` ở route protected), nhưng chuyển việc transcoding REST ↔ gRPC từ Kong xuống chính Go domain service khi cần expose trực tiếp:

- **Identity REST**: tiếp tục đi thẳng từ Kong tới `identity-service`.
- **First-party frontend (`ecommerce-fe`) & luồng cần tổng hợp**: toàn bộ commerce workflow phục vụ UI và các endpoint cần gộp dữ liệu nhiều service hoặc tùy biến shape đi qua `bff-service`. Không trộn lẫn lúc gọi BFF, lúc gọi thẳng domain service trong cùng một tính năng của frontend.
- **Direct domain REST (khi có nhu cầu mở API trực tiếp không qua BFF như partner/external API, webhook, hoặc unary endpoint độc lập được duyệt)**: không dùng plugin `grpc-gateway` của Kong. Go domain service tự expose HTTP song song cổng gRPC bằng thư viện Go `grpc-gateway` sinh từ `google.api.http` đã được review, cấu hình `protojson` (`camelCase`), custom marshaler bọc `{ "data", "message" }` và `runtime.WithErrorHandler` trả đúng chuẩn lỗi của repo; Kong chỉ proxy HTTP thông thường (`strip_path: false`). Nếu có sản phẩm partner API riêng cố ý dùng contract khác, phải tách prefix riêng và lập ADR mới.
- **Giao tiếp nội bộ**: microservice gọi nhau bằng direct private gRPC hoặc RabbitMQ; không gọi qua Kong, BFF hay cổng HTTP/`grpc-gateway` của service khác.

## Consequences
+ Giữ `.proto` làm nguồn sự thật duy nhất khi mở REST trực tiếp mà vẫn tuân thủ 100% chuẩn `{ "data", "message" }`, `camelCase` và error envelope của repo; Kong giữ vai trò edge gateway thuần HTTP.
− Khi mở một direct domain REST route, Go service cần cấu hình plugin `grpc-gateway` trong `buf.gen.yaml` và khởi tạo một lớp HTTP mux, custom marshaler cùng error handler nhỏ.
→ Không bật plugin `grpc-gateway` của Kong, không gắn `google.api.http` cho RPC nội bộ khi chưa được duyệt, và không mở public REST route lệch chuẩn envelope của repo nếu chưa có ADR mới cho contract riêng biệt.

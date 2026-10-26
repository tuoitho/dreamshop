# ADR-006: Error model xuyên gRPC/REST và idempotency semantics

## Status

Accepted

## Context

Client cần xử lý lỗi nhất quán qua BFF, trong khi microservice dùng gRPC. Nếu chỉ trả string/exception tùy service, UI không biết nên retry hay hiển thị lỗi nghiệp vụ; log cũng khó liên kết. Idempotency key bị xử lý như một conflict chung sẽ làm retry mạng của client trở nên sai.

## Decision

gRPC dùng status code chuẩn và domain `reason` có version, ưu tiên `google.rpc.ErrorInfo`. BFF map code sang REST envelope:

```json
{
  "code": "INVENTORY_INSUFFICIENT",
  "message": "Số lượng còn lại không đủ để đặt hàng.",
  "traceId": "01J..."
}
```

Không gửi stack trace hoặc internal detail cho client. Cùng `Idempotency-Key` và request fingerprint trả lại response/status của request đầu tiên trong thời hạn lưu (mặc định đề xuất 24 giờ). Chỉ cùng key nhưng payload khác mới trả `409` với `IDEMPOTENCY_KEY_REUSED`.

## Consequences

- Được: REST client có contract ổn định, gRPC caller có tín hiệu retry/failure rõ và support có thể tìm trace bằng `traceId`.
- Được: retry client không tạo duplicate order hay nhận false conflict.
- Mất: cần duy trì mapping table, error reason catalogue, fingerprint storage và policy retention.
- Trade-off: message cho client có thể ít chi tiết hơn log nội bộ; đây là chủ đích bảo vệ bảo mật và giữ compatibility.

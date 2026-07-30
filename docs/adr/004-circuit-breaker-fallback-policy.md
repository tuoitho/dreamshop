# ADR-004: Circuit breaker và fallback theo mức an toàn nghiệp vụ

## Status

Accepted

## Context

Synchronous gRPC call tới Catalog hoặc Inventory có thể timeout/down. Nếu callers tiếp tục chờ và retry vô hạn, application threads/connections có thể cạn và gây cascade failure. Tuy nhiên fallback sai, như dùng giá `0` khi Catalog không sẵn sàng, còn nguy hiểm hơn fail request.

## Decision

Mọi outbound gRPC/HTTP call có deadline rõ ràng, bounded retry với exponential backoff + jitter chỉ cho operation an toàn/idempotent, circuit breaker ở client và giới hạn concurrency/bulkhead. Timeout budget của downstream ngắn hơn upstream.

Fallback được quyết định theo use case:

- Product page read-only có thể trả cache/stale data với trạng thái rõ ràng.
- Create order, price snapshot, stock reservation và write command fail closed; trả stable error (`PRODUCT_UNAVAILABLE`, `UNAVAILABLE`/503 hoặc deadline/504), không bịa data.
- Event consumer retry theo policy broker/outbox, không lặp retry đồng bộ vô hạn.

## Consequences

- Được: giảm cascade failure và làm hành vi khi dependency down có thể dự đoán/quan sát.
- Được: tách circuit breaker (bảo vệ caller) khỏi business fallback (bảo vệ correctness).
- Mất: cần cấu hình threshold, timeout, retry budget và dashboard để tránh false-open/false-retry.
- Trade-off: một số request sẽ bị từ chối sớm thay vì chờ lâu; đây là lựa chọn ưu tiên tính đúng đắn và recovery của hệ thống.

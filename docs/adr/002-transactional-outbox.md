# ADR-002: Transactional outbox cho domain events

## Status

Accepted

## Context

Order và Inventory cần thay đổi state trong PostgreSQL rồi phát event qua RabbitMQ. Hai thao tác này không có atomic commit chung; nếu application crash sau DB commit nhưng trước publish, workflow có thể mất event. RabbitMQ cũng giao hàng at-least-once nên consumer có thể thấy duplicate sau retry/re-delivery.

## Decision

Mỗi service phát event ghi một outbox record trong cùng transaction với state domain. Một relay/publisher đọc outbox, publish với publisher confirm, và retry có backoff đến khi thành công. Consumer lưu inbox/dedup theo `event_id` trong transaction cùng side effect; event không xử lý được sau retry hữu hạn được chuyển DLQ.

Event có `event_id`, `event_type`, schema version, `occurred_at`, aggregate ID và trace context. Không dùng distributed transaction/2PC cho PostgreSQL và RabbitMQ.

## Consequences

- Được: không mất event do crash giữa DB commit và broker publish; re-delivery không double-reserve nếu consumer đúng.
- Được: có audit trail và cơ chế replay có kiểm soát.
- Mất: thêm bảng, relay, monitoring backlog/DLQ và eventual consistency.
- Trade-off: delivery là at-least-once, không phải exactly-once; database invariant và idempotency vẫn là bắt buộc.

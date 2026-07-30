# ADR-005: Saga compensation và vòng đời stock reservation

## Status

Accepted

## Context

Inventory reservation là asynchronous. Event có thể duplicate, đến muộn, hoặc Order đã bị cancel bởi timeout/reconciliation trước khi nhận được `InventoryReserved`. Chỉ bỏ qua event muộn ở Order sẽ để một reservation bị kẹt, làm available stock thấp hơn thực tế.

## Decision

MVP dùng state machine `PENDING_STOCK → CONFIRMED | CANCELLED`. Inventory chỉ tạo một reservation cho mỗi order. Nếu Order ở `CANCELLED` nhận `InventoryReserved`, Order ghi outbox `ReservationReleaseRequested` một cách idempotent; Inventory release reservation idempotent và phát `InventoryReleased`.

Một reconciliation job quét `PENDING_STOCK` quá hạn. Job cancel order theo state machine, tạo compensation khi cần và gửi alert/DLQ khi event không thể gắn với order sau retry hữu hạn. Payment được thêm sau này bằng state mới, không đổi nghĩa `CONFIRMED` của MVP.

## Consequences

- Được: late event không gây stock leak và terminal state không bị mở lại.
- Được: cơ chế recovery có thể test bằng event replay, consumer crash và manual timeout.
- Mất: thêm event, state transition, job reconciliation và test matrix.
- Trade-off: Saga vẫn eventual consistent trong thời gian ngắn; correctness dựa trên idempotency, compensation và inventory DB invariant thay vì distributed transaction.

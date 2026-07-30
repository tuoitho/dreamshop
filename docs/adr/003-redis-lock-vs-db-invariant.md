# ADR-003: Database invariant là cơ chế chống oversell chính

## Status

Accepted

## Context

Flash-sale/concurrent checkout có thể khiến nhiều consumer cố reserve cùng SKU. Redis distributed lock hấp dẫn vì nhanh nhưng có lease expiry, process pause/network partition và không đồng bộ transaction với inventory database. Nếu database không tự bảo vệ invariant, lock failure có thể làm stock âm.

## Decision

Inventory Service bảo vệ stock bằng database transaction và conditional update/row-level invariant, ví dụ chỉ reserve khi `available >= requested_quantity`, đồng thời đặt unique constraint để một order chỉ có một reservation. Redis lock không phải điều kiện đúng đắn và không được dùng thay DB check.

Redis lock chỉ là thử nghiệm bổ sung cho hot SKU sau khi test database invariant đã xanh; nếu dùng phải có owner token, TTL ngắn, observability và không được cho phép commit bypass database condition.

## Consequences

- Được: correctness nằm cùng nơi với dữ liệu source-of-truth và survive broker/Redis retry.
- Được: test concurrency đơn giản, có thể kiểm chứng bằng SQL invariant.
- Mất: contention DB tăng ở SKU nóng; cần index, transaction scope ngắn và load test.
- Trade-off: Redis lock vẫn có thể giảm contention trong một số demo, nhưng hệ thống vẫn đúng khi Redis lock unavailable hoặc expired.

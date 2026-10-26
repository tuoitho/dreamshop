# ADR-001: Service boundaries cho MVP e-commerce

## Status

Accepted (public routing updated by [ADR-008](008-hybrid-rest-bff-grpc-transcoding.md); `cart-service` added as the sixth service by [ADR-009](009-cart-service.md))

## Context

Roadmap chỉ có 8–12 tuần nhưng cần thực hành Java, Go, gRPC, RabbitMQ, Redis, Kong và first-party authentication trong một hệ thống có failure mode thật. Tách quá nhiều service (cart, payment, review, notification, search…) sẽ làm phần consistency và observability bị nông; giữ một monolith lại không đủ để thực hành giao tiếp phân tán.

## Decision

MVP có năm service:

- BFF (Go) là REST adapter cho commerce API của client.
- Identity (Java/Spring Boot/Spring Security) sở hữu credential, profile, role, session/refresh token và JWT issuer.
- Catalog (Go) sở hữu product data và cache.
- Order (Java/Spring Boot) sở hữu order state, idempotency và outbox.
- Inventory (Go) sở hữu stock/reservation và event consumer.

Mỗi service sở hữu database/schema riêng, không query hay foreign key xuyên service. Kong expose BFF và Identity Service: chỉ `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/auth/refresh` là anonymous; profile và commerce endpoint cần JWT. Microservice nội bộ giao tiếp trực tiếp bằng gRPC hoặc RabbitMQ.

## Consequences

- Được: phạm vi vẫn đủ nhỏ để hoàn thành, nhưng có synchronous call, event workflow, data ownership, custom authentication và mixed-language development.
- Được: BFF không trở thành internal message bus; public attack surface nhỏ hơn.
- Mất: chưa có payment, cart, notification, review hay search service thật.
- Trade-off: các feature bị hoãn chỉ được tách thành service khi có ownership/data/workflow rõ, không tách chỉ vì “microservices”.

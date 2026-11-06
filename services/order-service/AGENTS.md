# Order Service Instructions

These instructions apply to `services/order-service/` in addition to the root
[`AGENTS.md`](../../AGENTS.md).

## Ownership

This service owns:

- Order creation and order lifecycle.
- Order state transitions and order-owned snapshots.
- Command idempotency and replayed outcome semantics.
- Transactional outbox state for Order-owned changes.

This service does not own:

- Inventory stock, availability, or reservation invariants.
- Catalog product descriptions or current pricing truth.
- Identity credentials, profiles, sessions, or token issuance.
- Payment processing, which is outside the current service's ownership.

An order may retain immutable facts accepted at creation without becoming the
source of truth for Catalog or Inventory. Other services consume Order contracts
or events; they do not update Order tables or state transitions directly.

## Lifecycle and Workflow Rules

- Model allowed state transitions explicitly and reject invalid transitions.
- Keep a local transaction inside Order persistence.
- Store an idempotency key, request fingerprint, and reusable original outcome
  for retry-safe mutation commands.
- Persist Order state and its outbox record atomically when publishing an event.
- Treat event publication, delivery, and consumption as at-least-once.
- Interact with Identity, Catalog, Inventory, and Payment through direct private
  gRPC or approved RabbitMQ events (`ecommerce.events`).
- Use compensation/reconciliation or a saga/workflow for multi-service partial
  failure; do not use a distributed database transaction.
- Do not hold an Order database transaction open across a network call (including
  gRPC calls and RabbitMQ publisher confirms).
- Keep MoMo gateway credentials and HMAC signing inside `payment-service`, never
  inside `order-service`.

Define timeout, retry, cancellation, and compensation behavior for every
synchronous dependency. Preserve correlation across command, outbox, event, and
downstream response.

## Java Structure

The service is currently a Spring Boot skeleton. As capabilities are introduced,
keep responsibilities separated:

- Domain types enforce lifecycle and invariant rules.
- Application services coordinate Order use cases and dependency contracts.
- Controllers/gRPC handlers validate transport input and map outcomes.
- Persistence and messaging adapters translate Order-owned state and events.

Domain/application logic must not depend directly on controllers, broker APIs,
JPA entities, or another service implementation. Use Java 21 and Lombok for
boilerplate; use `camelCase` in Java and `snake_case` in database columns.

## Contracts and Errors

- Use generated Java types from `contracts/java/`; do not edit generated code.
- Prefer additive protobuf/event changes and inspect BFF/Inventory consumers.
- Keep public mutation success messages meaningful.
- Map business failures to stable error reasons/codes and keep stack traces out
  of client responses.
- Preserve the idempotency semantics in ADR-006.

## Verification

During Red/Green, run the changed method from this service directory:

```powershell
mvn test '-Dtest=OrderApplicationTest#isolatesActuatorOnDedicatedManagementPort'
```

Then run its test class:

```powershell
mvn test -Dtest=OrderApplicationTest
```

Finally run the affected reactor from repository root:

```powershell
mvn -pl services/order-service -am test
```

Use plain JUnit 5/Mockito for lifecycle, idempotency, and application logic.
Prefer focused Spring tests for transport/framework boundaries. Use
Testcontainers only when database, migration, concurrency, or broker behavior
requires real infrastructure.

## Related Steering

- [Architecture](../../docs/steering/architecture.md)
- [Service boundaries](../../docs/steering/service-boundaries.md)
- [API contracts](../../docs/steering/api-contracts.md)
- [Change impact](../../docs/steering/change-impact.md)
- [Testing](../../docs/steering/testing.md)
- [Delivery](../../docs/steering/delivery.md)

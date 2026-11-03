# Inventory Service Instructions

These instructions apply to `services/inventory-service/` in addition to the
root [`AGENTS.md`](../../AGENTS.md).

## Ownership

This service owns:

- Stock quantities and availability invariants.
- Reservation creation, expiration, confirmation, and release.
- Inventory state transitions and inventory event consumption.

This service does not own:

- Catalog product descriptions or pricing.
- Order lifecycle or order state.
- Identity credentials, profiles, or sessions.
- Client-specific composition or presentation.

Order and other services request or observe inventory behavior through approved
gRPC/event contracts. They do not write stock tables or reuse Inventory source.

## Invariants and Workflows

- Keep stock and reservation invariants atomic inside Inventory persistence.
- Never hold an Inventory transaction open across a network call.
- Make reservation commands idempotent using stable request identity and outcome
  semantics.
- Make event consumers safe against retry, redelivery, and out-of-order delivery
  within the contract's stated ordering guarantees.
- Record processed-event or business identities when duplicate side effects must
  be prevented.
- Publish inventory facts after local state commits, preferably through a
  transactional outbox when persistence and event publication are introduced.
- Change Order state through an event/API contract; never access Order storage.

Define reservation TTL, retry eligibility, compensation, and reconciliation
behavior as domain/contract semantics rather than transport defaults.

## Current Layout

- `cmd/server/`: process startup, signal handling, and dependency wiring.
- `internal/config/`: environment-backed configuration (`HTTP_ADDR`,
  `GRPC_ADDR`, `DATABASE_URL`, `IDENTITY_GRPC_ADDR`, `IDENTITY_GRPC_TIMEOUT`,
  `SEED_DEMO_DATA`).
- `internal/authorization/`: private gRPC client to Identity authorization (`inventory.view`, `inventory.update`, plus `product.update` or `inventory.update` for `DeleteStocks`).
- `internal/domain/`: SKU-keyed stock (`SkuID`, `ProductID`) and availability domain rules (`Quantity - Reserved`, `ErrStockBelowReserved`, `VariantHasReservedStockError`).
- `internal/store/`: embedded SQL migrations (`000001_create_stock.up.sql`, `000002_refactor_stock_to_sku.up.sql`),
  seed data, and PostgreSQL store adapter.
- `internal/grpc/`: `InventoryServiceServer` (`GetAvailability`, `BatchGetAvailability`, `GetStock`, `BatchGetStock`, `ListStocks`, `SetStock`, `AdjustStock`, `DeleteStocks`) gRPC handler.
- `internal/http/`: `/healthz` HTTP readiness/liveness handler.

As reservation and event capabilities are added, keep domain/application rules
independent of HTTP, gRPC, RabbitMQ, and database adapters. Broker and
persistence adapters implement service-owned interfaces and translate external
failures into stable application outcomes.

## Contracts and Data

- Use generated types from `contracts/go/`; do not edit generated files.
- Prefer additive protobuf/event changes and inspect `cart-service` (current
  `BatchGetAvailability` consumer) plus future Order or BFF consumers.
- Keep Inventory persistence/schema/migrations private to this service.
- A local product reference is a read-model key, not ownership of Catalog data.
- Preserve event metadata, correlation, and stable error reasons at boundaries.

## Go Conventions

- Use Go 1.24 and standard formatting.
- Use `PascalCase` for exported and `camelCase` for unexported identifiers.
- Propagate `context.Context` through storage, broker, and outbound calls.
- Wrap errors with operation context while preserving retry/business
  classification.
- Keep `_test.go` files beside the package under test.

## Verification

From this service directory:

```powershell
go test ./...
```

From repository root:

```powershell
go test ./services/inventory-service/...
```

Test stock calculations, reservation transitions, idempotency, and duplicate
event handling without infrastructure first. Add integration tests for database
concurrency, migrations, RabbitMQ delivery, and gRPC mapping when those adapters
exist.

## Related Steering

- [Architecture](../../docs/steering/architecture.md)
- [Service boundaries](../../docs/steering/service-boundaries.md)
- [API contracts](../../docs/steering/api-contracts.md)
- [Change impact](../../docs/steering/change-impact.md)
- [Testing](../../docs/steering/testing.md)
- [Delivery](../../docs/steering/delivery.md)

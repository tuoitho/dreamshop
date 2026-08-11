# Catalog Service Instructions

These instructions apply to `services/catalog-service/` in addition to the root
[`AGENTS.md`](../../AGENTS.md).

## Ownership

This service owns:

- Product and catalog data.
- Catalog query behavior.
- Catalog cache behavior and invalidation policy.

This service does not own:

- Inventory quantities, availability, or reservations.
- Order lifecycle or order state.
- Identity credentials, profiles, or sessions.
- Client-specific composition and presentation.

Other services consume catalog contracts, APIs, or events. They do not import
Catalog implementation code or read its persistence/cache directly.

## Domain and Data Boundaries

- Keep product identity and descriptive catalog truth inside this service.
- Do not infer stock availability from catalog data; Inventory owns that fact.
- Do not embed order workflow or identity authorization rules in catalog logic.
- Cache entries are derived data. Define their source, key, TTL/invalidation,
  stale-read tolerance, and recovery behavior.
- Catalog persistence, cache namespaces, and migrations are owned here and are
  not shared as runtime integration surfaces.

Expose behavior through protobuf/gRPC contracts or approved events. Use additive
contract changes and inspect BFF or other consumers before changing field
semantics.

## Current Layout

- `cmd/server/`: process startup and dependency wiring.
- `internal/config/`: environment-backed configuration.
- `internal/http/`: health and HTTP transport concerns.

As product capabilities are added, keep domain/application logic independent of
HTTP, gRPC, cache, and database adapters. Infrastructure adapters depend on
service-owned interfaces; domain code does not depend on Redis, a database
driver, or transport-generated types.

Configuration must be explicit and environment-backed. Validate required values
at startup and keep local defaults only when they are safe and documented.

## Go Conventions

- Use Go 1.24 and standard formatting.
- Export identifiers only across package boundaries.
- Use `PascalCase` for exported and `camelCase` for unexported identifiers.
- Propagate `context.Context` through storage, cache, and outbound calls.
- Wrap errors with useful operation context without discarding classification.
- Keep `_test.go` files beside the package under test.

## Verification

From this service directory:

```powershell
go test ./...
```

From repository root:

```powershell
go test ./services/catalog-service/...
```

Test product rules and cache policy without infrastructure first. Add adapter
integration tests for behavior that depends on a real database, Redis, gRPC, or
serialization boundary.

## Related Steering

- [Architecture](../../docs/steering/architecture.md)
- [Service boundaries](../../docs/steering/service-boundaries.md)
- [API contracts](../../docs/steering/api-contracts.md)
- [Change impact](../../docs/steering/change-impact.md)
- [Testing](../../docs/steering/testing.md)
- [Delivery](../../docs/steering/delivery.md)

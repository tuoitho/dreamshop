# BFF Service Instructions

These instructions apply to `services/bff-service/` in addition to the root
[`AGENTS.md`](../../AGENTS.md).

## Ownership

This service owns:

- Client-specific commerce composition.
- REST adaptation for workflows that need UI-specific shapes.
- Custom mapping from internal gRPC outcomes to the public REST contract.
- Edge orchestration that does not belong to one domain service.

This service does not own:

- Catalog product rules or data.
- Inventory availability or reservation invariants.
- Cart persistence or enrichment rules.
- Order lifecycle or state transitions.
- Identity credentials, profiles, sessions, or token issuance.

Keep BFF thin. It coordinates owning services and shapes client responses; it
does not become a second implementation of their domain rules.

## Boundaries and Routing

- Call domain services directly over private gRPC.
- Do not call internal services through Kong or a public transcoding route.
- Do not access another service's database, repository, or implementation code.
- Use generated contracts from `contracts/go/`; do not copy protobuf-generated
  types into this service.
- Keep public REST/JSON fields and envelopes aligned with
  [`api-contracts.md`](../../docs/steering/api-contracts.md).
- Preserve correlation and stable error codes when mapping gRPC failures.

Use BFF only when a route needs composition, client-specific response shaping,
custom error mapping, or orchestration. A reviewed unary RPC that maps directly
to REST may use Kong transcoding under ADR-008 instead.

## Current Layout

- `cmd/server/`: process startup and dependency wiring.
- `internal/config/`: environment-backed configuration and repository gateway/Compose contract tests.
- `internal/catalog/`: outbound gRPC client for `CatalogService`.
- `internal/inventory/`: outbound gRPC client for `InventoryService`.
- `internal/cart/`: outbound gRPC client for `CartService`.
- `internal/http/`: canonical `/api/v1` HTTP handlers (storefront products, cart, favorites, admin product soft-delete/restore), caller context, trace correlation, and response/error mapping.

Keep domain-service clients and adapters under internal packages. Handlers
validate transport input, invoke orchestration/application logic, and map
results; they do not contain downstream domain implementations.

Configuration must be loaded through the config package and environment
variables. Keep defaults suitable for local development and validate values that
are required for a safe startup.

## Go Conventions

- Use Go 1.24 and standard formatting.
- Export identifiers only when another package needs them.
- Keep unexported names in `camelCase` and exported names in `PascalCase`.
- Wrap errors with operation context; preserve error identity when callers need
  classification.
- Accept `context.Context` for outbound calls and propagate cancellation,
  deadlines, and correlation metadata.
- Place `_test.go` files beside the package under test.

## Verification

From this service directory:

```powershell
go test ./...
```

From repository root:

```powershell
go test ./services/bff-service/...
```

Add focused tests for config, handlers, composition, timeout/error mapping, and
contract envelopes. Use integration tests only when real gRPC/HTTP behavior is
required.

## Related Steering

- [Architecture](../../docs/steering/architecture.md)
- [Service boundaries](../../docs/steering/service-boundaries.md)
- [API contracts](../../docs/steering/api-contracts.md)
- [Change impact](../../docs/steering/change-impact.md)
- [Testing](../../docs/steering/testing.md)
- [Delivery](../../docs/steering/delivery.md)

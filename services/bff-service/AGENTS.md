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
- Do not call internal services through Kong or another service's
  HTTP/`grpc-gateway` route.
- Do not access another service's database, repository, or implementation code.
- Use generated contracts from `contracts/go/`; do not copy protobuf-generated
  types into this service.
- Keep public REST/JSON fields and envelopes aligned with
  [`api-contracts.md`](../../docs/steering/api-contracts.md).
- Preserve correlation and stable error codes when mapping gRPC failures.

First-party frontend commerce workflows and any route needing composition,
client-specific response shaping, custom error mapping, or orchestration belong
in BFF; do not mix BFF calls and direct domain-service calls within the same
frontend feature. When a standalone or external REST endpoint is approved to
bypass BFF, the owning Go service exposes it via service-side Go `grpc-gateway`
behind standard Kong HTTP proxying under [ADR-011](../../docs/adr/011-service-side-grpc-gateway-transcoding.md)
(never Kong's `grpc-gateway` plugin).

## Current Layout

- `cmd/server/`: process startup and dependency wiring.
- `internal/config/`: environment-backed configuration and repository gateway/Compose contract tests.
- `internal/catalog/`: outbound gRPC client for `CatalogService`.
- `internal/inventory/`: outbound gRPC client for `InventoryService`.
- `internal/cart/`: outbound gRPC client for `CartService`.
- `internal/order/`: outbound gRPC client for `OrderService`.
- `internal/payment/`: outbound gRPC client for `PaymentService`.
- `internal/http/`: canonical `/api/v1` HTTP handlers (storefront products with per-SKU variants, SKU-keyed cart with `productVersion`, favorites, orders CRUD/cancellation, public MoMo redirect/IPN callbacks, admin products with 2-phase unreserved SKU stock cleanup & media presign, admin SKU inventory `/api/v1/admin/inventory` and `/api/v1/admin/inventory/skus/{skuId}`), caller context, trace correlation, and response/error mapping.

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

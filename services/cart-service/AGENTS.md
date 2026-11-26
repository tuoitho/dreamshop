# Cart Service Instructions

These instructions apply to `services/cart-service/` in addition to the root
[`AGENTS.md`](../../AGENTS.md).

## Ownership

This service owns:

- Authenticated user shopping cart state, line-item quantities, and saved
  product favorites (`favorite_items`).
- Cart and favorites persistence in `cart_db`, migrations, and `sqlc` query
  bindings.
- Enriched cart and favorite projections assembled by querying Catalog and
  Inventory over private gRPC.
- Enforcement of `cart.view`, `cart.item.add`, `cart.item.update`,
  `cart.item.remove`, and `cart.clear` on Cart RPCs and `favorite.manage` on
  favorite RPCs evaluated via Identity's private authorization gRPC boundary.

This service does not own:

- Catalog product descriptions, pricing truth, or soft-delete state.
- Inventory stock truth or reservation lifecycle.
- Order creation, checkout submission, or order state transitions.
- Identity credentials, profiles, sessions, or role hierarchy storage.
- Public REST adaptation, which belongs to `bff-service`.

## Boundaries and Dependencies

- Expose cart operations over private gRPC (`127.0.0.1:50053` locally;
  `0.0.0.0:50053` on the internal container network); do not expose Cart
  directly through Kong.
- Persist only cart-owned identifiers (`user_id`, `product_id`,
  `sku_id`) and quantities in `cart_db`. Never copy or join Catalog or
  Inventory tables.
- Call Catalog (`127.0.0.1:50051`), Inventory (`127.0.0.1:50052`), and Identity
  authorization (`127.0.0.1:50054`, default `500ms` timeout) over direct
  private gRPC.
- Start `identity-service` before `cart-service`. On Cart and Favorite RPCs, if
  Identity authorization is unavailable, reject with `Unavailable` (mapped to
  `503` by BFF) without reading or mutating cart state; if permission is denied,
  return `PermissionDenied` (`403`).
- Use generated protobuf contracts from `contracts/go/`; do not edit generated
  contract or `sqlc` files by hand.

## Current Layout

- `cmd/server/`: process startup, signal handling, and dependency wiring.
- `internal/config/`: environment-backed configuration (`HTTP_ADDR`,
  `GRPC_ADDR`, `DATABASE_URL`, and downstream gRPC addresses/timeouts).
- `internal/domain/`: cart and line-item validation rules.
- `internal/store/`: PostgreSQL migrations (`000001`..`000004`), `sqlc` queries, and persistence
  adapter including idempotent post-order cart cleanup (`processed_events`).
- `internal/events/`: RabbitMQ consumer (`cart.order-events`) removing ordered cart items on `OrderPlaced` and `OrderConfirmed`, deduplicated once per order.
- `internal/catalog/`, `internal/inventory/`, `internal/authorization/`:
  outbound gRPC clients.
- `internal/grpc/`: `CartServiceServer` gRPC handler implementation.
- `internal/http/`: `/healthz` HTTP readiness/liveness handler.

## Go Conventions

- Use Go 1.24 and standard formatting.
- Use `PascalCase` for exported and `camelCase` for unexported identifiers.
- Propagate `context.Context` and configured deadlines across database and
  outbound gRPC calls.
- Keep `_test.go` files beside the package under test.

## Verification

From repository root, run the surface gate:

```powershell
make verify SERVICE=cart-service
```

Or run focused package tests directly:

```powershell
go test ./services/cart-service/internal/...
```

On Windows, `cmd/server` tests are skipped by the surface gate because they send
Unix `SIGTERM` via `syscall.Kill`; Linux and CI run the full
`./services/cart-service/...` suite.

## Related Steering

- [Architecture](../../docs/steering/architecture.md)
- [Service boundaries](../../docs/steering/service-boundaries.md)
- [API contracts](../../docs/steering/api-contracts.md)
- [Change impact](../../docs/steering/change-impact.md)
- [Testing](../../docs/steering/testing.md)
- [Delivery](../../docs/steering/delivery.md)

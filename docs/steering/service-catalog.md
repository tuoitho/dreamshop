# Service Catalog

This catalog describes the current local and container topology. It separates
implemented runtime behavior from partial capabilities, provisioned resources,
and accepted architecture that is not wired yet.

## Status Vocabulary

- `implemented`: the current process exposes and uses the documented scope.
- `partial`: production code exists for part of the owned capability; the
  missing behavior is stated explicitly.
- `configured`: deployment or local configuration provisions the capability,
  but application code does not currently consume it.
- `skeleton`: the process and health surface exist without owned business
  behavior.

## Application Surfaces

| Surface | Ownership | Runtime | Local/container ports | Health | Exposure | Datastore/resources | Inbound | Outbound | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| identity-service | Credentials, profiles, roles, sessions, refresh tokens, JWT issuance, and current RBAC authorization evaluation | Java 21 / Spring Boot 4.1 | HTTP `8080`; private gRPC local `127.0.0.1:50054`, container `0.0.0.0:50054` | `/actuator/health` | Auth, users, and locations through Kong; authorization gRPC has no Kong route | PostgreSQL `identity_db`, Flyway, RSA signing keys | Frontend through Kong; Cart over private gRPC | PostgreSQL | implemented |
| bff-service | Client-specific commerce REST adaptation | Go 1.24 | Local HTTP `8081`; container default `8080` | `/healthz` | `/api/v1/storefront/products`, `/api/v1/cart`, and local `/api/v1/admin/products` through Kong | Stateless | Frontend through Kong | Catalog gRPC `50051`; Cart gRPC `50053` | implemented |
| catalog-service | Product detail, search, filtering, pagination, and soft-delete | Go 1.24 | Canonical local HTTP `8085`; default/container HTTP `8081`; gRPC `50051` | `/healthz` | Private/local; no direct Kong route | PostgreSQL `catalog_db`, migrations, seed data | BFF and Cart over gRPC | PostgreSQL | implemented |
| inventory-service | Stock quantities and availability lookup | Go 1.24 | HTTP `8082`; gRPC `50052` | `/healthz` | Private/local; no Kong route | PostgreSQL `inventory_db`, migration, seed data | Cart over gRPC | PostgreSQL | partial: reservations and RabbitMQ workflows are not implemented |
| order-service | Planned order lifecycle, idempotency, and transactional outbox ownership | Java 21 / Spring Boot 4.1 | HTTP `8083` | `/actuator/health` | Private/local; no Kong route | `order_db` is provisioned but unused | None | None | skeleton |
| cart-service | Authenticated user cart lines and enriched cart projection | Go 1.24 | HTTP `8084`; gRPC `50053` | `/healthz` | Private service; REST is adapted by BFF | PostgreSQL `cart_db`, migration, sqlc queries | BFF over gRPC | Catalog gRPC `50051`; Inventory gRPC `50052`; Identity authorization `50054`; PostgreSQL | implemented |
| ecommerce-fe | Browser storefront and authenticated cart experience | React 19 / Vite 8 / TypeScript 6 | Local Vite `5173`; container Nginx `8080` (`/healthz`) | Vite dev server / `/healthz` | Local developer UI on `5173`; production SPA through Kong `/` | Browser state only | Browser users | Local Kong `http://localhost:8000/api/v1`; container relative `/api/v1` | implemented; order submission is not available |

## Platform Infrastructure

| Component | Ports | Exposure | Consumers | Status |
| --- | --- | --- | --- | --- |
| Kong API Gateway | Local proxy `8000` (production host `${PUBLIC_HTTP_PORT:-80}` -> `8000`); admin `8001` | Public ingress; admin local-only (`127.0.0.1:8001` in production) | Frontend, Identity, BFF | configured |
| PostgreSQL | `5432` | Local/container network | Identity, Catalog, Inventory, Cart; Order database is provisioned but unused | configured |
| Redis | `6379` | Local/container network | No application consumer yet; reserved for catalog/cache experiments | configured |
| RabbitMQ | AMQP `5672`; management `15672` | Local/container network; management local-only | No application consumer or producer yet | configured |

## Route and Dependency Notes

- Kong exposes `/api/v1/auth` (anchored public auth endpoints in production) and
  protected `/api/v1` Identity routes.
- Local Kong exposes `/api/v1/storefront`, protected `/api/v1/cart`, and local
  `/api/v1/admin` routes to the host-run BFF at `host.docker.internal:8081`.
  Production Kong (`deployments/kong/kong-production.yaml`) exposes `/` (`ecommerce-fe`),
  `GET /api/v1/storefront/products`, and protected `/api/v1/cart` to container
  `bff-service:8080`, omitting `/api/v1/admin`.
- The local admin route does not currently attach `caller-context`; do not use
  that configuration as a production authorization boundary.
- Internal service calls do not route through Kong or the BFF. They use direct
  private gRPC. RabbitMQ remains provisioned for future asynchronous workflows.
- Cart calls Identity directly for authorization at `127.0.0.1:50054` with a
  `500ms` timeout. Start Identity before Cart; an authorization outage returns
  add-item `503` without a Cart mutation, while permission denial returns `403`.
- Container BFF configuration remains on port `8080`; the host-run local
  default is `8081` so it matches Kong without conflicting with Identity.
- Catalog defaults to HTTP `8081` when run alone. Set `HTTP_ADDR=:8085` in the
  complete local stack so BFF can retain `8081`; Catalog gRPC remains `50051`.
- Cart persists only cart-owned identifiers and quantities. It resolves product
  and availability projections from Catalog and Inventory without accessing
  their databases.

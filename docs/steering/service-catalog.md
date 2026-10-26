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
| identity-service | Credentials, profiles, shipping addresses, locations, roles, sessions, refresh tokens, JWT issuance, and current RBAC authorization evaluation | Java 21 / Spring Boot 4.1 | HTTP `8080`; management `8090`; private gRPC local `127.0.0.1:50054`, container `0.0.0.0:50054` | `:8090/actuator/health` | Canonical `/api/v1/auth`, `/api/v1/users`, `/api/v1/locations`, `/api/v1/admin/users`, and `/api/v1/admin/rbac` through Kong (`strip_path: false`); management `:8090` and authorization gRPC have no Kong route | PostgreSQL `identity_db`, Flyway, RSA signing keys | Frontend through Kong; Catalog and Cart over private gRPC | PostgreSQL | implemented |
| bff-service | Client-specific commerce REST adaptation | Go 1.24 | Local HTTP `8081`; container default `8080` | `/healthz` | Canonical `/api/v1/storefront/products`, `/api/v1/cart`, and `/api/v1/admin/products` through Kong (`strip_path: false`) | Stateless | Frontend through Kong | Catalog gRPC `50051`; Cart gRPC `50053` | implemented |
| catalog-service | Product detail, search, filtering, pagination, and soft-delete | Go 1.24 | Canonical local HTTP `8085`; default/container HTTP `8081`; gRPC `50051` | `/healthz` | Private/local; no direct Kong route | PostgreSQL `catalog_db`, migrations, seed data | BFF and Cart over gRPC | Identity authorization `50054`; PostgreSQL | implemented |
| inventory-service | Stock quantities and availability lookup | Go 1.24 | HTTP `8082`; gRPC `50052` | `/healthz` | Private/local; no Kong route | PostgreSQL `inventory_db`, migration, seed data | Cart over gRPC | PostgreSQL | partial: reservations and RabbitMQ workflows are not implemented |
| order-service | Planned order lifecycle, idempotency, and transactional outbox ownership | Java 21 / Spring Boot 4.1 | HTTP `8083`; management `8093` | `:8093/actuator/health` | Private/local; no Kong route | `order_db` is provisioned but unused | None | None | skeleton |
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

- Backend HTTP handlers declare canonical `/api/v1/...` paths directly, so Kong
  forwards routes 1:1 with `strip_path: false` using a single Kong `service` per
  upstream (`identity-service` and `bff-service`, plus `ecommerce-fe` in
  production).
- Kong exposes public `/api/v1/auth` routes (anchored endpoints in production)
  and protected `/api/v1/users`, `/api/v1/locations`, `/api/v1/admin/users`, and
  `/api/v1/admin/rbac` routes to `identity-service`.
- Local Kong exposes `/api/v1/storefront/products`, protected `/api/v1/cart`,
  and protected `/api/v1/admin/products` routes to the host-run BFF at
  `host.docker.internal:8081`. Production Kong
  (`deployments/kong/kong-production.yaml`) exposes `/` (`ecommerce-fe`),
  `GET /api/v1/storefront/products`, protected `/api/v1/cart`, and protected
  `POST /api/v1/admin/products` to container `bff-service:8080`.
- Spring Boot Actuator endpoints run on dedicated internal management ports
  (`8090` on `identity-service`, `8093` on `order-service`) and are not exposed
  on application HTTP ports or through Kong.
- Internal service calls do not route through Kong or the BFF. They use direct
  private gRPC. RabbitMQ remains provisioned for future asynchronous workflows.
- Catalog and Cart call Identity directly for authorization at `127.0.0.1:50054`
  locally (`identity-service:50054` in containers) with a `500ms` timeout. Start
  Identity before Catalog and Cart; an authorization outage returns `503`
  without mutating state, while permission denial returns `403`.
- Container BFF configuration remains on port `8080`; the host-run local
  default is `8081` so it matches Kong without conflicting with Identity.
- Catalog defaults to HTTP `8081` when run alone. Set `HTTP_ADDR=:8085` in the
  complete local stack so BFF can retain `8081`; Catalog gRPC remains `50051`.
- Cart persists only cart-owned identifiers (`user_id`, `product_id`,
  `variant_label`) and quantities. It resolves product and availability
  projections from Catalog and Inventory without accessing their databases.

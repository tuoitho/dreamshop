# Service Catalog

This catalog describes the current local and container topology. It separates
implemented runtime behavior from configuration and accepted architecture that
is not wired in the current skeletons.

## Status Vocabulary

- `implemented`: the current process exposes or uses the capability.
- `configured`: deployment or local configuration provisions the capability.
- `accepted but not wired`: architecture assigns ownership, but the current
  process does not implement the integration.

## Application Surfaces

| Surface | Ownership | Runtime | Local/container ports | Health | Exposure | Datastore/resources | Inbound | Outbound | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| identity-service | Credentials, profiles, roles, sessions, refresh tokens, JWT issuance | Java 21 / Spring Boot | Local and container `8080` | `/actuator/health` | Public routes through Kong; direct host port for local development | PostgreSQL `identity_db`, Flyway, signing keys | Browser/frontend through Kong; protected callers through Kong | PostgreSQL | implemented |
| bff-service | Client-specific commerce composition and REST adaptation | Go 1.24 | Local host `8081`; container `8080` | `/healthz` | Public `/api/v1/storefront` through Kong | None currently | Frontend through Kong | Catalog gRPC address `127.0.0.1:50051` is configured; Catalog gRPC server is not implemented | configured |
| catalog-service | Product and catalog data | Go 1.24 | HTTP `8081` | `/healthz` | Private/local; no Kong route | `catalog_db`, Redis/cache, and internal gRPC ownership | BFF/internal callers when implemented | PostgreSQL, Redis, internal gRPC contract | accepted but not wired |
| inventory-service | Stock, reservations, and inventory event consumption | Go 1.24 | HTTP `8082` | `/healthz` | Private/local; no Kong route | `inventory_db`, RabbitMQ/event ownership | Order/internal callers when implemented | PostgreSQL and RabbitMQ | accepted but not wired |
| order-service | Order lifecycle, idempotency, and transactional outbox ownership | Java 21 / Spring Boot | Local and container `8083` | `/actuator/health` | Private/local; no Kong route | `order_db`, idempotency, transactional outbox | Internal callers when implemented | Inventory, payment/event dependencies are accepted but not wired | accepted but not wired |
| ecommerce-fe | Browser commerce experience | React 19 / Vite 8 / TypeScript | Vite `5173` | Vite dev server | Local developer UI | None currently | Browser users | Kong proxy `http://localhost:8000` | implemented |

## Platform Infrastructure

| Component | Ports | Exposure | Consumers | Status |
| --- | --- | --- | --- | --- |
| Kong API Gateway | Proxy `8000`; admin `8001` | Local public ingress; admin local-only | Frontend, Identity, BFF | configured |
| PostgreSQL | `5432` | Local/container network | Identity, Order, Catalog, Inventory | configured |
| Redis | `6379` | Local/container network | Catalog cache ownership | configured |
| RabbitMQ | AMQP `5672`; management `15672` | Local/container network; management local-only | Inventory/event workflow ownership | configured |

## Route and Dependency Notes

- Kong exposes `/api/v1/auth` and protected `/api/v1` routes to Identity.
- Kong exposes `/api/v1/storefront` to the host-run BFF at
  `host.docker.internal:8081`.
- Internal service calls do not route through Kong or the BFF. They use direct
  private gRPC or RabbitMQ events when those integrations are implemented.
- Container BFF configuration remains on port `8080`; the host-run local
  default is `8081` so it matches Kong without conflicting with Identity.
- A service owning a database or integration in the accepted architecture does
  not imply that the current skeleton has wired that resource.


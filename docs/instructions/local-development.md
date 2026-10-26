# Local Development

Local development runs infrastructure in Docker Compose and application
processes directly on the Windows host. Use separate terminals for long-lived
servers.

## Infrastructure

```powershell
make infra-up
```

This starts PostgreSQL, Redis, RabbitMQ, and Kong. Stop it without deleting
volumes when finished:

```powershell
make infra-down
```

## Application processes

Start each long-lived process in a separate terminal. For the complete
storefront/cart flow, start Identity first, then the other gRPC dependencies
before Cart and BFF:

```powershell
make identity
$env:HTTP_ADDR=':8085'; $env:GRPC_ADDR='127.0.0.1:50051'; $env:DATABASE_URL='postgres://ecommerce:change-me-local-only@127.0.0.1:5432/catalog_db?sslmode=disable'; go run ./services/catalog-service/cmd/server
$env:HTTP_ADDR=':8082'; $env:DATABASE_URL='postgres://ecommerce:change-me-local-only@127.0.0.1:5432/inventory_db?sslmode=disable'; go run ./services/inventory-service/cmd/server
make cart
make bff
npm --prefix ecommerce-fe run dev
```

The provider-first order is Identity, then Catalog and Cart, then BFF. If
Identity authorization is unavailable, Catalog soft-delete and Cart add-item
return `503` with `AUTHORIZATION_SERVICE_UNAVAILABLE` without mutating state; a
permission denial returns `403`.

Order is independent of the current browser flow and can be started separately
with `make order`.

Identity uses HTTP `8080` (Actuator management `8090`); BFF `8081`; Inventory
`8082`; Order `8083` (Actuator management `8093`); Cart HTTP `8084`; Catalog
HTTP `8085`; and the frontend `5173`. Catalog's standalone HTTP default is
`8081`, so the complete stack must override it to `8085` as shown above.
Internal gRPC listeners are Catalog `50051`, Inventory `50052`, and Cart
`50053`. Identity authorization is `127.0.0.1:50054`, and both Catalog and Cart
use a `500ms` timeout. Container BFF remains on HTTP `8080`.

## Access paths

- Identity health: `http://localhost:8090/actuator/health`.
- BFF health: `http://localhost:8081/healthz`.
- Catalog health: `http://localhost:8085/healthz` in the complete stack.
- Inventory health: `http://localhost:8082/healthz`.
- Order health: `http://localhost:8093/actuator/health`.
- Cart health: `http://localhost:8084/healthz`.
- Frontend: `http://localhost:5173`.
- Public API ingress: `http://localhost:8000` through Kong.

The browser uses Kong for public API calls. Internal service calls do not route
through Kong: BFF calls Catalog and Cart directly over gRPC, Catalog calls
Identity authorization directly over gRPC, and Cart calls Catalog, Inventory,
and Identity authorization directly over gRPC. Kong routes to a host-run
Identity or BFF process through `host.docker.internal`; it never routes the
authorization RPC.

## Local test accounts

When running `ecommerce-fe` in Vite `development` mode (`make fe` or
`npm --prefix ecommerce-fe run dev`), `http://localhost:5173/login` prefills the
customer account and provides `DEV` preset buttons for switching between the two
standard local accounts (omitted from production builds):

| Role | Email | Password | Provisioning |
| --- | --- | --- | --- |
| `CUSTOMER`, `SUPER_ADMIN` | `admin@example.com` | `AdminPassword123!` | Created automatically on local `identity-service` startup unless `IDENTITY_BOOTSTRAP_SUPER_ADMIN_*` overrides it (disabled in `compose.production.yml`). |
| `CUSTOMER` | `customer1@gmail.com` | `123456123456` | Register once per `postgres_data` volume through Kong. |

To create `customer1@gmail.com` on a fresh local database after Kong and
Identity are healthy:

```powershell
'{"email":"customer1@gmail.com","password":"123456123456","fullName":"Customer One"}' |
  curl.exe -sS -X POST "http://localhost:8000/api/v1/auth/register" -H "Content-Type: application/json" --data-binary "@-"
```

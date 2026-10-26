# Local Development

Local development runs infrastructure in Docker Compose and application
processes directly on the Windows host. Use separate terminals for long-lived
servers.

## Supported prerequisites

- Java 21 and Maven 3.9 or newer.
- Go 1.24 or newer within the 1.24 line used by the workspace.
- Node.js 22.12 or newer for Vite 8, with npm from that Node release.
- Docker Desktop with Docker Compose v2 or newer.
- Buf CLI 1.70 or newer.
- GNU Make when using Makefile shortcuts.

Check the installed tools:

```powershell
java -version
mvn -version
go version
node --version
npm --version
docker compose version
buf --version
make --version
```

## First checkout

From the repository root:

```powershell
git clone <repository-url>
Set-Location microservice
make identity-keys
npm --prefix ecommerce-fe ci
buf generate
```

`make <service>` and `deployments/docker/compose.foundation.yml` provide built-in
local-only defaults; copying `.env.example` to `.env` is optional when overriding
foundation container credentials with `docker compose --env-file .env`. Never
commit `.env` or generated PEM files, and never reuse development keys outside
local development.

## Narrow smoke checks

Ensure Docker Desktop is running (required by `identity-service` Testcontainers
tests), then run only the checks needed for the first change:

```powershell
buf lint
go test ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/internal/...
mvn --batch-mode --no-transfer-progress test
npm --prefix ecommerce-fe test
```

Read the [service catalog](../steering/service-catalog.md) before starting a
service. Do not start every application for a domain-only change.

## Infrastructure

```powershell
make infra-up
```

This starts PostgreSQL, Redis, RabbitMQ, and Kong with named volumes
(`postgres_data`, `redis_data`, `rabbitmq_data`). Stop containers without
deleting persisted volumes when finished:

```powershell
make infra-down
```

## Application processes

Ensure `make identity-keys` has been run at least once, then start each
long-lived process in a separate terminal. For the complete storefront/cart
flow, start Identity first, then Catalog and Inventory before Cart and BFF:

```powershell
make identity
make catalog
make inventory
make cart
make bff
make fe
```

The provider-first order is Identity, then Catalog and Inventory, then Cart,
then BFF. If Identity authorization is unavailable, Catalog soft-delete and Cart
add-item return `503` with `AUTHORIZATION_SERVICE_UNAVAILABLE` without mutating
state; a permission denial returns `403`.

Order is independent of the current browser flow and can be started separately
with `make order`.

Identity uses HTTP `8080` (Actuator management `8090`); BFF `8081`; Inventory
`8082`; Order `8083` (Actuator management `8093`); Cart HTTP `8084`; Catalog
HTTP `8085`; and the frontend `5173`. Catalog's standalone HTTP default is
`8081`; `make catalog` overrides `HTTP_ADDR` to `:8085` for the complete stack.
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

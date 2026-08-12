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

```powershell
make identity
make order
pwsh -NoProfile -File scripts/start-bff-local.ps1
$env:HTTP_ADDR=':8081'; go run ./services/catalog-service/cmd/server
$env:HTTP_ADDR=':8082'; $env:DATABASE_URL='postgres://ecommerce:change-me-local-only@127.0.0.1:5432/inventory_db?sslmode=disable'; go run ./services/inventory-service/cmd/server
npm --prefix ecommerce-fe run dev
```

Run each server command in its own terminal. Identity uses host port `8080`,
BFF uses host port `8081`, Order uses `8083`, Catalog uses `8081`, Inventory
uses `8082`, and the frontend uses `5173`. BFF and Catalog therefore cannot
bind host port `8081` at the same time without an explicit local override.
Container BFF remains on `8080`.

## Access paths

- Identity health: `http://localhost:8080/actuator/health`.
- BFF health: `http://localhost:8081/healthz`.
- Catalog health: `http://localhost:8081/healthz` when Catalog owns the port.
- Inventory health: `http://localhost:8082/healthz`.
- Order health: `http://localhost:8083/actuator/health`.
- Frontend: `http://localhost:5173`.
- Public API ingress: `http://localhost:8000` through Kong.

The browser uses Kong for public API calls. Internal service calls do not route
through Kong or the BFF. Kong routes to a host-run service through
`host.docker.internal`.

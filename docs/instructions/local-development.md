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

The provider-first order is Identity, Cart, then BFF. If Identity authorization
is unavailable, Cart add-item returns `503` with
`AUTHORIZATION_SERVICE_UNAVAILABLE` and does not mutate the cart; a permission
denial returns `403`.

Order is independent of the current browser flow and can be started separately
with `make order`.

Identity uses HTTP `8080`; BFF `8081`; Inventory `8082`; Order `8083`; Cart
HTTP `8084`; Catalog HTTP `8085`; and the frontend `5173`. Catalog's standalone
HTTP default is `8081`, so the complete stack must override it to `8085` as
shown above. Internal gRPC listeners are Catalog `50051`, Inventory `50052`,
and Cart `50053`. Identity authorization is `127.0.0.1:50054`, and Cart uses a
`500ms` timeout. Container BFF remains on HTTP `8080`.

## Access paths

- Identity health: `http://localhost:8080/actuator/health`.
- BFF health: `http://localhost:8081/healthz`.
- Catalog health: `http://localhost:8085/healthz` in the complete stack.
- Inventory health: `http://localhost:8082/healthz`.
- Order health: `http://localhost:8083/actuator/health`.
- Cart health: `http://localhost:8084/healthz`.
- Frontend: `http://localhost:5173`.
- Public API ingress: `http://localhost:8000` through Kong.

The browser uses Kong for public API calls. Internal service calls do not route
through Kong: BFF calls Catalog and Cart directly over gRPC, while Cart calls
Catalog, Inventory, and Identity authorization directly over gRPC. Kong routes
to a host-run Identity or BFF process through `host.docker.internal`; it never
routes the authorization RPC.

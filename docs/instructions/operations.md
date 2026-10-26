# Local Operations Runbook

For rollout, migration, secrets, and observability policy, use
[`docs/steering/delivery.md`](../steering/delivery.md). This page covers local
diagnosis only.

## Health and logs

```powershell
Invoke-RestMethod http://localhost:8090/actuator/health
Invoke-RestMethod http://localhost:8081/healthz
Invoke-RestMethod http://localhost:8082/healthz
Invoke-RestMethod http://localhost:8093/actuator/health
Invoke-RestMethod http://localhost:8084/healthz
Invoke-RestMethod http://localhost:8085/healthz
Invoke-RestMethod http://localhost:8000/api/v1/auth/.well-known/jwks.json
make infra-logs
docker compose -f deployments/docker/compose.foundation.yml logs postgres
```

The complete stack uses Identity private authorization gRPC at
`127.0.0.1:50054`. It reserves `8081` for BFF and overrides Catalog HTTP to
`8085`. Catalog's standalone default remains `8081`. Kong routes are checked
through `8000`; direct health endpoints are local diagnostics.

## Kong routing

Inspect declarative services and routes through the local admin API:

```powershell
Invoke-RestMethod http://localhost:8001/services
Invoke-RestMethod http://localhost:8001/routes
```

Kong forwards to host-run services using `host.docker.internal`. A route failure
usually means the target process is stopped, the host port is occupied, or the
container cannot resolve the host gateway.

## Common failures

- Missing Identity keys: run `make identity-keys`; keep generated PEM files
  under `deployments/secrets/identity/` untracked. (Local `make <service>`
  targets inject local-only defaults automatically and do not require `.env`.)
- Port conflict: inspect the process using `Get-NetTCPConnection -LocalPort
  <port>` and stop the intended local process, or choose an explicit override.
- PostgreSQL not ready: inspect `docker compose -f deployments/docker/compose.foundation.yml logs postgres` and wait for `pg_isready` health.
- RabbitMQ broker unavailable: inspect the RabbitMQ logs and management UI at
  `http://localhost:15672`.
- Catalog gRPC unavailable: verify Catalog is listening on `127.0.0.1:50051`;
  both BFF product calls and Cart enrichment depend on it.
- Inventory gRPC unavailable: verify Inventory is listening on
  `127.0.0.1:50052`; Cart keeps its persisted lines but returns degraded or
  unavailable stock projections while the dependency is down.
- Identity authorization unavailable: verify Identity is listening on
  `127.0.0.1:50054` and Catalog and Cart have `IDENTITY_GRPC_ADDR=127.0.0.1:50054`.
  Diagnose this port and address directly, never through Kong. Catalog
  soft-delete and Cart add-item return `503` with
  `AUTHORIZATION_SERVICE_UNAVAILABLE` without mutating state during an outage;
  permission denial returns `403`.
- Cart gRPC unavailable: verify Cart is listening on `127.0.0.1:50053` and that
  BFF uses the same `BFF_CART_GRPC_ADDR`.
- Storefront route returns an upstream error: verify BFF was started with
  `make bff` (`BFF_HTTP_ADDR=:8081`, whereas its standalone default is `:8080`)
  so containerized Kong can reach `host.docker.internal:8081`.

## Safe shutdown

Use `make infra-down` for routine shutdown. Do not use volume deletion during
normal development; removing volumes destroys local database state.

# Local Operations Runbook

For rollout, migration, secrets, and observability policy, use
[`docs/steering/delivery.md`](../steering/delivery.md). This page covers local
diagnosis only.

## Health and logs

```powershell
Invoke-RestMethod http://localhost:8080/actuator/health
Invoke-RestMethod http://localhost:8081/healthz
Invoke-RestMethod http://localhost:8082/healthz
Invoke-RestMethod http://localhost:8083/actuator/health
Invoke-RestMethod http://localhost:8000/api/v1/auth/.well-known/jwks.json
make infra-logs
docker compose -f deployments/docker/compose.foundation.yml logs postgres
```

The `8081` health URL belongs to whichever host process currently owns that
port. Kong public routes are checked through `8000`; direct health endpoints
are local diagnostics.

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

- Missing `.env`: copy `.env.example` to `.env`; defaults are local-only.
- Missing Identity keys: run `make identity-keys`; keep generated PEM files
  under `deployments/secrets/identity/` untracked.
- Port conflict: inspect the process using `Get-NetTCPConnection -LocalPort
  <port>` and stop the intended local process, or choose an explicit override.
- PostgreSQL not ready: inspect `docker compose -f deployments/docker/compose.foundation.yml logs postgres` and wait for `pg_isready` health.
- RabbitMQ broker unavailable: inspect the RabbitMQ logs and management UI at
  `http://localhost:15672`.
- Catalog gRPC unavailable: the address is configured for BFF, but the current
  Catalog skeleton does not implement a gRPC server.

## Safe shutdown

Use `make infra-down` for routine shutdown. Do not use volume deletion during
normal development; removing volumes destroys local database state.

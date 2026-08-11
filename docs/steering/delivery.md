# Delivery Steering

Delivery changes preserve service ownership, compatibility during mixed-version
rollouts, recoverability, security, and operational visibility.

## Database Migrations

- A migration belongs to the service that owns the schema.
- Use the service's migration mechanism; Identity uses Flyway under
  `src/main/resources/db/migration/`.
- Never edit an applied migration. Add a new ordered migration.
- Test schema behavior against the supported PostgreSQL version when semantics
  matter.
- Do not add cross-service foreign keys, views, triggers, or migration queries.

For an incompatible data shape, prefer expand/migrate/contract:

1. **Expand:** deploy a schema/application version that supports old and new
   representations.
2. **Migrate:** backfill or reconcile existing data with observable progress and
   restart-safe behavior.
3. **Contract:** remove the old representation only after every deployed version
   and dependent consumer no longer needs it.

Define rollback behavior before applying destructive or irreversible operations.

## Configuration and Secrets

- Configuration enters through documented environment variables or service
  configuration files, not hard-coded environment values.
- Add new variables to the appropriate example/configuration documentation and
  deployment manifests in the same change.
- Validate required configuration at startup and fail with an actionable message.
- Never commit private keys, production credentials, refresh tokens, access
  tokens, or populated secret files.
- Generated Identity keys under `deployments/secrets/identity/` are local-only
  and must not be reused outside local development.
- Frontend `VITE_*` values are public build-time configuration; never put secrets
  in them.

## Local Runtime

Run PostgreSQL, Redis, RabbitMQ, and Kong in Docker Compose. Run application
services directly from Windows terminals or an IDE for local development.

```powershell
docker compose -f deployments/docker/compose.foundation.yml up -d
```

When containerized Kong proxies to a Windows-hosted service, configure the
upstream as `host.docker.internal:<port>`. Internal services still communicate
directly; they do not route through Kong or BFF.

Do not use volume-deleting shutdown commands as a routine development step. Data
removal requires an explicit need and a verified target.

## Deployment and Routing Impact

For each delivery change, identify:

- Affected service artifacts and version order.
- Database migrations and compatibility window.
- Environment variables, secret references, ports, and network dependencies.
- RabbitMQ topology, Redis behavior, or retained-state changes.
- Kong routes/plugins and public security policy.
- Health/readiness checks, dashboards, alerts, and rollback steps.

Kong is the public ingress. Public route changes must preserve the selection
rules in [ADR-008](../adr/008-hybrid-rest-bff-grpc-transcoding.md): Identity REST
may route directly, approved simple RPCs may be transcoded, and client-specific
composition/custom mapping belongs in BFF. Internal traffic does not use edge
routing.

## Rollout Safety

- Prefer backward-compatible producers and tolerant consumers.
- Deploy consumers that understand a new optional contract before producers
  begin relying on it.
- Version breaking events/APIs and run old/new forms through a defined migration
  window.
- Keep retries bounded and side effects idempotent during partial rollout.
- Define compensation or reconciliation for cross-service partial failure.
- Do not require all services to restart or deploy atomically.

## Observability

Every deployed service exposes meaningful health signals and enough telemetry to
diagnose boundary failures. Preserve correlation across REST, gRPC, events, and
background work.

- REST errors expose a client-safe `traceId`.
- Internal logs/traces carry the corresponding trace/correlation metadata.
- Events carry `event_id` and `correlation_id`.
- Logs identify the owning service and outcome without leaking secrets or tokens.
- Metrics distinguish success, business rejection, transient dependency failure,
  retry, duplicate handling, and compensation when relevant.

## Delivery Verification

Run the narrowest relevant checks first, then expand according to impact:

- Migration/repository integration tests for schema changes.
- `buf lint`, regeneration, and producer/consumer tests for contracts.
- Service module tests and frontend test/lint/build as applicable.
- Docker/Kong verification for routing, health, configuration, or security
  changes.
- Critical E2E flow checks for multi-service rollout behavior.

Capture exact evidence. If Docker, Testcontainers, a broker, or another external
dependency is unavailable, report the precise blocker and every narrower check
that passed. Do not represent partial verification as a full rollout check.


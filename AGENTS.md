# Repository Instructions

This repository is a monorepo of independently deployable microservices, a
React frontend, shared contracts, and deployment configuration.

## Core Rules

- A monorepo is a source-code organization model, not a monolith. Physical
  proximity must not weaken service boundaries.
- Treat every service as an independent bounded context with one owner for each
  business capability.
- Never introduce direct imports between services or access another service's
  source, database, schema, tables, or migrations.
- Services communicate through approved contracts: direct private gRPC for
  synchronous calls and RabbitMQ events for asynchronous workflows.
- Kong is the public ingress. Internal service-to-service calls do not route
  through Kong or the BFF.
- Shared modules contain generated contracts or cross-cutting infrastructure,
  not service-owned domain logic.
- Scope changes to the smallest owning service, package, or application surface.
- Keep public API and event contracts backward compatible unless the task
  explicitly authorizes a breaking change. Prefer additive changes.
- Avoid unrelated refactors unless explicitly requested.

Accepted architecture decisions in [`docs/adr/`](docs/adr/) remain authoritative.
Detailed steering documents translate those decisions into implementation rules.

## Repository Map

- `services/identity-service/`: Java 21/Spring Boot; credentials, profiles,
  roles, sessions, refresh tokens, and JWT issuance.
- `services/order-service/`: Java 21/Spring Boot; order lifecycle, idempotency,
  and transactional outbox ownership.
- `services/bff-service/`: Go; client-specific commerce composition and REST
  adaptation.
- `services/catalog-service/`: Go; product/catalog data and catalog caching.
- `services/inventory-service/`: Go; stock, reservations, and inventory event
  consumption.
- `services/cart-service/`: Go; shopping cart, line items, and cart event
  consumption.
- `ecommerce-fe/`: React/Vite/TypeScript frontend.
- `contracts/`: protobuf sources, generated Java/Go modules, and HTTP contract
  material.
- `deployments/`: Docker Compose, Kong, PostgreSQL, and local secret setup.

Keep production code and tests within the owning application surface. Java tests
belong in `src/test/java`; Go tests use adjacent `_test.go` files; frontend tests
use `*.test.ts` or `*.test.tsx` under `ecommerce-fe/src`.

## Before Modifying Code

Identify:

1. The service or application surface that owns the requested behavior.
2. Whether REST, OpenAPI, protobuf, gRPC, or event contracts change.
3. Which consumers depend on each changed contract.
4. Whether persistence, migrations, configuration, secrets, routing, or
   deployment resources change.
5. The smallest test scopes that demonstrate the new or corrected behavior.

Classify affected files using the process in
[`docs/steering/change-impact.md`](docs/steering/change-impact.md). Inspect
dependent services, but modify them only when a contract change requires it.

## Development Process

- Behavior changes follow test-driven development: write or select a test, run
  it, confirm it fails for the missing behavior, then implement until it passes.
- Bugs, failing gates, and flaky tests require a root cause backed by evidence
  before any fix; follow the `debugging-services` skill.
- Contract changes follow the `contract-change` skill.
- Before handoff, an independent read-only reviewer that did not implement the
  change reviews the diff: `code-reviewer` for every change, plus
  `boundary-reviewer` when contracts, several services, persistence, routing,
  or communication paths change.
- Gate results come from exit codes of `make verify SERVICE=<surface>` and the
  commands in [`docs/instructions/build-and-test.md`](docs/instructions/build-and-test.md).

## Instruction Routing

Start with the routing index in
[`docs/steering/README.md`](docs/steering/README.md), then read only the topics
relevant to the change:

- [`architecture.md`](docs/steering/architecture.md): ownership, autonomy,
  databases, and shared code.
- [`service-boundaries.md`](docs/steering/service-boundaries.md): cross-service
  dependencies, calls, events, data, and transactions.
- [`api-contracts.md`](docs/steering/api-contracts.md): REST, protobuf, gRPC,
  events, compatibility, errors, and idempotency.
- [`change-impact.md`](docs/steering/change-impact.md): change classification
  and dependent-surface analysis.
- [`testing.md`](docs/steering/testing.md): test selection and progressive
  verification.
- [`delivery.md`](docs/steering/delivery.md): migrations, configuration,
  deployments, rollout, and observability.
- [`service-catalog.md`](docs/steering/service-catalog.md): current service
  topology, ports, health checks, exposure, datastores, and dependencies.

When a nearer `AGENTS.md` exists inside a service or the frontend, apply it in
addition to this file. Local instructions may specialize these rules but may not
relax repository-wide boundaries, compatibility, security, or verification
requirements.

Developer setup, local runtime, build/test, and operations procedures are routed
through [`docs/instructions/README.md`](docs/instructions/README.md).
Contribution policy is routed through [`docs/rules/README.md`](docs/rules/README.md).

## Commands

- `docker compose -f deployments/docker/compose.foundation.yml up -d` starts
  PostgreSQL, Redis, RabbitMQ, and Kong.
- `make infra-up` starts or refreshes the local foundation; `make infra-down`
  stops it without deleting volumes.
- `make identity-keys` generates local-only Identity signing keys.
- `make identity`, `make order`, `make cart`, or `make bff` runs the
  corresponding service directly from the host.
- `mvn test` builds and tests all Maven modules.
- `go test ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...`
  runs all Go script and service tests.
- `npm --prefix ecommerce-fe run dev` starts the frontend. Replace `dev` with
  `test`, `lint`, or `build` for the corresponding check.
- `buf lint` validates protobuf sources; `buf generate` regenerates Java and Go
  contract modules.
- `make verify SERVICE=<surface>` runs the boundary check and the build/test gate
  for one surface: a service directory name under `services/`, `ecommerce-fe`,
  or `contracts`. A zero exit code means the gate passed.
- `make check-boundaries` fails on cross-service imports, service-to-service
  build dependencies, foreign database references, internal service calls
  through Kong (`:8000`), and frontend imports of backend or contract source.

Run infrastructure in Docker Compose and application services directly from a
terminal or IDE. When containerized Kong proxies to a Windows-hosted
service, use `host.docker.internal:<port>` as its upstream.

## Local Port Topology

Use this canonical topology when running the complete local stack. Check the
port before starting a process (`Get-NetTCPConnection` on Windows or `ss -ltnp`
on Linux/WSL) and verify the listed health endpoint after startup.

| Surface | Local HTTP | Local gRPC | Notes |
| --- | --- | --- | --- |
| identity-service | `8080` | `127.0.0.1:50054` | Kong obtains JWKS from this service; authorization gRPC is private/local-only. |
| bff-service | `8081` | - | Kong local upstream is `host.docker.internal:8081`; set `BFF_HTTP_ADDR=0.0.0.0:8081`. Container BFF remains `8080`. |
| catalog-service | `8085` | `127.0.0.1:50051` | Its standalone HTTP default is `8081`; set `HTTP_ADDR=:8085` when BFF runs. |
| inventory-service | `8082` | `127.0.0.1:50052` | |
| order-service | `8083` | - | |
| cart-service | `8084` | `127.0.0.1:50053` | BFF calls Cart directly over gRPC. |
| ecommerce-fe | `5173` | - | Browser development server. |
| Kong | proxy `8000`; admin `8001` | - | Public local ingress. |
| PostgreSQL | `5432` | - | |
| Redis | `6379` | - | |
| RabbitMQ | AMQP `5672`; management `15672` | - | |

- The browser calls public APIs only through `http://localhost:8000/api/v1`.
  The current frontend routes Identity auth, profile, and location requests
  through Kong;
- Internal service calls use direct private gRPC. BFF calls Catalog at `50051`
  and Cart at `50053`; Catalog calls Identity authorization at `127.0.0.1:50054`
  with a `500ms` timeout; Cart calls Catalog at `50051`, Inventory at `50052`,
  and Identity authorization at `127.0.0.1:50054` with a `500ms` timeout. Start
  Identity before Catalog and Cart. Do not route these calls through Kong or use
  another service's HTTP port.
- Roll out authorization dependencies provider-first: Identity, then Catalog and
  Cart, then BFF.

## Naming

- REST/JSON, Java, and TypeScript fields use `camelCase`.
- Protobuf fields use `lower_snake_case`; generated clients convert them to the
  target language's conventions.
- Exported Go identifiers use `PascalCase`; unexported identifiers use
  `camelCase`.
- Database columns use `snake_case`.
- Use Java 21 and Lombok for boilerplate reduction.

## REST Response Contract

- Success: `{ "data": <payload>, "message": "..." }`.
- Success without a payload: `{ "data": null, "message": "..." }`.
- Pagination: put metadata inside `data` as
  `{ "items": [], "page": 1, "pageSize": 20, "totalItems": 0, "totalPages": 0 }`.
- Error: `{ "code": "STABLE_ERROR_CODE", "message": "...", "errors": {}, "traceId": "..." }`.
- Omit `errors` when no structured details exist. Never return an ad hoc error
  shape or expose stack traces.
- Business errors use the same error envelope and an appropriate non-2xx status.
- Mutation endpoints provide a meaningful success `message` suitable for client
  notifications.

Ngoài ra có thể dùng psql để execute db nếu cần.

## Long-running processes

Do not run long-lived servers or watch commands in the foreground.

Commands such as:
- `npm run dev`
- `mvn spring-boot:run`
- `docker compose up`
- `tail -f`
- watch-mode test runners

must either:
1. be started using the environment's supported background/session mechanism, or
2. be bounded with an explicit timeout.

After starting a background service:
- verify readiness with a health check or bounded polling,
- capture logs to a known location,
- record the PID/session when applicable,
- stop the process when it is no longer needed.

Never wait indefinitely for a command whose expected behavior is to remain running.

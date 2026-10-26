# Production Docker Compose Deployment Design

## Intent and Why

Local development in this repository intentionally runs only platform infrastructure (`postgres`, `redis`, `rabbitmq`, `kong`) in `deployments/docker/compose.foundation.yml` while developers run microservices and the Vite dev server directly from host terminals. That hybrid workflow optimizes inner-loop iteration on Windows, but it cannot deploy the platform as containers because:

- `cart-service` and `ecommerce-fe` do not have `Dockerfile`s;
- existing Go `Dockerfile`s (`bff-service`, `catalog-service`, `inventory-service`) copy `go.work` without `./scripts`, causing `go build` to fail inside Docker;
- internal gRPC listeners default to loopback (`127.0.0.1`), blocking container-to-container RPCs;
- `deployments/kong/kong-production.yaml` has no route for the frontend SPA and still carries a placeholder `https://frontend.example.com` CORS configuration;
- no production Compose manifest, `.dockerignore`, or production environment template exists.

This design adds a self-contained single-host production Docker Compose deployment (`deployments/docker/compose.production.yml`) and its supporting container files while leaving the local development setup untouched.

## Goals, Non-Goals, and Anti-Goals

### Goals

- Build and run the complete stack (`postgres`, `redis`, `rabbitmq`, `kong`, `identity-service`, `bff-service`, `catalog-service`, `inventory-service`, `cart-service`, `order-service`, and `ecommerce-fe`) on a single server through `deployments/docker/compose.production.yml`.
- Expose Kong as the sole public HTTP ingress (`${PUBLIC_HTTP_PORT:-80}:8000`) serving both `/api/v1/*` and the `ecommerce-fe` SPA (`/`), so a single frontend image built with `VITE_API_BASE_URL=/api/v1` works on any IP or domain without CORS or `SameSite=Lax` refresh-cookie issues.
- Keep all datastores, brokers, Kong Admin, and backend HTTP/gRPC ports isolated on an internal Docker network with zero host ports published.
- Persist stateful infrastructure data (`postgres`, `redis`, `rabbitmq`) across container restarts using named Docker volumes.
- Enforce deterministic healthchecks and provider-first startup order (`postgres` -> `identity-service`, `catalog-service`, `inventory-service` -> `cart-service` -> `bff-service` -> `kong`).
- Isolate multi-stage Docker builds per service with root `.dockerignore` protection and `GOWORK=off` for Go builds.

### Non-Goals

- Changing `deployments/docker/compose.foundation.yml`, `deployments/kong/kong.yaml`, or any host-based local development script.
- Multi-host orchestration (Kubernetes, Docker Swarm) or automated TLS certificate issuance (TLS termination is handled by an external reverse proxy/CDN in front of `${PUBLIC_HTTP_PORT}` or added separately).
- Implementing unbuilt domain features (`order-service` checkout workflows, Inventory RabbitMQ consumers, or Redis caching).
- Exposing `/api/v1/admin/products` publicly until an authenticated authorization boundary exists for that route.

### Anti-Goals

- Copying `.env*` secrets or `deployments/secrets/` private keys into any Docker build context or image layer.
- Allowing `identity-service` to silently create the local fallback `admin@example.com` / `AdminPassword123!` `SUPER_ADMIN` account in production when bootstrap variables are omitted.
- Publishing PostgreSQL (`5432`), Redis (`6379`), RabbitMQ (`5672`/`15672`), Kong Admin (`8001`), or internal microservice ports (`8080`-`8085`, `50051`-`50054`) on the host.
- Breaking `make check-boundaries`, `make verify`, or existing CI gates.

## Constraints

- **Architecture boundaries (`AGENTS.md`, `ADR-008`):** Kong is the sole public ingress. Internal service-to-service communication uses direct private gRPC on the internal network (`catalog-service:50051`, `inventory-service:50052`, `cart-service:50053`, `identity-service:50054`) and never routes through Kong or BFF.
- **Header trust boundary:** `identity-service` and `bff-service` trust `X-User-Id` and `X-User-Roles` injected by Kong's `caller-context` plugin; their HTTP listeners must never be directly reachable from outside the Docker network.
- **Generated contracts:** Tracked protobuf outputs under `contracts/go` and `contracts/java` are compiled directly during `docker build`; service `Dockerfile`s must not require `buf` inside builder stages.
- **Runtime privilege:** Application containers run as non-root users (`appuser` UID `10001` for Java/Go; unprivileged `nginx` UID `101` for `ecommerce-fe`).
- **Signing keys:** Identity RSA keys (`private.pem`, `public.pem`) are mounted read-only at container runtime from a host secret directory and never baked into images.

## Approaches Considered

### Approach 1 (Selected): Kong as Single Public Ingress for API and Frontend

Kong is the only container publishing a host port (`${PUBLIC_HTTP_PORT:-80}:8000`). Kong routes `/api/v1/auth` and `/api/v1` to `identity-service:8080`, `/api/v1/cart` and `/api/v1/storefront` to `bff-service:8080`, and catch-all `/` (`strip_path: false`) to `ecommerce-fe:8080` (unprivileged Nginx serving the static SPA). `ecommerce-fe` builds with `VITE_API_BASE_URL=/api/v1`.

- **Why chosen:** Strictly follows `ADR-008` ("Kong is the single public ingress"), eliminates cross-origin CORS and `SameSite=Lax` `HttpOnly` refresh-cookie failures (`withCredentials: true`), makes the `ecommerce-fe` image domain-agnostic (no rebuild when changing IP/domain), and gives upstream TLS proxies a single target port.
- **Trade-off:** Static SPA asset requests traverse Kong before reaching Nginx. On a single-host Compose deployment, OpenResty/Kong proxy overhead for static assets is negligible.

### Approach 2 (Not Selected): Separate Host Ports for Frontend Nginx and Kong

`ecommerce-fe` publishes its own web port (for example `:80`) while Kong publishes a separate API port (for example `:8000`) serving only `/api/v1/*`.

- **Why rejected:** Vite bakes `VITE_API_BASE_URL` into the JavaScript bundle at build time, requiring a frontend image rebuild whenever the public API hostname changes. It also requires synchronizing CORS origins in Kong's static declarative config and sharing a registrable domain so browsers do not drop `SameSite=Lax` refresh cookies.

## Architecture and Component Design

### 1. Request and Dependency Topology

```text
Internet / Upstream TLS Proxy
        |
        v  (${PUBLIC_HTTP_PORT:-80} -> 8000)
  +-----------+
  |   kong    |  (DB-less, caller-context plugin, Admin bound to 127.0.0.1:8001 only)
  +-----------+
    |       |       \
    |       |        \  GET /* (strip_path: false)
    |       |         v
    |       |    +--------------+
    |       |    | ecommerce-fe | (nginx-unprivileged :8080, SPA fallback)
    |       |    +--------------+
    |       |
    |       +--> /api/v1/storefront, /api/v1/cart -> bff-service:8080
    |                                                   |-- gRPC -> catalog-service:50051 -> catalog_db
    |                                                   +-- gRPC -> cart-service:50053    -> cart_db
    |                                                                  |-- gRPC -> catalog-service:50051
    |                                                                  |-- gRPC -> inventory-service:50052 -> inventory_db
    |                                                                  +-- gRPC -> identity-service:50054
    |
    +----------> /api/v1/auth, /api/v1 ------------> identity-service:8080 (gRPC :50054) -> identity_db

  order-service:8083 (health skeleton on internal network; order_db provisioned)
  redis:6379, rabbitmq:5672 (provisioned on internal network with persistent volumes)
```

### 2. Build Context Hygiene (`.dockerignore`)

Add a root `.dockerignore` so root-context builds exclude:

- `.git`, `.github`, `.agents`, `.opencode`, `.superpowers`, `.worktrees`
- `**/node_modules`, `**/target`, `ecommerce-fe/dist`
- `.env`, `.env.*`
- `deployments/secrets`

This prevents local credentials, RSA private keys, and build artifacts from entering the Docker daemon context or image layers.

### 3. Backend `Dockerfile`s (Build Context: Repository Root `.`)

- **Go services (`bff-service`, `catalog-service`, `inventory-service`, and new `cart-service`):**
  - Builder stage (`golang:1.24-alpine`): set `ENV GOWORK=off` and do **not** copy `go.work`. Copy `contracts/go` and the target service's `go.mod`/`go.sum`, run `go mod download`, then copy only `services/<service-name>` and build a static binary (`CGO_ENABLED=0 go build`). Because each service's `go.mod` already declares `replace ecommerce.local/contracts/go => ../../contracts/go`, disabling `go.work` resolves the missing `./scripts` failure and isolates Docker layer caching per service.
  - Runtime stage (`alpine:3.20`): run as `appuser` (UID `10001`). Document internal listeners with `EXPOSE`:
    - `bff-service`: `8080`
    - `catalog-service`: `8081`, `50051`
    - `inventory-service`: `8082`, `50052`
    - `cart-service`: `8084`, `50053`
- **Java services (`identity-service` and `order-service`):**
  - Builder stage (`maven:3.9-eclipse-temurin-21-alpine`): copy root `pom.xml`, `contracts/java`, child `pom.xml` files (`services/identity-service/pom.xml`, `services/order-service/pom.xml`) required by the Maven reactor, and **only** the target service's `src/` directory before running `mvn clean package -pl services/<service-name> -am -DskipTests`.
  - Runtime stage (`eclipse-temurin:21-jre-alpine`): run as `appuser` (UID `10001`), exposing `8080 50054` (`identity-service`) and `8083` (`order-service`). Update `services/order-service/src/main/resources/application.yml` to `port: ${SERVER_PORT:8083}` so container `SERVER_PORT` behaves consistently with `identity-service`.

### 4. Frontend Container (`ecommerce-fe/Dockerfile` and `ecommerce-fe/nginx.conf`)

- Build context: `ecommerce-fe` (with a local `ecommerce-fe/.dockerignore` excluding `node_modules`, `dist`, `.env*`, `.opencode`, `.agents`).
- Builder stage (`node:22.12-alpine`):
  - `ARG VITE_API_BASE_URL=/api/v1`
  - `ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}`
  - `npm ci` followed by `npm run build`.
- Runtime stage (`nginxinc/nginx-unprivileged:1.27-alpine`):
  - Runs as unprivileged UID `101`, listening on `8080`.
  - `ecommerce-fe/nginx.conf` provides `/healthz` returning `200`, immutable cache headers for `/assets/`, `no-cache` for `/index.html`, and `try_files $uri $uri/ /index.html` so React Router paths (`/tim-kiem`, `/san-pham/:id`, `/gio-hang`, `/gio-hang/dat-hang`, `/login`, `/register`, `/profile`) resolve cleanly on browser refresh.

### 5. Kong Production Declarative Config (`deployments/kong/kong-production.yaml`)

- **Builder call — Explicit public route allowlist and header stripping:** Broad prefix-stripped public routes (`/api/v1/auth` and `/api/v1/storefront` with `strip_path: true` and no `caller-context`) would let unauthenticated requests reach `identity-service` protected paths with spoofed `X-User-Id`/`X-User-Roles` headers (for example `/api/v1/auth/users/me`) or reach `bff-service`'s unauthenticated `POST /products/{id}/delete` and `CartHandler` paths (for example `/api/v1/storefront/products/{id}/delete` or `/api/v1/storefront/items`). Production Kong therefore locks public routes down to exact allowed operations and strips client-supplied `X-User-Id` and `X-User-Roles` via `request-transformer` on every unauthenticated service:
  - Public Identity auth services:
    - `POST ~/api/v1/auth/register$` (`strip_path: true`) -> `http://identity-service:8080/register`
    - `POST ~/api/v1/auth/login$` (`strip_path: true`) -> `http://identity-service:8080/login`
    - `POST ~/api/v1/auth/refresh$` (`strip_path: true`) -> `http://identity-service:8080/refresh`
    - `POST ~/api/v1/auth/logout$` (`strip_path: true`) -> `http://identity-service:8080/logout`
    - `GET ~/api/v1/auth/\.well-known/jwks\.json$` (`strip_path: true`) -> `http://identity-service:8080/.well-known/jwks.json`
  - Protected Identity service (`http://identity-service:8080`): `identity-users-protected` (`paths: ["/api/v1"]`, `strip_path: true`) guarded by `caller-context` (`jwks_uri: http://identity-service:8080/.well-known/jwks.json`, `issuer: http://identity-service:8080`, `audience: ecommerce-api`), which clears and overwrites `X-User-Id` and `X-User-Roles` after RS256 verification.
  - Protected BFF Cart service (`http://bff-service:8080`): `cart-api` (`paths: ["/api/v1/cart"]`, `strip_path: true`) guarded by `caller-context`.
  - Public BFF Storefront service (`http://bff-service:8080/products` with `request-transformer` header removal): `GET`-only routes `~/api/v1/storefront/products$` and `/api/v1/storefront/products/` (`methods: ["GET"]`, `strip_path: true`), blocking `POST /products/{id}/delete` and all Cart handlers on the storefront prefix.
  - Frontend SPA service (`http://ecommerce-fe:8080` with `request-transformer` header removal): `frontend-spa` (`paths: ["/"]`, `methods: ["GET", "HEAD"]`, `strip_path: false`).
- **Builder call — Internal-only health endpoints in production:** Service `/actuator/health` and `/healthz` endpoints are not exposed through production Kong; Docker healthchecks probe them on `ecommerce-prod-net`, and external smoke checks verify container health plus public `/`, `/api/v1/storefront/products`, and `/api/v1/auth/.well-known/jwks.json`.
- Remove the hardcoded `https://frontend.example.com` CORS plugin block because SPA and API share the same public origin through Kong.

### 6. Production Compose & Environment (`deployments/docker/compose.production.yml` & `.env.production.example`)

- **Project & Isolation:** `name: ecommerce-prod`, internal bridge network `ecommerce-prod-net`, and `restart: unless-stopped` on all services. Avoid fixed `container_name` collisions with `compose.foundation.yml` so operators and CI can validate `compose.production.yml` independently.
- **Persistent Volumes:**
  - `postgres_data:/var/lib/postgresql/data` (plus read-only mount of `../postgres/init/01-create-databases.sql` for first-boot database creation)
  - `redis_data:/data` (`redis-server --appendonly yes --requirepass ...` left without password only if unused, or standard `redis-server --appendonly yes` on isolated internal network since no service consumes Redis yet)
  - `rabbitmq_data:/var/lib/rabbitmq`
- **Fail-Fast Secrets & Safe Defaults:**
  - Require explicit `POSTGRES_PASSWORD` and `RABBITMQ_DEFAULT_PASS` via `${POSTGRES_PASSWORD:?...}` and `${RABBITMQ_DEFAULT_PASS:?...}`. Document URL-safe password generation (`[A-Za-z0-9_-]`) in `.env.production.example` because Go services receive `DATABASE_URL` as a connection URI.
  - Mount Identity RSA keys read-only from `${IDENTITY_KEYS_DIR:-../secrets/identity}:/run/secrets/identity:ro` and set `IDENTITY_JWT_PRIVATE_KEY_PATH=/run/secrets/identity/private.pem` and `IDENTITY_JWT_PUBLIC_KEY_PATH=/run/secrets/identity/public.pem`.
  - Explicitly pass `IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL=${IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL:-}`, `IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD=${IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD:-}`, and `IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME=${IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME:-}` so `identity-service` disables `SUPER_ADMIN` auto-bootstrap unless all three values are intentionally provided in `.env.production`.
  - Default `IDENTITY_SESSION_COOKIE_SECURE=${IDENTITY_SESSION_COOKIE_SECURE:-true}` (operators testing over plain HTTP before placing TLS in front can set `false` in `.env.production`).
- **Internal Bindings & Service Discovery:**
  - `identity-service`: `SERVER_PORT=8080`, `IDENTITY_GRPC_HOST=0.0.0.0`, `IDENTITY_GRPC_PORT=50054`, `IDENTITY_DB_URL=jdbc:postgresql://postgres:5432/identity_db`, `IDENTITY_JWT_ISSUER=http://identity-service:8080`, `IDENTITY_JWT_AUDIENCE=ecommerce-api`.
  - `catalog-service`: `HTTP_ADDR=:8081`, `GRPC_ADDR=0.0.0.0:50051`, `DATABASE_URL=postgres://${POSTGRES_USER:-ecommerce}:${POSTGRES_PASSWORD}@postgres:5432/catalog_db?sslmode=disable`.
  - `inventory-service`: `HTTP_ADDR=:8082`, `GRPC_ADDR=0.0.0.0:50052`, `DATABASE_URL=postgres://${POSTGRES_USER:-ecommerce}:${POSTGRES_PASSWORD}@postgres:5432/inventory_db?sslmode=disable`.
  - `cart-service`: `HTTP_ADDR=:8084`, `GRPC_ADDR=0.0.0.0:50053`, `DATABASE_URL=postgres://${POSTGRES_USER:-ecommerce}:${POSTGRES_PASSWORD}@postgres:5432/cart_db?sslmode=disable`, `CATALOG_GRPC_ADDR=catalog-service:50051`, `INVENTORY_GRPC_ADDR=inventory-service:50052`, `IDENTITY_GRPC_ADDR=identity-service:50054`, `IDENTITY_GRPC_TIMEOUT=500ms`.
  - `bff-service`: `BFF_HTTP_ADDR=:8080`, `BFF_CATALOG_GRPC_ADDR=catalog-service:50051`, `BFF_CART_GRPC_ADDR=cart-service:50053`.
  - `order-service`: `SERVER_PORT=8083`.
  - `kong`: `KONG_DATABASE=off`, `KONG_DECLARATIVE_CONFIG=/usr/local/kong/declarative/kong.yml`, `KONG_ADMIN_LISTEN=127.0.0.1:8001`, ports `["${PUBLIC_HTTP_PORT:-80}:8000"]`.
- **Healthchecks & Startup Order (`depends_on` with `condition: service_healthy`):**
  1. `postgres` (`pg_isready`), `redis` (`redis-cli ping`), `rabbitmq` (`rabbitmq-diagnostics -q ping`)
  2. `identity-service` (`/actuator/health/readiness`), `catalog-service` (`/healthz`), `inventory-service` (`/healthz`), `order-service` (`/actuator/health/readiness`), `ecommerce-fe` (`/healthz`)
  3. `cart-service` (`/healthz`, depends on `postgres`, `identity-service`, `catalog-service`, `inventory-service` healthy so provider-first authorization is ready)
  4. `bff-service` (`/healthz`, depends on `catalog-service` and `cart-service` healthy)
  5. `kong` (`kong health`, depends on `identity-service`, `bff-service`, and `ecommerce-fe` healthy)

### 7. Tooling, Documentation, and Verification

- **`Makefile`:** Add non-intrusive production helpers (`prod-up`, `prod-down`, `prod-logs`, `prod-config`) using `deployments/docker/compose.production.yml` and `--env-file $(PROD_ENV_FILE)` (defaulting to `.env.production`).
- **Documentation:** Add `docs/instructions/production-deployment.md` (linked from `docs/instructions/README.md`) covering `.env.production` setup, RSA key provisioning, build/deploy/upgrade commands, TLS reverse-proxy guidance (including `X-Forwarded-Proto` / `IDENTITY_SESSION_COOKIE_SECURE`), and safe shutdown without `-v`.
- **Automated Tests:** Add Go configuration tests verifying `deployments/kong/kong-production.yaml` and `deployments/docker/compose.production.yml` invariants (Kong single host port, no internal service/datastore host ports, explicit public route/method allowlist with `request-transformer` stripping `X-User-Id`/`X-User-Roles`, `/` SPA route with `strip_path: false`, no unauthenticated `/api/v1/admin` or `/products/{id}/delete` exposure, Identity bootstrap defaults cleared).

## What Is Left to the Builder

- Exact healthcheck `interval`, `timeout`, `retries`, and `start_period` values (giving Spring Boot JVM containers sufficient `start_period`).
- Exact Nginx static caching, compression (`gzip`), and security header directives in `ecommerce-fe/nginx.conf`.
- Image tag naming convention in `compose.production.yml` (for example `ecommerce/<surface>:${IMAGE_TAG:-latest}` paired with `build:` blocks).
- Exact structure and examples in `docs/instructions/production-deployment.md`.

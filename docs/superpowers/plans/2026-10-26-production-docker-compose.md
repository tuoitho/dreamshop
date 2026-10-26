# Production Docker Compose Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a self-contained single-host production Docker Compose stack (`deployments/docker/compose.production.yml`) with hardened Kong single-ingress routing, isolated multi-stage `Dockerfile`s for all backend services and `ecommerce-fe`, environment templates, and automated configuration tests without altering the local hybrid development setup.

**Architecture:** Kong (`${PUBLIC_HTTP_PORT:-80}:8000`) is the only container publishing a host port, routing `/api/v1/*` to internal `identity-service` and `bff-service` containers (with explicit public-route allowlists and `X-User-Id`/`X-User-Roles` header stripping) and `/` (`strip_path: false`) to an unprivileged Nginx container serving the `ecommerce-fe` SPA built with `VITE_API_BASE_URL=/api/v1`. PostgreSQL, Redis, RabbitMQ, and all 6 backend microservices run on `ecommerce-prod-net` with persistent named volumes, `0.0.0.0` internal gRPC bindings, healthchecks, and provider-first `service_healthy` startup ordering.

**Tech Stack:** Docker / Docker Compose v2, Kong 3.8 (DB-less declarative YAML + Lua `caller-context` + `request-transformer`), Go 1.24 (`GOWORK=off` multi-stage Alpine images), Java 21 / Maven 3.9 / Spring Boot 4.1 (`eclipse-temurin:21-jre-alpine`), Node.js 22.12 / Vite 8 / `nginxinc/nginx-unprivileged:1.27-alpine`, Go `testing` + `gopkg.in/yaml.v3`.

**Spec:** `docs/superpowers/specs/2026-10-26-production-docker-compose-design.md`

## Global Constraints

- Do not modify `deployments/docker/compose.foundation.yml`, `deployments/kong/kong.yaml`, or local `scripts/start-local` behavior.
- Never commit directly to `main`; create/use a feature branch (for example `feat/production-docker-compose`) before making commits, and inspect `git status` / `git diff --cached` before each path-scoped commit.
- Kong is the sole public ingress (`${PUBLIC_HTTP_PORT:-80}:8000`). Zero host ports may be published on `postgres`, `redis`, `rabbitmq`, `identity-service`, `bff-service`, `catalog-service`, `inventory-service`, `cart-service`, `order-service`, or `ecommerce-fe`, and Kong Admin must listen only on `127.0.0.1:8001` inside its container.
- Internal service-to-service calls use direct private gRPC (`catalog-service:50051`, `inventory-service:50052`, `cart-service:50053`, `identity-service:50054`) and never route through Kong or BFF.
- Every Go `Dockerfile` must set `ENV GOWORK=off`, omit `go.work`, and copy only `contracts/go` plus its own service directory.
- Application runtime stages must execute as non-root users (`appuser` UID `10001` for Java/Go; `nginxinc/nginx-unprivileged` UID `101` for `ecommerce-fe`).
- `.env*` files and `deployments/secrets/` must be excluded from Docker build contexts via `.dockerignore`; Identity RSA keys are mounted read-only at runtime (`/run/secrets/identity`).
- Preserve `make check-boundaries` and `make verify` compatibility across all surfaces.

## Review Focus

- Unauthenticated header-spoofing (`X-User-Id`, `X-User-Roles`) or method/path traversal through Kong public routes (`/api/v1/auth/*` hitting `/users/me` or `POST /api/v1/storefront/products/{id}/delete` soft-deleting catalog items) -> Kong production config restricts storefront routes to `GET`, restricts auth routes to exact anchored paths/methods, and attaches `request-transformer` header removal on every unauthenticated Kong service (tested in Task 3).
- Silent creation of the local fallback `admin@example.com` / `AdminPassword123!` `SUPER_ADMIN` account in production when `IDENTITY_BOOTSTRAP_SUPER_ADMIN_*` variables are unset -> `compose.production.yml` explicitly passes `${IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL:-}`, `${IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD:-}`, and `${IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME:-}` so bootstrap is disabled unless all three are supplied (tested in Task 4).
- Starting production with missing database or broker passwords -> `compose.production.yml` uses `${POSTGRES_PASSWORD:?...}` and `${RABBITMQ_DEFAULT_PASS:?...}` so `docker compose config` fails fast when unset (tested in Task 4).
- Root `go.work` referencing `./scripts` breaking Go `docker build`, or local `.env` / `deployments/secrets` leaking into build context -> root `.dockerignore` blocks `.env*` and `deployments/secrets`, and all Go `Dockerfile`s set `ENV GOWORK=off` without copying `go.work` (tested in Task 1).
- Browser refresh on React Router paths (`/gio-hang`, `/san-pham/:id`, `/login`) returning `404` -> `ecommerce-fe/nginx.conf` serves `try_files $uri $uri/ /index.html` on `:8080` and Kong's `/` route uses `strip_path: false` (tested in Task 2 and Task 3).

---

## File Structure

| Surface | Files | Responsibility |
| --- | --- | --- |
| Build hygiene & backend images | `.dockerignore`, `services/cart-service/Dockerfile`, `services/bff-service/Dockerfile`, `services/catalog-service/Dockerfile`, `services/inventory-service/Dockerfile`, `services/identity-service/Dockerfile`, `services/order-service/Dockerfile`, `services/order-service/src/main/resources/application.yml` | Exclude secrets/build artifacts from Docker context; build isolated non-root backend images with `GOWORK=off` for Go and module-scoped copies for Maven. |
| Frontend image | `ecommerce-fe/.dockerignore`, `ecommerce-fe/nginx.conf`, `ecommerce-fe/Dockerfile` | Build static SPA with `VITE_API_BASE_URL=/api/v1` and serve on unprivileged Nginx `:8080` with `/healthz` and SPA fallback. |
| Edge gateway | `deployments/kong/kong-production.yaml` | Production DB-less Kong config with exact public route/method allowlist, `request-transformer` header stripping, protected `caller-context` routes, and `/` SPA upstream. |
| Production orchestration & docs | `deployments/docker/compose.production.yml`, `.env.production.example`, `Makefile`, `docs/instructions/production-deployment.md`, `docs/instructions/README.md` | Full-stack container orchestration, persistent volumes, healthcheck ordering, fail-fast secret checks, Make helpers, and operator runbook. |
| Config verification tests | `services/bff-service/internal/config/dockerfile_production_test.go`, `services/bff-service/internal/config/kong_production_test.go`, `services/bff-service/internal/config/compose_production_test.go` | Automated regression checks enforcing Dockerfiles, `.dockerignore`, Kong routing security, and Compose isolation invariants. |

---

### Task 1: Build Context Hygiene and Backend `Dockerfile` Isolation

**Files:**
- Create: `.dockerignore`
- Create: `services/cart-service/Dockerfile`
- Modify: `services/bff-service/Dockerfile`
- Modify: `services/catalog-service/Dockerfile`
- Modify: `services/inventory-service/Dockerfile`
- Modify: `services/identity-service/Dockerfile`
- Modify: `services/order-service/Dockerfile`
- Modify: `services/order-service/src/main/resources/application.yml`
- Test: `services/bff-service/internal/config/dockerfile_production_test.go`

**Interfaces:**
- Consumes: tracked contract modules `contracts/go` and `contracts/java`
- Produces: root-context buildable `Dockerfile`s for all 6 backend services (`identity-service`, `order-service`, `bff-service`, `catalog-service`, `inventory-service`, `cart-service`) and root `.dockerignore`

- [ ] **Step 1: Write the failing backend Dockerfile and `.dockerignore` verification test**

Create `services/bff-service/internal/config/dockerfile_production_test.go` with `TestBackendDockerfilesAndRootDockerignore`:
- Resolve repo root via `runtime.Caller(0)` (`../../../..`, matching `kong_development_test.go`).
- Assert `.dockerignore` exists and contains lines excluding `.git`, `**/node_modules`, `**/target`, `ecommerce-fe/dist`, `.env`, `.env.*`, and `deployments/secrets`.
- For Go services (`bff-service`, `catalog-service`, `inventory-service`, `cart-service`), assert `services/<svc>/Dockerfile` exists, contains `ENV GOWORK=off`, does **not** contain `COPY go.work`, does **not** contain `COPY services ./services`, contains `USER appuser`, and exposes expected ports (`bff-service`: `8080`; `catalog-service`: `8081` and `50051`; `inventory-service`: `8082` and `50052`; `cart-service`: `8084` and `50053`).
- For Java services (`identity-service`, `order-service`), assert `services/<svc>/Dockerfile` exists, does **not** contain `COPY services ./services`, copies only its own `src` directory (`services/<svc>/src`), and contains `USER appuser`.
- Assert `services/order-service/src/main/resources/application.yml` contains `${SERVER_PORT:8083}`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/bff-service/internal/config -run TestBackendDockerfilesAndRootDockerignore -v`
Expected: FAIL (`open .../.dockerignore: no such file or directory` / missing `cart-service/Dockerfile`).

- [ ] **Step 3: Create `.dockerignore` and update/create the 6 backend `Dockerfile`s + `order-service` port binding**

1. Create root `.dockerignore` excluding `.git`, `.github`, `.agents`, `.opencode`, `.superpowers`, `.worktrees`, `**/node_modules`, `**/target`, `ecommerce-fe/dist`, `.env`, `.env.*`, `deployments/secrets`.
2. Update `services/bff-service/Dockerfile`, `services/catalog-service/Dockerfile`, `services/inventory-service/Dockerfile`, and create `services/cart-service/Dockerfile` using `golang:1.24-alpine` builder (`ENV GOWORK=off`, copy `contracts/go` + `services/<svc>/go.mod` & `go.sum`, `RUN go mod download`, copy `services/<svc>`, `RUN CGO_ENABLED=0 go build -o /<bin> ./cmd/server`) and `alpine:3.20` runtime (`adduser -D -u 10001 appuser`, `USER appuser`, appropriate `ENV` defaults and `EXPOSE` ports: `bff` `8080`, `catalog` `8081 50051`, `inventory` `8082 50052`, `cart` `8084 50053`).
3. Update `services/identity-service/Dockerfile` and `services/order-service/Dockerfile` so the builder copies `pom.xml`, `contracts/java`, `services/identity-service/pom.xml`, `services/order-service/pom.xml`, and only `services/<target>/src` before `mvn clean package -pl services/<target> -am -DskipTests`.
4. Update `services/order-service/src/main/resources/application.yml` line 2 from `port: 8083` to `port: ${SERVER_PORT:8083}`.

- [ ] **Step 4: Run unit test and verify a real Go Docker build succeeds without `./scripts`**

Run: `go test ./services/bff-service/internal/config -run TestBackendDockerfilesAndRootDockerignore -v`
Expected: PASS

Run: `docker build -f services/cart-service/Dockerfile -t ecommerce/cart-service:test .`
Expected: Exit code `0` (confirms `GOWORK=off` + `.dockerignore` builds cleanly against `contracts/go`).

---

### Task 2: Frontend Production Container (`ecommerce-fe`)

**Files:**
- Create: `ecommerce-fe/.dockerignore`
- Create: `ecommerce-fe/nginx.conf`
- Create: `ecommerce-fe/Dockerfile`
- Modify: `services/bff-service/internal/config/dockerfile_production_test.go`

**Interfaces:**
- Consumes: `ecommerce-fe/package.json`, `ecommerce-fe/package-lock.json`, `ecommerce-fe/src/lib/api.ts` (`VITE_API_BASE_URL`)
- Produces: `ecommerce-fe` container image listening on HTTP `:8080` as unprivileged user with `/healthz` (`200`) and SPA fallback (`try_files $uri $uri/ /index.html`)

- [ ] **Step 1: Add failing frontend container test `TestFrontendDockerfileAndNginxConfig`**

In `services/bff-service/internal/config/dockerfile_production_test.go`, add `TestFrontendDockerfileAndNginxConfig` asserting:
- `ecommerce-fe/.dockerignore` exists and excludes `node_modules`, `dist`, `.env`, `.env.*`, `.opencode`, `.agents`.
- `ecommerce-fe/Dockerfile` exists, builds from `node:22.12-alpine`, declares `ARG VITE_API_BASE_URL=/api/v1` and `ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}`, runs `npm ci` and `npm run build`, uses `nginxinc/nginx-unprivileged:1.27-alpine`, copies `nginx.conf` to `/etc/nginx/conf.d/default.conf`, and exposes `8080`.
- `ecommerce-fe/nginx.conf` exists, listens on `8080`, defines `location = /healthz` returning `200`, and defines `try_files $uri $uri/ /index.html;` for SPA routing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/bff-service/internal/config -run TestFrontendDockerfileAndNginxConfig -v`
Expected: FAIL (`ecommerce-fe/Dockerfile` not found).

- [ ] **Step 3: Create `ecommerce-fe/.dockerignore`, `ecommerce-fe/nginx.conf`, and `ecommerce-fe/Dockerfile`**

1. Create `ecommerce-fe/.dockerignore` excluding `node_modules`, `dist`, `.env`, `.env.*`, `.opencode`, `.agents`.
2. Create `ecommerce-fe/nginx.conf` listening on `8080`, serving `/usr/share/nginx/html`, enabling `gzip` for text/js/css/json/svg, returning `200 "ok\n"` on `location = /healthz`, setting `Cache-Control "public, max-age=31536000, immutable"` on `/assets/`, and `try_files $uri $uri/ /index.html;` on `/`.
3. Create `ecommerce-fe/Dockerfile` (`node:22.12-alpine` -> `nginxinc/nginx-unprivileged:1.27-alpine`, port `8080`).

- [ ] **Step 4: Run test and verify `ecommerce-fe` image builds**

Run: `go test ./services/bff-service/internal/config -run TestFrontendDockerfileAndNginxConfig -v`
Expected: PASS

Run: `docker build -t ecommerce/ecommerce-fe:test ./ecommerce-fe`
Expected: Exit code `0`.

---

### Task 3: Hardened Kong Production Routing (`deployments/kong/kong-production.yaml`)

**Files:**
- Modify: `deployments/kong/kong-production.yaml`
- Test: `services/bff-service/internal/config/kong_production_test.go`

**Interfaces:**
- Consumes: internal container endpoints `http://identity-service:8080`, `http://bff-service:8080`, `http://ecommerce-fe:8080`, and custom plugin `caller-context`
- Produces: hardened `deployments/kong/kong-production.yaml` mounted by `kong` in Task 4

- [ ] **Step 1: Write failing Kong production security and routing tests**

Create `services/bff-service/internal/config/kong_production_test.go` with `TestProductionKongLocksDownPublicRoutesAndServesSPA` asserting:
- `_format_version` is `"3.0"` and no top-level `cors` plugin references `frontend.example.com`.
- Every service in `services` without a `caller-context` plugin attaches a `request-transformer` plugin whose `config.remove.headers` includes both `X-User-Id` and `X-User-Roles` (either at service or route level).
- Public Identity auth routes allow only exact anchored paths and methods:
  - `POST` `~/api/v1/auth/register$` -> `http://identity-service:8080/register` (`strip_path: true`)
  - `POST` `~/api/v1/auth/login$` -> `http://identity-service:8080/login` (`strip_path: true`)
  - `POST` `~/api/v1/auth/refresh$` -> `http://identity-service:8080/refresh` (`strip_path: true`)
  - `POST` `~/api/v1/auth/logout$` -> `http://identity-service:8080/logout` (`strip_path: true`)
  - `GET` `~/api/v1/auth/\.well-known/jwks\.json$` -> `http://identity-service:8080/.well-known/jwks.json` (`strip_path: true`)
  - No broad unauthenticated `/api/v1/auth` prefix route exists.
- Protected routes `/api/v1` (`http://identity-service:8080`) and `/api/v1/cart` (`http://bff-service:8080`) attach `caller-context` with `jwks_uri: http://identity-service:8080/.well-known/jwks.json`, `issuer: http://identity-service:8080`, and `audience: ecommerce-api`.
- Public storefront routes target `http://bff-service:8080/products`, allow only `GET` (`methods: ["GET"]`) on `~/api/v1/storefront/products$` and `/api/v1/storefront/products/`, and no route exposes `/api/v1/storefront` without `/products` or `/api/v1/admin`.
- Frontend SPA service targets `http://ecommerce-fe:8080` with route `frontend-spa` (`paths: ["/"]`, `methods: ["GET", "HEAD"]`, `strip_path: false`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/bff-service/internal/config -run TestProductionKongLocksDownPublicRoutesAndServesSPA -v`
Expected: FAIL (broad `/api/v1/auth` and `/api/v1/storefront` routes, missing `request-transformer`, missing `ecommerce-fe`, `frontend.example.com` present).

- [ ] **Step 3: Update `deployments/kong/kong-production.yaml`**

Rewrite `deployments/kong/kong-production.yaml` to implement the exact service/route/plugin specification from Section 5 of the design doc.

- [ ] **Step 4: Run Go test and validate schema with Kong 3.8 `kong config parse`**

Run: `go test ./services/bff-service/internal/config -run TestProductionKongLocksDownPublicRoutesAndServesSPA -v`
Expected: PASS

Run: `docker build -t ecommerce/kong:test ./deployments/kong` followed by `docker run --rm -v "${PWD}/deployments/kong/kong-production.yaml:/tmp/kong.yml:ro" ecommerce/kong:test kong config parse /tmp/kong.yml`
Expected: `parse successful` with exit code `0`.

---

### Task 4: Production Docker Compose Manifest, Env Template, Makefile Targets, and Runbook

**Files:**
- Create: `deployments/docker/compose.production.yml`
- Create: `.env.production.example`
- Create: `docs/instructions/production-deployment.md`
- Modify: `Makefile`
- Modify: `docs/instructions/README.md`
- Test: `services/bff-service/internal/config/compose_production_test.go`

**Interfaces:**
- Consumes: `Dockerfile`s from Tasks 1-2, `deployments/kong/kong-production.yaml` from Task 3, `deployments/postgres/init/01-create-databases.sql`
- Produces: `deployments/docker/compose.production.yml`, `.env.production.example`, `make prod-config` / `make prod-up` / `make prod-down` / `make prod-logs`, and `docs/instructions/production-deployment.md`

- [ ] **Step 1: Write failing production Compose invariant tests**

Create `services/bff-service/internal/config/compose_production_test.go` with `TestProductionComposeEnforcesIsolationSecretsAndStartupOrder` asserting:
- `deployments/docker/compose.production.yml` parses as valid YAML with `name: ecommerce-prod`, network `ecommerce-prod-net`, and top-level volumes `postgres_data`, `redis_data`, `rabbitmq_data`.
- All 11 expected services exist (`postgres`, `redis`, `rabbitmq`, `kong`, `identity-service`, `bff-service`, `catalog-service`, `inventory-service`, `cart-service`, `order-service`, `ecommerce-fe`), each connected to `ecommerce-prod-net`, with `restart: unless-stopped` and a `healthcheck` block.
- Only `kong` defines `ports`; all other 10 services have no `ports` field.
- `kong` sets `KONG_ADMIN_LISTEN` to `127.0.0.1:8001` and mounts `../kong/kong-production.yaml:/usr/local/kong/declarative/kong.yml:ro`.
- `postgres` and `rabbitmq` reference required password interpolation (`${POSTGRES_PASSWORD:?` and `${RABBITMQ_DEFAULT_PASS:?`).
- `identity-service` sets `IDENTITY_GRPC_HOST` to `0.0.0.0`, `IDENTITY_JWT_ISSUER` to `http://identity-service:8080`, `IDENTITY_SESSION_COOKIE_SECURE` defaulting to `true` (`${IDENTITY_SESSION_COOKIE_SECURE:-true}`), and explicitly overrides `IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL`, `IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD`, and `IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME` with empty defaults (`:-}`).
- `catalog-service`, `inventory-service`, and `cart-service` bind `GRPC_ADDR` to `0.0.0.0:50051`, `0.0.0.0:50052`, and `0.0.0.0:50053` respectively; `cart-service` `depends_on` includes `postgres`, `identity-service`, `catalog-service`, and `inventory-service` with `condition: service_healthy`; `bff-service` `depends_on` includes `catalog-service` and `cart-service` with `condition: service_healthy`.
- `.env.production.example` exists and documents `PUBLIC_HTTP_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `RABBITMQ_DEFAULT_USER`, `RABBITMQ_DEFAULT_PASS`, `IDENTITY_KEYS_DIR`, `IDENTITY_SESSION_COOKIE_SECURE`, and optional `IDENTITY_BOOTSTRAP_SUPER_ADMIN_*` variables without leaving local fallback passwords active.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./services/bff-service/internal/config -run TestProductionComposeEnforcesIsolationSecretsAndStartupOrder -v`
Expected: FAIL (`compose.production.yml` not found).

- [ ] **Step 3: Create `compose.production.yml`, `.env.production.example`, `Makefile` targets, and `production-deployment.md`**

1. Create `deployments/docker/compose.production.yml` implementing Section 6 of the spec (including generous JVM `start_period: 40s` on `identity-service` and `order-service` readiness probes using BusyBox `wget -qO- http://127.0.0.1:<port>/actuator/health/readiness > /dev/null`, Go/Nginx probes using `wget -qO- http://127.0.0.1:<port>/healthz > /dev/null`, and `kong health` on `kong`).
2. Create `.env.production.example` with clear comments (including URL-safe password requirements `[A-Za-z0-9_-]` for PostgreSQL URI compatibility, RSA key directory path, `IDENTITY_SESSION_COOKIE_SECURE=true`, and commented/blank `IDENTITY_BOOTSTRAP_SUPER_ADMIN_*`).
3. Update `Makefile` (`PROD_ENV_FILE ?= .env.production`, adding `prod-config`, `prod-up`, `prod-down`, `prod-logs` to `.PHONY` and `help` without modifying existing targets).
4. Create `docs/instructions/production-deployment.md` and link it from `docs/instructions/README.md`.

- [ ] **Step 4: Verify Go tests, fail-fast secret check, valid `docker compose config`, and repository gates**

Run: `go test ./services/bff-service/... -v`
Expected: PASS

Run: `docker compose -f deployments/docker/compose.production.yml config --quiet` (with `POSTGRES_PASSWORD` unset)
Expected: Non-zero exit code reporting `POSTGRES_PASSWORD` is required.

Run (with temporary test env values in shell, not written to `.env`):
`$env:POSTGRES_PASSWORD='test_prod_password_12345'; $env:RABBITMQ_DEFAULT_PASS='test_rabbit_password_12345'; docker compose -f deployments/docker/compose.production.yml config --quiet`
Expected: Exit code `0`.

Run repository gates:
- `make check-boundaries`
- `make verify`
- `go test ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/internal/...`
Expected: All exit with code `0`.

# Canonical `/api/v1` Backend Paths, Management Port Separation, and Kong Service Consolidation Design

## Intent and Why

Previously, HTTP handlers in `identity-service` and `bff-service` omitted the public `/api/v1/...` prefix (for example `/register`, `/users/me`, `/locations/provinces`, `/admin/users`, `/products`, `/items`) while Kong routes stripped `/api/v1/...` (`strip_path: true`). That mismatch created two operational and security problems:

1. **Kong service proliferation in production:** Because `strip_path: true` removes the matched route prefix, locking routes down in `deployments/kong/kong-production.yaml` required **14 separate Kong `services` entries** (`identity-auth-register`, `identity-auth-login`, `identity-auth-refresh`, `identity-auth-logout`, `identity-auth-jwks`, `identity-users`, `identity-locations`, `identity-admin-users`, `identity-admin-rbac`, `bff-service`, `bff-cart-items`, `bff-admin-products`, `bff-storefront`, and `ecommerce-fe`), each hardcoding a path suffix in `url:` (`http://identity-service:8080/users`, `http://bff-service:8080/items`, `http://bff-service:8080/products`, etc.) and duplicating `caller-context` or `request-transformer` plugin blocks. Local `deployments/kong/kong.yaml` similarly needed a separate `bff-admin-products` service (`http://host.docker.internal:8081/products`) so admin soft-delete would not collide with storefront `/products`.
2. **Shared application and management port on Spring Boot services:** Running Spring Boot Actuator (`/actuator/health`, `/actuator/info`, and future metrics endpoints) on the same HTTP listener (`8080` / `8083`) as public traffic meant any broad prefix-stripped Kong route (such as `/api/v1/auth` with `strip_path: true` in `deployments/kong/kong.yaml`) inadvertently exposed `/actuator/*` publicly as `/api/v1/auth/actuator/*`.

This design aligns backend HTTP route mappings directly with their canonical public `/api/v1/...` paths, moves Spring Boot `/actuator` endpoints onto dedicated internal management ports, and consolidates Kong to **one upstream service per backend** with `strip_path: false` across both local development and production.

## Goals, Non-Goals, and Anti-Goals

### Goals

- Align every public REST controller/handler in `identity-service` and `bff-service` with its exact public `/api/v1/...` path so upstream paths match ingress paths 1:1.
- Separate Spring Boot `/actuator` endpoints onto dedicated internal management ports (`8090` for `identity-service`, `8093` default on host / `8090` or `8093` configurable via `MANAGEMENT_SERVER_PORT` for `order-service`) so application HTTP ports (`8080`, `8083`) never serve `/actuator/*`.
- Consolidate `deployments/kong/kong-production.yaml` from **14 services down to 3** (`identity-service`, `bff-service`, `ecommerce-fe`) and `deployments/kong/kong.yaml` from **3 services down to 2** (`identity-service`, `bff-service`), with `strip_path: false` on every route.
- Preserve the exact public REST URLs, HTTP methods, request/response JSON envelopes, refresh-token cookie behavior (`Path=/api/v1/auth`), JWT validation (`caller-context`), and spoofing protection (`request-transformer` removing `X-User-Id` and `X-User-Roles` on unauthenticated routes) seen by `ecommerce-fe` and external callers.
- Remove reliance on accidentally leaked Kong health paths (`/api/v1/auth/actuator/health` and `/api/v1/storefront/healthz`) in `scripts/verify-foundation` and `ecommerce-fe`.

### Non-Goals

- Changing any protobuf/gRPC contract (`contracts/proto/*`), internal gRPC ports (`50051`–`50054`), or service-to-service communication paths.
- Adding separate management HTTP listeners to Go services (`bff-service`, `catalog-service`, `inventory-service`, `cart-service`), whose `/healthz` handlers return a static JSON payload without runtime internals and are already unreachable through Kong `strip_path: false` `/api/v1/*` routes.

### Anti-Goals

- Attaching Kong's `request-transformer` header-removal plugin at the **Kong `service` level** on `identity-service` or `bff-service` — Kong executes `caller-context` at `PRIORITY = 1001` before `request-transformer` at `PRIORITY = 801`, so a service-level `request-transformer` would strip the verified `X-User-Id` and `X-User-Roles` headers set by `caller-context` on protected routes.
- Using `server.servlet.context-path=/api/v1` in Spring Boot, which hides full paths from `@RequestMapping` declarations and breaks root-scoped internal endpoints such as `/.well-known/jwks.json`.
- Publishing management ports (`8090`, `8093`) on the Docker host in `deployments/docker/compose.production.yml`.

## Constraints

- **Architecture & boundaries (`AGENTS.md`, `ADR-007`, `ADR-008`):** Kong remains the sole public ingress; internal service calls continue using direct private gRPC; `make check-boundaries` and `make verify` for all affected surfaces must pass.
- **Kong 3.8 route matching (`kong-production.yaml`):** Because `frontend-spa` matches `paths: ["/"]` with `methods: ["GET", "HEAD"]`, every route in `kong-production.yaml` must declare explicit `methods` so Kong 3.8 `match_weight` rules never allow `frontend-spa` to shadow API routes.
- **Local CORS preflight (`kong.yaml`):** Local browser development runs `ecommerce-fe` on `http://localhost:5173` cross-origin against Kong on `http://localhost:8000`. Routes in `deployments/kong/kong.yaml` must continue matching `OPTIONS` preflight requests (either by omitting `methods` restrictions in dev where no `/` SPA route exists, or by including `OPTIONS`) so the global `cors` plugin (`PRIORITY = 2000`, ahead of `caller-context` `1001`) handles preflight cleanly.
- **JWKS compatibility:** Kong's `caller-context` plugin fetches JWKS internally over the container/host network (`http://identity-service:8080/.well-known/jwks.json` in production, `http://host.docker.internal:8080/.well-known/jwks.json` locally), while public clients verify `/api/v1/auth/.well-known/jwks.json` through Kong. `JwksController` will serve both `/.well-known/jwks.json` and `/api/v1/auth/.well-known/jwks.json` on port `8080`.

## Approaches Considered

### Approach 1 (Selected): Explicit `/api/v1/...` Controller/Handler Paths + Dedicated Spring Boot Management Port

Declare full `/api/v1/...` paths directly on Spring MVC controllers, Spring Security matchers, and Go `http.ServeMux` registrations; move Spring Boot Actuator to `management.server.port` (`8090` in `identity-service`, `8093` in `order-service`); keep Go `/healthz` on existing listeners; and consolidate Kong into one `service` per upstream with `strip_path: false` and route-scoped plugins.

- **Why chosen:** Every endpoint's exact public URL is grep-searchable in source code, `kong-production.yaml` shrinks from 13 services to 3 without path rewriting tricks, Spring Boot Actuator is physically isolated from the public-facing application port, and Go services avoid unnecessary extra ports.
- **Trade-off:** Existing controller/handler HTTP tests in `identity-service` and `bff-service`, Kong config tests, and `scripts/verify-foundation` must be updated to use the canonical `/api/v1/...` paths and internal management port.

### Approach 2 (Not Selected): Spring `server.servlet.context-path: /api/v1` + Extra Health Ports on All Go Services

Set `server.servlet.context-path: /api/v1` in `identity-service` to avoid editing `@RequestMapping` on non-auth controllers, and add a second HTTP server port on `bff-service` dedicated to `/healthz`.

- **Why rejected:** `context-path` obscures real URLs when reading or searching Java controllers, breaks root `/.well-known/jwks.json` unless special-cased, and still requires editing `AuthController` (`/register` -> `/auth/register`). Adding a second HTTP listener to `bff-service` complicates local port topology without meaningful security gain because `/healthz` exposes only static liveness text and is not matched by `/api/v1/*` Kong routes.

## Architecture and Component Design

### 1. `identity-service` Route & Management Port Refactoring

- **Controller mappings (`services/identity-service`):**
  - `AuthController`: `@RequestMapping("/api/v1/auth")` with `@PostMapping("/register")`, `@PostMapping("/login")`, `@PostMapping("/refresh")`, `@PostMapping("/logout")`.
  - `JwksController`: `@GetMapping({"/.well-known/jwks.json", "/api/v1/auth/.well-known/jwks.json"})` so both internal RFC 8414 JWKS lookups (`jwks_uri`) and public `/api/v1/auth/.well-known/jwks.json` calls succeed without path rewriting.
  - `MeController`: `@GetMapping("/api/v1/users/me")`, `@PatchMapping("/api/v1/users/me")`.
  - `ShippingAddressController`: `@RequestMapping("/api/v1/users/me/addresses")` (covering `GET`, `POST`, `PATCH /{addressId}`, `DELETE /{addressId}`, `PUT /{addressId}/default`).
  - `LocationController`: `@GetMapping("/api/v1/locations/provinces")`, `@GetMapping("/api/v1/locations/provinces/{provinceCode}/wards")`.
  - `AdminUserController`: `@RequestMapping("/api/v1/admin/users")`.
  - `RbacAdminController`: `@RequestMapping("/api/v1/admin/rbac/items")`.
- **Security configuration (`SecurityConfiguration.java`):**
  - Update `authorizeHttpRequests` matchers on the application `SecurityFilterChain` (`:8080`) to `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`, `/.well-known/jwks.json`, `/api/v1/auth/.well-known/jwks.json` (`permitAll()`), and `/api/v1/users/me`, `/api/v1/users/me/addresses/**`, `/api/v1/admin/users/**`, `/api/v1/admin/rbac/**`, `/api/v1/locations/provinces`, `/api/v1/locations/provinces/**` (`authenticated()`), followed by `anyRequest().denyAll()`.
  - Remove `/actuator/health/**` from the main `:8080` matcher list. Ensure the child management web context (`:8090`) permits `GET /actuator/health`, `/actuator/health/**`, and `/actuator/info` cleanly (via a `@ManagementContextConfiguration` `SecurityFilterChain` or `EndpointRequest` matcher verified with a real embedded-port integration test).
- **Management port configuration:**
  - `services/identity-service/src/main/resources/application.yml`: add `management.server.port: ${MANAGEMENT_SERVER_PORT:8090}`.
  - `services/identity-service/Dockerfile`: add `ENV MANAGEMENT_SERVER_PORT=8090` and `EXPOSE 8080 8090 50054`.
  - `scripts/start-local/main.go`: set default `MANAGEMENT_SERVER_PORT=8090` for `identity-service` (and `MANAGEMENT_SERVER_PORT=8093` for `order-service`).
  - `services/order-service/src/main/resources/application.yml` & `Dockerfile`: add `management.server.port: ${MANAGEMENT_SERVER_PORT:8093}` (so running `identity-service` and `order-service` concurrently on a local Windows host never collides on `8090`) and `EXPOSE 8083 8093` (or `MANAGEMENT_SERVER_PORT=8090` in container if overridden; keeping `8093` everywhere for `order-service` avoids an unnecessary local-vs-container difference).

### 2. `bff-service` Route Refactoring

- **`ProductsHandler` (`services/bff-service/internal/http/products.go`):**
  - Register `GET /api/v1/storefront/products` and `GET /api/v1/storefront/products/{id}`.
- **`CartHandler` (`services/bff-service/internal/http/cart.go`):**
  - Register `GET /api/v1/cart`, `DELETE /api/v1/cart`, `POST /api/v1/cart/items`, `PATCH /api/v1/cart/items/{itemId}`, `DELETE /api/v1/cart/items/{itemId}`. (Check whether trailing-slash requests if any exist; standard Go 1.22 exact path patterns `GET /api/v1/cart` and `DELETE /api/v1/cart` match `/api/v1/cart` directly without matching `/api/v1/cart/extra`, unlike `GET /{$}` which was only needed when the path was root `/`).
- **`AdminProductsHandler` (`services/bff-service/internal/http/admin_products.go`):**
  - Register `POST /api/v1/admin/products/{id}/delete`.
- **`HealthHandler` (`services/bff-service/cmd/server/main.go`):**
  - Remains at `GET /healthz` on the BFF HTTP listener (`:8081` locally, `:8080` in container).

### 3. Kong Declarative Configurations (`strip_path: false`)

#### Production (`deployments/kong/kong-production.yaml`) — 3 Services Total

1. **`identity-service`** (`url: http://identity-service:8080`):
   - `identity-auth-public`:
     - `paths`: `["~/api/v1/auth/register$", "~/api/v1/auth/login$", "~/api/v1/auth/refresh$", "~/api/v1/auth/logout$"]`
     - `methods`: `["POST"]`
     - `strip_path`: `false`
     - `plugins`: `request-transformer` removing `X-User-Id`, `X-User-Roles`
   - `identity-auth-jwks`:
     - `paths`: `['~/api/v1/auth/\.well-known/jwks\.json$']`
     - `methods`: `["GET"]`
     - `strip_path`: `false`
     - `plugins`: `request-transformer` removing `X-User-Id`, `X-User-Roles`
   - `identity-protected`:
     - `paths`: `["/api/v1/users", "/api/v1/locations", "/api/v1/admin/users", "/api/v1/admin/rbac"]`
     - `methods`: `["GET", "POST", "PUT", "PATCH", "DELETE"]`
     - `strip_path`: `false`
     - `plugins`: `caller-context` (`jwks_uri: http://identity-service:8080/.well-known/jwks.json`, `issuer: http://identity-service:8080`, `audience: ecommerce-api`)
2. **`bff-service`** (`url: http://bff-service:8080`):
   - `cart-api`:
     - `paths`: `["/api/v1/cart"]`
     - `methods`: `["GET", "POST", "PATCH", "DELETE"]`
     - `strip_path`: `false`
     - `plugins`: `caller-context` (`jwks_uri: http://identity-service:8080/.well-known/jwks.json`, `issuer: http://identity-service:8080`, `audience: ecommerce-api`)
   - `admin-products-api`:
     - `paths`: `["/api/v1/admin/products"]`
     - `methods`: `["POST"]`
     - `strip_path`: `false`
     - `plugins`: `caller-context` (`jwks_uri: http://identity-service:8080/.well-known/jwks.json`, `issuer: http://identity-service:8080`, `audience: ecommerce-api`)
   - `storefront-api`:
     - `paths`: `["/api/v1/storefront/products"]`
     - `methods`: `["GET"]`
     - `strip_path`: `false`
     - `plugins`: `request-transformer` removing `X-User-Id`, `X-User-Roles`
3. **`ecommerce-fe`** (`url: http://ecommerce-fe:8080`):
   - `frontend-spa`:
     - `paths`: `["/"]`
     - `methods`: `["GET", "HEAD"]`
     - `strip_path`: `false`
     - `plugins`: `request-transformer` removing `X-User-Id`, `X-User-Roles`

#### Local Development (`deployments/kong/kong.yaml`) — 2 Services Total (+ Global CORS)

1. **`identity-service`** (`url: http://host.docker.internal:8080`):
   - `identity-auth-public`:
     - `paths`: `["~/api/v1/auth/register$", "~/api/v1/auth/login$", "~/api/v1/auth/refresh$", "~/api/v1/auth/logout$", '~/api/v1/auth/\.well-known/jwks\.json$']` (or `/api/v1/auth` since `/actuator` is now isolated on `:8090` and `SecurityConfiguration` denies unlisted paths; anchored paths keep local and prod behavior identical!)
     - `strip_path`: `false`
     - `plugins`: `request-transformer` removing `X-User-Id`, `X-User-Roles`
   - `identity-protected`:
     - `paths`: `["/api/v1/users", "/api/v1/locations", "/api/v1/admin/users", "/api/v1/admin/rbac"]`
     - `strip_path`: `false`
     - `plugins`: `caller-context` (`jwks_uri: http://host.docker.internal:8080/.well-known/jwks.json`, `issuer: http://localhost:8080`, `audience: ecommerce-api`)
2. **`bff-service`** (`url: http://host.docker.internal:8081`):
   - `cart-api`:
     - `paths`: `["/api/v1/cart"]`
     - `strip_path`: `false`
     - `plugins`: `caller-context` (`jwks_uri: http://host.docker.internal:8080/.well-known/jwks.json`, `issuer: http://localhost:8080`, `audience: ecommerce-api`)
   - `storefront-api`:
     - `paths`: `["/api/v1/storefront/products"]`
     - `strip_path`: `false`
     - `plugins`: `request-transformer` removing `X-User-Id`, `X-User-Roles`
   - `admin-products-api`:
     - `paths`: `["/api/v1/admin/products"]`
     - `strip_path`: `false`
     - `plugins`: `caller-context` (`jwks_uri: http://host.docker.internal:8080/.well-known/jwks.json`, `issuer: http://localhost:8080`, `audience: ecommerce-api`)
- Global `cors` plugin for `http://localhost:5173` remains unchanged; omitting `methods` in `kong.yaml` ensures `OPTIONS` preflight requests match every dev route and are answered by `cors` (`PRIORITY = 2000`).

### 4. Compose Healthchecks, Foundation Verification, Frontend Cleanup, and Docs

- **`deployments/docker/compose.production.yml`:**
  - `identity-service` healthcheck probes `http://127.0.0.1:8090/actuator/health/readiness` (and sets `MANAGEMENT_SERVER_PORT: "8090"`).
  - `order-service` healthcheck probes `http://127.0.0.1:8093/actuator/health/readiness` (and sets `MANAGEMENT_SERVER_PORT: "8093"`).
- **`scripts/verify-foundation/main.go` (& `main_test.go`):**
  - Replace the leaked health probes (`/api/v1/storefront/healthz`, `/api/v1/auth/actuator/health`) with real public Kong probes (`/api/v1/storefront/products` for BFF and `/api/v1/auth/.well-known/jwks.json` for Identity).
- **`ecommerce-fe/src/lib/api.ts` (& `api.test.ts`):**
  - Remove unused `fetchHealth()` (which called `/auth/actuator/health`) and update the two unit tests in `api.test.ts` that used `fetchHealth()` as a generic `GET` request to use `fetchCurrentUser()`.
- **Documentation & Steering Updates:**
  - Update port topology and health-check URLs in `AGENTS.md`, `docs/steering/service-catalog.md`, `docs/instructions/local-development.md`, `docs/instructions/operations.md`, `docs/instructions/production-deployment.md`, and `services/identity-service/identity.http` (`8090` for Identity actuator, `8093` for Order actuator).

## What Is Left to the Builder

- Exact Spring Boot 4 management security configuration mechanism (`@ManagementContextConfiguration` vs `EndpointRequest`) verified by embedded-port tests.
- Exact route naming and ordering inside `kong.yaml` and `kong-production.yaml` as long as service count, `strip_path: false`, method allowlists, and plugin bindings satisfy all security invariants.
- Test organization across Java (`identity-service`, `order-service`), Go (`bff-service`, `scripts`), and TypeScript (`ecommerce-fe`).

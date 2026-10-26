# Canonical `/api/v1` Backend Paths and Kong Service Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align all public HTTP endpoints in `identity-service` and `bff-service` with their exact `/api/v1/...` public paths, isolate Spring Boot `/actuator` onto dedicated internal management ports (`8090` / `8093`), and consolidate Kong declarative configs to one service per upstream (`strip_path: false`) across local development and production.

**Architecture:** `identity-service` (`:8080`) and `bff-service` (local `:8081`, container `:8080`) register canonical `/api/v1/...` routes directly in Spring MVC / Spring Security and Go `http.ServeMux`, while Spring Boot Actuator moves to dedicated internal management listeners (`8090` on `identity-service`, `8093` on `order-service`). Kong routes forward paths verbatim (`strip_path: false`) to a single upstream `service` per backend (`identity-service` and `bff-service`, plus `ecommerce-fe` in production), scoping `caller-context` to protected routes and `request-transformer` header stripping to unauthenticated routes.

**Tech Stack:** Java 21 / Spring Boot 4.1 (Spring MVC, Spring Security, Spring Boot Actuator), Go 1.24 (`net/http` `ServeMux`), Kong 3.8 DB-less declarative YAML (`_format_version: "3.0"`), Docker Compose v2, TypeScript / Vitest (`ecommerce-fe`).

**Spec:** `docs/superpowers/specs/2026-10-26-canonical-api-paths-and-kong-consolidation-design.md`

## Global Constraints

- Do **not** touch or stage pre-existing unrelated workspace changes (`RbacPersistenceTest.java`, `RbacTestData.java`, `IdentityDatabaseMigrationTest.java`); leave working tree uncommitted unless the user explicitly asks to commit.
- Do **not** modify protobuf definitions (`contracts/proto/*`), generated gRPC code, or internal gRPC addresses (`50051`–`50054`).
- Never attach Kong `request-transformer` (`remove.headers: [X-User-Id, X-User-Roles]`) at the **Kong `service` level** on `identity-service` or `bff-service` (`request-transformer` runs at priority `801`, after `caller-context` at priority `1001`, and would erase verified caller headers on protected routes).
- Do **not** use `server.servlet.context-path=/api/v1` in Spring Boot; declare explicit `/api/v1/...` paths on controllers and security matchers.
- Do **not** publish management ports (`8090`, `8093`) on the Docker host in `deployments/docker/compose.production.yml`.
- Every route in `deployments/kong/kong-production.yaml` must declare explicit `methods` (preventing Kong 3.8 `match_weight` shadowing by `frontend-spa` `/`), while routes in `deployments/kong/kong.yaml` must remain matchable by `OPTIONS` preflight requests for `http://localhost:5173`.
- Preserve `make check-boundaries` and surface verification gates (`identity-service`, `order-service`, `bff-service`, `ecommerce-fe`, `scripts`).

## Review Focus

- Service-level `request-transformer` stripping `X-User-Id`/`X-User-Roles` after `caller-context` (`PRIORITY = 1001` vs `801`) on shared Kong services -> `kong_production_test.go` and `kong_development_test.go` assert `identity-service` and `bff-service` carry no service-level `request-transformer`, and unauthenticated routes carry route-level `request-transformer` (tested in Task 3).
- Spring Security `anyRequest().denyAll()` blocking child management port `:8090` health/readiness probes -> `ManagementPortIsolationTest` starts a real embedded server (`RANDOM_PORT` + `management.server.port=0`) and asserts `:managementPort/actuator/health` and `:managementPort/actuator/health/readiness` return `200` without credentials while `:serverPort/actuator/health` returns `401` (tested in Task 1).
- Local browser CORS `OPTIONS` preflight failing with `404` on `deployments/kong/kong.yaml` if routes restrict `methods` without `OPTIONS` -> `kong_development_test.go` asserts every dev route either omits `methods` or includes `OPTIONS`, with `strip_path: false` (tested in Task 3).
- Kong `caller-context` failing internal JWKS fetch (`/.well-known/jwks.json`) when public route uses `/api/v1/auth/.well-known/jwks.json` with `strip_path: false` -> `JwksApiTest` asserts both `GET /.well-known/jwks.json` and `GET /api/v1/auth/.well-known/jwks.json` return `200` with identical JWKS payloads (tested in Task 1).
- Unlisted subpaths under `/api/v1/cart` (such as `GET /api/v1/cart/unknown`) accidentally matching `GET /api/v1/cart` -> Go 1.22 exact pattern `GET /api/v1/cart` (no trailing slash) rejects subpaths with `404`, pinned in `cart_test.go` (tested in Task 2).

---

## File Structure

| Surface | Files | Responsibility |
| --- | --- | --- |
| Identity & Order Java services | `services/identity-service/src/main/java/com/ecommerce/identity/**/api/*Controller.java`, `SecurityConfiguration.java`, `ManagementSecurityConfiguration.java` (if needed), `services/identity-service/src/main/resources/application.yml`, `services/identity-service/Dockerfile`, `services/identity-service/src/test/java/com/ecommerce/identity/**/*ApiTest.java`, `ManagementPortIsolationTest.java`, `services/order-service/src/main/resources/application.yml`, `services/order-service/Dockerfile`, `services/order-service/src/test/java/com/ecommerce/order/OrderApplicationTest.java` | Move Identity REST controllers & security matchers to `/api/v1/...`, dual-map JWKS (`/.well-known/jwks.json` + `/api/v1/auth/.well-known/jwks.json`), and isolate `/actuator` onto port `8090` (`identity-service`) and `8093` (`order-service`). |
| BFF Go service | `services/bff-service/internal/http/products.go`, `cart.go`, `admin_products.go`, `products_test.go`, `cart_test.go`, `admin_products_test.go`, `services/bff-service/cmd/server/main_test.go` | Register `/api/v1/storefront/products`, `/api/v1/cart`, and `/api/v1/admin/products/{id}/delete` directly on BFF `ServeMux` while keeping `/healthz` internal. |
| Kong & Docker Compose | `deployments/kong/kong-production.yaml`, `deployments/kong/kong.yaml`, `deployments/docker/compose.production.yml`, `services/bff-service/internal/config/kong_production_test.go`, `kong_development_test.go`, `config_test.go`, `compose_production_test.go`, `dockerfile_production_test.go` | Consolidate Kong to 1 service per upstream (`strip_path: false`) with route-scoped plugins, and point production container healthchecks at management ports `8090` / `8093`. |
| Scripts, Frontend & Docs | `scripts/start-local/main.go`, `main_test.go`, `scripts/verify-foundation/main.go`, `main_test.go`, `ecommerce-fe/src/lib/api.ts`, `api.test.ts`, `AGENTS.md`, `docs/steering/service-catalog.md`, `docs/instructions/local-development.md`, `operations.md`, `production-deployment.md`, `services/identity-service/identity.http` | Set local management port defaults, probe real public routes in `verify-foundation`, remove dead `/auth/actuator/health` helper in `ecommerce-fe`, and synchronize port/health docs. |

---

### Task 1: `identity-service` & `order-service` Canonical `/api/v1` Paths and Management Port Isolation

**Files:**
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/auth/api/AuthController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/security/api/JwksController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/api/MeController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/address/api/ShippingAddressController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/location/api/LocationController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/api/AdminUserController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/api/RbacAdminController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/security/config/SecurityConfiguration.java`
- Create (if required by Spring Boot 4 child management security): `services/identity-service/src/main/java/com/ecommerce/identity/security/config/ManagementSecurityConfiguration.java`
- Modify: `services/identity-service/src/main/resources/application.yml`
- Modify: `services/identity-service/Dockerfile`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/auth/api/RegisterApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/auth/api/LoginApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/auth/api/RefreshApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/security/api/JwksApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/api/MeApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/address/api/ShippingAddressApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/location/api/LocationApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/admin/api/AdminUserApiTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/api/RbacAdminApiTest.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/security/config/ManagementPortIsolationTest.java`
- Modify: `services/order-service/src/main/resources/application.yml`
- Modify: `services/order-service/Dockerfile`
- Modify: `services/order-service/src/test/java/com/ecommerce/order/OrderApplicationTest.java`

**Interfaces:**
- Produces HTTP routes on `identity-service` (`:8080`):
  - `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/logout`
  - `GET /.well-known/jwks.json` and `GET /api/v1/auth/.well-known/jwks.json`
  - `GET /api/v1/users/me`, `PATCH /api/v1/users/me`
  - `GET|POST /api/v1/users/me/addresses`, `PATCH|DELETE /api/v1/users/me/addresses/{addressId}`, `PUT /api/v1/users/me/addresses/{addressId}/default`
  - `GET /api/v1/locations/provinces`, `GET /api/v1/locations/provinces/{provinceCode}/wards`
  - `GET /api/v1/admin/users`, `GET /api/v1/admin/users/{userId}`, `PATCH /api/v1/admin/users/{userId}/status`, `PUT|DELETE /api/v1/admin/users/{userId}/roles/{roleName}`
  - `GET|POST /api/v1/admin/rbac/items`, `GET|PATCH|DELETE /api/v1/admin/rbac/items/{name}`, `PUT|DELETE /api/v1/admin/rbac/items/{parent}/children/{child}`
- Produces management listeners:
  - `identity-service`: `management.server.port: ${MANAGEMENT_SERVER_PORT:8090}` (`/actuator/health`, `/actuator/health/readiness`, `/actuator/info`)
  - `order-service`: `management.server.port: ${MANAGEMENT_SERVER_PORT:8093}` (`/actuator/health`, `/actuator/health/readiness`, `/actuator/info`)

- [ ] **Step 1: Write failing tests for `/api/v1/...` routes, dual JWKS paths, and management port isolation**

1. Update request paths in `RegisterApiTest`, `LoginApiTest`, `RefreshApiTest`, `MeApiTest`, `ShippingAddressApiTest`, `LocationApiTest`, `AdminUserApiTest`, and `RbacAdminApiTest` to `/api/v1/auth/...`, `/api/v1/users/...`, `/api/v1/locations/...`, `/api/v1/admin/users...`, and `/api/v1/admin/rbac/items...`. In `RegisterApiTest#permitsPublicEndpointsButDeniesUnlistedEndpoints`, assert `mockMvc.perform(get("/actuator/health")).andExpect(status().isUnauthorized())` and `mockMvc.perform(post("/register")).andExpect(status().isUnauthorized())` (confirming unprefixed legacy routes and main-port actuator are rejected).
2. In `JwksApiTest`, assert both `GET /.well-known/jwks.json` and `GET /api/v1/auth/.well-known/jwks.json` return `200 OK` with the expected RSA `kid`.
3. Create `ManagementPortIsolationTest` extending `PostgresIntegrationTestSupport` (or `@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT, properties = "management.server.port=0")` with Testcontainers PostgreSQL + RSA keys) using `@LocalServerPort` and `@LocalManagementPort`:
   - `GET http://127.0.0.1:<serverPort>/actuator/health` -> `401`
   - `GET http://127.0.0.1:<managementPort>/actuator/health` -> `200` (`"status":"UP"`)
   - `GET http://127.0.0.1:<managementPort>/actuator/health/readiness` -> `200` (`"status":"UP"`)
4. In `OrderApplicationTest`, use `@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT, properties = "management.server.port=0")` and assert `@LocalServerPort != @LocalManagementPort`, `:serverPort/actuator/health` returns `404`, and `:managementPort/actuator/health/readiness` returns `200`.

- [ ] **Step 2: Run focused tests to verify they fail**

Run: `mvn -pl services/identity-service test "-Dtest=RegisterApiTest,JwksApiTest,ManagementPortIsolationTest"` and `mvn -pl services/order-service test`
Expected: FAIL (`401`/`404` on `/api/v1/...` routes and management port not separated yet).

- [ ] **Step 3: Implement canonical `/api/v1` mappings and management port separation in `identity-service` and `order-service`**

1. Update controller annotations in `AuthController` (`@RequestMapping("/api/v1/auth")`), `JwksController` (`@GetMapping({"/.well-known/jwks.json", "/api/v1/auth/.well-known/jwks.json"})`), `MeController` (`/api/v1/users/me`), `ShippingAddressController` (`@RequestMapping("/api/v1/users/me/addresses")` + relative method paths), `LocationController` (`/api/v1/locations/...`), `AdminUserController` (`@RequestMapping("/api/v1/admin/users")`), and `RbacAdminController` (`@RequestMapping("/api/v1/admin/rbac/items")`).
2. Update `SecurityConfiguration.java` matchers to `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`, `/.well-known/jwks.json`, `/api/v1/auth/.well-known/jwks.json` (`permitAll()`), and `/api/v1/users/me`, `/api/v1/users/me/addresses/**`, `/api/v1/admin/users/**`, `/api/v1/admin/rbac/**`, `/api/v1/locations/provinces`, `/api/v1/locations/provinces/**` (`authenticated()`), removing `/actuator/health/**` from the main port filter chain. Add `@ManagementContextConfiguration` security bean (`ManagementSecurityConfiguration`) if the child management context inherits `denyAll()`, permitting `GET /actuator/health`, `/actuator/health/**`, and `/actuator/info` on the management port only.
3. Add `management.server.port: ${MANAGEMENT_SERVER_PORT:8090}` to `services/identity-service/src/main/resources/application.yml` and update `services/identity-service/Dockerfile` (`ENV MANAGEMENT_SERVER_PORT=8090`, `EXPOSE 8080 8090 50054`). Note: if `@SpringBootTest` `MOCK` tests share static port `8090`, verify full `mvn test` reactor has zero port-binding conflicts (override `management.server.port=0` in `PostgresIntegrationTestSupport` if Spring starts a real management connector during `MOCK` tests).
4. Add `management.server.port: ${MANAGEMENT_SERVER_PORT:8093}` to `services/order-service/src/main/resources/application.yml` and update `services/order-service/Dockerfile` (`ENV MANAGEMENT_SERVER_PORT=8093`, `EXPOSE 8083 8093`).

- [ ] **Step 4: Run `identity-service` and `order-service` test suites to verify they pass**

Run: `mvn -pl services/identity-service,services/order-service -am test`
Expected: PASS (exit code `0`).

---

### Task 2: `bff-service` Canonical `/api/v1` HTTP Routes

**Files:**
- Modify: `services/bff-service/internal/http/products.go`
- Modify: `services/bff-service/internal/http/cart.go`
- Modify: `services/bff-service/internal/http/admin_products.go`
- Modify: `services/bff-service/internal/http/products_test.go`
- Modify: `services/bff-service/internal/http/cart_test.go`
- Modify: `services/bff-service/internal/http/admin_products_test.go`
- Modify: `services/bff-service/cmd/server/main_test.go`

**Interfaces:**
- Produces HTTP routes on `bff-service` (local `:8081`, container `:8080`):
  - `GET /api/v1/storefront/products`
  - `GET /api/v1/storefront/products/{id}`
  - `GET /api/v1/cart`
  - `DELETE /api/v1/cart`
  - `POST /api/v1/cart/items`
  - `PATCH /api/v1/cart/items/{itemId}`
  - `DELETE /api/v1/cart/items/{itemId}`
  - `POST /api/v1/admin/products/{id}/delete`
  - `GET /healthz` (unchanged internal health route)

- [ ] **Step 1: Update `bff-service` handler & server mux tests to `/api/v1/...` and add exact-path negative check**

Update `products_test.go`, `cart_test.go`, `admin_products_test.go`, and `cmd/server/main_test.go` to send requests to `/api/v1/storefront/products`, `/api/v1/storefront/products/{id}`, `/api/v1/cart`, `/api/v1/cart/items`, `/api/v1/cart/items/{itemId}`, and `/api/v1/admin/products/{id}/delete` (keeping `/healthz` at `/healthz`). Add a test case in `cart_test.go` asserting `GET /api/v1/cart/unknown` returns `404` and legacy `GET /products` returns `404` on `buildMux`.

- [ ] **Step 2: Run `bff-service` HTTP & server tests to verify they fail**

Run: `go test ./services/bff-service/internal/http/... ./services/bff-service/cmd/server/...`
Expected: FAIL (`404` on `/api/v1/...` paths).

- [ ] **Step 3: Update `Register` methods in `products.go`, `cart.go`, and `admin_products.go`**

1. `ProductsHandler.Register`: `mux.HandleFunc("GET /api/v1/storefront/products", h.search)` and `mux.HandleFunc("GET /api/v1/storefront/products/{id}", h.detail)`.
2. `CartHandler.Register`: `mux.Handle("GET /api/v1/cart", ...)`, `mux.Handle("POST /api/v1/cart/items", ...)`, `mux.Handle("PATCH /api/v1/cart/items/{itemId}", ...)`, `mux.Handle("DELETE /api/v1/cart/items/{itemId}", ...)`, `mux.Handle("DELETE /api/v1/cart", ...)`.
3. `AdminProductsHandler.Register`: `mux.Handle("POST /api/v1/admin/products/{id}/delete", correlateTrace(RequireCaller(http.HandlerFunc(h.softDelete))))`.

- [ ] **Step 4: Run `bff-service` HTTP & server tests to verify they pass**

Run: `go test ./services/bff-service/internal/http/... ./services/bff-service/cmd/server/...`
Expected: PASS.

---

### Task 3: Kong Service Consolidation (`strip_path: false`) and Production Compose Healthchecks

**Files:**
- Modify: `deployments/kong/kong-production.yaml`
- Modify: `deployments/kong/kong.yaml`
- Modify: `deployments/docker/compose.production.yml`
- Modify: `services/bff-service/internal/config/kong_production_test.go`
- Modify: `services/bff-service/internal/config/kong_development_test.go`
- Modify: `services/bff-service/internal/config/config_test.go`
- Modify: `services/bff-service/internal/config/compose_production_test.go`
- Modify: `services/bff-service/internal/config/dockerfile_production_test.go`

**Interfaces:**
- Consumes canonical backend routes from Task 1 (`identity-service:8080`, management `8090`; `order-service:8083`, management `8093`) and Task 2 (`bff-service:8080` / local `:8081`).
- Produces consolidated Kong configs:
  - `kong-production.yaml`: exactly 3 services (`identity-service`, `bff-service`, `ecommerce-fe`), all routes `strip_path: false`, explicit `methods`, route-level `caller-context` on protected routes, route-level `request-transformer` removing `X-User-Id` & `X-User-Roles` on unauthenticated routes.
  - `kong.yaml`: exactly 2 services (`identity-service`, `bff-service`), all routes `strip_path: false`, route-level plugins, `OPTIONS` preflight unblocked for global `cors`.

- [ ] **Step 1: Update config verification tests for consolidated Kong services and management healthcheck ports**

1. Update `kong_production_test.go`:
   - Assert `len(doc.Services) == 3` with exact service URLs `http://identity-service:8080`, `http://bff-service:8080`, and `http://ecommerce-fe:8080`.
   - Assert neither `identity-service` nor `bff-service` carries a service-level `request-transformer` plugin.
   - Assert every route sets `strip_path: false` and non-empty `methods`.
   - Assert `identity-service` exposes anchored public auth `POST` routes (`~/api/v1/auth/register$`, `~/api/v1/auth/login$`, `~/api/v1/auth/refresh$`, `~/api/v1/auth/logout$`) and `GET ~/api/v1/auth/\.well-known/jwks\.json$` with route-level `request-transformer`, plus protected prefixes `/api/v1/users`, `/api/v1/locations`, `/api/v1/admin/users`, `/api/v1/admin/rbac` guarded by `caller-context`.
   - Assert `bff-service` exposes `cart-api` (`paths: ["/api/v1/cart"]`, `methods: ["GET", "POST", "PATCH", "DELETE"]`, `caller-context`), `admin-products-api` (`paths: ["/api/v1/admin/products"]`, `methods: ["POST"]`, `caller-context`), and `storefront-api` (`paths: ["/api/v1/storefront/products"]`, `methods: ["GET"]`, route-level `request-transformer`).
2. Update `config_test.go` (`TestProductionKongProtectsCartWithProductionCallerContext`) to expect `strip_path: false` on both `cart-api` and `admin-products-api` under the single `bff-service` (`http://bff-service:8080`).
3. Update `kong_development_test.go`:
   - Assert `len(services) == 2` (`identity-service` -> `http://host.docker.internal:8080` and `bff-service` -> `http://host.docker.internal:8081`).
   - Assert every route sets `strip_path: false`, neither service attaches service-level `request-transformer`, every route either omits `methods` or includes `OPTIONS`, `admin-products-api` is a route on `bff-service` with `paths: ["/api/v1/admin/products"]`, and no route uses broad `/api/v1` or `/api/v1/admin`.
4. Update `compose_production_test.go` and `dockerfile_production_test.go`:
   - Assert `identity-service` sets `MANAGEMENT_SERVER_PORT: "8090"`, probes `http://127.0.0.1:8090/actuator/health/readiness`, and exposes `8080`, `8090`, `50054` in `Dockerfile` (with zero host `ports` in Compose).
   - Assert `order-service` sets `MANAGEMENT_SERVER_PORT: "8093"`, probes `http://127.0.0.1:8093/actuator/health/readiness`, and exposes `8083`, `8093` in `Dockerfile` (with zero host `ports` in Compose).

- [ ] **Step 2: Run `bff-service/internal/config` tests to verify they fail**

Run: `go test ./services/bff-service/internal/config/... -v`
Expected: FAIL (`len(doc.Services) == 13`, `strip_path: true`, old healthcheck URLs).

- [ ] **Step 3: Update `kong-production.yaml`, `kong.yaml`, and `compose.production.yml`**

1. Rewrite `deployments/kong/kong-production.yaml` into the 3 consolidated services (`identity-service`, `bff-service`, `ecommerce-fe`) with `strip_path: false` and route-scoped plugins per the spec.
2. Rewrite `deployments/kong/kong.yaml` into the 2 consolidated services (`identity-service`, `bff-service`) with `strip_path: false`, route-scoped plugins, anchored public auth paths, scoped protected paths, and global `cors`.
3. Update `deployments/docker/compose.production.yml` `identity-service` (`MANAGEMENT_SERVER_PORT: "8090"`, readiness probe `:8090`) and `order-service` (`MANAGEMENT_SERVER_PORT: "8093"`, readiness probe `:8093`).

- [ ] **Step 4: Run `bff-service` full test suite and Docker Compose config checks**

Run: `go test ./services/bff-service/... -v`
Expected: PASS.

---

### Task 4: Tooling (`start-local`, `verify-foundation`), Frontend Cleanup, Docs, and Full Verification

**Files:**
- Modify: `scripts/start-local/main.go`
- Modify: `scripts/start-local/main_test.go`
- Modify: `scripts/verify-foundation/main.go`
- Modify: `scripts/verify-foundation/main_test.go`
- Modify: `ecommerce-fe/src/lib/api.ts`
- Modify: `ecommerce-fe/src/lib/api.test.ts`
- Modify: `AGENTS.md`
- Modify: `docs/steering/service-catalog.md`
- Modify: `docs/instructions/local-development.md`
- Modify: `docs/instructions/operations.md`
- Modify: `docs/instructions/production-deployment.md`
- Modify: `services/identity-service/identity.http`

**Interfaces:**
- Consumes consolidated Kong public routes (`/api/v1/storefront/products`, `/api/v1/auth/.well-known/jwks.json`) and internal management ports (`8090` / `8093`).

- [ ] **Step 1: Update `scripts` and `ecommerce-fe` tests first**

1. In `scripts/start-local/main_test.go`, assert `identity-service` sets default `MANAGEMENT_SERVER_PORT=8090` and `order-service` sets default `MANAGEMENT_SERVER_PORT=8093` (when unset in base env).
2. In `scripts/verify-foundation/main_test.go`, update mock Kong server expected probe paths from `/api/v1/storefront/healthz` and `/api/v1/auth/actuator/health` to `/api/v1/storefront/products` and `/api/v1/auth/.well-known/jwks.json`.
3. In `ecommerce-fe/src/lib/api.test.ts`, replace `fetchHealth` imports/calls in the two generic `GET` interceptor unit tests with `fetchCurrentUser` (and assert `fetchHealth` is no longer exported from `./api`).

- [ ] **Step 2: Run `scripts` and `ecommerce-fe` tests to verify they fail**

Run: `go test ./scripts/...` and `npm --prefix ecommerce-fe run test`
Expected: FAIL.

- [ ] **Step 3: Implement `start-local`, `verify-foundation`, `ecommerce-fe/src/lib/api.ts`, and documentation updates**

1. In `scripts/start-local/main.go`, call `setDefaultEnv(env, "MANAGEMENT_SERVER_PORT", "8090")` for `identity-service` and `setDefaultEnv(env, "MANAGEMENT_SERVER_PORT", "8093")` for `order-service`.
2. In `scripts/verify-foundation/main.go`, probe `base+"/api/v1/storefront/products"` (`BFF`) and `base+"/api/v1/auth/.well-known/jwks.json"` (`Identity`).
3. Remove `fetchHealth` from `ecommerce-fe/src/lib/api.ts`.
4. Update port tables and health check commands in `AGENTS.md`, `docs/steering/service-catalog.md`, `docs/instructions/local-development.md`, `docs/instructions/operations.md`, `docs/instructions/production-deployment.md`, and `services/identity-service/identity.http` (`http://localhost:8090/actuator/health` for Identity, `http://localhost:8093/actuator/health` for Order, and `/api/v1/...` requests in `identity.http`).

- [ ] **Step 4: Run full progressive verification gates**

Run:
- `go test ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...`
- `make verify SERVICE=identity-service`
- `make verify SERVICE=order-service`
- `make verify SERVICE=bff-service`
- `make verify SERVICE=ecommerce-fe`
- `make check-boundaries`
Expected: All commands exit with code `0`.

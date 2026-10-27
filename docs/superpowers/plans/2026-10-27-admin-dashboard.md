# Admin Dashboard (`admin-fe`) & Commerce Admin APIs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone administrative console (`admin-fe/`) designed via OpenDesign (local Antigravity runtime) covering Overview KPIs, Products CRUD + Soft-Delete/Restore (with direct MinIO presigned image & video uploads and orphan lifecycle cleanup), Inventory Stock Management, Users Administration, and RBAC Hierarchy, plus storefront video gallery playback in `ecommerce-fe/`, backed by additive contracts and handlers across `identity-service`, `catalog-service`, `inventory-service`, `bff-service`, MinIO, and Kong.

**Architecture:** Per `ADR-011` and `ADR-012`, Identity REST (`/api/v1/auth`, `/api/v1/users`, `/api/v1/admin/users`, `/api/v1/admin/rbac`) routes directly from Kong to `identity-service` (extended with `V10` commerce permissions, `GET /api/v1/users/me/permissions`, and optional user `status` filter + counts), while all first-party `admin-fe` commerce workflows (`/api/v1/admin/products` including `/api/v1/admin/products/media/presign`, and `/api/v1/admin/inventory`) route through `bff-service`. `bff-service` composes additive private gRPC calls to `catalog-service` (`:50051`) and `inventory-service` (`:50052`), both enforcing granular RBAC permissions via `identity-service` private authorization gRPC (`:50054`). `catalog-service` issues 5-minute Presigned S3 `PUT` URLs against MinIO (`product-media` bucket), tracks metadata and `PENDING -> ATTACHED -> ORPHANED` lifecycle in `catalog_db.product_media_assets`, and sweeps expired `PENDING`/`ORPHANED` objects after a 24-hour grace period. `admin-fe/` runs on port `5174` locally and serves under `/admin` via Nginx behind Kong in production Compose.

**Tech Stack:** Protobuf / Buf, Java 21 / Spring Boot 4.1 / Flyway / Testcontainers, Go 1.24 (`google.golang.org/grpc`, `pgx/v5`, `minio-go/v7`, `net/http`), MinIO S3, Kong 3.8 declarative config, OpenDesign (local Antigravity runtime), React 19 / TypeScript 6 / Vite 8 / Tailwind CSS v4 / `radix-ui` (`shadcn/ui`) / `@tanstack/react-table` / `react-hook-form` / `zod` / `recharts` / Vitest + Testing Library + `oxlint`.

**Spec:** `docs/superpowers/specs/2026-10-27-admin-dashboard-design.md`

## Global Constraints

- Service boundaries & `ADR-011` (`AGENTS.md`, `docs/steering/service-boundaries.md`, `docs/adr/011-service-side-grpc-gateway-transcoding.md`): Zero cross-service source imports, zero foreign database queries, and zero internal calls through Kong (`:8000`) or another service's HTTP port. Identity REST routes directly to `identity-service`; all `admin-fe` commerce endpoints route through `bff-service`; do not attach `google.api.http` to internal Catalog/Inventory RPCs or use Kong's `grpc-gateway` plugin.
- Contract compatibility (`docs/steering/api-contracts.md`): Protobuf changes in `contracts/proto/catalog/v1/catalog.proto` and `contracts/proto/inventory/v1/inventory.proto` must be strictly additive (`lower_snake_case`), preserve existing field numbers and RPCs, pass `buf format --diff --exit-code`, `buf lint`, and `buf breaking` against `main`, and synchronize `contracts/go` and `contracts/java` via `buf generate`.
- REST envelope (`AGENTS.md`, `docs/steering/api-contracts.md`): All endpoints return `camelCase` JSON wrapped in `{ "data": ..., "message": "..." }` (pagination `{ "items", "page", "pageSize", "totalItems", "totalPages" }` inside `data`) and `{ "code", "message", "errors"?, "traceId" }` on failure.
- Frontend boundaries & session security (`ecommerce-fe/AGENTS.md`): `admin-fe/` must not import from `ecommerce-fe/`, `services/`, or `contracts/`. Keep access tokens strictly in memory, use the `HttpOnly` refresh cookie at `/api/v1/auth` with single-flight refresh coalescing and single retry on `401`, and rely on effective RBAC permissions (`GET /api/v1/users/me/permissions`) rather than hardcoded role names.
- OpenDesign execution runtime: Commission OpenDesign prototype generation using the local Antigravity runtime (`agent: "antigravity"`).
- Windows verification compatibility: Keep `cart-service` untouched and ensure all Go, Maven, and `admin-fe` npm verification commands pass cleanly on Windows PowerShell.

## Review Focus

1. **Explicit `SecurityConfiguration` matcher & active-account check on `GET /api/v1/users/me/permissions`:** `SecurityConfiguration` ends with `anyRequest().denyAll()`; `/api/v1/users/me/permissions` must be explicitly registered under `.authenticated()`, and disabled accounts (`AccountStatus.DISABLED`) must not receive active permissions (`404 USER_NOT_FOUND` via `MeService` active check + `u.status = 'ACTIVE'` in `RbacRepository`).
2. **Atomic `reserved` invariant on `SetStock` & negative `AdjustStock` in `inventory-service`:** Calling `SetStock` with `quantity < reserved` or `AdjustStock` with `delta < 0` when `quantity + delta < reserved` (including when no `stock` row exists yet so `quantity == 0`) must fail atomically with `codes.FailedPrecondition` (`STOCK_BELOW_RESERVED` -> `409 STOCK_BELOW_RESERVED`), never violating `check (reserved <= quantity)`.
3. **Granular RBAC isolation between `product.view` and `inventory.view` in `bff-service`:** A `product.view`-only operator calling `GET /api/v1/admin/products` receives `200 OK` with `stockDegraded: true` and `stock: null` when `BatchGetStock` returns `PermissionDenied`, whereas an `inventory.view`-only operator calling `GET /api/v1/admin/inventory` succeeds via `ListInventoryCatalogItems` (including soft-deleted products) without requiring `product.view`.
4. **Product slug collision, validation bounds, and transactional media/child reconciliation in `catalog-service` (`ADR-012`):** Duplicate `id` on `CreateProduct` returns `codes.AlreadyExists` (`PRODUCT_ID_EXISTS` -> `409`), `originalPrice > 0 && originalPrice < price` or empty `variants` returns `codes.InvalidArgument` (`400 INVALID_REQUEST`), `CreateProductMediaUpload` enforces MIME/size bounds (`image/{jpeg,png,webp,avif} <= 5 MB`, `video/{mp4,webm} <= 30 MB`), `UpdateProduct` atomically replaces `product_images`, `product_videos`, `product_variants`, and `product_details` while transitioning removed `product_media_assets` to `ORPHANED` (`expires_at = now() + 24h`) and rejecting cross-product media hijacking (`MEDIA_ALREADY_ATTACHED` -> `409`), and `SoftDeleteProduct` preserves `ATTACHED` media so restored products never lose files.
5. **Customer-only login rejection, mid-flight session switch, direct MinIO `PUT` without bearer leakage, and 2-step initial stock failure recovery in `admin-fe`:** Authenticating with a `CUSTOMER`-only account (zero admin permissions) renders the "Không có quyền truy cập trang quản trị" guard screen with logout instead of broken `403` tables; session switches ignore stale in-flight responses; browser presigned `PUT` uploads send raw bytes with `Content-Type` via native `fetch` (never attaching Axios `Authorization: Bearer`); removing a media item in `ProductDrawer` only mutates form state until Save; and if `CreateProduct` succeeds while optional second-step `SetStock` fails, the Product Drawer closes/switches to created state and offers a clear retry/inventory action rather than re-POSTing `CreateProduct` (`409`).

---

## File Structure

- **Contracts (`contracts/`):**
  - Modify `contracts/proto/catalog/v1/catalog.proto` & `contracts/proto/inventory/v1/inventory.proto`; regenerate `contracts/go/**` and `contracts/java/src/main/generated/**`.
- **Identity Service (`services/identity-service/`):**
  - Create `src/main/resources/db/migration/V10__add_commerce_admin_permissions.sql`.
  - Modify `RbacPermissions.java`, `RbacRepository.java`, `MeService.java`, `MeController.java`, `SecurityConfiguration.java`, `IdentityUserRepository.java`, `AdminUserPage.java`, `AdminUserPageResponse.java`, `AdminUserService.java`, `AdminUserController.java`, plus corresponding tests (`IdentityDatabaseMigrationTest.java`, `RbacPersistenceTest.java`, `MeControllerTest.java`, `AdminUserControllerTest.java`, `AdminUserServiceTest.java`).
- **Catalog Service (`services/catalog-service/`):**
  - Create `internal/store/migrations/000003_add_admin_catalog_and_media.up.sql`, `internal/domain/admin.go` (+ `admin_test.go`), `internal/media/minio.go` (+ `minio_test.go`), `internal/media/sweeper.go` (+ `sweeper_test.go`), `internal/grpc/admin.go` (+ `admin_test.go`).
  - Modify `go.mod`, `internal/config/config.go` (+ `config_test.go`), `internal/domain/product.go`, `internal/domain/search.go`, `internal/store/store.go` (+ `store_test.go`), `internal/grpc/server.go` (+ `server_test.go`), `cmd/server/main.go`, `Dockerfile`.
- **Inventory Service (`services/inventory-service/`):**
  - Create `internal/authorization/client.go` (+ `client_test.go`), `internal/grpc/admin.go` (+ `admin_test.go`).
  - Modify `internal/config/config.go` (+ `config_test.go`), `internal/domain/stock.go` (+ `stock_test.go`), `internal/store/store.go` (+ `store_test.go`), `internal/grpc/server.go` (+ `server_test.go`), `cmd/server/main.go`, `Dockerfile`.
- **BFF Service (`services/bff-service/`):**
  - Modify `internal/catalog/client.go` (+ `client_test.go`), `internal/inventory/client.go` (+ `client_test.go`), `internal/http/products.go` (+ `products_test.go`), `internal/http/admin_products.go` (+ `admin_products_test.go`), `cmd/server/main.go` (+ `main_test.go`).
  - Create `internal/http/admin_inventory.go` (+ `admin_inventory_test.go`).
- **Gateway, Compose, Scripts & Docs:**
  - Modify `deployments/docker/compose.foundation.yml`, `deployments/docker/compose.production.yml`, `.env.production.example`, `deployments/kong/kong.yaml`, `deployments/kong/kong-production.yaml`, `.dockerignore`, `services/bff-service/internal/config/compose_foundation_test.go`, `kong_development_test.go`, `kong_production_test.go`, `compose_production_test.go`, `dockerfile_production_test.go`.
  - Modify `scripts/internal/boundaries/boundaries.go` (+ `scripts/check-boundaries/main_test.go`), `scripts/verify-service/main.go` (+ `main_test.go`), `scripts/start-local/main.go` (+ `main_test.go`), `Makefile`, `AGENTS.md`, `docs/steering/service-catalog.md`, `services/catalog-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/bff-service/AGENTS.md`.
- **Admin Dashboard (`admin-fe/`) & Storefront Video Gallery (`ecommerce-fe/`):**
  - Create standalone React + Vite + TS + Tailwind v4 app (`package.json`, `tsconfig*.json`, `vite.config.ts`, `vitest.config.ts`, `Dockerfile`, `nginx.conf`, `.dockerignore`, `AGENTS.md`, `index.html`, `src/main.tsx`, `src/App.tsx`, `src/index.css`).
  - Create `src/lib/` (`types.ts`, `auth-session.ts`, `permissions.ts`, `api.ts`, `media-upload.ts`, `categories.ts`, `format.ts`, `utils.ts` + tests).
  - Create `src/context/` & `src/hooks/` (`auth-context.ts`, `auth-provider.tsx`, `use-auth.ts` + tests).
  - Create `src/components/ui/` (`shadcn/ui` primitives) and `src/components/layout/` (`admin-shell.tsx`, `sidebar-nav.tsx`, `top-bar.tsx`) + `src/routes/require-admin-auth.tsx`.
  - Create `src/pages/` (`login-page.tsx`, `overview-page.tsx`, `products-page.tsx`, `product-drawer.tsx`, `inventory-page.tsx`, `stock-dialog.tsx`, `users-page.tsx`, `user-detail-sheet.tsx`, `rbac-page.tsx` + Vitest suites).
  - Modify `ecommerce-fe/src/lib/types.ts` and `ecommerce-fe/src/pages/product-detail-page.tsx` (+ `product-detail-page.test.tsx`) to play product `videos` in the storefront gallery.

---

### Task 1: Protobuf Contracts (`catalog.proto` & `inventory.proto`)

**Files:**
- Modify: `contracts/proto/catalog/v1/catalog.proto`
- Modify: `contracts/proto/inventory/v1/inventory.proto`
- Modify: `contracts/go/catalog/v1/*`, `contracts/go/inventory/v1/*` (generated)
- Modify: `contracts/java/src/main/generated/com/ecommerce/contracts/catalog/v1/*`, `contracts/java/src/main/generated/com/ecommerce/contracts/inventory/v1/*` (generated)

**Interfaces:**
- Produces (`ecommerce.catalog.v1.CatalogService`):
  - `ListAdminProducts(ListAdminProductsRequest) returns (ListAdminProductsResponse)`
  - `ListInventoryCatalogItems(ListInventoryCatalogItemsRequest) returns (ListInventoryCatalogItemsResponse)`
  - `GetAdminProduct(GetAdminProductRequest) returns (GetProductResponse)`
  - `CreateProduct(CreateProductRequest) returns (GetProductResponse)`
  - `UpdateProduct(UpdateProductRequest) returns (GetProductResponse)`
  - `CreateProductMediaUpload(CreateProductMediaUploadRequest) returns (CreateProductMediaUploadResponse)`
  - Additive `repeated string videos = 22;` on `GetProductResponse`, `int32 video_count = 21;` on `AdminProductSummary`, `repeated string videos = 19;` on `CreateProductRequest` / `UpdateProductRequest`, plus `CreateProductMediaUploadRequest` & `CreateProductMediaUploadResponse` (exact fields and numbers in Spec Section 2).
- Produces (`ecommerce.inventory.v1.InventoryService`):
  - `GetStock(GetStockRequest) returns (GetStockResponse)`
  - `BatchGetStock(BatchGetStockRequest) returns (BatchGetStockResponse)`
  - `ListStocks(ListStocksRequest) returns (ListStocksResponse)`
  - `SetStock(SetStockRequest) returns (SetStockResponse)`
  - `AdjustStock(AdjustStockRequest) returns (AdjustStockResponse)`
  - Messages: `StockItem`, `GetStockRequest`, `GetStockResponse`, `BatchGetStockRequest`, `BatchGetStockResponse`, `ListStocksRequest`, `ListStocksResponse`, `SetStockRequest`, `SetStockResponse`, `AdjustStockRequest`, `AdjustStockResponse` (exact fields and numbers in Spec Section 2).

- [ ] **Step 1: Add additive admin RPCs and messages to `catalog.proto` and `inventory.proto`**
  - Keep existing RPCs (`GetProduct`, `SearchProducts`, `SoftDeleteProduct`, `GetAvailability`) and field numbers untouched; do not add `google.api.http` annotations (`ADR-011`).

- [ ] **Step 2: Format, lint, breaking-check, and regenerate Go & Java contracts**
  - Run: `buf format -w && buf format --diff --exit-code && buf lint && buf breaking --against .git#branch=main && buf generate`
  - Expected: Exit code `0`, clean `git diff` after regeneration.

- [ ] **Step 3: Compile Go and Java contract consumers**
  - Run: `go build ./services/catalog-service/... ./services/inventory-service/... ./services/bff-service/... ./services/cart-service/... && mvn -pl contracts/java -am test-compile`
  - Expected: Exit code `0`.

---

### Task 2: `identity-service` V10 Permissions, Effective Permissions API, & User Status Filter/Counts

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V10__add_commerce_admin_permissions.sql`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/RbacPermissions.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/security/config/SecurityConfiguration.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/application/MeService.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/api/MeController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/application/AdminUserPage.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/api/AdminUserPageResponse.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/application/AdminUserService.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/api/AdminUserController.java`
- Test: `IdentityDatabaseMigrationTest.java`, `RbacPersistenceTest.java`, `MeControllerTest.java`, `AdminUserServiceTest.java`, `AdminUserControllerTest.java`

**Interfaces:**
- Produces:
  - Seeded system permissions attached to `ADMIN`: `product.view`, `product.create`, `product.update`, `inventory.view`, `inventory.update`.
  - `GET /api/v1/users/me/permissions` -> `200 OK` `{ "data": ["..."], "message": "Permissions retrieved." }` (`401` unauthenticated; `404 USER_NOT_FOUND` if caller account is missing or `DISABLED`).
  - `GET /api/v1/admin/users?q=&status=ACTIVE|DISABLED&page=1&pageSize=20` -> `{ "data": { "items": [...], "page": 1, "pageSize": 20, "totalItems": N, "totalPages": P, "activeCount": A, "disabledCount": D }, "message": "Users retrieved." }`.

- [ ] **Step 1: Write failing Identity tests for V10 permissions, `/api/v1/users/me/permissions` (including Review Focus #1), and admin user `status` filter + counts**
  - Update `IdentityDatabaseMigrationTest` seeded permission list with the 5 new permissions and `ADMIN` child assertions.
  - Add `RbacPersistenceTest` cases verifying `findAllEffectivePermissions(userId)` returns all permissions for `SUPER_ADMIN`, admin permissions for `ADMIN`, customer permissions for `CUSTOMER`, and `Set.of()` for `DISABLED` users.
  - Add `MeControllerTest` cases for `GET /api/v1/users/me/permissions` returning `200` for authenticated active caller, `401` without caller header, and `404` when `MeService` throws `UserNotFoundException` for a disabled account.
  - Add `AdminUserServiceTest` and `AdminUserControllerTest` cases for `list(query, status, page, pageSize)` returning filtered `items` plus global `activeCount` and `disabledCount`.

- [ ] **Step 2: Run targeted Identity tests to verify RED failure**
  - Run: `mvn -f services/identity-service/pom.xml test -Dtest=IdentityDatabaseMigrationTest,RbacPersistenceTest,MeControllerTest,AdminUserServiceTest,AdminUserControllerTest`
  - Expected: FAIL.

- [ ] **Step 3: Implement V10 migration, `RbacRepository.findAllEffectivePermissions`, `/api/v1/users/me/permissions`, `SecurityConfiguration` matcher, and `AdminUserService` status filter + counts**
  - Create `V10__add_commerce_admin_permissions.sql` upserting the 5 permissions into `auth_items` and `('ADMIN', <perm>)` into `auth_item_children`.
  - Add `.requestMatchers(HttpMethod.GET, "/api/v1/users/me", "/api/v1/users/me/permissions").authenticated()` in `SecurityConfiguration.java`.
  - In `MeService.permissions(UUID userId)`, verify active user exists via `repository.findWithProfileById(userId).filter(u -> u.status() == AccountStatus.ACTIVE).orElseThrow(UserNotFoundException::new)` then return `rbac.findAllEffectivePermissions(userId)`.
  - Extend `IdentityUserRepository.searchAdminUsers(String query, AccountStatus status, Pageable pageable)` (`and (:status is null or u.status = :status)`) plus `countByStatus(AccountStatus status)`, and add optional `@RequestParam(value = "status", required = false) AccountStatus status` on `AdminUserController#list` while keeping an overload `list(String, int, int)` on `AdminUserService` if existing tests call it directly.

- [ ] **Step 4: Run full `identity-service` test suite to verify GREEN**
  - Run: `mvn -pl services/identity-service -am test`
  - Expected: PASS (`BUILD SUCCESS`).

---

### Task 3: `catalog-service` Admin Product CRUD, MinIO Presigned Media Uploads, & Orphan Cleanup (`ADR-012`)

**Files:**
- Create: `services/catalog-service/internal/store/migrations/000003_add_admin_catalog_and_media.up.sql`
- Create: `services/catalog-service/internal/domain/admin.go`
- Create: `services/catalog-service/internal/domain/admin_test.go`
- Create: `services/catalog-service/internal/media/minio.go`
- Create: `services/catalog-service/internal/media/minio_test.go`
- Create: `services/catalog-service/internal/media/sweeper.go`
- Create: `services/catalog-service/internal/media/sweeper_test.go`
- Create: `services/catalog-service/internal/grpc/admin.go`
- Create: `services/catalog-service/internal/grpc/admin_test.go`
- Modify: `services/catalog-service/go.mod`, `services/catalog-service/go.sum`
- Modify: `services/catalog-service/internal/config/config.go`, `services/catalog-service/internal/config/config_test.go`
- Modify: `services/catalog-service/internal/domain/product.go`
- Modify: `services/catalog-service/internal/domain/search.go`
- Modify: `services/catalog-service/internal/store/store.go`
- Modify: `services/catalog-service/internal/store/store_test.go`
- Modify: `services/catalog-service/internal/grpc/server.go`
- Modify: `services/catalog-service/internal/grpc/server_test.go`
- Modify: `services/catalog-service/cmd/server/main.go`
- Modify: `services/catalog-service/Dockerfile`

**Interfaces:**
- Consumes: `AuthorizationService.CheckPermission` (`product.view`, `product.create`, `product.update`, `inventory.view`) and MinIO S3 API (`PutObject` presigning + `RemoveObject` orphan cleanup).
- Produces (`domain.Repository` additions, `media.Signer`, & `CatalogServiceServer` RPCs):
  - `ListAdmin(ctx, AdminSearchFilter) (*AdminSearchResult, error)`
  - `ListInventoryItems(ctx, InventoryCatalogFilter) ([]*InventoryCatalogItem, error)`
  - `Create(ctx, *Product) (*Product, error)` (`domain.ErrProductExists` on PK conflict -> `codes.AlreadyExists` `PRODUCT_ID_EXISTS`; `domain.ErrMediaAlreadyAttached` -> `codes.FailedPrecondition` `MEDIA_ALREADY_ATTACHED`)
  - `Update(ctx, *Product) (*Product, error)` (`domain.ErrProductNotFound` -> `codes.NotFound`; transitions removed `product_media_assets` to `ORPHANED` with `expires_at = now() + 24h`)
  - `CreateMediaUpload(ctx, *MediaAsset) error` & `SweepExpiredMedia(ctx, batchSize, deleter) (int, error)`
  - `CreateProductMediaUpload` RPC (authorizes `product.create` or `product.update`, validates `image/{jpeg,png,webp,avif} <= 5MB` and `video/{mp4,webm} <= 30MB`, records `PENDING` asset, returns signed `PUT` URL + `public_url`).

- [ ] **Step 1: Write failing unit & store tests for admin validation, slug generation, MinIO presigning/URL rewriting, orphan sweeper, `Create`/`Update` child & media reconciliation, `ListAdmin`, `ListInventoryItems`, and gRPC authorization (Review Focus #4)**
  - In `admin_test.go` (`internal/domain`): test `ValidateAndPrepareCreate` / `ValidateAndPrepareUpdate` (auto-slug generation from Vietnamese name when `id == ""`, custom `id` regex validation, `originalPrice < price` rejection, empty variants rejection, invalid colorHex/media URL rejection, `videos` max 3 bound) and `ValidateMediaUpload` (`image/* <= 5MB`, `video/* <= 30MB`, unsupported MIME / oversized rejection).
  - In `minio_test.go` & `sweeper_test.go`: test presigned URL host-to-relative rewriting when `MINIO_PUBLIC_BASE_URL=/product-media` vs full local `http://localhost:9000/product-media`, and test `Sweeper` deleting expired `PENDING`/`ORPHANED` objects idempotently while retaining row on S3 failure.
  - In `store_test.go`: test migration `000003` (`updated_at`, `product_videos`, `product_media_assets`), `Create` duplicate ID returning `domain.ErrProductExists`, `Create`/`Update` attaching `PENDING` media, `Update` transitioning removed image/video URLs to `ORPHANED` (`expires_at > now()`) while re-attaching within grace window, rejecting media attached to a different product (`domain.ErrMediaAlreadyAttached`), `SoftDelete` preserving `ATTACHED` status, `ListAdmin`, and `ListInventoryItems`.
  - In `admin_test.go` (`internal/grpc`): test `ListAdminProducts`, `ListInventoryCatalogItems`, `GetAdminProduct`, `CreateProduct`, `UpdateProduct`, and `CreateProductMediaUpload` (`product.create` OR `product.update` authorization, `MEDIA_STORAGE_UNAVAILABLE` when signer unconfigured/failing).

- [ ] **Step 2: Run `catalog-service` tests to verify RED failure**
  - Run: `go test ./services/catalog-service/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement migration `000003`, `minio-go/v7` media signer + orphan sweeper, domain validation/slugging, transactional PostgreSQL store methods, and gRPC handlers**
  - Add `Videos []string`, `CreatedAt`, and `UpdatedAt` to `domain.Product` / `AdminSummary`.
  - Implement `internal/media` (`minio-go/v7` bucket policy bootstrap for anonymous `s3:GetObject`, `PresignPut`, `RemoveObject`, and periodic `Sweeper`).
  - Implement `Create`, `Update`, `CreateMediaAsset`, `SweepExpiredMedia`, `ListAdmin`, and `ListInventoryItems` in `store.go` using `pgx.Tx` for atomic parent + child + `product_media_assets` reconciliation.
  - Wire MinIO config/env defaults in `internal/config/config.go`, `cmd/server/main.go`, and `services/catalog-service/Dockerfile`.

- [ ] **Step 4: Run `catalog-service` tests to verify GREEN**
  - Run: `go test ./services/catalog-service/...`
  - Expected: PASS.

---

### Task 4: `inventory-service` Private Authorization Wiring & Stock Read/Mutation RPCs

**Files:**
- Create: `services/inventory-service/internal/authorization/client.go`
- Create: `services/inventory-service/internal/authorization/client_test.go`
- Create: `services/inventory-service/internal/grpc/admin.go`
- Create: `services/inventory-service/internal/grpc/admin_test.go`
- Modify: `services/inventory-service/internal/config/config.go`
- Modify: `services/inventory-service/internal/config/config_test.go`
- Modify: `services/inventory-service/internal/domain/stock.go`
- Modify: `services/inventory-service/internal/domain/stock_test.go`
- Modify: `services/inventory-service/internal/store/store.go`
- Modify: `services/inventory-service/internal/store/store_test.go`
- Modify: `services/inventory-service/internal/grpc/server.go`
- Modify: `services/inventory-service/internal/grpc/server_test.go`
- Modify: `services/inventory-service/cmd/server/main.go`
- Modify: `services/inventory-service/Dockerfile`

**Interfaces:**
- Consumes: `AuthorizationService.CheckPermission` at `IDENTITY_GRPC_ADDR` (`127.0.0.1:50054` default, `500ms` `IDENTITY_GRPC_TIMEOUT`).
- Produces (`InventoryServiceServer` RPCs):
  - `GetStock`, `BatchGetStock`, `ListStocks` (require `inventory.view`)
  - `SetStock`, `AdjustStock` (require `inventory.update`; return `codes.FailedPrecondition` with `ErrorInfo{Reason: "STOCK_BELOW_RESERVED", Domain: "ecommerce.inventory.v1"}` when resulting `quantity < reserved` or `< 0`, `codes.PermissionDenied` with `INVENTORY_PERMISSION_DENIED`, `codes.Unavailable` with `AUTHORIZATION_UNAVAILABLE`).

- [ ] **Step 1: Write failing tests for config, authorization client, store atomic upsert/adjust (Review Focus #2), and gRPC handlers**
  - Test `config.Load()` defaults (`IDENTITY_GRPC_ADDR=127.0.0.1:50054`, `IDENTITY_GRPC_TIMEOUT=500ms`) and validation.
  - Test `store.BatchGet`, `store.ListAll`, `store.SetQuantity` (including `quantity < reserved` returning `domain.ErrStockBelowReserved`), and `store.AdjustQuantity` (`delta > 0` creating missing row, `delta < 0` on missing row or `quantity + delta < reserved` returning `domain.ErrStockBelowReserved`).
  - Test `GetAvailability` still works without `user_id` while `GetStock`, `BatchGetStock`, `ListStocks`, `SetStock`, and `AdjustStock` enforce `inventory.view` / `inventory.update` and propagate `x-trace-id`.

- [ ] **Step 2: Run `inventory-service` tests to verify RED failure**
  - Run: `go test ./services/inventory-service/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement authorization client, config, atomic store operations, `NewServer(repo, authClient)`, `cmd/server/main.go` wiring, and `Dockerfile` env vars**
  - Mirror `services/catalog-service/internal/authorization/client.go` inside `services/inventory-service/internal/authorization/client.go` (no cross-service import).
  - Add `ENV IDENTITY_GRPC_ADDR="identity-service:50054"` and `ENV IDENTITY_GRPC_TIMEOUT="500ms"` to `services/inventory-service/Dockerfile`.

- [ ] **Step 4: Run `inventory-service` tests to verify GREEN**
  - Run: `go test ./services/inventory-service/...`
  - Expected: PASS.

---

### Task 5: `bff-service` Admin Products (Including Media Presign), Storefront Video Mapping, & Inventory Composition Handlers

**Files:**
- Modify: `services/bff-service/internal/catalog/client.go`
- Modify: `services/bff-service/internal/catalog/client_test.go`
- Modify: `services/bff-service/internal/inventory/client.go`
- Modify: `services/bff-service/internal/inventory/client_test.go`
- Modify: `services/bff-service/internal/http/products.go`
- Modify: `services/bff-service/internal/http/products_test.go`
- Modify: `services/bff-service/internal/http/admin_products.go`
- Modify: `services/bff-service/internal/http/admin_products_test.go`
- Create: `services/bff-service/internal/http/admin_inventory.go`
- Create: `services/bff-service/internal/http/admin_inventory_test.go`
- Modify: `services/bff-service/cmd/server/main.go`
- Modify: `services/bff-service/cmd/server/main_test.go`

**Interfaces:**
- Produces BFF REST endpoints (admin routes wrapped in `correlateTrace(RequireCaller(...))`):
  - `GET /api/v1/storefront/products/{id}` -> includes additive `videos: []string` on `productDetail`
  - `POST /api/v1/admin/products/media/presign` -> `200 OK`, message `"Upload URL generated."`, `{ uploadId, mediaType, uploadUrl, publicUrl, objectKey, expiresInSeconds }`
  - `GET /api/v1/admin/products` -> `{ items, page, pageSize, totalItems, totalPages, activeCount, deletedCount, categories, stockDegraded }`
  - `GET /api/v1/admin/products/{id}` -> full product detail (`images` + `videos`) + `stock` (`{ quantity, reserved, availableQuantity, updatedAt } | null`)
  - `POST /api/v1/admin/products` -> `201 Created`, message `"Product created."`
  - `PUT /api/v1/admin/products/{id}` -> `200 OK`, message `"Product updated."`
  - `POST /api/v1/admin/products/{id}/delete` -> `200 OK`, message `"Product availability updated."`
  - `GET /api/v1/admin/inventory` -> `{ items, page, pageSize, totalItems, totalPages, summary: { totalTrackedProducts, inStockCount, lowStockCount, outOfStockCount, totalQuantity, totalReserved, totalAvailable } }`
  - `PUT /api/v1/admin/inventory/{productId}` -> `200 OK`, message `"Stock quantity updated."`
  - `POST /api/v1/admin/inventory/{productId}/adjust` -> `200 OK`, message `"Stock adjusted."`

- [ ] **Step 1: Write failing BFF client and HTTP handler tests (including Review Focus #3)**
  - Test `catalog.Client` mapping `PRODUCT_ID_EXISTS` -> `ErrProductExists`, `MEDIA_ALREADY_ATTACHED` -> `ErrMediaAlreadyAttached`, `MEDIA_STORAGE_UNAVAILABLE` -> `ErrMediaStorageUnavailable`, `PRODUCT_PERMISSION_DENIED` -> `ErrPermissionDenied`, `AUTHORIZATION_UNAVAILABLE` -> `ErrAuthorizationUnavailable`, `codes.NotFound` -> `ErrProductNotFound`, plus outgoing `x-trace-id` propagation.
  - Test `inventory.Client` mapping `INVENTORY_PERMISSION_DENIED` -> `ErrPermissionDenied`, `AUTHORIZATION_UNAVAILABLE` -> `ErrAuthorizationUnavailable`, `STOCK_BELOW_RESERVED` -> `ErrStockBelowReserved`.
  - Test `ProductsHandler.detail` mapping `videos` from `GetProductResponse`.
  - Test `AdminProductsHandler`: `POST /media/presign` (`200`, `400 INVALID_REQUEST`, `403 FORBIDDEN`, `503 MEDIA_STORAGE_UNAVAILABLE`), `GET` list with stock enrichment + graceful `stockDegraded: true`, `GET /{id}` (`images` + `videos`), `POST` (`201`, `409 PRODUCT_ID_EXISTS`, `409 MEDIA_ALREADY_ATTACHED`, `400 INVALID_REQUEST`), `PUT /{id}` (`200`, `404 PRODUCT_NOT_FOUND`), `POST /{id}/delete`.
  - Test `AdminInventoryHandler`: `GET` merging `ListInventoryCatalogItems` + `ListStocks` (zero-stock default for catalog items lacking a `stock` row, soft-deleted item inclusion, `stockStatus` filtering `in_stock` `> 5` / `low_stock` `1..5` / `out_of_stock` `0`, accurate KPI `summary`), `403 FORBIDDEN` when `ListStocks` denies `inventory.view`, `PUT /{productId}` & `POST /{productId}/adjust` (`404 PRODUCT_NOT_FOUND` when Catalog lacks product, `409 STOCK_BELOW_RESERVED` on conflict).

- [ ] **Step 2: Run `bff-service` handler/client tests to verify RED failure**
  - Run: `go test ./services/bff-service/internal/catalog/... ./services/bff-service/internal/inventory/... ./services/bff-service/internal/http/... ./services/bff-service/cmd/...`
  - Expected: FAIL.

- [ ] **Step 3: Implement BFF gRPC client extensions, `ProductsHandler` `videos` field, `AdminProductsHandler`, `AdminInventoryHandler`, and `buildMux` wiring**
  - Update `NewAdminProductsHandler(catalogClient, inventoryClient)` and wire `NewAdminInventoryHandler(catalogClient, inventoryClient)` in `cmd/server/main.go`.

- [ ] **Step 4: Run `bff-service` tests (excluding `internal/config` Compose/Kong assertions reserved for Task 6) to verify GREEN**
  - Run: `go test ./services/bff-service/internal/catalog/... ./services/bff-service/internal/inventory/... ./services/bff-service/internal/http/... ./services/bff-service/cmd/...`
  - Expected: PASS.

---

### Task 6: MinIO Infrastructure, Kong Gateway, Production Compose, Monorepo Tooling, & Surface Registration

**Files:**
- Modify: `deployments/docker/compose.foundation.yml`
- Modify: `deployments/docker/compose.production.yml`
- Modify: `.env.production.example`
- Modify: `deployments/kong/kong.yaml`
- Modify: `deployments/kong/kong-production.yaml`
- Modify: `.dockerignore`
- Modify: `services/bff-service/internal/config/compose_foundation_test.go`
- Modify: `services/bff-service/internal/config/kong_development_test.go`
- Modify: `services/bff-service/internal/config/kong_production_test.go`
- Modify: `services/bff-service/internal/config/compose_production_test.go`
- Modify: `services/bff-service/internal/config/dockerfile_production_test.go`
- Modify: `scripts/internal/boundaries/boundaries.go`
- Modify: `scripts/check-boundaries/main_test.go`
- Modify: `scripts/verify-service/main.go`
- Modify: `scripts/verify-service/main_test.go`
- Modify: `scripts/start-local/main.go`
- Modify: `scripts/start-local/main_test.go`
- Modify: `Makefile`
- Modify: `AGENTS.md`, `docs/steering/service-catalog.md`, `services/catalog-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/bff-service/AGENTS.md`

**Interfaces:**
- Produces:
  - Foundation Compose `minio` (`:9000` API with CORS `http://localhost:5173,http://localhost:5174`, `127.0.0.1:9001` Console, `minio_data` volume) & Production Compose internal `minio` (`minio_data` volume, fail-fast `${MINIO_ROOT_USER:?...}` / `${MINIO_ROOT_PASSWORD:?...}`, zero host ports).
  - Kong dev route `admin-inventory-api` (`/api/v1/admin/inventory`) and CORS origin `http://localhost:5174`.
  - Kong prod route `admin-inventory-api` (`GET, POST, PUT, PATCH`), expanded `admin-products-api` (`GET, POST, PUT, PATCH, DELETE`), `minio` service (`http://minio:9000`, route `product-media-storage` on `/product-media` with `GET, HEAD, PUT`, `preserve_host: false`, `request-transformer`), and `admin-fe` service (`http://admin-fe:8080`, route `admin-spa` on `/admin`).
  - CLI/Make support: `make admin-fe`, `make verify SERVICE=admin-fe`, and `check-boundaries` scanning `admin-fe/src` (forbidding imports from `services/`, `contracts/`, and `ecommerce-fe/`).

- [ ] **Step 1: Write failing tests in `services/bff-service/internal/config/*_test.go` and `scripts/*/*_test.go`**
  - Update `compose_foundation_test.go` for `minio` + `minio_data`.
  - Update `kong_development_test.go` to require `http://localhost:5174` in CORS origins and `admin-inventory-api` on `bff-service`.
  - Update `kong_production_test.go` to expect 5 services (`identity-service`, `bff-service`, `ecommerce-fe`, `admin-fe`, `minio`), `admin-products-api` methods `["GET", "POST", "PUT", "PATCH", "DELETE"]`, `admin-inventory-api` methods `["GET", "POST", "PUT", "PATCH"]`, `product-media-storage` on `minio` (`paths: ["/product-media"]`, `methods: ["GET", "HEAD", "PUT"]`, `strip_path: false`, `preserve_host: false`, `request-transformer`), and `admin-spa` on `admin-fe` (`paths: ["/admin"]`, `methods: ["GET", "HEAD"]`, `strip_path: false`, `request-transformer`).
  - Update `compose_production_test.go` and `dockerfile_production_test.go` for `minio`, `catalog-service` MinIO env/depends_on, `inventory-service` -> `identity-service:50054`, `admin-fe` service + `kong.depends_on`, `.env.production.example` MinIO keys, and `admin-fe/dist` in `.dockerignore`.
  - Add boundary test verifying `admin-fe/src` rejects imports from `services/*`, `contracts/*`, and `ecommerce-fe/*`; add `verify-service` and `start-local` unit tests for `admin-fe`.

- [ ] **Step 2: Run `scripts` and `bff-service/internal/config` tests to verify RED failure**
  - Run: `go test ./scripts/... ./services/bff-service/internal/config/...`
  - Expected: FAIL.

- [ ] **Step 3: Update Foundation & Production Compose, `.env.production.example`, Kong configs, `.dockerignore`, `admin-fe` container files, Go scripts, `Makefile`, and steering/AGENTS docs**
  - Apply all configuration, script, and documentation updates (creating `admin-fe/Dockerfile`, `admin-fe/nginx.conf`, and `admin-fe/.dockerignore`).

- [ ] **Step 4: Run `scripts` and `bff-service` tests to verify GREEN**
  - Run: `go test ./scripts/... ./services/bff-service/...`
  - Expected: PASS.

---

### Task 7: OpenDesign Visual Prototype (Local Antigravity) & `admin-fe/` Foundation + Auth Shell

**Files:**
- OpenDesign Workspace: project `"S-Shop Admin Control Center"` (generated via local Antigravity runtime and inspected via `get_artifact`)
- Create: `admin-fe/AGENTS.md`, `admin-fe/package.json`, `admin-fe/tsconfig.json`, `admin-fe/tsconfig.app.json`, `admin-fe/tsconfig.node.json`, `admin-fe/vite.config.ts`, `admin-fe/vitest.config.ts`, `admin-fe/index.html`
- Create: `admin-fe/src/index.css`, `admin-fe/src/test/setup.ts`, `admin-fe/src/lib/utils.ts`, `admin-fe/src/lib/format.ts`, `admin-fe/src/lib/categories.ts`, `admin-fe/src/lib/types.ts`, `admin-fe/src/lib/auth-session.ts`, `admin-fe/src/lib/permissions.ts`, `admin-fe/src/lib/api.ts`, `admin-fe/src/lib/media-upload.ts`
- Create: `admin-fe/src/components/ui/*` (`button.tsx`, `input.tsx`, `textarea.tsx`, `label.tsx`, `badge.tsx`, `card.tsx`, `dialog.tsx`, `sheet.tsx`, `select.tsx`, `tabs.tsx`, `switch.tsx`, `table.tsx`, `dropdown-menu.tsx`, `skeleton.tsx`, `separator.tsx`, `alert-dialog.tsx`)
- Create: `admin-fe/src/context/auth-context.ts`, `admin-fe/src/context/auth-provider.tsx`, `admin-fe/src/hooks/use-auth.ts`, `admin-fe/src/routes/require-admin-auth.tsx`
- Create: `admin-fe/src/components/layout/admin-shell.tsx`, `admin-fe/src/pages/login-page.tsx`, `admin-fe/src/App.tsx`, `admin-fe/src/main.tsx`
- Test: `admin-fe/src/lib/api.test.ts`, `admin-fe/src/lib/media-upload.test.ts`, `admin-fe/src/context/auth-provider.test.tsx`, `admin-fe/src/pages/login-page.test.tsx`

**Interfaces:**
- Produces:
  - OpenDesign-anchored token system in `admin-fe/src/index.css` and `AdminShell` layout.
  - Typed API client (`api.ts`) + direct MinIO uploader (`media-upload.ts`) covering Auth, Current Operator, Admin Products (`presignProductMediaUpload` + direct `fetch` `PUT` without bearer headers), Admin Inventory, Admin Users, and Admin RBAC.
  - `useAuth()` hook exposing `{ user, permissions, status, can(perm), hasAnyAdminAccess, login, logout, refreshPermissions }`.

- [ ] **Step 1: Generate OpenDesign visual prototype via local Antigravity runtime**
  - Call OpenDesign `collect_brief` (`skip: true` with project title `"S-Shop Admin Control Center"` and comprehensive Vietnamese commerce admin brief) -> `confirm_brief` -> `create_project` -> `start_run` with `agent: "antigravity"`.
  - Poll `get_run` until terminal `succeeded`, then pull the design bundle once via `get_artifact` to extract palette, typography, sidebar/topbar structure, KPI card styling, data table density, and drawer/modal patterns.

- [ ] **Step 2: Scaffold `admin-fe/` (`port: 5174`, `base: process.env.VITE_BASE_PATH || "/"`) and install dependencies**
  - Include `react`, `react-dom`, `react-router-dom`, `axios`, `zod`, `react-hook-form`, `@hookform/resolvers`, `@tanstack/react-table`, `recharts`, `lucide-react`, `radix-ui`, `class-variance-authority`, `clsx`, `tailwind-merge`, `sonner`, plus dev dependencies (`vite`, `@vitejs/plugin-react`, `@tailwindcss/vite`, `tailwindcss`, `typescript`, `vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`, `oxlint`).
  - Run: `npm --prefix admin-fe install`

- [ ] **Step 3: Write failing Vitest tests for `api.ts` (single-flight refresh, envelope/error mapping), `media-upload.ts` (MIME/size pre-check + direct `PUT` without Authorization header), `AuthProvider` (session restore, mid-flight epoch guard, customer-only rejection — Review Focus #5), and `LoginPage`**
  - Run: `npm --prefix admin-fe run test`
  - Expected: FAIL.

- [ ] **Step 4: Implement `src/lib/*`, `shadcn/ui` primitives, `AuthProvider`, `RequireAdminAuth`, `AdminShell`, and `LoginPage` styled with the OpenDesign design tokens**
  - Gate console routes in `RequireAdminAuth`: unauthenticated redirects to `/login`; authenticated with `hasAnyAdminAccess === false` renders the access-denied screen with operator email and `"Đăng xuất / Đổi tài khoản"` action; Sidebar links render dynamically per `can(...)`.

- [ ] **Step 5: Run `admin-fe` tests, lint, and build to verify GREEN**
  - Run: `npm --prefix admin-fe run test && npm --prefix admin-fe run lint && npm --prefix admin-fe run build`
  - Expected: PASS.

---

### Task 8: `admin-fe/` Overview KPIs, Products CRUD (With Direct MinIO Image/Video Upload), & Inventory Stock Management

**Files:**
- Create: `admin-fe/src/pages/overview-page.tsx`
- Create: `admin-fe/src/pages/overview-page.test.tsx`
- Create: `admin-fe/src/pages/products-page.tsx`
- Create: `admin-fe/src/pages/product-drawer.tsx`
- Create: `admin-fe/src/pages/products-page.test.tsx`
- Create: `admin-fe/src/pages/inventory-page.tsx`
- Create: `admin-fe/src/pages/stock-dialog.tsx`
- Create: `admin-fe/src/pages/inventory-page.test.tsx`
- Modify: `admin-fe/src/App.tsx`

**Interfaces:**
- Consumes: `useAuth()`, `api.ts`, `media-upload.ts` (`uploadProductMediaFile`).

- [ ] **Step 1: Write failing Vitest suites for `OverviewPage`, `ProductsPage` + `ProductDrawer` (including direct MinIO image/video upload, UI-only `×` removal before Save, and 2-step initial stock failure recovery — Review Focus #5), and `InventoryPage` + `StockDialog`**
  - `overview-page.test.tsx`: renders permitted KPI cards, category/stock charts, and low-stock watchlist; skips unauthorized sections cleanly when operator only holds `inventory.view`.
  - `products-page.test.tsx`: renders paginated TanStack Table, filters by status (`Tất cả` / `Đang bán` / `Đã ẩn`) and search query, uploads an image + video via presigned `PUT` in `ProductDrawer`, removes an existing media item in form state without calling delete before Save, creates a product with preset category + variant + optional initial stock, handles 2-step partial failure (`createAdminProduct` `201` + `setProductStock` `502` -> refreshes product list without re-POSTing product and shows retry stock prompt), edits an existing product, and toggles soft-delete/restore.
  - `inventory-page.test.tsx`: renders enriched stock rows (including soft-deleted badge and zero-stock items), filters by `Sắp hết` / `Hết hàng`, submits `SetStock` and `AdjustStock` (`+/-`), and displays inline error on `409 STOCK_BELOW_RESERVED`.

- [ ] **Step 2: Run Vitest on the new suites to verify RED failure**
  - Run: `npm --prefix admin-fe run test`
  - Expected: FAIL.

- [ ] **Step 3: Implement `OverviewPage`, `ProductsPage`, `ProductDrawer`, `InventoryPage`, and `StockDialog` and wire routes in `App.tsx`**
  - Use `@tanstack/react-table` for `ProductsPage` and `InventoryPage`, `react-hook-form` + `zod` for `ProductDrawer` and `StockDialog`, and `recharts` (`ResponsiveContainer`, `BarChart` / `PieChart`) for `OverviewPage`.

- [ ] **Step 4: Run `admin-fe` tests, lint, and build to verify GREEN**
  - Run: `npm --prefix admin-fe run test && npm --prefix admin-fe run lint && npm --prefix admin-fe run build`
  - Expected: PASS.

---

### Task 9: `admin-fe/` Users Administration & RBAC Hierarchy

**Files:**
- Create: `admin-fe/src/pages/users-page.tsx`
- Create: `admin-fe/src/pages/user-detail-sheet.tsx`
- Create: `admin-fe/src/pages/users-page.test.tsx`
- Create: `admin-fe/src/pages/rbac-page.tsx`
- Create: `admin-fe/src/pages/rbac-page.test.tsx`
- Modify: `admin-fe/src/App.tsx`

**Interfaces:**
- Consumes: `useAuth()`, `api.ts` (`listAdminUsers`, `getAdminUser`, `updateAdminUserStatus`, `assignAdminUserRole`, `revokeAdminUserRole`, `listRbacItems`, `createRbacItem`, `updateRbacItem`, `deleteRbacItem`, `addRbacChild`, `removeRbacChild`).

- [ ] **Step 1: Write failing Vitest suites for `UsersPage` + `UserDetailSheet` and `RbacPage`**
  - `users-page.test.tsx`: lists users with `ACTIVE` / `DISABLED` filter tabs and global counts, searches by email/name, opens `UserDetailSheet`, toggles status (`ACTIVE` <-> `DISABLED`) with confirmation, assigns/revokes roles, and surfaces backend protection errors (`SELF_DISABLE_FORBIDDEN`, `LAST_SUPER_ADMIN_REQUIRED`, `SUPER_ADMIN_ASSIGNMENT_FORBIDDEN`).
  - `rbac-page.test.tsx`: renders Roles & Permissions with `systemItem` lock indicators, creates/updates/deletes custom items, inspects direct vs. inherited hierarchy permissions, attaches/detaches child permissions/roles, and surfaces hierarchy cycle conflict (`409`).

- [ ] **Step 2: Run Vitest to verify RED failure**
  - Run: `npm --prefix admin-fe run test`
  - Expected: FAIL.

- [ ] **Step 3: Implement `UsersPage`, `UserDetailSheet`, and `RbacPage`, and wire `/users` and `/rbac` in `App.tsx`**
  - Gate mutation controls with `can("user.status.update")`, `can("user.role.assign")`, and `can("rbac.manage")` so read-only auditors (`user.view`, `rbac.view`) can inspect without broken action buttons.

- [ ] **Step 4: Run `admin-fe` tests, lint, and build to verify GREEN**
  - Run: `npm --prefix admin-fe run test && npm --prefix admin-fe run lint && npm --prefix admin-fe run build`
  - Expected: PASS.

---

### Task 10: Storefront Video Gallery (`ecommerce-fe/`) & Full Repository Verification

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Modify: `ecommerce-fe/src/pages/product-detail-page.tsx`
- Modify: `ecommerce-fe/src/pages/product-detail-page.test.tsx`

**Interfaces:**
- Consumes: `ProductDetail.videos?: string[]` from `GET /api/v1/storefront/products/{id}`.

- [ ] **Step 1: Write failing Vitest test in `ecommerce-fe/src/pages/product-detail-page.test.tsx` for mixed Image + Video gallery selection and HTML5 `<video>` playback**
  - Run: `npm --prefix ecommerce-fe run test -- src/pages/product-detail-page.test.tsx`
  - Expected: FAIL.

- [ ] **Step 2: Implement `videos?: string[]` in `ecommerce-fe/src/lib/types.ts` and mixed image/video gallery rendering in `ecommerce-fe/src/pages/product-detail-page.tsx`**
  - Render `<img />` when active gallery item is an image and `<video controls playsInline preload="metadata" />` when active item is a video, with a Play icon indicator on video thumbnails.

- [ ] **Step 3: Run full surface & repository verification gates**
  - Run:
    - `make check-boundaries`
    - `make verify SERVICE=contracts`
    - `make verify SERVICE=identity-service`
    - `make verify SERVICE=catalog-service`
    - `make verify SERVICE=inventory-service`
    - `make verify SERVICE=bff-service`
    - `make verify SERVICE=ecommerce-fe`
    - `make verify SERVICE=admin-fe`
    - `make prod-config` (using `.env.production.example` if `.env.production` is absent)
  - Expected: Exit code `0` across every gate, followed by independent `code-reviewer` and `boundary-reviewer` checks.

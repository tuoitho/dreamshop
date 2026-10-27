# Product Favorites Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an end-to-end, account-synchronized Product Favorites (`favorites`) capability restricted to customer accounts via Identity RBAC, updating product cards, product detail pages, site navigation, and a dedicated `/yeu-thich` management page.

**Architecture:** `identity-service` seeds a system `favorite.manage` permission under `CUSTOMER` via Flyway `V9`. `cart-service` (`cart_db`) persists `(user_id, product_id)` rows in `favorite_items`, authorizes every favorite RPC against `Identity` authorization gRPC (`:50054`), verifies product existence/state on `AddFavorite`, and enriches favorite lists concurrently from `CatalogService` (`:50051`) and `InventoryService` (`:50052`). `bff-service` adapts the gRPC contract to `/api/v1/favorites` behind Kong `caller-context` JWT authentication, and `ecommerce-fe` provides session-safe `FavoritesProvider` state across `SiteHeader`, `ProductCard`, `ProductDetailPage`, and `FavoritesPage` (`/yeu-thich`).

**Tech Stack:** Protobuf / Buf, Java 21 / Spring Boot 4.1 / Flyway / Testcontainers, Go 1.24 (`google.golang.org/grpc`, `pgx/v5`, `sqlc`, `net/http`), Kong 3.8 declarative config, React 19 / TypeScript 6 / Vite 8 / Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-27-product-favorites-design.md`

## Global Constraints

- Service boundaries (`AGENTS.md`, `docs/steering/service-boundaries.md`): No cross-service code imports, foreign database queries, or internal calls through Kong (`:8000`). `bff-service` calls `cart-service` (`:50053`), and `cart-service` calls `catalog-service` (`:50051`), `inventory-service` (`:50052`), and `identity-service` (`:50054`) over direct private gRPC.
- Contract compatibility (`docs/steering/api-contracts.md`): Protobuf additions in `contracts/proto/cart/v1/cart.proto` must be strictly additive (`lower_snake_case`), pass `buf format --diff --exit-code`, `buf lint`, and `buf breaking` against `main`, and synchronize generated Go and Java outputs via `buf generate`.
- REST envelope (`AGENTS.md`, `docs/steering/api-contracts.md`): All `/api/v1/favorites` responses use `camelCase` JSON fields, `{ "data": ..., "message": "..." }` for `2xx`, and `{ "code": "...", "message": "...", "traceId": "..." }` for errors.
- Windows verification compatibility (`services/cart-service/AGENTS.md`): Do not modify `services/cart-service/cmd/server/main.go` or `main_test.go` in ways that break the Linux `syscall.Kill` signal test suite; run `./services/cart-service/internal/...` for Go tests on Windows alongside `go build ./services/cart-service/...`.
- Terminology & toast conventions: Use `favorite` / `favorites` consistently across all layers. Do not trigger manual mutation toasts in React components for `/favorites` mutations because `ecommerce-fe/src/lib/api.ts` already toasts mutation `message` responses automatically.

## Review Focus

1. **Trace propagation & caller cancellation on `cart-service` favorite RPCs:** Propagate only validated `x-trace-id` (`<= 64` safe characters) to `AuthorizationService`, return context cancellation status when the caller cancels instead of masking it as `AUTHORIZATION_UNAVAILABLE`, and handle empty favorite lists without spawning workers or calling Catalog/Inventory.
2. **Idempotent duplicate add & idempotent remove of absent `productId`:** Re-adding an already-favorited `product_id` succeeds (`ON CONFLICT (user_id, product_id) DO NOTHING`) while keeping its original `created_at` sort position; removing a `product_id` that is not in `favorite_items` returns `200 OK` with the current `FavoriteList` rather than `404`.
3. **Input validation & URL encoding on `productId`:** Whitespace-only or `> 128`-character `productId` is rejected with `InvalidArgument` / `400 INVALID_PARAMETER`, unknown or trailing JSON values on `POST /api/v1/favorites/items` return `400 INVALID_PARAMETER`, and `ecommerce-fe/src/lib/api.ts` encodes `productId` with `encodeURIComponent` on `DELETE /favorites/items/{productId}`.
4. **Mid-flight session switch / logout in `FavoritesProvider`:** In-flight `getFavorites()` or mutation promises resolving after logout or switching to a second user are ignored via session-version and operation-generation guards so stale `items` or `forbidden` flags never leak across sessions.
5. **Orphan hard-deleted favorite & single- vs multi-variant purchase CTA on `FavoritesPage`:** Hard-deleted products (`name === ""`, `status === "UNAVAILABLE"`) render `"Sản phẩm không còn tồn tại"` without a broken `/san-pham/:id` link or `"0 ₫"` price while keeping `"Bỏ yêu thích"` active; purchasable 1-variant items call `addItem(productId, variants[0].label, 1)`, whereas multi-variant items render a `"Chọn phân loại"` link to `/san-pham/:id`.

---

## File Structure

- **Modify `contracts/proto/cart/v1/cart.proto`** + regenerate `contracts/go/cart/v1/*` and `contracts/java/src/main/generated/com/ecommerce/contracts/cart/v1/*`: Additive `GetFavorites`, `AddFavorite`, `RemoveFavorite`, `ClearFavorites` RPCs, `FavoriteList`, `FavoriteItem`, `FavoriteVariant`, and `FavoriteItemStatus`.
- **Create `docs/adr/010-product-favorites-in-cart-service.md`**, **Modify `docs/steering/architecture.md`, `docs/steering/service-catalog.md`, `services/cart-service/AGENTS.md`, `services/bff-service/AGENTS.md`**: Record ownership decision and updated routes/capabilities.
- **Create `services/identity-service/src/main/resources/db/migration/V9__add_favorite_manage_permission.sql`**, **Modify `IdentityDatabaseMigrationTest.java` & `RbacPersistenceTest.java`**: Seed system permission `favorite.manage` assigned to `CUSTOMER` and verify RBAC inheritance/denial.
- **Create `services/cart-service/internal/store/migrations/000002_create_favorites.up.sql` & `services/cart-service/internal/store/queries/favorites.sql`**, regenerate `services/cart-service/internal/store/db/*`, **Modify `services/cart-service/internal/domain/cart.go` (+ `cart_test.go`) & `services/cart-service/internal/store/store.go` (+ `store_test.go`)**: Persistence and status-resolution logic for favorites.
- **Create `services/cart-service/internal/grpc/favorites.go` (+ `favorites_test.go`)**, **Modify `services/cart-service/internal/grpc/server.go` (+ `server_test.go`)**: gRPC favorite handlers, `favorite.manage` authorization, Catalog verification on `AddFavorite`, and concurrent Catalog + Inventory enrichment.
- **Modify `services/bff-service/internal/cart/client.go` (+ `client_test.go`)**, **Create `services/bff-service/internal/http/favorites.go` (+ `favorites_test.go`)**, **Modify `services/bff-service/internal/http/cart_test.go`, `services/bff-service/cmd/server/main.go` (+ `main_test.go`), `deployments/kong/kong.yaml`, `deployments/kong/kong-production.yaml`, `services/bff-service/internal/config/kong_development_test.go`, `services/bff-service/internal/config/kong_production_test.go`**: BFF `/api/v1/favorites` REST endpoints, error mapping, and Kong gateway routes.
- **Modify `ecommerce-fe/src/lib/types.ts`, `ecommerce-fe/src/lib/api.ts` (+ `api.test.ts`), `ecommerce-fe/src/main.tsx` (+ `main.test.tsx`), `ecommerce-fe/src/components/layout/site-header.tsx` (+ `site-header.test.tsx`)**, **Create `ecommerce-fe/src/context/favorites-context.ts`, `ecommerce-fe/src/context/favorites-provider.tsx` (+ `favorites-provider.test.tsx`), `ecommerce-fe/src/hooks/use-favorites.ts`**: Frontend API adapter, session-aware `FavoritesProvider`, and header navigation/badge.
- **Modify `ecommerce-fe/src/components/product-card.tsx` (+ `product-card.test.tsx`), `ecommerce-fe/src/pages/product-detail-page.tsx` (+ `product-detail-page.test.tsx`), `ecommerce-fe/src/App.tsx` (+ `App.test.tsx`)**, **Create `ecommerce-fe/src/pages/favorites-page.tsx` (+ `favorites-page.test.tsx`)**: Product card & detail favorite buttons and `/yeu-thich` management page.

---

### Task 1: Protobuf Favorites Contract & Architecture Decision Record

**Files:**
- Modify: `contracts/proto/cart/v1/cart.proto`
- Modify: `contracts/go/cart/v1/cart.pb.go` (generated)
- Modify: `contracts/go/cart/v1/cart_grpc.pb.go` (generated)
- Modify: `contracts/java/src/main/generated/com/ecommerce/contracts/cart/v1/*` (generated)
- Create: `docs/adr/010-product-favorites-in-cart-service.md`
- Modify: `docs/steering/architecture.md`
- Modify: `docs/steering/service-catalog.md`
- Modify: `services/cart-service/AGENTS.md`
- Modify: `services/bff-service/AGENTS.md`

**Interfaces:**
- Consumes: `ecommerce.common.v1.Money` (`common/v1/common.proto`)
- Produces (`ecommerce.cart.v1`):
  - `CartService` RPCs:
    - `GetFavorites(GetFavoritesRequest) returns (GetFavoritesResponse)`
    - `AddFavorite(AddFavoriteRequest) returns (AddFavoriteResponse)`
    - `RemoveFavorite(RemoveFavoriteRequest) returns (RemoveFavoriteResponse)`
    - `ClearFavorites(ClearFavoritesRequest) returns (ClearFavoritesResponse)`
  - Messages: `GetFavoritesRequest`, `GetFavoritesResponse`, `AddFavoriteRequest`, `AddFavoriteResponse`, `RemoveFavoriteRequest`, `RemoveFavoriteResponse`, `ClearFavoritesRequest`, `ClearFavoritesResponse`, `FavoriteList`, `FavoriteVariant`, `FavoriteItem` (fields `1..18` matching Spec Section 2), `FavoriteItemStatus` (`UNSPECIFIED=0`, `AVAILABLE=1`, `LIMITED=2`, `OUT_OF_STOCK=3`, `UNAVAILABLE=4`, `UNKNOWN=5`).

- [ ] **Step 1: Update `contracts/proto/cart/v1/cart.proto` with additive favorite RPCs, messages, and `FavoriteItemStatus` enum**
  - Add the 4 RPCs and messages/enum with exact field numbers from Spec Section 2 without altering any existing `CartService` RPC or `Cart*` message field numbers.

- [ ] **Step 2: Format, lint, breaking-check, and regenerate Go & Java contracts**
  - Run: `buf format -w && buf format --diff --exit-code && buf lint && buf generate`
  - Expected: Exit code `0`, updating `contracts/go/cart/v1/` and `contracts/java/src/main/generated/com/ecommerce/contracts/cart/v1/`.

- [ ] **Step 3: Document ADR-010 and align steering/service docs**
  - Create `docs/adr/010-product-favorites-in-cart-service.md` (< 30 lines, lightweight ADR format) recording `cart-service` ownership of `favorite_items` and `favorite.manage` enforcement.
  - Update `docs/steering/architecture.md`, `docs/steering/service-catalog.md`, `services/cart-service/AGENTS.md`, and `services/bff-service/AGENTS.md` to mention `favorite_items` / `/api/v1/favorites`.

- [ ] **Step 4: Verify Go and Java consumers still compile cleanly with generated contracts**
  - Run: `go build ./services/cart-service/... ./services/bff-service/... && mvn -pl contracts/java -am test-compile`
  - Expected: Exit code `0` (`UnimplementedCartServiceServer` satisfies new RPC stubs until Task 3 implements them).

---

### Task 2: Identity RBAC `favorite.manage` Permission Migration

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V9__add_favorite_manage_permission.sql`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`

**Interfaces:**
- Produces: System RBAC permission `favorite.manage` (`type = 'PERMISSION'`, `system_item = true`) linked via `auth_item_children` (`parent_name = 'CUSTOMER'`, `child_name = 'favorite.manage'`).

- [ ] **Step 1: Write failing migration and RBAC persistence assertions**
  - In `IdentityDatabaseMigrationTest#createsAccountTablesAndSeedsCustomerRole`, add `"favorite.manage"` (alphabetically between `"cart.item.add"` and `"product.delete"`) to the `containsExactly(...)` permission list, assert `CUSTOMER -> favorite.manage` count is `1L`, and assert `ADMIN -> favorite.manage` count is `0L`. Add `upgradesV8SchemaWithProtectedFavoriteManagePermission` testing `V9__add_favorite_manage_permission.sql` directly.
  - In `RbacPersistenceTest`, add `grantsFavoriteManageToCustomerAndSuperAdminButNotAdmin` verifying `findAllowedPermissions(USER_ID, List.of("favorite.manage"))` returns `["favorite.manage"]` for `CUSTOMER` and `SUPER_ADMIN`, and `[]` for `ADMIN`.

- [ ] **Step 2: Run Identity tests to verify RED failure**
  - Run: `mvn -f services/identity-service/pom.xml test -Dtest=IdentityDatabaseMigrationTest,RbacPersistenceTest`
  - Expected: FAIL (`V9__add_favorite_manage_permission.sql` missing / `"favorite.manage"` absent from `auth_items`).

- [ ] **Step 3: Create `V9__add_favorite_manage_permission.sql`**
  - Follow `V6__add_cart_item_add_permission.sql`: upsert `('favorite.manage', 'PERMISSION', 'Manage product favorites for the current user', true, now(), now())` into `auth_items` and insert `('CUSTOMER', 'favorite.manage')` into `auth_item_children` with `on conflict do nothing`.

- [ ] **Step 4: Run Identity tests to verify GREEN**
  - Run: `mvn -f services/identity-service/pom.xml test -Dtest=IdentityDatabaseMigrationTest,RbacPersistenceTest`
  - Expected: PASS (`BUILD SUCCESS`).

---

### Task 3: `cart-service` Favorites Persistence, Authorization & Enrichment

**Files:**
- Modify: `services/cart-service/internal/domain/cart.go`
- Modify: `services/cart-service/internal/domain/cart_test.go`
- Create: `services/cart-service/internal/store/migrations/000002_create_favorites.up.sql`
- Create: `services/cart-service/internal/store/queries/favorites.sql`
- Modify: `services/cart-service/internal/store/db/*` (generated via `sqlc generate`)
- Modify: `services/cart-service/internal/store/store.go`
- Modify: `services/cart-service/internal/store/store_test.go`
- Modify: `services/cart-service/internal/grpc/server.go`
- Create: `services/cart-service/internal/grpc/favorites.go`
- Modify: `services/cart-service/internal/grpc/server_test.go`
- Create: `services/cart-service/internal/grpc/favorites_test.go`

**Interfaces:**
- Consumes: `cartv1`, `catalogv1`, `inventoryv1`, `authorizationClient.CheckPermission(ctx, userID, "favorite.manage")`
- Produces:
  - `domain.FavoriteRecord { ID string; ProductID string }`
  - `domain.FavoriteStatus` (`FavoriteStatusAvailable`, `FavoriteStatusLimited`, `FavoriteStatusOutOfStock`, `FavoriteStatusUnavailable`, `FavoriteStatusUnknown`) and `domain.ResolveFavoriteStatus(found, deleted bool, variantCount int, inventoryDegraded bool, available int32) FavoriteStatus` (with `lowStockThreshold = 5`)
  - `Store` methods: `ListFavorites(ctx, userID string) ([]domain.FavoriteRecord, error)`, `AddFavorite(ctx, userID, productID string) error`, `RemoveFavorite(ctx, userID, productID string) error`, `ClearFavorites(ctx, userID string) error`
  - `Server` gRPC methods: `GetFavorites`, `AddFavorite`, `RemoveFavorite`, `ClearFavorites` with `ErrorInfo` reasons `"FAVORITE_PERMISSION_DENIED"`, `"AUTHORIZATION_UNAVAILABLE"`, and `"FAVORITE_PRODUCT_UNAVAILABLE"` in domain `"ecommerce.cart.v1"`.

- [ ] **Step 1: Write failing unit & store tests for favorite domain rules, SQL persistence, and gRPC RPCs**
  - In `internal/domain/cart_test.go`: test `ResolveFavoriteStatus` across `AVAILABLE` (`available = 6`), `LIMITED` (`1..5`), `OUT_OF_STOCK` (`<= 0`), `UNAVAILABLE` (`!found`, `deleted`, or `variantCount == 0`), and `UNKNOWN` (`inventoryDegraded = true` on active product).
  - In `internal/store/store_test.go`: add `TestStoreFavoritesIntegration` verifying newest-first ordering (`created_at DESC, id DESC`), idempotent duplicate `AddFavorite` preserving original ordering, user isolation, idempotent `RemoveFavorite` on absent product, and `ClearFavorites`.
  - In `internal/grpc/favorites_test.go` (updating `fakeStore` in `server_test.go` with favorite stub methods): test:
    - UUID validation on `user_id` and non-empty / `<= 128`-char validation on `product_id` (`codes.InvalidArgument`).
    - `favorite.manage` check on all 4 RPCs: denied -> `codes.PermissionDenied` (`FAVORITE_PERMISSION_DENIED`), nil/error -> `codes.Unavailable` (`AUTHORIZATION_UNAVAILABLE`), caller `context.Canceled` -> `codes.Canceled`, and validated `x-trace-id` propagation.
    - `AddFavorite` Catalog pre-check: `NotFound` -> `codes.NotFound`, Catalog `Unavailable` -> `codes.Unavailable` without calling `store.AddFavorite`, soft-deleted or variant-less product -> `codes.FailedPrecondition` (`FAVORITE_PRODUCT_UNAVAILABLE`), `OUT_OF_STOCK` product -> succeeds and persists!
    - Enrichment on `GetFavorites` / mutations: empty list short-circuits without Catalog/Inventory calls; active + stock `> 5` -> `AVAILABLE`; stock `1..5` -> `LIMITED`; stock `0` -> `OUT_OF_STOCK`; soft-deleted or `NotFound` -> `UNAVAILABLE` (`degraded: false`); Catalog non-`NotFound` error -> `UNAVAILABLE` (`degraded: true`); Inventory error on active product -> `UNKNOWN` with Catalog fields populated (`degraded: true`).

- [ ] **Step 2: Run `cart-service` tests to verify RED failure**
  - Run: `go test ./services/cart-service/internal/...`
  - Expected: FAIL (`undefined: ResolveFavoriteStatus`, missing `Store` & `Server` favorite methods).

- [ ] **Step 3: Implement migration, `sqlc` queries, `Store` methods, domain status resolver, and gRPC favorite handlers**
  - Create `000002_create_favorites.up.sql` and `queries/favorites.sql`, then run `sqlc generate -f services/cart-service/sqlc.yaml`.
  - Implement `Store` methods in `internal/store/store.go`, `ResolveFavoriteStatus` in `internal/domain/cart.go`, and `GetFavorites`, `AddFavorite`, `RemoveFavorite`, `ClearFavorites` + concurrent enrichment (`enrichmentConcurrency = 8`) in `internal/grpc/favorites.go` (extending `cartStore` in `server.go`).

- [ ] **Step 4: Run `cart-service` build and tests to verify GREEN**
  - Run: `go build ./services/cart-service/... && go test ./services/cart-service/internal/...`
  - Expected: PASS (and if PostgreSQL container is running, run with `DATABASE_URL="postgres://ecommerce:change-me-local-only@127.0.0.1:5432/cart_db?sslmode=disable"` to exercise `TestStoreFavoritesIntegration` too).

---

### Task 4: `bff-service` `/api/v1/favorites` Adaptation & Kong Gateway Routing

**Files:**
- Modify: `services/bff-service/internal/cart/client.go`
- Modify: `services/bff-service/internal/cart/client_test.go`
- Create: `services/bff-service/internal/http/favorites.go`
- Create: `services/bff-service/internal/http/favorites_test.go`
- Modify: `services/bff-service/internal/http/cart_test.go`
- Modify: `services/bff-service/cmd/server/main.go`
- Modify: `services/bff-service/cmd/server/main_test.go`
- Modify: `deployments/kong/kong.yaml`
- Modify: `deployments/kong/kong-production.yaml`
- Modify: `services/bff-service/internal/config/kong_development_test.go`
- Modify: `services/bff-service/internal/config/kong_production_test.go`

**Interfaces:**
- Consumes: `cartv1.CartServiceClient` (`GetFavorites`, `AddFavorite`, `RemoveFavorite`, `ClearFavorites`)
- Produces:
  - `cart.Client` methods:
    - `GetFavorites(ctx context.Context, userID string) (*cartv1.GetFavoritesResponse, error)`
    - `AddFavorite(ctx context.Context, userID, productID string) (*cartv1.AddFavoriteResponse, error)`
    - `RemoveFavorite(ctx context.Context, userID, productID string) (*cartv1.RemoveFavoriteResponse, error)`
    - `ClearFavorites(ctx context.Context, userID string) (*cartv1.ClearFavoritesResponse, error)`
  - Sentinel errors in `internal/cart`: `ErrProductNotFound`, `ErrProductUnavailable` (alongside `ErrPermissionDenied`, `ErrAuthorizationUnavailable`)
  - `http.NewFavoritesHandler(client cart.Client) *FavoritesHandler` registering `GET /api/v1/favorites`, `POST /api/v1/favorites/items`, `DELETE /api/v1/favorites/items/{productId}`, `DELETE /api/v1/favorites`
  - Kong protected route `favorites-api` (`/api/v1/favorites`, `strip_path: false`, `caller-context`).

- [ ] **Step 1: Write failing tests for `internal/cart`, `internal/http/favorites_test.go`, `cmd/server/main_test.go`, and Kong config tests**
  - In `internal/cart/client_test.go`: test `GetFavorites`, `AddFavorite`, `RemoveFavorite`, `ClearFavorites`, and mapping of `FAVORITE_PERMISSION_DENIED` -> `ErrPermissionDenied`, `AUTHORIZATION_UNAVAILABLE` -> `ErrAuthorizationUnavailable`, `FAVORITE_PRODUCT_UNAVAILABLE` -> `ErrProductUnavailable`, and `codes.NotFound` on `AddFavorite` -> `ErrProductNotFound`.
  - In `internal/http/favorites_test.go`: test `401 UNAUTHORIZED` when `X-User-Id` is missing; `200 OK` JSON envelope & `camelCase` field mapping (`variants`, `imageHue`, `status`, `availableQuantity`, `degraded`) and exact messages (`"Favorites retrieved."`, `"Product added to favorites."`, `"Product removed from favorites."`, `"Favorites cleared."`); `400 INVALID_PARAMETER` on malformed/unknown-field/trailing JSON or blank/`>128`-char `productId`; `403 FORBIDDEN`; `404 PRODUCT_NOT_FOUND`; `409 PRODUCT_UNAVAILABLE`; `503 AUTHORIZATION_SERVICE_UNAVAILABLE`; `502 FAVORITES_UNAVAILABLE`; and `X-Trace-Id` correlation.
  - In `cmd/server/main_test.go`: add favorite stubs on `fakeCartClient` and assert `GET /api/v1/favorites` with `X-User-Id` returns `200`.
  - In `kong_development_test.go` and `kong_production_test.go`: add `"favorites-api"` expectation (`service: "bff-service"`, `paths: ["/api/v1/favorites"]`, `methods: ["GET", "POST", "DELETE"]` in prod, `protected: true`).

- [ ] **Step 2: Run `bff-service` tests to verify RED failure**
  - Run: `go test ./services/bff-service/...`
  - Expected: FAIL (missing `FavoritesHandler`, missing `Client` methods, missing `favorites-api` route in Kong YAML).

- [ ] **Step 3: Implement `internal/cart/client.go`, `internal/http/favorites.go`, `cmd/server/main.go`, `kong.yaml`, and `kong-production.yaml`**
  - Update `fakeCartClient` in `internal/http/cart_test.go` with no-op favorite methods (or scope `CartHandler` / `FavoritesHandler` cleanly while `cart.Client` exposes all methods).
  - Register `favoritesHandler.Register(mux)` in `buildMux` (`services/bff-service/cmd/server/main.go`).
  - Add `favorites-api` under `bff-service` in both `deployments/kong/kong.yaml` and `deployments/kong/kong-production.yaml`.

- [ ] **Step 4: Run `bff-service` tests to verify GREEN**
  - Run: `go test ./services/bff-service/...`
  - Expected: PASS.

---

### Task 5: `ecommerce-fe` Favorites API Adapter, Session-Safe Context & Header Navigation

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Modify: `ecommerce-fe/src/lib/api.ts`
- Modify: `ecommerce-fe/src/lib/api.test.ts`
- Create: `ecommerce-fe/src/context/favorites-context.ts`
- Create: `ecommerce-fe/src/context/favorites-provider.tsx`
- Create: `ecommerce-fe/src/context/favorites-provider.test.tsx`
- Create: `ecommerce-fe/src/hooks/use-favorites.ts`
- Modify: `ecommerce-fe/src/main.tsx`
- Modify: `ecommerce-fe/src/main.test.tsx`
- Modify: `ecommerce-fe/src/components/layout/site-header.tsx`
- Modify: `ecommerce-fe/src/components/layout/site-header.test.tsx`

**Interfaces:**
- Consumes: `/api/v1/favorites` REST endpoints; `AuthContext` (`ready`, `isAuthenticated`, `user`)
- Produces:
  - Types in `src/lib/types.ts`: `FavoriteItemStatus`, `FavoriteItem`, `FavoriteData`
  - API functions in `src/lib/api.ts`: `getFavorites(): Promise<FavoriteData>`, `addFavoriteItem(productId: string): Promise<FavoriteData>`, `removeFavoriteItem(productId: string): Promise<FavoriteData>`, `clearFavorites(): Promise<FavoriteData>`
  - `useFavorites(): FavoritesContextValue` (`items`, `totalCount`, `isFavorite(productId)`, `isPending(productId)`, `addFavorite(productId)`, `removeFavorite(productId)`, `toggleFavorite(productId)`, `clearFavorites()`, `loading`, `degraded`, `forbidden`, `loadError`, `retryLoad()`).

- [ ] **Step 1: Write failing tests for API functions, `FavoritesProvider` session/RBAC/concurrency handling, and `SiteHeader` favorites badge**
  - In `src/lib/api.test.ts`: test `getFavorites`, `addFavoriteItem` (triggers single `toast.success` from interceptor), `removeFavoriteItem` (uses `encodeURIComponent(productId)`), and `clearFavorites`.
  - In `src/context/favorites-provider.test.tsx`: test guest empty state without calling `getFavorites`; customer load populating `items`, `totalCount`, and `isFavorite`; `403 FORBIDDEN` on `getFavorites` setting `forbidden = true` and `loadError = null` without error toast; `toggleFavorite` adding when absent and removing when present while tracking `isPending(productId)`; discarding stale in-flight `getFavorites` / `addFavorite` responses across logout or user switch; and `retryLoad()`.
  - In `src/components/layout/site-header.test.tsx`: test `"Sản phẩm yêu thích"` icon link to `/yeu-thich` with count badge (`3`, `99+` when `> 99`), dropdown & mobile drawer links when authenticated and `!forbidden`, and hiding favorites controls when `forbidden === true`.

- [ ] **Step 2: Run focused Vitest tests to verify RED failure**
  - Run: `npm --prefix ecommerce-fe run test -- --run src/lib/api.test.ts src/context/favorites-provider.test.tsx src/components/layout/site-header.test.tsx`
  - Expected: FAIL (`addFavoriteItem` / `FavoritesProvider` undefined).

- [ ] **Step 3: Implement `types.ts`, `api.ts`, `FavoritesProvider`, `useFavorites`, `main.tsx`, and `SiteHeader`**
  - Define a safe default guest `FavoritesContextValue` in `favorites-context.ts` (`items: []`, `totalCount: 0`, `isFavorite: () => false`, `isPending: () => false`, `forbidden: false`, `loading: false`, `degraded: false`, `loadError: null`, no-op async actions) so isolated page tests (`home-page.test.tsx`, `search-page.test.tsx`) that render `ProductCard` without `FavoritesProvider` continue passing without extra wrappers.
  - Mount `<FavoritesProvider>` inside `<AuthProvider>` in `src/main.tsx` (and update `src/main.test.tsx` if needed).
  - Add the `Heart` link + count badge + user dropdown item + mobile drawer item in `SiteHeader` when `!forbidden`.

- [ ] **Step 4: Run focused Vitest tests to verify GREEN**
  - Run: `npm --prefix ecommerce-fe run test -- --run src/lib/api.test.ts src/context/favorites-provider.test.tsx src/components/layout/site-header.test.tsx src/main.test.tsx`
  - Expected: PASS.

---

### Task 6: `ecommerce-fe` Product Surfaces (`ProductCard`, `ProductDetailPage`) & `/yeu-thich` Management Page

**Files:**
- Modify: `ecommerce-fe/src/components/product-card.tsx`
- Modify: `ecommerce-fe/src/components/product-card.test.tsx`
- Modify: `ecommerce-fe/src/pages/product-detail-page.tsx`
- Modify: `ecommerce-fe/src/pages/product-detail-page.test.tsx`
- Create: `ecommerce-fe/src/pages/favorites-page.tsx`
- Create: `ecommerce-fe/src/pages/favorites-page.test.tsx`
- Modify: `ecommerce-fe/src/App.tsx`
- Modify: `ecommerce-fe/src/App.test.tsx`

**Interfaces:**
- Consumes: `useFavorites()`, `useCart()`, `useAuth()`, `formatVND`, `formatRating`, `ProductImage`
- Produces:
  - Updated `ProductCard` with sibling favorite toggle button (`aria-label` `"Thêm <name> vào yêu thích"` / `"Bỏ <name> khỏi yêu thích"`, `aria-pressed`)
  - Updated `ProductDetailPage` synced with `useFavorites()`
  - `FavoritesPage` mounted at `<Route path="/yeu-thich" element={<RequireAuth><FavoritesPage /></RequireAuth>} />` in `App.tsx`.

- [ ] **Step 1: Write failing tests for `ProductCard`, `ProductDetailPage`, `FavoritesPage`, and `/yeu-thich` routing**
  - In `src/components/product-card.test.tsx`: test guest click redirecting to `/login` with `state.from`; authenticated customer click calling `toggleFavorite(product.id)` without navigating to `/san-pham/:id` and reflecting `aria-pressed="true"`; hiding button when `forbidden === true`.
  - In `src/pages/product-detail-page.test.tsx`: update existing wishlist test and add cases for: guest click redirecting to `/login`; authenticated customer toggling via `useFavorites` (including `OUT_OF_STOCK` products remaining favoritable!); hiding when `forbidden === true`; disabling add on discontinued (`UNAVAILABLE`) product when `!favorited` while keeping unfavorite enabled when `favorited === true`.
  - In `src/pages/favorites-page.test.tsx` and `src/App.test.tsx`: test `/yeu-thich` auth guard; loading skeleton; `forbidden` non-customer notice; `loadError` + `"Thử lại"`; empty state + `"Khám phá sản phẩm"`; `degraded` banner; filter tabs (`Tất cả`, `Còn hàng`, `Hết hàng / Ngừng bán`); single remove & `"Xóa tất cả"`; 1-variant `"Thêm vào giỏ"` calling `addItem(productId, variantLabel, 1)` vs multi-variant `"Chọn phân loại"` linking to `/san-pham/:id`; and orphan hard-deleted fallback (`"Sản phẩm không còn tồn tại"`, no broken link, no `"0 ₫"`).

- [ ] **Step 2: Run Vitest tests to verify RED failure**
  - Run: `npm --prefix ecommerce-fe run test -- --run src/components/product-card.test.tsx src/pages/product-detail-page.test.tsx src/pages/favorites-page.test.tsx src/App.test.tsx`
  - Expected: FAIL (`favorites-page.tsx` missing, `ProductCard` missing favorite button, `ProductDetailPage` still using local `useState`).

- [ ] **Step 3: Implement `ProductCard`, `ProductDetailPage`, `FavoritesPage`, and `App.tsx` route**
  - In `ProductCard`: place the favorite icon `Button` in the bottom action container alongside the cart link button (outside `<Link>`) so HTML semantics remain valid and badges never overlap.
  - In `ProductDetailPage`: wire `useFavorites()` (`isFavorite`, `isPending`, `toggleFavorite`, `forbidden`), remove local `wishlisted` state and duplicate `toast.success(...)`.
  - Build `FavoritesPage` (`src/pages/favorites-page.tsx`) with responsive S-Shop card layout, filter pills, status badges, orphan item fallback, single/clear removal, and 1-variant vs multi-variant cart actions, and mount `/yeu-thich` in `App.tsx`.

- [ ] **Step 4: Run full frontend and repository verification gates**
  - Run:
    - `npm --prefix ecommerce-fe run test -- --run`
    - `npm --prefix ecommerce-fe run lint`
    - `npm --prefix ecommerce-fe run build`
    - `make check-boundaries`
  - Expected: All commands exit `0`.

# Product Favorites End-to-End Design

## Intent and Why

Shoppers browsing S-Shop currently have no durable way to save products they are interested in for later comparison or restock tracking:

1. `ProductDetailPage` renders a heart button (`Thêm vào yêu thích`), but its state lives in component-local `useState(false)` and is lost immediately on navigation or page reload.
2. `ProductCard` (used on `HomePage`, `SearchPage`, and related-product grids) has no favorite toggle, forcing shoppers to open every detail page even if they just want to bookmark items while browsing.
3. `SiteHeader` and `App` provide no route or management screen where a customer can review saved products alongside live pricing, discounts, stock status, or discontinuation notices.
4. No backend contract, database table, or RBAC permission exists to persist favorites per customer account across sessions and devices.

This design introduces an account-synchronized **Product Favorites** (`favorites`) capability across `identity-service`, `contracts`, `cart-service`, `bff-service`, Kong, and `ecommerce-fe`, restricted exclusively to accounts holding customer favorite permissions via Identity RBAC.

## Goals, Non-Goals, and Anti-Goals

### Goals

- Persist product-level favorites (`(user_id, product_id)`) per authenticated account on the backend so favorites stay synchronized across reloads, tabs, and devices.
- Enforce customer-only access (`favorite.manage` permission assigned to the `CUSTOMER` role in Identity RBAC) via `Identity` private authorization gRPC (`127.0.0.1:50054` locally, `identity-service:50054` in containers), failing closed (`503`) if authorization is unreachable and returning `403 FORBIDDEN` for authenticated accounts without customer permission (such as `ADMIN`-only accounts).
- Enrich favorite listings dynamically over private gRPC from `CatalogService` (current name, brand, category, price, original price, images, rating, sold count, badge, variants, soft-delete state) and `InventoryService` (live stock status and available quantity), degrading gracefully when downstream inventory is temporarily unavailable.
- Expose canonical authenticated REST endpoints under `/api/v1/favorites` through Kong (`strip_path: false`, `caller-context` JWT verification) and `bff-service`.
- Update `ProductDetailPage` and `ProductCard` with server-synchronized favorite toggle buttons, add a header entry with item count badge in `SiteHeader`, and build a dedicated management page at `/yeu-thich` (`FavoritesPage`) supporting status filtering, single-item removal, bulk clear, and quick add-to-cart / variant selection navigation.
- Record the capability ownership extension in a lightweight ADR (`docs/adr/010-product-favorites-in-cart-service.md`) and keep steering/service catalog docs aligned.

### Non-Goals

- Guest/anonymous `localStorage` favorites or guest-to-account merge flows: unauthenticated users attempting to favorite a product or visit `/yeu-thich` are redirected to `/login` with return-location state, matching the `/gio-hang` flow.
- Variant-level favoriting: favorites are tracked by `product_id` so shoppers can bookmark products directly from listing cards without selecting a variant first.
- Price-drop or back-in-stock push/email notifications, multiple named favorite folders, or public shareable favorite lists.

### Anti-Goals

- Duplicating catalog pricing, product metadata, or inventory stock truth inside `cart_db` (`favorite_items` stores only `id`, `user_id`, `product_id`, and `created_at`).
- Enforcing role checks solely in React components by inspecting role names (`user.roles.includes("CUSTOMER")`); Identity RBAC via private gRPC remains the authoritative source of truth so custom RBAC hierarchy assignments continue to work.
- Emitting duplicate toast notifications on favorite mutations (`ecommerce-fe/src/lib/api.ts` already toasts mutation `message` envelopes automatically).

## Constraints

- **Service boundaries (`AGENTS.md`, `docs/steering/service-boundaries.md`):** No cross-service code imports, foreign database queries, or internal calls through Kong (`:8000`). `bff-service` calls `cart-service` (`:50053`), and `cart-service` calls `catalog-service` (`:50051`), `inventory-service` (`:50052`), and `identity-service` (`:50054`) over direct private gRPC.
- **Contract compatibility (`docs/steering/api-contracts.md`):** Protobuf additions in `contracts/proto/cart/v1/cart.proto` must be strictly additive (`lower_snake_case`), pass `buf format --diff --exit-code`, `buf lint`, and `buf breaking` against `main`, and synchronize generated Go and Java outputs via `buf generate`.
- **REST envelope (`AGENTS.md`, `docs/steering/api-contracts.md`):** All `/api/v1/favorites` responses use `camelCase` JSON fields, `{ "data": ..., "message": "..." }` for `2xx`, and `{ "code": "...", "message": "...", "traceId": "..." }` for errors.
- **Windows verification compatibility (`services/cart-service/AGENTS.md`):** Keep `services/cart-service/cmd/server/main.go` and `main_test.go` compatible without requiring changes to the Unix `syscall.Kill` signal test suite.
- **Terminology:** Use `favorite` / `favorites` consistently across permissions, protobuf messages/RPCs, database tables, BFF handlers, Kong routes, and frontend modules (rather than `wishlist`).

## Approaches Considered

### Approach 1 (Selected): Extend `cart-service` (`cart_db`) with Product Favorites

Store `favorite_items (id, user_id, product_id, created_at)` in `cart_db`, add additive favorite RPCs (`GetFavorites`, `AddFavorite`, `RemoveFavorite`, `ClearFavorites`) to `CartService` in `contracts/proto/cart/v1/cart.proto`, enforce `favorite.manage` via `Identity` authorization gRPC (`:50054`), enrich items from `CatalogService` (`:50051`) and `InventoryService` (`:50052`), and expose `/api/v1/favorites` via `bff-service` and Kong.

- **Why chosen:** `cart-service` already owns authenticated customer shopping lists and already maintains configured, battle-tested private gRPC clients to Catalog, Inventory, and Identity Authorization, plus an existing BFF gRPC connection. Extending `cart-service` keeps `catalog-service` free of per-user state and avoids introducing a seventh microservice, database, container, and port pair for a lightweight capability (`docs/steering/architecture.md`).
- **Trade-off:** Broadens `cart-service`'s scope from cart lines alone to customer cart + saved favorite products, documented explicitly in `docs/adr/010-product-favorites-in-cart-service.md`.

### Approach 2 (Not Selected): Dedicated `favorite-service` Microservice

Create a new Go service (`services/favorite-service`) with its own `favorite_db`, gRPC server, Dockerfile, Compose service, Makefile target, `start-local` entry, and BFF upstream client.

- **Why rejected:** Duplicates the exact outbound gRPC topology and enrichment logic of `cart-service` while increasing local and container operational overhead without distinct scaling, transaction, or workflow requirements.

### Approach 3 (Not Selected): Persist Favorites Inside `catalog-service` (`catalog_db`)

Store `(user_id, product_id)` inside `catalog_db`, join `products` locally on read, and let `bff-service` enrich stock availability from `inventory-service`.

- **Why rejected:** Mixes per-user mutable state into `catalog-service`, which is scoped strictly to shared product/catalog truth and catalog caching (`services/catalog-service/AGENTS.md`).

## Architecture and Component Design

### 1. Identity RBAC Permission (`services/identity-service`)

- **Flyway Migration (`src/main/resources/db/migration/V9__add_favorite_manage_permission.sql`):**
  - Upsert system permission `favorite.manage` (`type = 'PERMISSION'`, `description = 'Manage product favorites for the current user'`, `system_item = true`) into `auth_items`.
  - Insert `('CUSTOMER', 'favorite.manage')` into `auth_item_children` (`on conflict do nothing`).
  - `ADMIN` and custom non-customer roles do not inherit `CUSTOMER`, so they are denied unless explicitly granted `favorite.manage` through RBAC administration; `SUPER_ADMIN` retains its existing global RBAC wildcard invariant in `RbacRepository` (matching `cart.item.add`).
- **Migration Verification (`IdentityDatabaseMigrationTest.java` & `RbacPersistenceTest.java`):**
  - Update the seeded permission list assertion and verify `CUSTOMER -> favorite.manage` exists while `ADMIN -> favorite.manage` does not, and verify `RbacRepository` grants `favorite.manage` to `CUSTOMER` (and `SUPER_ADMIN` via wildcard) while denying `ADMIN`-only users.

### 2. Protobuf Contract (`contracts/proto/cart/v1/cart.proto`)

Add four unary RPCs to `ecommerce.cart.v1.CartService` (keeping existing RPCs and field numbers 100% untouched):

```protobuf
rpc GetFavorites(GetFavoritesRequest) returns (GetFavoritesResponse);
rpc AddFavorite(AddFavoriteRequest) returns (AddFavoriteResponse);
rpc RemoveFavorite(RemoveFavoriteRequest) returns (RemoveFavoriteResponse);
rpc ClearFavorites(ClearFavoritesRequest) returns (ClearFavoritesResponse);
```

Additive messages and enum:

- `GetFavoritesRequest { string user_id = 1; }` / `GetFavoritesResponse { FavoriteList favorites = 1; }`
- `AddFavoriteRequest { string user_id = 1; string product_id = 2; }` / `AddFavoriteResponse { FavoriteList favorites = 1; }`
- `RemoveFavoriteRequest { string user_id = 1; string product_id = 2; }` / `RemoveFavoriteResponse { FavoriteList favorites = 1; }`
- `ClearFavoritesRequest { string user_id = 1; }` / `ClearFavoritesResponse { FavoriteList favorites = 1; }`
- `FavoriteList { repeated FavoriteItem items = 1; bool degraded = 2; }`
- `FavoriteVariant { string label = 1; string color_hex = 2; }`
- `FavoriteItem`:
  - `string item_id = 1;`
  - `string product_id = 2;`
  - `string name = 3;`
  - `string brand = 4;`
  - `string category_id = 5;`
  - `string category_name = 6;`
  - `ecommerce.common.v1.Money price = 7;`
  - `ecommerce.common.v1.Money original_price = 8;`
  - `double rating = 9;`
  - `int64 review_count = 10;`
  - `int64 sold_count = 11;`
  - `string image_url = 12;`
  - `string image_emoji = 13;`
  - `repeated uint32 image_hue = 14;`
  - `string badge = 15;`
  - `repeated FavoriteVariant variants = 16;`
  - `FavoriteItemStatus status = 17;`
  - `int32 available_quantity = 18;`
- `enum FavoriteItemStatus`:
  - `FAVORITE_ITEM_STATUS_UNSPECIFIED = 0;`
  - `FAVORITE_ITEM_STATUS_AVAILABLE = 1;`
  - `FAVORITE_ITEM_STATUS_LIMITED = 2;`
  - `FAVORITE_ITEM_STATUS_OUT_OF_STOCK = 3;`
  - `FAVORITE_ITEM_STATUS_UNAVAILABLE = 4;`
  - `FAVORITE_ITEM_STATUS_UNKNOWN = 5;`

### 3. `cart-service` Persistence, Authorization, and Enrichment

- **Migration (`internal/store/migrations/000002_create_favorites.up.sql`):**
  ```sql
  CREATE TABLE favorite_items (
      id uuid primary key default gen_random_uuid(),
      user_id uuid not null,
      product_id text not null,
      created_at timestamptz not null default now(),
      unique (user_id, product_id)
  );

  CREATE INDEX ix_favorite_items_user_created
      ON favorite_items (user_id, created_at desc, id desc);
  ```
- **`sqlc` Queries (`internal/store/queries/favorites.sql`) & `Store` Adapter:**
  - `ListFavorites(user_id)`: returns `id`, `product_id` ordered by `created_at DESC, id DESC` (most recently favorited first).
  - `AddFavorite(user_id, product_id)`: `INSERT INTO favorite_items (user_id, product_id) VALUES (..., ...) ON CONFLICT (user_id, product_id) DO NOTHING` (idempotent under duplicate or concurrent clicks; preserves original `created_at` when already present).
  - `RemoveFavorite(user_id, product_id)`: `DELETE FROM favorite_items WHERE user_id = ... AND product_id = ...` (idempotent: removing an already-absent `product_id` succeeds without error and returns the current list).
  - `ClearFavorites(user_id)`: `DELETE FROM favorite_items WHERE user_id = ...`.
- **Authorization & Input Validation (`internal/grpc/`):**
  - Validate `user_id` as a canonical UUID (`InvalidArgument` on failure).
  - For `AddFavorite` and `RemoveFavorite`, trim `product_id`, require non-empty and `len(product_id) <= 128` (`InvalidArgument` on failure).
  - Every favorite RPC (`GetFavorites`, `AddFavorite`, `RemoveFavorite`, `ClearFavorites`) checks `s.authorization.CheckPermission(authorizationCtx, userID, "favorite.manage")` propagating validated `x-trace-id`:
    - Nil client or downstream error (when caller context is not canceled): return `codes.Unavailable` with `ErrorInfo{Reason: "AUTHORIZATION_UNAVAILABLE", Domain: "ecommerce.cart.v1"}`.
    - `!allowed`: return `codes.PermissionDenied` with `ErrorInfo{Reason: "FAVORITE_PERMISSION_DENIED", Domain: "ecommerce.cart.v1"}`.
- **Catalog Verification on `AddFavorite`:**
  - Before inserting into `favorite_items`, call `s.catalog.GetProduct(ctx, productID)`:
    - `codes.NotFound` or `product == nil`: return `codes.NotFound` (`"product not found"`).
    - Other error: return `codes.Unavailable` (`"catalog service is unavailable"`).
    - `product.GetDeleted() || len(product.GetVariants()) == 0`: return `codes.FailedPrecondition` with `ErrorInfo{Reason: "FAVORITE_PRODUCT_UNAVAILABLE", Domain: "ecommerce.cart.v1"}` (discontinued products cannot be newly favorited, whereas `OUT_OF_STOCK` products *can* be favorited so shoppers can track restocks).
- **Read Enrichment (`GetFavorites` and post-mutation responses):**
  - Fetch saved `favorite_items` for `user_id` and enrich unique `product_id`s concurrently (`enrichmentConcurrency = 8`) via `CatalogService.GetProduct` and `InventoryService.GetAvailability`:
    - Low-stock threshold matches storefront convention: `lowStockThreshold = 5` (`1..5` -> `LIMITED`, `> 5` -> `AVAILABLE`, `<= 0` -> `OUT_OF_STOCK`).
    - Soft-deleted (`product.GetDeleted()`), variantless (`len(variants) == 0`), or hard-deleted (`codes.NotFound` from Catalog): `status = FAVORITE_ITEM_STATUS_UNAVAILABLE`, `available_quantity = 0`, `degraded = false` (orphan hard-deleted items retain `item_id` and `product_id` with empty `name` so the customer can see and remove them, matching `CartPage` behavior).
    - Catalog non-`NotFound` error: `status = FAVORITE_ITEM_STATUS_UNAVAILABLE`, `degraded = true`.
    - Catalog succeeded (active product with variants) + Inventory error: populate all catalog metadata (`name`, `price`, `images`, `variants`, etc.) and set `status = FAVORITE_ITEM_STATUS_UNKNOWN`, `available_quantity = 0`, `degraded = true`.

### 4. Kong Gateway & `bff-service` REST Adaptation (`/api/v1/favorites`)

- **Kong Configuration (`deployments/kong/kong.yaml` & `deployments/kong/kong-production.yaml`):**
  - Add protected route `favorites-api` under `bff-service`:
    - `paths: ["/api/v1/favorites"]`, `strip_path: false`
    - `methods: ["GET", "POST", "DELETE"]` in `kong-production.yaml` (omitted in `kong.yaml` so browser `OPTIONS` CORS preflights match, matching `cart-api`)
    - `caller-context` plugin configured identically to `cart-api`.
  - Update `kong_development_test.go`, `kong_production_test.go`, and `docs/steering/service-catalog.md`.
- **BFF gRPC Client (`services/bff-service/internal/cart/client.go`):**
  - Extend `Client` with `GetFavorites`, `AddFavorite`, `RemoveFavorite`, and `ClearFavorites`, mapping:
    - `FAVORITE_PERMISSION_DENIED` -> `ErrPermissionDenied`
    - `AUTHORIZATION_UNAVAILABLE` -> `ErrAuthorizationUnavailable`
    - `FAVORITE_PRODUCT_UNAVAILABLE` -> `ErrProductUnavailable`
    - `codes.NotFound` on `AddFavorite` -> `ErrProductNotFound`
- **BFF HTTP Handler (`services/bff-service/internal/http/favorites.go`, wired in `cmd/server/main.go`):**
  - All endpoints wrapped in `correlateTrace(RequireCaller(...))`:
    - `GET /api/v1/favorites` -> `200 OK`, message `"Favorites retrieved."`
    - `POST /api/v1/favorites/items` (body `{ "productId": "<id>" }`) -> `200 OK`, message `"Product added to favorites."`
    - `DELETE /api/v1/favorites/items/{productId}` -> `200 OK`, message `"Product removed from favorites."`
    - `DELETE /api/v1/favorites` -> `200 OK`, message `"Favorites cleared."`
  - JSON payload shape (`favoriteData`):
    ```json
    {
      "items": [
        {
          "itemId": "uuid",
          "productId": "dt-1",
          "name": "...",
          "brand": "...",
          "categoryId": "...",
          "categoryName": "...",
          "price": 26990000,
          "originalPrice": 29990000,
          "rating": 4.9,
          "reviewCount": 128,
          "soldCount": 540,
          "imageUrl": "...",
          "imageEmoji": "📱",
          "imageHue": [210, 250],
          "badge": "hot",
          "variants": [{ "label": "Đen Titan 256GB", "colorHex": "#1c1c1e" }],
          "status": "AVAILABLE",
          "availableQuantity": 12
        }
      ],
      "degraded": false
    }
    ```
  - Error mapping:
    - Missing `X-User-Id` -> `401 UNAUTHORIZED`
    - Malformed JSON / empty `productId` / `codes.InvalidArgument` -> `400 INVALID_PARAMETER`
    - `ErrPermissionDenied` -> `403 FORBIDDEN` (`"You do not have permission to manage favorites."`)
    - `ErrProductNotFound` -> `404 PRODUCT_NOT_FOUND` (`"Product not found."`)
    - `ErrProductUnavailable` -> `409 PRODUCT_UNAVAILABLE` (`"Product is no longer available."`)
    - `ErrAuthorizationUnavailable` -> `503 AUTHORIZATION_SERVICE_UNAVAILABLE` (`"Authorization service is unavailable."`)
    - Other gRPC errors -> `502 FAVORITES_UNAVAILABLE` (`"Favorites service is unavailable."`)

### 5. Frontend Experience (`ecommerce-fe`)

- **API Adapter & Types (`src/lib/types.ts`, `src/lib/api.ts`):**
  - Add `FavoriteItemStatus` (`"AVAILABLE" | "LIMITED" | "OUT_OF_STOCK" | "UNAVAILABLE" | "UNKNOWN"`), `FavoriteItem`, and `FavoriteData`.
  - Add `getFavorites()`, `addFavoriteItem(productId)`, `removeFavoriteItem(productId)`, and `clearFavorites()`.
- **Session-Safe Context (`src/context/favorites-context.ts`, `src/context/favorites-provider.tsx`, `src/hooks/use-favorites.ts`, mounted inside `AuthProvider` in `src/main.tsx`):**
  - Uses the same session-version and operation-generation guards as `CartProvider` so logout or account switching immediately clears state and ignores stale in-flight responses.
  - On authenticated session ready, calls `getFavorites()`:
    - On `200 OK`: populates `items`, `degraded`, `forbidden = false`, `loadError = null`.
    - On `ApiClientError` with `code === "FORBIDDEN"` (or HTTP `403`): sets `forbidden = true`, `items = []`, `loadError = null` without triggering any error toast (since `GET` errors are silent in `api.ts`).
    - On other errors: sets `loadError` so `/yeu-thich` can display a retry state.
  - Exposes `items`, `totalCount`, `isFavorite(productId)`, `isPending(productId)`, `addFavorite(productId)`, `removeFavorite(productId)`, `toggleFavorite(productId)`, `clearFavorites()`, `loading`, `degraded`, `forbidden`, `loadError`, and `retryLoad()`.
  - If a mutation returns `code === "FORBIDDEN"`, sets `forbidden = true` so UI controls adapt immediately.
- **Product Card Update (`src/components/product-card.tsx`):**
  - When `!forbidden`, renders an accessible icon button (`aria-label` `"Thêm <name> vào yêu thích"` / `"Bỏ <name> khỏi yêu thích"`, `aria-pressed={favorited}`) alongside the cart action in the card footer (as a sibling of `<Link>`, never nested inside `<a>`).
  - Unauthenticated click navigates to `/login` with `{ state: { from: location.pathname + location.search } }`.
  - Authenticated click invokes `toggleFavorite(product.id)` (disabled while `isPending(product.id)`). Relies on `api.ts`'s mutation interceptor for toast feedback (no duplicate component toast).
- **Product Detail Page Update (`src/pages/product-detail-page.tsx`):**
  - Replaces local `wishlisted` `useState` with `useFavorites()`.
  - Hides the favorite button when `forbidden === true` (non-customer accounts such as `ADMIN`).
  - Redirects unauthenticated clicks to `/login` with `{ state: { from: "/san-pham/" + product.id } }`.
  - Disables adding when `!favorited && availability.status === "UNAVAILABLE"` (`product.deleted` or no variants), while keeping the button enabled when `favorited === true` so customers can always remove discontinued products from favorites.
  - Keeps `OUT_OF_STOCK` products favoritable so customers can bookmark out-of-stock items.
- **Header & Navigation (`src/components/layout/site-header.tsx`, `src/App.tsx`):**
  - Register `<Route path="/yeu-thich" element={<RequireAuth><FavoritesPage /></RequireAuth>} />` inside `StoreLayout` in `App.tsx`.
  - In `SiteHeader`: when `!forbidden`, show a `Heart` icon button linking to `/yeu-thich` (`aria-label="Sản phẩm yêu thích"`) next to the cart icon with a red count badge (`totalCount > 99 ? "99+" : totalCount`), plus a `"Sản phẩm yêu thích"` link in the authenticated user dropdown and mobile drawer.
- **Favorites Management Page (`src/pages/favorites-page.tsx`):**
  - **States:**
    - Initial loading: responsive skeleton grid/list (`role="status"`).
    - Non-customer account (`forbidden === true`): clear notice (`"Tính năng sản phẩm yêu thích chỉ dành cho tài khoản khách hàng."`) with `"Về trang chủ"` action.
    - Load error (`loadError !== null`): alert card with error message and `"Thử lại"` button calling `retryLoad()`.
    - Empty (`items.length === 0`): empty state with heart illustration, `"Chưa có sản phẩm yêu thích"`, and `"Khám phá sản phẩm"` CTA linking to `/tim-kiem`.
    - Degraded (`degraded === true`): amber status banner explaining that live stock/product data could not be fully verified right now.
  - **Management Controls (when `items.length > 0`):**
    - Filter pills: `"Tất cả (N)"`, `"Còn hàng"`, `"Hết hàng / Ngừng bán"`.
    - `"Xóa tất cả"` button (disabled while clearing) to empty the favorites list in one action.
    - Per-item card/row displaying thumbnail (with `ProductImage` fallback and orphan hard-deleted item fallback `"Sản phẩm không còn tồn tại"`), brand, product title link, formatted rating, current & strikethrough original price + discount badge, stock status badge (`Còn hàng`, `Chỉ còn X sản phẩm`, `Tạm hết hàng`, `Ngừng kinh doanh`, or degraded `Tạm chưa rõ tồn kho`), `"Bỏ yêu thích"` button, and purchase action:
      - Purchasable (`AVAILABLE` / `LIMITED` / `UNKNOWN`) with `variants.length === 1`: `"Thêm vào giỏ"` calls `useCart().addItem(item.productId, item.variants[0].label, 1)` directly.
      - Purchasable with `variants.length > 1`: `"Chọn phân loại"` links to `/san-pham/${item.productId}`.
      - Non-purchasable (`OUT_OF_STOCK` / `UNAVAILABLE`): disabled button showing `"Tạm hết hàng"` or `"Ngừng kinh doanh"`.

## What Is Left to the Builder

- Exact responsive Tailwind spacing, typography hierarchy, and card vs. list visual polish on `FavoritesPage`, matching existing S-Shop surfaces (`CartPage`, `SearchPage`, `ProductCard`).
- Internal package/file split inside `services/cart-service/internal/` (for example, keeping favorite RPC handlers in `internal/grpc/favorites.go` alongside `server.go` and domain helpers in `internal/domain/`).
- Focused unit and integration test structure across `identity-service`, `cart-service`, `bff-service`, and `ecommerce-fe` prior to running `make verify`.

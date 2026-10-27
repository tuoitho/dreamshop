# Admin Dashboard (`admin-fe`) and Commerce Admin APIs End-to-End Design

## Intent and Why

The repository currently exposes a customer storefront (`ecommerce-fe`) and partial administrative backend capabilities, leaving operators without a dedicated administrative console or complete commerce management APIs:

1. **Identity administrative APIs exist without an operator UI:** `identity-service` already implements `/api/v1/admin/users` (paginated user search, detail lookup, `ACTIVE` / `DISABLED` status transitions, role assignment/revocation) and `/api/v1/admin/rbac/items` (role/permission CRUD and hierarchy management), but neither has a frontend interface.
2. **Catalog management is soft-delete-only:** `catalog-service` and `bff-service` implement `SoftDeleteProduct` (`POST /api/v1/admin/products/{id}/delete`), while `CatalogService.SearchProducts` unconditionally filters `deleted_at IS NULL`. Operators cannot create products, edit product metadata/pricing/variants/images, or list soft-deleted products to restore them.
3. **Inventory management is read-only per single product:** `inventory-service` implements only `GetAvailability(product_id)` without private authorization integration, batch/list stock inspection (`quantity`, `reserved`, `available`), or stock mutation RPCs (`SetStock`, `AdjustStock`).
4. **Storefront separation:** Mixing administrative tables, RBAC editors, and inventory controls into `ecommerce-fe` would bloat customer bundles and blur security/UX concerns.

This design introduces a standalone administrative web application (`admin-fe/`) designed first via OpenDesign (using the local Antigravity runtime) and built with React 19, Vite 8, TypeScript 6, Tailwind CSS v4, `shadcn/ui`, TanStack Table, React Hook Form, Zod, and Recharts, alongside additive full-stack contracts and backend handlers across `identity-service`, `contracts`, `catalog-service`, `inventory-service`, `bff-service`, and Kong.

## Goals, Non-Goals, and Anti-Goals

### Goals

- Deliver an independent frontend surface at `admin-fe/` (local dev port `5174`, containerized Nginx on port `8080` routed under `/admin` in production Compose) covering 5 operational modules: **Overview KPIs**, **Products CRUD + Soft-Delete/Restore**, **Inventory Stock Management**, **Users Administration**, and **RBAC Roles/Permissions Hierarchy**.
- Design the visual system and screen prototypes in OpenDesign using the local Antigravity runtime prior to implementing `admin-fe/` components, pulling the generated artifact bundle via OpenDesign to anchor layout, palette, typography, and component craft.
- Seed granular commerce administrative permissions (`product.view`, `product.create`, `product.update`, `inventory.view`, `inventory.update`) in `identity-service` Flyway migrations alongside existing `product.delete`, `user.*`, and `rbac.*` permissions, and expose `GET /api/v1/users/me/permissions` so `admin-fe/` adapts navigation and controls to effective RBAC permissions (including custom operator roles).
- Extend `CatalogService` and `InventoryService` with additive protobuf RPCs and enforce permissions via `Identity` private authorization gRPC (`127.0.0.1:50054` locally, `identity-service:50054` in containers, `500ms` timeout, failing closed with `503` on outage and `403` on denial).
- Extend `bff-service` with thin composition handlers under `/api/v1/admin/products` and `/api/v1/admin/inventory` so paginated admin tables receive combined product and stock projections without browser N+1 requests or cross-database queries.
- Integrate `admin-fe/` into repository verification and boundary gates (`scripts/check-boundaries`, `scripts/verify-service`, `scripts/start-local`, `Makefile`, Kong dev/production configs, and Docker Compose).

### Non-Goals

- Order management workflows: `order-service` is currently a skeleton without persisted order domain state and remains out of scope until order lifecycle capabilities are implemented.
- Binary image/file upload storage (S3/MinIO): product images continue to use validated URLs (consistent with `catalog_db.product_images` and `ecommerce-fe`), with live URL thumbnail previews in the product editor.
- Inventory reservations or RabbitMQ asynchronous workflows: stock mutations operate atomically on `inventory_db.stock` while respecting existing `reserved` quantities (`quantity >= reserved`).
- Public self-registration inside `admin-fe/`: administrative operators authenticate with existing accounts provisioned via bootstrap `SUPER_ADMIN` or assigned roles by an administrator.

### Anti-Goals

- Importing backend code, protobuf source, or `ecommerce-fe/` source across project directories (`admin-fe/` owns its UI primitives, session adapter, and API types independently).
- Hardcoding `user.roles.includes("ADMIN")` as the sole authorization gate in React or BFF; Identity RBAC permissions remain authoritative on every backend mutation/read and drive UI capability visibility via effective permissions.
- Mutating both `catalog_db` and `inventory_db` inside one synchronous BFF HTTP mutation without distributed transaction guarantees.

## Constraints

- **Service boundaries & ADR-011 routing (`AGENTS.md`, `docs/steering/service-boundaries.md`, `docs/adr/011-service-side-grpc-gateway-transcoding.md`):** Zero cross-service source imports, zero foreign database references, and zero internal calls through Kong (`:8000`) or another service's HTTP port. Per `ADR-011`, Identity REST (`/api/v1/auth`, `/api/v1/users`, `/api/v1/admin/users`, `/api/v1/admin/rbac`) routes directly from Kong to `identity-service`, while all first-party frontend commerce workflows (`/api/v1/admin/products` and `/api/v1/admin/inventory`) route uniformly through `bff-service` without mixing direct domain HTTP/`grpc-gateway` calls or attaching `google.api.http` to internal Catalog/Inventory RPCs. `bff-service` calls `catalog-service` (`:50051`) and `inventory-service` (`:50052`) over private gRPC; `catalog-service` and `inventory-service` call `identity-service` (`:50054`) over private gRPC.
- **Contract compatibility (`docs/steering/api-contracts.md`):** All protobuf changes in `contracts/proto/catalog/v1/catalog.proto` and `contracts/proto/inventory/v1/inventory.proto` are strictly additive (`lower_snake_case`), preserve existing field numbers and RPCs, pass `buf format --diff --exit-code`, `buf lint`, and `buf breaking` against `main`, and synchronize `contracts/go` and `contracts/java` via `buf generate`.
- **REST contract (`AGENTS.md`, `docs/steering/api-contracts.md`):** All new endpoints return `camelCase` JSON wrapped in `{ "data": ..., "message": "..." }` (with pagination `{ "items", "page", "pageSize", "totalItems", "totalPages" }` inside `data`) and `{ "code", "message", "errors"?, "traceId" }` on failure.
- **Session security (`ecommerce-fe/AGENTS.md`):** Access tokens live exclusively in memory; refresh tokens use the server-managed `HttpOnly` cookie at `/api/v1/auth` with single-flight refresh coalescing and single retry on `401`.
- **Rollout order:** `identity-service` (permissions + authorization gRPC) -> `catalog-service` & `inventory-service` -> `bff-service` -> Kong -> `admin-fe`.

## Approaches Considered

### Approach 1 (Selected): Thin BFF Admin Composition + Separate Catalog & Inventory Mutations

Extend `CatalogService` and `InventoryService` with additive admin RPCs protected by Identity private gRPC authorization, let `bff-service` compose enriched product + stock views for `/api/v1/admin/products` and `/api/v1/admin/inventory`, keep Catalog CRUD and Inventory stock updates as distinct endpoints (with an optional 2-step UI flow on product creation that offers explicit retry if initial stock setting fails), and build `admin-fe/` from an OpenDesign (local Antigravity runtime) visual prototype.

- **Why chosen:** Preserves clean ownership between `catalog-service` and `inventory-service`, eliminates browser N+1 requests and pagination mismatches on the Inventory screen (including newly created products that do not yet have a row in `inventory_db.stock`), and avoids partial-write inconsistency across two databases.
- **Trade-off:** Adds composition handlers and gRPC client methods in `bff-service`, plus authorization gRPC wiring in `inventory-service`.

### Approach 2 (Not Selected): BFF Pass-Through with Client-Side Joining in `admin-fe`

Expose unenriched Catalog and Inventory endpoints separately and merge product metadata with stock rows in the browser.

- **Why rejected:** Inventory rows store only `product_id`, `quantity`, and `reserved`; searching inventory by product name/category or surfacing zero-stock products lacking a `stock` row would force the browser to download the entire catalog client-side.

### Approach 3 (Not Selected): Combined Synchronous Product + Stock Write in One BFF Endpoint

Accept `initialQuantity` inside `POST /api/v1/admin/products` and `PUT /api/v1/admin/products/{id}` so BFF mutates `catalog-service` and `inventory-service` sequentially in one request.

- **Why rejected:** Without a saga or transactional outbox between Catalog and Inventory, a transient network/authorization failure on the second call leaves partial state that is ambiguous to retry idempotently.

## Architecture and Component Design

### 1. Identity Service (`services/identity-service`)

- **Flyway Migration (`src/main/resources/db/migration/V10__add_commerce_admin_permissions.sql`):**
  - Upsert system permissions (`type = 'PERMISSION'`, `system_item = true`) into `auth_items`:
    - `product.view`: `"View all catalog products including soft-deleted items"`
    - `product.create`: `"Create catalog products"`
    - `product.update`: `"Update catalog products"`
    - `inventory.view`: `"View inventory stock levels"`
    - `inventory.update`: `"Adjust and set inventory stock quantities"`
  - Attach all five permissions as children of `ADMIN` in `auth_item_children` (`on conflict do nothing`). `SUPER_ADMIN` automatically resolves all `PERMISSION` rows in `auth_items` through the existing `RbacRepository` wildcard rule.
- **Constants (`RbacPermissions.java`):** Add constants for `PRODUCT_VIEW`, `PRODUCT_CREATE`, `PRODUCT_UPDATE`, `PRODUCT_DELETE`, `INVENTORY_VIEW`, and `INVENTORY_UPDATE`.
- **Effective Permissions Endpoint (`MeController.java`, `MeService.java`, `RbacRepository.java`):**
  - Add `RbacRepository.findAllEffectivePermissions(UUID userId)` returning all `PERMISSION` names reachable by an `ACTIVE` user's assigned roles (or all `PERMISSION` items when `SUPER_ADMIN` is reachable).
  - Expose `GET /api/v1/users/me/permissions` returning `ApiResponse<Set<String>>` (`200 OK`, message `"Permissions retrieved."`). Covered by Kong's existing protected `/api/v1/users` route.
- **Additive User Status Filter & Summary Counts (`AdminUserController.java`, `AdminUserService.java`, `AdminUserPageResponse.java`, `IdentityUserRepository.java`) *(Builder call)*:**
  - Extend `GET /api/v1/admin/users` with an optional `@RequestParam(value = "status", required = false) AccountStatus status` filter (`ACTIVE` | `DISABLED`).
  - Add additive `activeCount` and `disabledCount` fields to `AdminUserPageResponse` so server-paginated status tabs on `/users` and the Overview `Active / Total Users` KPI card remain exact regardless of user count.

### 2. Protobuf Contracts (`contracts/proto/`)

#### `contracts/proto/catalog/v1/catalog.proto`

Add additive RPCs and messages to `ecommerce.catalog.v1.CatalogService`:

```protobuf
rpc ListAdminProducts(ListAdminProductsRequest) returns (ListAdminProductsResponse);
rpc ListInventoryCatalogItems(ListInventoryCatalogItemsRequest) returns (ListInventoryCatalogItemsResponse);
rpc GetAdminProduct(GetAdminProductRequest) returns (GetProductResponse);
rpc CreateProduct(CreateProductRequest) returns (GetProductResponse);
rpc UpdateProduct(UpdateProductRequest) returns (GetProductResponse);
```

- `AdminProductSummary`: mirrors `ProductSummary` fields `1..16` plus `bool deleted = 17;`, `int32 variant_count = 18;`, `string created_at = 19;`, `string updated_at = 20;`.
- `CategoryMetric`: `string category_id = 1;`, `string category_name = 2;`, `int64 active_product_count = 3;`.
- `InventoryCatalogItem`: lightweight catalog projection (`string id = 1;`, `string name = 2;`, `string brand = 3;`, `string category_id = 4;`, `string category_name = 5;`, `string image_url = 6;`, `string emoji = 7;`, `repeated uint32 image_hue = 8;`, `bool deleted = 9;`).
- `ListAdminProductsRequest`:
  - `string user_id = 1;`
  - `string query = 2;`
  - `string category_id = 3;`
  - `string status = 4;` (`""` / `"all"`, `"active"`, `"deleted"`)
  - `string sort = 5;` (`""` / `"newest"`, `"updated"`, `"price_asc"`, `"price_desc"`, `"popularity"`)
  - `int32 page = 6;`
  - `int32 page_size = 7;` (default `20`, max `200`)
- `ListAdminProductsResponse`:
  - `repeated AdminProductSummary items = 1;`
  - `int32 page = 2;`
  - `int32 page_size = 3;`
  - `int64 total_items = 4;`
  - `int32 total_pages = 5;`
  - `int64 active_count = 6;`
  - `int64 deleted_count = 7;`
  - `repeated CategoryMetric categories = 8;` *(Builder call: returns distinct category metrics across active products for Overview charts and filter dropdowns)*
- `ListInventoryCatalogItemsRequest { string user_id = 1; string query = 2; string category_id = 3; bool include_deleted = 4; }` / `ListInventoryCatalogItemsResponse { repeated InventoryCatalogItem items = 1; }` *(Builder call: authorizes against `inventory.view` so warehouse-only operators without `product.view` can inspect/restock both active and soft-deleted catalog items without hitting `SearchProducts`'s 50-item active-only cap)*
- `GetAdminProductRequest { string id = 1; string user_id = 2; }`
- `CreateProductRequest` / `UpdateProductRequest`:
  - `string user_id = 1;`
  - `string id = 2;` (optional custom slug on create; required target ID on update)
  - `string name = 3;`
  - `string short_description = 4;`
  - `string description = 5;`
  - `string brand = 6;`
  - `string category_id = 7;`
  - `string category_name = 8;`
  - `ecommerce.common.v1.Money price = 9;`
  - `ecommerce.common.v1.Money original_price = 10;`
  - `repeated string images = 11;`
  - `bool is_new = 12;`
  - `string badge = 13;`
  - `bool featured = 14;`
  - `string emoji = 15;`
  - `repeated uint32 image_hue = 16;`
  - `repeated ProductVariant variants = 17;`
  - `repeated string details = 18;`

#### `contracts/proto/inventory/v1/inventory.proto`

Add additive RPCs and messages to `ecommerce.inventory.v1.InventoryService`:

```protobuf
rpc GetStock(GetStockRequest) returns (GetStockResponse);
rpc BatchGetStock(BatchGetStockRequest) returns (BatchGetStockResponse);
rpc ListStocks(ListStocksRequest) returns (ListStocksResponse);
rpc SetStock(SetStockRequest) returns (SetStockResponse);
rpc AdjustStock(AdjustStockRequest) returns (AdjustStockResponse);
```

- `StockItem`:
  - `string product_id = 1;`
  - `int32 quantity = 2;`
  - `int32 reserved = 3;`
  - `int32 available_quantity = 4;`
  - `string updated_at = 5;` (RFC3339, empty when row has not been initialized yet)
- `GetStockRequest { string user_id = 1; string product_id = 2; }` / `GetStockResponse { StockItem stock = 1; }`
- `BatchGetStockRequest { string user_id = 1; repeated string product_ids = 2; }` / `BatchGetStockResponse { repeated StockItem items = 1; }`
- `ListStocksRequest { string user_id = 1; }` / `ListStocksResponse { repeated StockItem items = 1; }`
- `SetStockRequest { string user_id = 1; string product_id = 2; int32 quantity = 3; }` / `SetStockResponse { StockItem stock = 1; }`
- `AdjustStockRequest { string user_id = 1; string product_id = 2; int32 delta = 3; }` / `AdjustStockResponse { StockItem stock = 1; }`

### 3. `catalog-service` Extensions

- **Migration (`internal/store/migrations/000003_add_products_updated_at.up.sql`):**
  - `ALTER TABLE products ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();`
- **Authorization (`internal/grpc/`):**
  - `ListAdminProducts` & `GetAdminProduct` check `product.view`.
  - `ListInventoryCatalogItems` checks `inventory.view`.
  - `CreateProduct` checks `product.create`.
  - `UpdateProduct` checks `product.update`.
  - `SoftDeleteProduct` continues checking `product.delete` (and updates `updated_at = now()`).
  - Unauthenticated/missing `user_id` -> `codes.InvalidArgument`; permission denied -> `codes.PermissionDenied` (`ErrorInfo{Reason: "PRODUCT_PERMISSION_DENIED", Domain: "ecommerce.catalog.v1"}`); Identity gRPC outage -> `codes.Unavailable` (`ErrorInfo{Reason: "AUTHORIZATION_UNAVAILABLE", Domain: "ecommerce.catalog.v1"}`).
- **Domain Validation & Transactional Store (`internal/domain/`, `internal/store/`):**
  - `name` (`1..200` chars), `brand` (`1..100`), `category_id` (`1..64`), `category_name` (`1..100`), `price > 0` (`currency = "VND"`), `original_price == 0 || original_price >= price`, at least 1 `ProductVariant` with non-empty `label` (`<= 80` chars) and optional valid hex color (`^#[0-9a-fA-F]{6}$` or empty), max `10` images (`http://` or `https://` URLs), max `20` variants, max `20` details. *(Builder call: Product Drawer provides the 8 canonical storefront categories—`dien-thoai`, `laptop`, `tai-nghe`, `dong-ho`, `thoi-trang`, `gia-dung`, `lam-dep`, `sach`—as primary presets while allowing custom `categoryId`/`categoryName` and surfacing all distinct database categories dynamically via `ListAdminProductsResponse.categories`.)*
  - On `CreateProduct`: if `id` is empty, generate a URL-safe slug from `name` + short random suffix; if `id` is provided, validate `^[a-z0-9][a-z0-9-]{1,63}$`. Primary-key collision returns `domain.ErrProductExists` -> `codes.AlreadyExists` (`PRODUCT_ID_EXISTS`).
  - On `UpdateProduct`: atomically update `products` (`updated_at = now()`) and replace `product_images`, `product_variants`, and `product_details` inside one transaction. Missing `id` returns `domain.ErrProductNotFound` -> `codes.NotFound`.
  - `ListAdminProducts`: returns paginated `AdminProductSummary` matching `query` (`name`, `brand`, `category_name`, or `id` `ILIKE`), `category_id`, and `status` (`all` / `active` / `deleted`), alongside global `active_count`, `deleted_count`, and `categories` metrics.
  - `ListInventoryCatalogItems`: returns lightweight projections (`id`, `name`, `brand`, `category_id`, `category_name`, primary `image_url`, `emoji`, `image_hue`, `deleted`) matching `query`, `category_id`, and `include_deleted` (capped at `1000` rows ordered by `deleted_at IS NOT NULL ASC, updated_at DESC, id ASC`).

### 4. `inventory-service` Extensions

- **Private Authorization Client (`internal/authorization/`, `internal/config/`, `cmd/server/`):**
  - Add `IDENTITY_GRPC_ADDR` (default `127.0.0.1:50054` locally, `identity-service:50054` in Dockerfile/Compose) and `IDENTITY_GRPC_TIMEOUT` (`500ms`), propagating `x-trace-id` metadata identically to `catalog-service`.
  - Update `services/inventory-service/Dockerfile`, `deployments/docker/compose.production.yml` (`depends_on: identity-service: condition: service_healthy`), `scripts/start-local/main.go`, `dockerfile_production_test.go`, and `compose_production_test.go`.
  - Keep `GetAvailability` unauthenticated for storefront/cart internal calls; require `inventory.view` on `GetStock`, `BatchGetStock`, and `ListStocks`, and `inventory.update` on `SetStock` and `AdjustStock`.
- **Atomic Stock Mutations (`internal/domain/`, `internal/store/`):**
  - `BatchGetStock`: up to `200` `product_ids`; returns a `StockItem` per requested ID in order (missing rows default to `quantity = 0, reserved = 0, available_quantity = 0`).
  - `ListStocks`: returns all persisted `stock` rows ordered by `updated_at DESC, product_id ASC`.
  - `SetStock(product_id, quantity)`:
    - Rejects `quantity < 0` with `codes.InvalidArgument`.
    - Executes single-statement conditional upsert:
      ```sql
      INSERT INTO stock (product_id, quantity, reserved, updated_at)
      VALUES ($1, $2, 0, now())
      ON CONFLICT (product_id) DO UPDATE
      SET quantity = EXCLUDED.quantity, updated_at = now()
      WHERE stock.reserved <= EXCLUDED.quantity
      RETURNING product_id, quantity, reserved, updated_at
      ```
    - If `pgx.ErrNoRows` (meaning the row exists with `reserved > quantity`), returns `domain.ErrStockBelowReserved` -> `codes.FailedPrecondition` (`ErrorInfo{Reason: "STOCK_BELOW_RESERVED", Domain: "ecommerce.inventory.v1"}`).
  - `AdjustStock(product_id, delta)`:
    - Rejects `delta == 0` with `codes.InvalidArgument`.
    - For `delta > 0`: `INSERT INTO stock (product_id, quantity, reserved, updated_at) VALUES ($1, $2, 0, now()) ON CONFLICT (product_id) DO UPDATE SET quantity = stock.quantity + $2, updated_at = now() RETURNING ...`.
    - For `delta < 0`: `UPDATE stock SET quantity = quantity + $2, updated_at = now() WHERE product_id = $1 AND quantity + $2 >= reserved RETURNING ...`; distinguishes missing row (`quantity == 0` cannot subtract -> `STOCK_BELOW_RESERVED` / `INSUFFICIENT_STOCK`) from `reserved` violation, returning `codes.FailedPrecondition` (`STOCK_BELOW_RESERVED`).

### 5. `bff-service` Admin Endpoints & Kong Routing

#### BFF HTTP Routes (`services/bff-service/internal/http/`)

All admin handlers use `correlateTrace(RequireCaller(...))`:

- **Products (`/api/v1/admin/products`):**
  - `GET /api/v1/admin/products` (`q`, `categoryId`, `status`, `sort`, `page`, `pageSize`):
    - Calls `CatalogService.ListAdminProducts`.
    - Calls `InventoryService.BatchGetStock` for the page's product IDs. If Inventory returns `PermissionDenied` (operator has `product.view` without `inventory.view`) or transient error, still returns `200 OK` with `stockDegraded: true` and `stock: null` on each item instead of failing product listing.
    - Returns `{ items, page, pageSize, totalItems, totalPages, activeCount, deletedCount, stockDegraded }`.
  - `GET /api/v1/admin/products/{id}`: calls `CatalogService.GetAdminProduct` + optional `InventoryService.GetStock`.
  - `POST /api/v1/admin/products`: validates JSON payload, calls `CatalogService.CreateProduct` -> `201 Created`, message `"Product created."`.
  - `PUT /api/v1/admin/products/{id}`: validates JSON payload, calls `CatalogService.UpdateProduct` -> `200 OK`, message `"Product updated."`.
  - `POST /api/v1/admin/products/{id}/delete`: existing soft-delete/restore endpoint preserved (`200 OK`, message `"Product availability updated."`).
- **Inventory (`/api/v1/admin/inventory`):**
  - `GET /api/v1/admin/inventory` (`q`, `categoryId`, `stockStatus` in `all | in_stock | low_stock | out_of_stock`, `includeDeleted` default `true`, `page`, `pageSize`):
    - Calls `InventoryService.ListStocks` (verifying `inventory.view` first so unauthorized callers immediately receive `403 FORBIDDEN`) and `CatalogService.ListInventoryCatalogItems` (also authorized via `inventory.view`, returning both active and soft-deleted catalog items when `includeDeleted=true` so warehouse operators can inspect reserved/physical stock or restock hidden items prior to restore).
    - Merges catalog items with `ListStocks` map (defaulting missing rows to `quantity = 0, reserved = 0, availableQuantity = 0`), computes summary KPIs (`totalTrackedProducts`, `inStockCount`, `lowStockCount` for `1..5`, `outOfStockCount` for `0`, `totalQuantity`, `totalReserved`) across the matched catalog scope, applies `stockStatus` filter, and slices the requested `page` / `pageSize`.
  - `PUT /api/v1/admin/inventory/{productId}` (body `{ "quantity": <int> }`):
    - Verifies product exists via `CatalogService.GetProduct` (`404 PRODUCT_NOT_FOUND` if missing; soft-deleted products remain restockable), then calls `InventoryService.SetStock` -> `200 OK`, message `"Stock quantity updated."`.
  - `POST /api/v1/admin/inventory/{productId}/adjust` (body `{ "delta": <int> }`):
    - Verifies product exists via `CatalogService.GetProduct`, then calls `InventoryService.AdjustStock` -> `200 OK`, message `"Stock adjusted."`.
- **Error Mapping:**
  - `400 INVALID_REQUEST` (validation / `InvalidArgument` with structured `errors` map when applicable)
  - `401 UNAUTHORIZED` (missing caller context)
  - `403 FORBIDDEN` (`PermissionDenied`)
  - `404 PRODUCT_NOT_FOUND` (`NotFound`)
  - `409 PRODUCT_ID_EXISTS` (`AlreadyExists`) / `409 STOCK_BELOW_RESERVED` (`FailedPrecondition`)
  - `503 AUTHORIZATION_SERVICE_UNAVAILABLE` (`AUTHORIZATION_UNAVAILABLE`)
  - `502 CATALOG_UNAVAILABLE` / `502 INVENTORY_UNAVAILABLE`

#### Kong & Production Compose (`deployments/`)

- `deployments/kong/kong.yaml`:
  - Add `"http://localhost:5174"` to global `cors.config.origins`.
  - Add protected route `admin-inventory-api` (`paths: ["/api/v1/admin/inventory"]`, `strip_path: false`, `caller-context`) on `bff-service`.
- `deployments/kong/kong-production.yaml`:
  - Expand `admin-products-api` `methods` to `["GET", "POST", "PUT", "PATCH", "DELETE"]`.
  - Add `admin-inventory-api` (`paths: ["/api/v1/admin/inventory"]`, `methods: ["GET", "POST", "PUT", "PATCH"]`, `strip_path: false`, `caller-context`).
  - Add `admin-fe` service (`http://admin-fe:8080`) with route `admin-spa` (`paths: ["/admin"]`, `methods: ["GET", "HEAD"]`, `strip_path: false`, `request-transformer` stripping `X-User-Id` and `X-User-Roles`).
- `deployments/docker/compose.production.yml`:
  - Wire `inventory-service` to `identity-service:50054` (`depends_on` + env vars).
  - Add `admin-fe` container service (built with `VITE_BASE_PATH=/admin/` and `VITE_API_BASE_URL=/api/v1`) and add `admin-fe` to `kong.depends_on`.

### 6. OpenDesign Workflow & Standalone `admin-fe/` Application

#### Step A: OpenDesign Visual Prototype (Local Antigravity Runtime)
- Collect/confirm design brief and create a dedicated OpenDesign project (`"S-Shop Admin Control Center"`).
- Commission generation via OpenDesign's local Antigravity runtime for an executive commerce operations console (distinctive industrial-precision editorial aesthetic, Vietnamese UI copy, crisp data density, clear status badges, KPI cards, product drawer, stock adjustment modal, user & RBAC hierarchy views).
- Pull generated design artifacts via `get_artifact()` and translate tokens (palette, type scale, spacing, radii, component states) into `admin-fe/src/index.css` and layout/UI components.

#### Step B: `admin-fe/` Architecture & Modules
- **Location & Tooling:** `admin-fe/` at repository root; dev server on `http://localhost:5174`; `Makefile` target `make admin-fe`; `make verify SERVICE=admin-fe` running `vitest run`, `oxlint`, and `tsc -b && vite build`; `scripts/check-boundaries` scanning `admin-fe/src` to forbid backend/contract/cross-app imports; `admin-fe/AGENTS.md` documenting local rules.
- **Session & Permission Architecture (`src/lib/`, `src/context/`):**
  - In-memory access token + single-flight `/auth/refresh` interceptor (`withCredentials: true`), matching `ecommerce-fe` security guarantees.
  - `AuthProvider` restores session on boot, loads `/users/me` + `/users/me/permissions`, exposes `can(permission)` helper, and gates console entry on holding at least one administrative permission (showing a clear "Không có quyền quản trị" screen with account switch/logout for customer-only accounts).
- **Screens & Workflows (`src/pages/`):**
  1. **Login (`/login`):** Focused operator sign-in with field validation, server error handling (`INVALID_CREDENTIALS`, `ACCOUNT_DISABLED`), and redirect restoration.
  2. **Overview (`/`)**: Real-time KPI cards (Active/Hidden Products, Stock Health breakdown, Total/Active Users, RBAC Roles & Permissions), Recharts stock/category breakdown, and Low/Out-of-Stock quick-action watchlist. Each widget renders conditionally based on `can(...)` so partial-permission operators never hit `403` noise.
  3. **Products (`/products`):** TanStack Table with search, category filter, status tabs (`Tất cả`, `Đang bán`, `Đã ẩn`), sort selector, stock indicator, Soft-Delete / Restore action, and Create/Edit Product Drawer (`react-hook-form` + `zod`) supporting dynamic image URLs with preview, color/label variants, specification details, pricing validation (`originalPrice >= price`), and optional 2-step initial stock setup on creation.
  4. **Inventory (`/inventory`):** Enriched stock table with `Tất cả` / `Còn hàng` / `Sắp hết (1-5)` / `Hết hàng (0)` tabs, search & category filters, KPI summary strip (`Tổng tồn vật lý`, `Đang giữ chỗ`, `Khả dụng`), and Stock Adjustment Dialog supporting both **Đặt tồn kho thực tế** (`SetStock`) and **Nhập / Xuất tương đối** (`AdjustStock` `+/-`) with live preview and `STOCK_BELOW_RESERVED` conflict feedback.
  5. **Users (`/users`):** Server-paginated user directory (`/admin/users`), search by email/name, status filter/badges (`ACTIVE` / `DISABLED`), User Detail Sheet with status toggle (`PATCH /admin/users/{id}/status`) and role assignment/revocation (`PUT` / `DELETE /admin/users/{id}/roles/{role}`) with clear feedback on `SUPER_ADMIN` / self-lock invariants.
  6. **RBAC (`/rbac`):** Role & Permission catalog (`/admin/rbac/items`), Create/Edit/Delete custom items (with `systemItem` lock badges preventing rename/delete of built-in items), and interactive Hierarchy Manager (`PUT` / `DELETE /admin/rbac/items/{parent}/children/{child}`) showing direct vs. inherited permissions and surfacing cycle-detection conflicts clearly.

## What Is Left to the Builder

- Exact OpenDesign visual token palette, font pairing, sidebar density, and responsive breakpoints derived from the generated OpenDesign prototype.
- Internal Go package/file organization within `catalog-service`, `inventory-service`, and `bff-service` (following each service's `AGENTS.md` layout).
- Reusable `shadcn/ui` component granularity under `admin-fe/src/components/ui/` and test fixture details across Go, Java, and Vitest suites.

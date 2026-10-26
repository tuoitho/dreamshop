# Storefront UI/UX Consistency & Product Availability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify the storefront layout shell across account/auth pages, remove developer soft-delete controls from `ProductDetailPage`, enrich storefront catalog responses in `bff-service` with live `InventoryService` stock availability, and resolve UI/UX and product lifecycle edge cases end-to-end across `ecommerce-fe`.

**Architecture:** `bff-service` adds a private outbound gRPC `inventory.Client` (`127.0.0.1:50052` locally, `inventory-service:50052` in Docker Compose) to enrich `GET /api/v1/storefront/products` (bounded concurrency) and `GET /api/v1/storefront/products/{id}` with additive `status`, `availableQuantity`, and `inventoryDegraded` fields, degrading gracefully to `status: "UNKNOWN"` when `inventory-service` is unreachable. `ecommerce-fe` wraps `/profile`, `/login`, and `/register` in `StoreLayout`, switches `HomePage` to live storefront API data, removes the temporary `DEV` soft-delete UI from `ProductDetailPage`, and renders consistent availability states and responsive navigation across all storefront screens.

**Tech Stack:** Go 1.24 (`net/http`, `google.golang.org/grpc`, `spf13/viper`), React 19, TypeScript 6, Vite 8, React Router 7, Tailwind CSS 4, Radix UI, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-26-storefront-ui-ux-and-product-availability-design.md`

## Global Constraints

- Service boundaries (`AGENTS.md`, `docs/steering/service-boundaries.md`): `bff-service` calls `inventory-service` directly over private gRPC (`127.0.0.1:50052` locally, `inventory-service:50052` in Docker Compose) using generated `ecommerce.local/contracts/go/inventory/v1`. Never route internal calls through Kong (`:8000`) or import across services.
- Additive REST compatibility (`docs/steering/api-contracts.md`): Keep all existing fields on `/api/v1/storefront/products` and `/api/v1/storefront/products/{id}` unchanged; new fields (`status`, `availableQuantity`, `inventoryDegraded`) use `camelCase` and frontend components tolerate their absence.
- Graceful inventory degradation: Storefront product search and detail must NEVER return `502` when `inventory-service` fails; return `200 OK` with `status: "UNKNOWN"`, `inventoryDegraded: true`, and `availableQuantity` omitted.
- Admin endpoint preservation: Keep `POST /api/v1/admin/products/{id}/delete` in `bff-service` and `softDeleteProduct` in `ecommerce-fe/src/lib/api.ts` intact; remove only the temporary `DEV` UI/handlers on `ProductDetailPage`.
- Session & toast conventions: Preserve `<Toaster richColors position="top-center" />` in `ecommerce-fe/src/main.tsx`, keep access tokens in memory, and keep HTTP calls inside `ecommerce-fe/src/lib/` adapters.
- Working tree safety: Do NOT run `git reset`, `git checkout --`, `git restore`, or `git commit` without explicit user instruction because pre-existing staged/unstaged user changes exist on `main`.

## Review Focus

1. **Partial or negative inventory responses during search enrichment:** If `InventoryService.GetAvailability` returns a negative `available_quantity`, `bff-service` clamps it to `OUT_OF_STOCK` (`availableQuantity: 0`); if `GetAvailability` errors for one product on a multi-item search page, only that item becomes `status: "UNKNOWN"` (`inventoryDegraded: true`) while sibling items retain their resolved stock statuses.
2. **Discontinued (`deleted: true`) or variant-less (`variants: []`) product detail:** `bff-service` short-circuits without calling `InventoryService` and returns `status: "UNAVAILABLE"`, `availableQuantity: 0`; on the frontend, `resolveProductAvailability` also treats `deleted === true` or empty `variants` on `ProductDetail` as `"UNAVAILABLE"` even when `status` is absent or `"AVAILABLE"`.
3. **Quantity clamping on `LIMITED` stock:** When `ProductDetailPage` loads or reloads a `LIMITED` product with `availableQuantity < quantity` (e.g., `availableQuantity = 2`), `quantity` clamps to `maxQuantity` (`2`) and the `+` button disables at `2` so purchase actions never submit `quantity > availableQuantity`.
4. **Mid-action race when stock depletes or product is discontinued before `Add to Cart` / `Buy Now`:** When `addItem` resolves at HTTP 200 (via `CartProvider`) and the resulting cart line has `status === "OUT_OF_STOCK"` or `"UNAVAILABLE"`, `ProductDetailPage` shows a specific `toast.error` (`"Sản phẩm vừa tạm hết hàng"` / `"Sản phẩm đã ngừng kinh doanh"`), reloads product detail, and does not show `"Đã thêm vào giỏ"` or navigate to `/gio-hang/dat-hang`.
5. **Hard-deleted orphan cart line (`item.name === ""`, `item.unitPrice === 0`, `item.status === "UNAVAILABLE"`):** `CartPage` renders fallback title `"Sản phẩm không còn tồn tại"`, `"📦"` icon, accessible checkbox/remove button names, `"Không khả dụng"` instead of `"0 ₫"`, non-broken plain text instead of a dead `/san-pham/:id` link, and supports removing all unavailable items in one click while preserving the exact `<section>` -> `<div className="grid gap-3">` -> row `<label>` DOM hierarchy expected by existing accessibility/layout tests.

---

## File Structure

- **Create `services/bff-service/internal/inventory/client.go`** (+ `client_test.go`): gRPC client adapter for `inventoryv1.InventoryServiceClient.GetAvailability` with per-call timeout.
- **Modify `services/bff-service/internal/config/config.go`** (+ `config_test.go`, `compose_production_test.go`): Load & validate `BFF_INVENTORY_GRPC_ADDR` (default `127.0.0.1:50052`) and `BFF_INVENTORY_GRPC_TIMEOUT` (default `3s`).
- **Modify `services/bff-service/Dockerfile`, `deployments/docker/compose.production.yml`, `scripts/start-local/main.go`, `scripts/start-local/main_test.go`, `docs/steering/service-catalog.md`**: Wire production container DNS (`inventory-service:50052`), `depends_on`, local launcher env defaults, and service catalog documentation.
- **Modify `services/bff-service/internal/http/products.go`** (+ `products_test.go`) & **`services/bff-service/cmd/server/main.go`** (+ `main_test.go`): Enrich storefront product search (bounded concurrency `8`) and product detail responses with `status`, `availableQuantity`, and `inventoryDegraded`.
- **Create `ecommerce-fe/src/lib/product-availability.ts`** (+ `product-availability.test.ts`) & **`ecommerce-fe/src/lib/shipping.ts`**, **Modify `ecommerce-fe/src/lib/types.ts`**: Shared product availability state/badge/limit resolver and shared shipping fee constants.
- **Modify `ecommerce-fe/src/App.tsx`** (+ `App.test.tsx`), **`ecommerce-fe/src/routes/require-auth.tsx`**, **`ecommerce-fe/index.html`**, **`ecommerce-fe/src/components/layout/site-header.tsx`** (+ `site-header.test.tsx`), **`ecommerce-fe/src/components/layout/site-footer.tsx`**, **`ecommerce-fe/src/pages/login-page.tsx`** (+ `login-page.test.tsx`), **`ecommerce-fe/src/pages/register-page.tsx`**: Unified `StoreLayout` shell, mobile account access & drawer toggle fix, search input URL sync, active nav links, registration banner, auth redirect, and real footer links.
- **Modify `ecommerce-fe/src/components/product-card.tsx`** (+ `product-card.test.tsx`) & **`ecommerce-fe/src/pages/product-detail-page.tsx`** (+ `product-detail-page.test.tsx`): Rating `toFixed(1)`, availability badges/overlays/banners, removal of `DEV` soft-delete UI, quantity capping, mid-action race handling, full description rendering, and interactive wishlist/share buttons.
- **Modify `ecommerce-fe/src/pages/home-page.tsx`** (+ `home-page.test.tsx`) & **`ecommerce-fe/src/pages/search-page.tsx`** (+ `search-page.test.tsx`): Live storefront API loading on `HomePage`, dynamic hero/CTA, search heading casing & sale title, URL-synced price slider, and mobile filter count badge.
- **Modify `ecommerce-fe/src/components/ui/searchable-select.tsx`**, **`ecommerce-fe/src/pages/profile-page.tsx`** (+ `profile-page.test.tsx`), **`ecommerce-fe/src/pages/cart-page.tsx`** (+ `cart-page.test.tsx`), **`ecommerce-fe/src/pages/order-preview-page.tsx`** (+ `order-preview-page.test.tsx`): `ProfilePage` `signOut` resilience & `aria-invalid`/`readOnly` polish, `CartPage` `imageUrl` & hard-deleted item fallback & bulk unavailable removal & loading skeleton, `OrderPreviewPage` thumbnails, unit pricing, low-stock warning, and shipping recipient card (resolving province/ward names via `src/lib/locations.ts` when `provinceCode` is set).

---

### Task 1: `bff-service` Inventory gRPC Client, Config & Topology Wiring

**Files:**
- Create: `services/bff-service/internal/inventory/client.go`
- Create: `services/bff-service/internal/inventory/client_test.go`
- Modify: `services/bff-service/internal/config/config.go`
- Modify: `services/bff-service/internal/config/config_test.go`
- Modify: `services/bff-service/internal/config/compose_production_test.go`
- Modify: `services/bff-service/Dockerfile`
- Modify: `deployments/docker/compose.production.yml`
- Modify: `scripts/start-local/main.go`
- Modify: `scripts/start-local/main_test.go`
- Modify: `docs/steering/service-catalog.md`

**Interfaces:**
- Consumes: `ecommerce.local/contracts/go/inventory/v1` (`InventoryServiceClient`, `GetAvailabilityRequest`, `GetAvailabilityResponse`)
- Produces:
  - `inventory.Client` interface: `GetAvailability(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error)`
  - `inventory.NewClient(target string, timeout time.Duration) (*client, error)` with `Close() error`
  - `config.Config.InventoryGRPCAddr string` (`BFF_INVENTORY_GRPC_ADDR`, default `"127.0.0.1:50052"`)
  - `config.Config.InventoryGRPCTimeout time.Duration` (`BFF_INVENTORY_GRPC_TIMEOUT`, default `3*time.Second`)

- [ ] **Step 1: Write failing tests for `internal/inventory`, `internal/config`, `compose_production_test.go`, and `scripts/start-local`**
  - Create `services/bff-service/internal/inventory/client_test.go` testing `GetAvailability` forwarding `product_id`, returning `available_quantity`, enforcing per-call `DeadlineExceeded`, and propagating `Unavailable` errors.
  - Extend `services/bff-service/internal/config/config_test.go` and `compose_production_test.go` for `InventoryGRPCAddr`, `InventoryGRPCTimeout`, blank address / non-positive timeout rejection, `Dockerfile` `ENV BFF_INVENTORY_GRPC_ADDR="inventory-service:50052"` / `ENV BFF_INVENTORY_GRPC_TIMEOUT="3s"`, and `compose.production.yml` `bff-service` `BFF_INVENTORY_GRPC_ADDR` + `depends_on: inventory-service`.
  - Extend `scripts/start-local/main_test.go` to assert `BFF_INVENTORY_GRPC_ADDR=127.0.0.1:50052` and `BFF_INVENTORY_GRPC_TIMEOUT=3s`.

- [ ] **Step 2: Run Go tests to verify RED failure**
  - Run: `go test ./services/bff-service/internal/inventory/... ./services/bff-service/internal/config/... ./scripts/start-local/...`
  - Expected: FAIL (`undefined: NewClient`, `cfg.InventoryGRPCAddr undefined`, missing `BFF_INVENTORY_GRPC_ADDR`).

- [ ] **Step 3: Implement `inventory/client.go`, `config.go`, `Dockerfile`, `compose.production.yml`, `scripts/start-local/main.go`, and `docs/steering/service-catalog.md`**

- [ ] **Step 4: Run Go tests to verify GREEN**
  - Run: `go test ./services/bff-service/internal/inventory/... ./services/bff-service/internal/config/... ./scripts/start-local/...`
  - Expected: PASS (exit code 0).

---

### Task 2: `bff-service` Storefront Product Availability Composition

**Files:**
- Modify: `services/bff-service/internal/http/products.go`
- Modify: `services/bff-service/internal/http/products_test.go`
- Modify: `services/bff-service/cmd/server/main.go`
- Modify: `services/bff-service/cmd/server/main_test.go`

**Interfaces:**
- Consumes: `catalog.Client`, `inventory.Client`, `config.Config`
- Produces:
  - `http.NewProductsHandler(catalogClient catalog.Client, inventoryClient inventory.Client) *ProductsHandler`
  - Additive JSON fields on `GET /api/v1/storefront/products` items and `GET /api/v1/storefront/products/{id}` `data`:
    - `status`: `"AVAILABLE"` (`> 5`), `"LIMITED"` (`1..5`), `"OUT_OF_STOCK"` (`<= 0`), `"UNAVAILABLE"` (`deleted: true` or `len(variants) == 0` on detail), `"UNKNOWN"` (when `inventoryClient == nil` or `GetAvailability` errors)
    - `availableQuantity`: `*int32` (`omitempty`; `0` for `OUT_OF_STOCK`/`UNAVAILABLE`, positive count for `LIMITED`/`AVAILABLE`, `nil`/omitted for `UNKNOWN`)
    - `inventoryDegraded`: `bool` (`omitempty`; `true` only when `status == "UNKNOWN"`)

- [ ] **Step 1: Write failing tests in `products_test.go` and `cmd/server/main_test.go`**
  - Update `newTestMux` and `buildMux` signatures; test:
    - Search enrichment across mixed items (`AVAILABLE`, `LIMITED`, `OUT_OF_STOCK` including negative stock clamped to `0`, and partial single-item inventory error yielding `UNKNOWN` + `inventoryDegraded: true` without failing sibling items).
    - Detail enrichment for `AVAILABLE` (`availableQuantity: 20`), `LIMITED` (`availableQuantity: 3`), `OUT_OF_STOCK` (`availableQuantity: 0`), `deleted: true` (`status: "UNAVAILABLE"`, `availableQuantity: 0`, zero inventory calls), empty `variants` (`status: "UNAVAILABLE"`, `availableQuantity: 0`, zero inventory calls), and inventory outage (`200 OK`, `status: "UNKNOWN"`, `inventoryDegraded: true`, `availableQuantity` omitted).

- [ ] **Step 2: Run Go tests to verify RED failure**
  - Run: `go test ./services/bff-service/...`
  - Expected: FAIL (`too many arguments in call to NewProductsHandler` / missing JSON fields).

- [ ] **Step 3: Implement availability enrichment with bounded concurrency (`maxInventoryConcurrency = 8`) in `products.go` and wire `inventory.NewClient` in `cmd/server/main.go`**

- [ ] **Step 4: Run Go tests to verify GREEN**
  - Run: `go test ./services/bff-service/...`
  - Expected: PASS (exit code 0).

---

### Task 3: `ecommerce-fe` Types, Availability Resolver & Shared Shipping Constants

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Create: `ecommerce-fe/src/lib/product-availability.ts`
- Create: `ecommerce-fe/src/lib/product-availability.test.ts`
- Create: `ecommerce-fe/src/lib/shipping.ts`

**Interfaces:**
- Produces:
  - `ProductAvailabilityStatus = "AVAILABLE" | "LIMITED" | "OUT_OF_STOCK" | "UNAVAILABLE" | "UNKNOWN"`
  - `ProductSummary.status?: ProductAvailabilityStatus`, `ProductSummary.availableQuantity?: number`, `ProductSummary.inventoryDegraded?: boolean`
  - `formatRating(rating: number): string` -> `Number(rating || 0).toFixed(1)`
  - `resolveProductAvailability(product: ProductSummary | ProductDetail): ResolvedProductAvailability` (`status`, `purchasable`, `maxQuantity`, `availableQuantity`, `badgeLabel`, `detailLabel`)
  - `FREE_SHIPPING_THRESHOLD = 299000`, `STANDARD_SHIPPING_FEE = 25000` in `src/lib/shipping.ts`

- [ ] **Step 1: Write failing unit tests in `ecommerce-fe/src/lib/product-availability.test.ts`**
  - Cover `AVAILABLE` (`availableQuantity = 25` -> `maxQuantity = 10`, `availableQuantity = 7` -> `maxQuantity = 7`), `LIMITED` (`availableQuantity = 3` -> `maxQuantity = 3`, `badgeLabel = "Chỉ còn 3"`), `OUT_OF_STOCK` (`purchasable = false`, `maxQuantity = 0`), `UNAVAILABLE` via `deleted: true` or `variants: []` even if `status: "AVAILABLE"` (`purchasable = false`), `UNKNOWN` / omitted `status` (`purchasable = true`, `maxQuantity = 10`), and `formatRating(4.900000095367432) === "4.9"`.

- [ ] **Step 2: Run Vitest on `product-availability.test.ts` to verify RED failure**
  - Run: `npm --prefix ecommerce-fe run test -- src/lib/product-availability.test.ts`
  - Expected: FAIL (`Cannot find module './product-availability'`).

- [ ] **Step 3: Implement `types.ts`, `product-availability.ts`, and `shipping.ts`**

- [ ] **Step 4: Run Vitest to verify GREEN**
  - Run: `npm --prefix ecommerce-fe run test -- src/lib/product-availability.test.ts`
  - Expected: PASS.

---

### Task 4: `ecommerce-fe` Unified Layout Shell, Mobile Header, Auth Pages & Footer

**Files:**
- Modify: `ecommerce-fe/src/App.tsx`
- Create: `ecommerce-fe/src/App.test.tsx`
- Modify: `ecommerce-fe/src/routes/require-auth.tsx`
- Modify: `ecommerce-fe/index.html`
- Modify: `ecommerce-fe/src/components/layout/site-header.tsx`
- Modify: `ecommerce-fe/src/components/layout/site-header.test.tsx`
- Modify: `ecommerce-fe/src/components/layout/site-footer.tsx`
- Modify: `ecommerce-fe/src/pages/login-page.tsx`
- Modify: `ecommerce-fe/src/pages/login-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/register-page.tsx`

**Interfaces:**
- Consumes: `StoreLayout`, `SiteHeader`, `SiteFooter`, `RequireAuth`, `AuthContext`, `CartContext`
- Produces: Unified `StoreLayout` wrapping `/profile`, `/login`, `/register`; responsive account controls & bug-free mobile drawer in `SiteHeader`; post-registration notice & authenticated redirect on `LoginPage`/`RegisterPage`; navigable links in `SiteFooter`.

- [ ] **Step 1: Write failing tests in `App.test.tsx`, `site-header.test.tsx`, and `login-page.test.tsx`**
  - `App.test.tsx`: navigating to `/profile` (authenticated) and `/login` renders `SiteHeader` (`S-Shop`, search input) and `SiteFooter` (`Hỗ trợ khách hàng`).
  - `site-header.test.tsx`:
    - Login/user button is not hidden on mobile (`not.toHaveClass("hidden")`).
    - Clicking `"Mở menu"` opens mobile drawer, clicking `"Mở menu"` a second time closes it (verifying the `pointerdown` outside-click bug is fixed), and mobile drawer renders account actions.
    - Search input syncs with `/tim-kiem?q=iphone` and active nav link sets `aria-current="page"`.
  - `login-page.test.tsx`:
    - Displays registration success alert/status when `location.state?.registered` is `true`.
    - Redirects already-authenticated user (`ready: true, isAuthenticated: true`) to `"/"` (or `state.from`).

- [ ] **Step 2: Run Vitest on `App.test.tsx`, `site-header.test.tsx`, and `login-page.test.tsx` to verify RED failure**
  - Run: `npm --prefix ecommerce-fe run test -- src/App.test.tsx src/components/layout/site-header.test.tsx src/pages/login-page.test.tsx`
  - Expected: FAIL on `/profile` missing header/footer, mobile button `hidden` class, hamburger toggle reopening drawer, missing registration banner, and missing authenticated redirect.

- [ ] **Step 3: Implement `App.tsx`, `require-auth.tsx`, `index.html`, `site-header.tsx`, `site-footer.tsx`, `login-page.tsx`, and `register-page.tsx`**

- [ ] **Step 4: Run Vitest to verify GREEN**
  - Run: `npm --prefix ecommerce-fe run test -- src/App.test.tsx src/components/layout/site-header.test.tsx src/pages/login-page.test.tsx`
  - Expected: PASS.

---

### Task 5: `ecommerce-fe` `ProductCard` & `ProductDetailPage` Availability States & Soft-Delete UI Removal

**Files:**
- Modify: `ecommerce-fe/src/components/product-card.tsx`
- Create: `ecommerce-fe/src/components/product-card.test.tsx`
- Modify: `ecommerce-fe/src/pages/product-detail-page.tsx`
- Modify: `ecommerce-fe/src/pages/product-detail-page.test.tsx`

**Interfaces:**
- Consumes: `resolveProductAvailability`, `formatRating` from `src/lib/product-availability.ts`; `ProductSummary`, `ProductDetail` from `src/lib/types.ts`
- Produces: Updated `ProductCard` and `ProductDetailPage` with full lifecycle state coverage and zero admin soft-delete controls.

- [ ] **Step 1: Write failing tests in `product-card.test.tsx` and `product-detail-page.test.tsx`**
  - `product-card.test.tsx`: formats `4.900000095367432` as `4.9`; renders `"Tạm hết hàng"` badge on `OUT_OF_STOCK`, `"Ngừng kinh doanh"` on `UNAVAILABLE`, and `"Chỉ còn 2"` on `LIMITED`.
  - `product-detail-page.test.tsx`:
    - Remove the 4 obsolete `DEV` soft-delete mutation tests and assert `"Xóa mềm sản phẩm"` / `"Khôi phục sản phẩm"` never render even when `DEV` is `true`.
    - Test `deleted: true` / `status: "UNAVAILABLE"` renders `"Sản phẩm ngừng kinh doanh"` notice and disables `"Ngừng kinh doanh"` + `"Mua ngay"` + quantity stepper.
    - Test `status: "OUT_OF_STOCK"` renders `"Tạm hết hàng"` badge and disables `"Tạm hết hàng"` + `"Mua ngay"` + quantity stepper.
    - Test `status: "LIMITED", availableQuantity: 2` caps stepper at `2` and shows `"Sắp hết — Chỉ còn 2 sản phẩm"`.
    - Test mid-action race on `Add to Cart` when `addCartItem` returns the item with `status: "OUT_OF_STOCK"` or `"UNAVAILABLE"` -> calls `toast.error`, triggers `getProduct` reload, and does not show `"Đã thêm vào giỏ"`.
    - Test float rating `4.900000095367432` renders `4.9`, full `product.description` is visible alongside `product.shortDescription`, and clicking `"Thêm vào yêu thích"` toggles `aria-pressed`.

- [ ] **Step 2: Run Vitest on `product-card.test.tsx` and `product-detail-page.test.tsx` to verify RED failure**
  - Run: `npm --prefix ecommerce-fe run test -- src/components/product-card.test.tsx src/pages/product-detail-page.test.tsx`
  - Expected: FAIL on raw float rating, missing availability states, and `DEV` soft-delete presence.

- [ ] **Step 3: Implement `product-card.tsx` and `product-detail-page.tsx`**
  - Note: Keep `vi.mock("sonner", ...)` compatible (`toast.success`, `toast.error`, `toast.info` or use `toast.success` / `toast.error`).

- [ ] **Step 4: Run Vitest to verify GREEN**
  - Run: `npm --prefix ecommerce-fe run test -- src/components/product-card.test.tsx src/pages/product-detail-page.test.tsx`
  - Expected: PASS.

---

### Task 6: `ecommerce-fe` Live `HomePage` & `SearchPage` Filter UX

**Files:**
- Modify: `ecommerce-fe/src/pages/home-page.tsx`
- Create: `ecommerce-fe/src/pages/home-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/search-page.tsx`
- Modify: `ecommerce-fe/src/pages/search-page.test.tsx`

**Interfaces:**
- Consumes: `searchProducts` (`src/lib/api.ts`), `AuthContext` (`useContext(AuthContext)` so `HomePage` works with or without `AuthProvider` in isolated tests), `ProductCard`

- [ ] **Step 1: Write failing tests in `home-page.test.tsx` and `search-page.test.tsx`**
  - `home-page.test.tsx`:
    - Calls `searchProducts({ sort: "pho-bien", pageSize: 20 }, signal)`, shows loading status then live products with dynamic hero price (`formatVND`), filters featured items by category tab (`aria-pressed`), retries on API error, and switches bottom CTA from `"Đăng ký miễn phí"` (guest) to `"Khám phá ưu đãi hôm nay"` (authenticated).
  - `search-page.test.tsx`:
    - Preserves original casing in heading (`?q=iPhone 15` -> `Kết quả cho "iPhone 15"`).
    - Shows `"Sản phẩm khuyến mãi"` heading when `?giam-gia=1` has no `q` or `danh-muc`.
    - Syncs slider range text when navigating with `?gia=2000000-10000000` (`2.0tr - 10.0tr`) and resets to `0.0tr - 10.0tr` when clearing filters.

- [ ] **Step 2: Run Vitest on `home-page.test.tsx` and `search-page.test.tsx` to verify RED failure**
  - Run: `npm --prefix ecommerce-fe run test -- src/pages/home-page.test.tsx src/pages/search-page.test.tsx`
  - Expected: FAIL (`HomePage` uses static `products`, `SearchPage` lowercases heading and shows `"Tất cả sản phẩm"` on `?giam-gia=1`).

- [ ] **Step 3: Implement `home-page.tsx` and `search-page.tsx`**

- [ ] **Step 4: Run Vitest to verify GREEN**
  - Run: `npm --prefix ecommerce-fe run test -- src/pages/home-page.test.tsx src/pages/search-page.test.tsx`
  - Expected: PASS.

---

### Task 7: `ecommerce-fe` `ProfilePage`, `CartPage` & `OrderPreviewPage` Resilience & Edge Cases

**Files:**
- Modify: `ecommerce-fe/src/components/ui/searchable-select.tsx`
- Modify: `ecommerce-fe/src/pages/profile-page.tsx`
- Modify: `ecommerce-fe/src/pages/profile-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/cart-page.tsx`
- Modify: `ecommerce-fe/src/pages/cart-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/order-preview-page.tsx`
- Modify: `ecommerce-fe/src/pages/order-preview-page.test.tsx`

**Interfaces:**
- Consumes: `FREE_SHIPPING_THRESHOLD`, `STANDARD_SHIPPING_FEE` from `src/lib/shipping.ts` (re-exported from `cart-page.tsx`); `AuthContext` (`useContext(AuthContext)` in `OrderPreviewPage`); `fetchProvinces`, `fetchWards` from `src/lib/locations.ts`

- [ ] **Step 1: Write failing tests in `profile-page.test.tsx`, `cart-page.test.tsx`, and `order-preview-page.test.tsx`**
  - `profile-page.test.tsx`:
    - `signOut` navigates to `"/"` even when `logout()` rejects (`try / catch / finally`).
    - Marks `Phường/xã` trigger with `aria-invalid="true"` when `wardCode` validation fails.
  - `cart-page.test.tsx`:
    - Renders `<img src={item.imageUrl}>` when `item.imageUrl` is non-empty and falls back to `ProductImage` on error/empty.
    - Renders orphan hard-deleted item (`name: ""`, `imageEmoji: ""`, `unitPrice: 0`, `status: "UNAVAILABLE"`) as `"Sản phẩm không còn tồn tại"` without `"0 ₫"` or broken empty `aria-label`s.
    - Disables `"Xóa đã chọn"` when `selectedItems.length === 0`.
    - `"Xóa tất cả"` button in `"Sản phẩm không khả dụng"` removes all unavailable items while keeping `<div className="grid gap-3">` as a direct child of `<section aria-labelledby="unavailable-heading">`.
  - `order-preview-page.test.tsx`:
    - Displays unit price × quantity (`100.000 ₫ × 2`) and item thumbnail.
    - Shows low-stock warning when a `LIMITED` item has `quantity > availableQuantity`.
    - Displays shipping recipient summary card when `AuthContext` provides `user` (with link to `/profile` when phone or address is incomplete).

- [ ] **Step 2: Run Vitest on `profile-page.test.tsx`, `cart-page.test.tsx`, and `order-preview-page.test.tsx` to verify RED failure**
  - Run: `npm --prefix ecommerce-fe run test -- src/pages/profile-page.test.tsx src/pages/cart-page.test.tsx src/pages/order-preview-page.test.tsx`
  - Expected: FAIL on new assertions.

- [ ] **Step 3: Implement `searchable-select.tsx`, `profile-page.tsx`, `cart-page.tsx`, and `order-preview-page.tsx`**

- [ ] **Step 4: Run Vitest to verify GREEN**
  - Run: `npm --prefix ecommerce-fe run test -- src/pages/profile-page.test.tsx src/pages/cart-page.test.tsx src/pages/order-preview-page.test.tsx`
  - Expected: PASS.

---

### Task 8: Full Gate Verification, Live `bff-service` Reload & Independent Reviews

**Files:**
- Verify all modified files across `services/bff-service`, `scripts/start-local`, `deployments/docker`, `docs/steering`, and `ecommerce-fe`.

- [ ] **Step 1: Run Go, Frontend, Boundary & Surface Verification Gates**
  - Run: `go test ./scripts/... ./services/bff-service/...`
  - Run: `npm --prefix ecommerce-fe run test`
  - Run: `npm --prefix ecommerce-fe run lint`
  - Run: `npm --prefix ecommerce-fe run build`
  - Run: `go run ./scripts/check-boundaries`
  - Run: `go run ./scripts/verify-service -service bff-service`
  - Run: `go run ./scripts/verify-service -service ecommerce-fe`
  - Expected: Exit code 0 on every command.

- [ ] **Step 2: Reload local `bff-service` (`:8081`) and smoke-test live endpoints**
  - Restart the local host `bff-service` process on `:8081` with `BFF_INVENTORY_GRPC_ADDR=127.0.0.1:50052` in the background, verify `http://localhost:8081/healthz` and `http://localhost:8000/api/v1/storefront/products?pageSize=5` + `/api/v1/storefront/products/dt-2` (`OUT_OF_STOCK`) + `/api/v1/storefront/products/lt-1` (`LIMITED`).

- [ ] **Step 3: Dispatch independent read-only `code-reviewer` and `boundary-reviewer` subagents**
  - Address any Critical/Important findings via RED -> GREEN tests and re-run affected verification gates.

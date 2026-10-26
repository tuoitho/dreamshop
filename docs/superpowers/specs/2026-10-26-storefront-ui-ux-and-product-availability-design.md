# Storefront UI/UX Consistency and End-to-End Product Availability Design

## Intent and Why

Several storefront user-experience and state-consistency gaps currently break continuity across browsing, account management, product inspection, and cart checkout:

1. **Broken layout continuity and navigation dead-ends:** `/profile`, `/login`, and `/register` render outside `StoreLayout`, leaving `/profile` without `SiteHeader` or `SiteFooter` and trapping users on account/auth screens without storefront navigation. On mobile viewports (`< 640px`), `SiteHeader` hides both the authenticated user menu and the login button (`hidden sm:inline-flex`) while the mobile drawer renders only category links, making login, profile access, and logout unreachable on mobile. Clicking the mobile menu toggle button while the drawer is open also triggers the outside-`pointerdown` listener before `onClick`, closing and immediately reopening the drawer.
2. **Storefront product availability blind spots:** `InventoryService.GetAvailability` holds authoritative stock quantities (including zero-stock items such as `dt-2` and low-stock items such as `lt-1` and `dt-1`), and `CatalogService.GetProduct` returns `deleted: true` for soft-deleted products (for example when a shopper follows a link from an existing cart line). However, `bff-service`'s `ProductsHandler` only queries `CatalogService` and never queries `InventoryService`, while `ProductDetailPage` ignores `product.deleted` and renders temporary `DEV` soft-delete/restore controls that belong in a dedicated Admin UI. Consequently, shoppers see active "Thêm vào giỏ" and "Mua ngay" controls on discontinued or out-of-stock products, can increment quantity up to 10 when only 1 unit exists in stock, and only discover unavailability after entering `/gio-hang`.
3. **Static vs. live data and page-level UX defects:**
   - `HomePage` reads static demo items from `src/data/products.ts` instead of `/api/v1/storefront/products`, showing emoji placeholders instead of product photography, displaying soft-deleted products that fail or appear discontinued on click, hardcoding `3.450.000đ` on a `26.990.000đ` hero item, and prompting already-authenticated users to register.
   - `ProductCard` and `ProductDetailPage` render raw float32-to-float64 ratings such as `4.900000095367432` instead of `4.9`.
   - `ProductDetailPage` hides full `description` whenever `shortDescription` is present and renders inert favorite/share buttons with no feedback.
   - `SearchPage` lowercases the displayed search query in the heading, shows `"Tất cả sản phẩm"` instead of `"Sản phẩm khuyến mãi"` when filtering `?giam-gia=1`, and fails to synchronize the price slider when URL price filters change or reset.
   - `ProfilePage` lacks `try/finally` safety on `signOut`, navigates inconsistently relative to `SiteHeader`, gives no visual cue that `Email` is `readOnly`, and omits `aria-invalid` on `Select` and `SearchableSelect` controls.
   - `CartPage` ignores `item.imageUrl`, leaves `"Xóa đã chọn"` enabled when nothing is selected, lacks a loading skeleton, and renders empty labels/`0 ₫` if a cart line's product was hard-deleted (`CatalogService` `NotFound`). `OrderPreviewPage` omits item thumbnails, per-unit pricing, low-stock excess warnings, and shipping recipient context from the user profile.
   - `SiteFooter` styles non-interactive `<span>` elements as clickable links, `RequireAuth` renders an unstyled `<p>` during session restoration, and `index.html` uses `lang="en"` with `<title>ecommerce-fe</title>`.

This design unifies the storefront shell, removes admin soft-delete controls from `ProductDetailPage`, composes real-time inventory availability in `bff-service` with graceful degradation, and resolves all identified UI/UX defects across `ecommerce-fe`.

## Goals, Non-Goals, and Anti-Goals

### Goals

- Wrap `/profile`, `/login`, and `/register` inside `StoreLayout` so `SiteHeader` and `SiteFooter` remain consistent across storefront, account, and auth routes.
- Enrich `GET /api/v1/storefront/products` and `GET /api/v1/storefront/products/{id}` in `bff-service` via direct private gRPC calls to `InventoryService.GetAvailability`, exposing additive `status`, `availableQuantity`, and `inventoryDegraded` fields without breaking existing consumers.
- Reflect every product lifecycle and stock state (`AVAILABLE`, `LIMITED`, `OUT_OF_STOCK`, `UNAVAILABLE` / soft-deleted, hard-deleted `PRODUCT_NOT_FOUND`, and `UNKNOWN` when inventory is degraded) accurately and consistently across `HomePage`, `SearchPage`, `ProductCard`, `ProductDetailPage`, `CartPage`, and `OrderPreviewPage`.
- Remove the `DEV` soft-delete/restore UI from `ProductDetailPage` while keeping the `softDeleteProduct` adapter in `src/lib/api.ts` intact for the upcoming Admin product management interface.
- Fix mobile account navigation, drawer toggle behavior, search query synchronization, active navigation highlighting, rating formatting, live `HomePage` product loading, `SearchPage` filter UX, `ProfilePage` accessibility/read-only styling/logout resilience, `CartPage` thumbnails and hard-deleted item fallbacks, `OrderPreviewPage` item/recipient details, `SiteFooter` links, `RequireAuth` loading UI, and `index.html` metadata.

### Non-Goals

- Building the dedicated Admin product management UI (owned by a separate admin feature).
- Implementing order creation or inventory reservation mutations on `OrderPreviewPage` (`order-service` remains a skeleton).
- Modifying any `.proto` file, database schema, Kong route definition, or `catalog-service` / `inventory-service` / `cart-service` / `identity-service` implementation.

### Anti-Goals

- Failing storefront product search or product detail with `502 Bad Gateway` when `inventory-service` is temporarily unreachable — catalog browsing must degrade gracefully while `cart-service` continues enforcing stock state on cart projections.
- Reproducing server authorization or business invariants in React components.
- Removing `POST /api/v1/admin/products/{id}/delete` in `bff-service` or `softDeleteProduct` in `ecommerce-fe/src/lib/api.ts`.

## Constraints

- **Service boundaries (`AGENTS.md`, `docs/steering/service-boundaries.md`):** `bff-service` calls `inventory-service` directly over private gRPC (`127.0.0.1:50052` locally, `inventory-service:50052` in Docker Compose) using generated `ecommerce.local/contracts/go/inventory/v1`. No cross-service imports, Kong loopback calls, or database sharing.
- **Additive REST compatibility (`docs/steering/api-contracts.md`):** Existing fields on `/api/v1/storefront/products` and `/api/v1/storefront/products/{id}` remain unchanged in name and type; new availability fields are strictly additive (`camelCase`), and frontend components tolerate their absence so existing tests and partial payloads remain valid.
- **Session and toast conventions (`ecommerce-fe/AGENTS.md`, `src/main.tsx`):** Access tokens stay in memory; `<Toaster richColors position="top-center" />` in `src/main.tsx` is preserved; API calls stay in `src/lib/` adapters.

## Approaches Considered

### Approach 1 (Selected): `bff-service` Inventory Composition + Full `ecommerce-fe` UI/UX Alignment

Add an outbound `InventoryService` gRPC client in `bff-service` to enrich storefront product summaries and product details with live stock availability (using bounded concurrency and graceful degradation on inventory errors), remove the `DEV` soft-delete UI from `ProductDetailPage`, and align all storefront pages, layout shell, and cart/checkout previews in `ecommerce-fe`.

- **Why chosen:** `bff-service` exists specifically for client-specific commerce composition across Catalog, Inventory, and Cart. Composing `InventoryService.GetAvailability` in BFF requires zero protobuf or domain-service changes and lets unauthenticated and authenticated shoppers see accurate `"Tạm hết hàng"`, `"Chỉ còn X sản phẩm"`, and `"Ngừng kinh doanh"` states before adding to cart.
- **Trade-off:** Touches both `bff-service` (plus local/Compose env wiring for `BFF_INVENTORY_GRPC_ADDR`) and `ecommerce-fe`.

### Approach 2 (Not Selected): Frontend-Only Status Handling Without BFF Inventory Composition

Leave `bff-service` unchanged; handle only `product.deleted` and empty `variants` on `ProductDetailPage` while relying on `CartPage` to reveal `OUT_OF_STOCK` and `LIMITED` states after a user logs in and adds the item.

- **Why rejected:** Shoppers viewing `HomePage`, `SearchPage`, or `ProductDetailPage` still see out-of-stock items (`dt-2`) as purchasable with up to 10 units, creating a misleading add-to-cart / buy-now failure flow.

## Architecture and Component Design

### 1. `bff-service` Storefront Availability Composition

- **Inventory gRPC Client (`services/bff-service/internal/inventory/`):**
  - Define `Client` interface with `GetAvailability(ctx context.Context, productID string) (*inventoryv1.GetAvailabilityResponse, error)` and `Close() error`, backed by `inventoryv1.NewInventoryServiceClient` with per-call timeout (`BFF_INVENTORY_GRPC_TIMEOUT`, default `3s`).
- **Configuration & Runtime Wiring:**
  - `services/bff-service/internal/config/config.go`: Add `InventoryGRPCAddr` (`BFF_INVENTORY_GRPC_ADDR`, default `"127.0.0.1:50052"`) and `InventoryGRPCTimeout` (`BFF_INVENTORY_GRPC_TIMEOUT`, default `3s`), validating non-empty address and positive timeout.
  - `services/bff-service/cmd/server/main.go`: Construct `inventory.NewClient(cfg.InventoryGRPCAddr, cfg.InventoryGRPCTimeout)` and pass it into `NewProductsHandler(catalogClient, inventoryClient)`.
  - `scripts/start-local/main.go`: Set default `BFF_INVENTORY_GRPC_ADDR=127.0.0.1:50052` and `BFF_INVENTORY_GRPC_TIMEOUT=3s` for `bff-service`.
  - `services/bff-service/Dockerfile` & `deployments/docker/compose.production.yml`: Set `BFF_INVENTORY_GRPC_ADDR="inventory-service:50052"` and `BFF_INVENTORY_GRPC_TIMEOUT="3s"`, and add `inventory-service` (`condition: service_healthy`) to `bff-service.depends_on` in `compose.production.yml` (updating `compose_production_test.go` and `docs/steering/service-catalog.md`).
- **Storefront Response Enrichment (`services/bff-service/internal/http/products.go`):**
  - Low-stock display threshold constant: `lowStockThreshold = 5`.
  - Additive fields on both `productSummary` and `productDetail`:
    - `Status string` (`json:"status"`): one of `"AVAILABLE"`, `"LIMITED"`, `"OUT_OF_STOCK"`, `"UNAVAILABLE"`, or `"UNKNOWN"`.
    - `AvailableQuantity *int32` (`json:"availableQuantity,omitempty"`): present when known (`0` for `UNAVAILABLE` and `OUT_OF_STOCK`, exact positive stock for `LIMITED` and `AVAILABLE`), omitted (`nil`) when `Status == "UNKNOWN"`.
    - `InventoryDegraded bool` (`json:"inventoryDegraded,omitempty"`): `true` only when `InventoryService.GetAvailability` fails or `inventoryClient` is nil.
  - Resolution rules:
    - **Detail (`GET /api/v1/storefront/products/{id}`):**
      - If `resp.GetDeleted()` or `len(resp.GetVariants()) == 0`: skip calling `InventoryService`; return `status: "UNAVAILABLE"`, `availableQuantity: 0`, `deleted: resp.GetDeleted()`.
      - Otherwise call `inventory.GetAvailability(ctx, id)`:
        - On error: log/degrade gracefully with `status: "UNKNOWN"`, `availableQuantity: nil`, `inventoryDegraded: true`.
        - `available <= 0`: `status: "OUT_OF_STOCK"`, `availableQuantity: 0`.
        - `1 <= available <= lowStockThreshold`: `status: "LIMITED"`, `availableQuantity: available`.
        - `available > lowStockThreshold`: `status: "AVAILABLE"`, `availableQuantity: available`.
    - **Search (`GET /api/v1/storefront/products`):**
      - Fetch page from `CatalogService.SearchProducts` (which already excludes soft-deleted rows via `deleted_at IS NULL`).
      - Query `InventoryService.GetAvailability` concurrently across unique product IDs on the page with bounded worker concurrency (`maxInventoryConcurrency = 8`), propagating incoming `x-trace-id` / request context cancellation.
      - Apply the same per-item resolution (`OUT_OF_STOCK`, `LIMITED`, `AVAILABLE`, or `UNKNOWN` + `inventoryDegraded: true` if an item's lookup fails).

### 2. End-to-End Product Lifecycle & Edge-Case Behavior (`ecommerce-fe`)

| Scenario | Listing (`HomePage` / `SearchPage` / Related) | `ProductDetailPage` (`/san-pham/:id`) | `CartPage` (`/gio-hang`) & `OrderPreviewPage` |
| --- | --- | --- | --- |
| **Available (`AVAILABLE`, `availableQuantity > 5`)** | Normal card, formatted rating (`4.9`), real image with emoji fallback. | Shows `"Còn hàng (X sản phẩm)"`; quantity stepper capped at `Math.min(10, availableQuantity)`. | Selectable, normal quantity controls, included in order preview. |
| **Low Stock (`LIMITED`, `1..5`)** | Subtle `"Chỉ còn X"` stock note on card. | Amber stock badge `"Sắp hết — Chỉ còn X sản phẩm"`; quantity stepper auto-clamped and capped at `availableQuantity`. | If cart `quantity > availableQuantity`, warns `"Chỉ còn X sản phẩm"` with `"Đặt về X"` quick-fix button; `OrderPreviewPage` also displays an inline warning if quantity exceeds `availableQuantity`. |
| **Out of Stock (`OUT_OF_STOCK`, `availableQuantity === 0` or missing stock row)** | `"Tạm hết hàng"` overlay badge on image; muted card image; remains clickable so shoppers can read specs and browse related items. | `"Tạm hết hàng"` badge; quantity stepper disabled; primary button disabled with label `"Tạm hết hàng"`, `"Mua ngay"` disabled; related products remain active below. | Auto-deselected, checkbox disabled, moved to `"Sản phẩm không khả dụng"` with `"Hết hàng"` label and one-click `"Xóa"` / `"Xóa sản phẩm không khả dụng"`. Excluded from `OrderPreviewPage`. |
| **Soft-Deleted / Discontinued (`deleted === true` or `variants.length === 0`, `UNAVAILABLE`)** | Automatically excluded by `CatalogService.SearchProducts`. | Accessible via direct link or cart item link; shows prominent `"Sản phẩm ngừng kinh doanh"` alert banner + badge; disables variant/quantity/purchase controls (`"Ngừng kinh doanh"`). | Auto-deselected, checkbox disabled, grouped under `"Sản phẩm không khả dụng"` with `"Không còn kinh doanh"` label. Excluded from `OrderPreviewPage`. |
| **Hard-Deleted / Unknown ID (`PRODUCT_NOT_FOUND` 404)** | Never returned by search. | Renders `"Không tìm thấy sản phẩm"` empty state with actions to return to `"Về trang chủ"` or `"Xem tất cả sản phẩm"`. | Orphan cart line (`item.name === ""`) falls back to `"Sản phẩm không còn tồn tại"`, `"📦"` placeholder, hides `0 ₫`, unlinks broken detail URL, and keeps `"Xóa"` functional. |
| **Mid-Action Race / Inventory Degraded (`UNKNOWN`)** | Renders normally without stock count badge when `status === "UNKNOWN"`. | Allows authenticated add-to-cart attempt (capped at `10`). If `!cart.degraded` and `CartService` resolves the matching line as `OUT_OF_STOCK` or `UNAVAILABLE`, suppresses the green add-to-cart mutation toast in `api.ts`, shows a specific error toast (`"Sản phẩm vừa tạm hết hàng"` / `"Sản phẩm đã ngừng kinh doanh"`), refreshes product detail, and aborts `Buy Now` navigation (leaving the line in `/gio-hang`'s `"Sản phẩm không khả dụng"` group per `CartService` design). If `cart.degraded` is `true` (inventory temporarily unreachable), `"Thêm vào giỏ"` treats the save as added without falsely claiming discontinuation or reloading `getProduct`, while `"Mua ngay"` aborts checkout with `"Tạm thời chưa kiểm tra được tồn kho, vui lòng kiểm tra giỏ hàng"`. | `CartPage` displays the existing amber `degraded` banner when `cart.degraded` is true. |

### 3. Storefront Shell, Routing, and Page-Level UI/UX Improvements

1. **Unified Routing Shell (`src/App.tsx`, `src/routes/require-auth.tsx`, `index.html`):**
   - Nest `/profile`, `/login`, and `/register` inside `<Route element={<StoreLayout />}>` alongside `/`, `/tim-kiem`, `/san-pham/:id`, `/gio-hang`, and `/gio-hang/dat-hang`.
   - Style `RequireAuth` session restoration (`!ready`) as a centered status card with spinner inside `<main>` while preserving the `"Đang khôi phục phiên..."` text.
   - Update `index.html` to `<html lang="vi">` and `<title>S-Shop — Mua sắm trực tuyến chính hãng</title>`.
2. **Header Navigation & Mobile Drawer (`src/components/layout/site-header.tsx`):**
   - Keep the account control accessible on all viewport widths: render the user dropdown trigger (or `/login` link when unauthenticated) on mobile as an icon button (`sr-only` text below `md`, visible label from `md` upward so existing accessible names remain unchanged), and include account links (`Hồ sơ` / `Đăng xuất` or `Đăng nhập` / `Đăng ký`) at the bottom of the mobile drawer.
   - Wrap both the mobile toggle button and mobile drawer inside `menuRef` (or exclude the toggle button from the document `pointerdown` outside-click handler) so tapping the menu button while open closes the drawer reliably, and close the drawer automatically on route changes.
   - Sync the search input value with URL `?q=` when navigating to/from `/tim-kiem`.
   - Apply active visual styling and `aria-current="page"` to `NAV_LINKS` matching the current pathname and category/sale query parameter.
3. **Auth Pages (`src/pages/login-page.tsx`, `src/pages/register-page.tsx`):**
   - Replace full-viewport `min-h-svh` wrappers with layout-friendly vertical centering (`py-12`) inside `StoreLayout`.
   - Display a confirmation notice on `LoginPage` when `location.state?.registered` is true (`"Đăng ký thành công! Vui lòng đăng nhập để tiếp tục."`).
   - Redirect already-authenticated users (`ready && isAuthenticated && !submitting`) away from `/login` and `/register` to their `from` target or `"/"`.
4. **Live `HomePage` (`src/pages/home-page.tsx`):**
   - Fetch products from `searchProducts({ sort: "pho-bien", pageSize: 20 })` with loading skeletons and an inline retry state on error, eliminating stale static product cards and soft-deleted items.
   - Derive hero showcase product, dynamic `"Giá chỉ từ"` callout (`formatVND`), Flash Sale items (`originalPrice > price`), category tabs (`type="button"`, `aria-pressed`), and best-sellers from the live catalog response.
   - Keep `"Xem tất cả"` links accessible on mobile as well as desktop, add `:` separators to the Flash Sale countdown badges, and adapt the bottom CTA banner for authenticated users (`"Khám phá ưu đãi hôm nay"` -> `/tim-kiem?giam-gia=1`).
5. **Product Card & Product Detail (`src/components/product-card.tsx`, `src/pages/product-detail-page.tsx`):**
   - Format ratings with one decimal place (`Number(product.rating).toFixed(1)`).
   - Remove `softDeletePending`, `handleSoftDelete`, and the `import.meta.env.DEV` soft-delete button block from `ProductDetailPage`.
   - Render full `product.description` inside the `"Chi tiết sản phẩm"` card when `product.shortDescription` is displayed in the summary box so full descriptions are never lost.
   - Wire `"Thêm vào yêu thích"` (local toggle state with `aria-pressed` and toast feedback) and `"Chia sẻ sản phẩm"` (clipboard / Web Share API with toast feedback).
6. **Search Page (`src/pages/search-page.tsx`):**
   - Preserve original user casing of `?q=` in the heading while trimming whitespace, and show `"Sản phẩm khuyến mãi"` when `giamGia` is active without `q` or `danh-muc`.
   - Synchronize `sliderValue` whenever `priceRangeParam` (`gia`) changes or filters are cleared (`[0, 10000000]` default, clamping unbounded max to slider max).
   - Show active filter count on the mobile `"Bộ lọc"` toggle button.
7. **Profile Page (`src/pages/profile-page.tsx`):**
   - Wrap `signOut` in `try / catch / finally` and navigate to `"/"` (consistent with `SiteHeader`).
   - Style `readOnly` `Email` input distinctly (`bg-muted/60 text-muted-foreground`) and pass `aria-invalid` to `SelectTrigger` (`gender`) and `SearchableSelect` (`province`, `ward`) when validation errors exist.
8. **Cart, Order Preview & Footer (`src/pages/cart-page.tsx`, `src/pages/order-preview-page.tsx`, `src/components/layout/site-footer.tsx`):**
   - `CartItemRow`: render `item.imageUrl` with `onError` fallback to `ProductImage`; handle hard-deleted items (`!item.name`) gracefully; disable `"Xóa đã chọn"` when `selectedItems.length === 0 || loading`; add `"Xóa tất cả"` action on the `"Sản phẩm không khả dụng"` group; render a skeleton layout during initial cart load.
   - Extract shared shipping constants (`FREE_SHIPPING_THRESHOLD`, `STANDARD_SHIPPING_FEE`) into `src/lib/shipping.ts` (re-exported by `cart-page.tsx` if needed).
   - `OrderPreviewPage`: display item thumbnails, unit price × quantity, low-stock warning if `item.status === "LIMITED" && item.quantity > item.availableQuantity`, and a shipping recipient summary card populated from `useAuth().user` with a quick link to `/profile` when phone or address is incomplete.
   - `SiteFooter`: convert dead `<span>` support items into navigable links (`/tim-kiem`, `/tim-kiem?giam-gia=1`, `/gio-hang`, `/profile`) or clearly labeled informational text without fake pointer styling.

### 4. What Is Left to the Builder

- Exact Tailwind spacing/color tokens within the existing Emerald/Stone palette in `src/index.css`.
- Internal helper decomposition (for example a small `src/lib/product-availability.ts` helper shared by `ProductCard` and `ProductDetailPage`).
- Exact skeleton placeholder counts on `HomePage` and `CartPage`, provided accessible `role="status"` labels are present.

## Verification Strategy

1. **`bff-service` unit & contract tests:**
   - `go test ./services/bff-service/...` covering `config.Load` (`BFF_INVENTORY_GRPC_ADDR` / `BFF_INVENTORY_GRPC_TIMEOUT`), `internal/inventory` client, `ProductsHandler` availability mapping (`AVAILABLE`, `LIMITED`, `OUT_OF_STOCK`, `UNAVAILABLE` for `deleted: true` and empty variants, `UNKNOWN` + `inventoryDegraded: true` on inventory failure), and `compose_production_test.go`.
2. **`scripts` tests:**
   - `go test ./scripts/...` verifying `start-local` sets `BFF_INVENTORY_GRPC_ADDR` and `BFF_INVENTORY_GRPC_TIMEOUT` defaults.
3. **`ecommerce-fe` unit, lint & build gates:**
   - `npm --prefix ecommerce-fe run test` (updating removed DEV soft-delete tests in `product-detail-page.test.tsx` and adding tests for `StoreLayout` wrapping `/profile`, mobile `SiteHeader` account access & toggle, `HomePage` live API states, `ProductCard` / `ProductDetailPage` availability states & rating formatting, `CartPage` `imageUrl` & hard-deleted item fallback, `OrderPreviewPage` recipient summary, `SearchPage` heading/slider sync, and `ProfilePage` `signOut` resilience).
   - `npm --prefix ecommerce-fe run lint` and `npm --prefix ecommerce-fe run build`.
4. **Repository surface gates:**
   - `make check-boundaries`
   - `make verify SERVICE=bff-service`
   - `make verify SERVICE=ecommerce-fe`

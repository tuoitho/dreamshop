# Variant SKU Pricing and Inventory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor product variants across `contracts`, `catalog-service`, `inventory-service`, `cart-service`, `bff-service`, `ecommerce-fe`, `admin-fe`, and the pending `order-service` checkout saga docs into persistent `sku_id` entities with independent per-SKU selling/original prices and per-SKU physical/reserved stock, completely removing legacy `product_id`/`variant_label` inventory and cart shims.

**Architecture:** `catalog-service` owns `product_variants` keyed by `sku_id` (`price_amount_minor`, `original_price_amount_minor`) and atomically derives representative `price_amount_minor`, `original_price_amount_minor`, and `max_price_amount_minor` onto `products` during writes so search/filter queries stay `JOIN`-free. `inventory-service` keys `stock` by `sku_id` (retaining indexed `product_id` metadata) and exposes pure SKU RPCs including `BatchGetAvailability` and two-phase `DeleteStocks(dry_run)`. `cart-service` keys `cart_items` by `(cart_id, sku_id)` and enriches cart/favorites in a single `BatchGetAvailability` round-trip, while `bff-service`, `ecommerce-fe`, and `admin-fe` expose and render per-SKU pricing and stock end-to-end.

**Tech Stack:** Protobuf v3 (`buf`), Go 1.24 (`pgx/v5`, `sqlc`, `grpc-go`, `net/http`), PostgreSQL 16 migrations, Java 21 / Maven (`contracts/java`, `order-service`), React 19 / TypeScript / Vite / Vitest (`ecommerce-fe`, `admin-fe`).

**Spec:** `docs/superpowers/specs/2026-11-02-variant-sku-pricing-and-inventory-design.md`

## Global Constraints

- Never introduce cross-service code imports, foreign `<other>_db` references, or internal calls through Kong (`:8000`); `make check-boundaries` must exit `0`.
- Remove legacy `product_id`-keyed inventory RPCs/routes, product-level input prices on `CreateProductRequest`/`UpdateProductRequest`, `variant_label` input on `AddItemRequest`/`cart_items`, and remove the `buf breaking` step from `scripts/verify-service/main.go` and `.github/workflows/ci.yml`.
- Preserve `sku_id` values on `UpdateProduct` via `INSERT ... ON CONFLICT (sku_id) DO UPDATE`; never wipe-and-reinsert `product_variants` by `product_id`.
- Enforce `sku_id` format `^[a-z0-9][a-z0-9-]{1,95}$`, `1..20` variants per product, case-insensitively unique variant `label` (`1..80` chars) within a product, `price_amount_minor > 0`, and `original_price_amount_minor >= price_amount_minor`.
- Enforce PostgreSQL stock invariants `CHECK (quantity >= 0)` and `CHECK (reserved >= 0 AND reserved <= quantity)` on `stock(sku_id)`.
- Preserve REST/JSON `camelCase` fields, `{ "data": <payload>, "message": "..." }` success envelopes, and `{ "code", "message", "errors", "traceId" }` error envelopes.
- Never hold a database transaction open across outbound gRPC or object-storage calls.
- On Windows PowerShell, test `cart-service` with `go test ./services/cart-service/internal/...` (or `make verify SERVICE=cart-service`) because `cmd/server` tests use Unix `syscall.Kill`.
- Files, comments, migrations, UI copy, and tests must read as permanent domain artifacts without prompt or request history phrasing.

## Review Focus

- Reordering or renaming retained variants on `UpdateProduct` (`[v1, v2]` -> `[v2, v1]`) must preserve each variant's `sku_id` without `(product_id, position)` key collisions -> pinned in Task 2 (`TestStoreUpdatePreservesAndReordersVariantSkuIDs`).
- Submitting a `sku_id` on `CreateProduct` or `UpdateProduct` that already belongs to another `product_id` must fail with `409 SKU_ALREADY_EXISTS` (`codes.AlreadyExists`, reason `SKU_ALREADY_EXISTS`) instead of hijacking the other product's row -> pinned in Task 2 (`TestStoreUpdateRejectsCrossProductSkuCollision`) and Task 5 (`TestAdminProductsUpdateSkuConflict`).
- Removing a variant whose SKU has `reserved > 0` must fail with `409 VARIANT_HAS_RESERVED_STOCK` before Catalog mutates, AND an invalid Catalog payload must not delete unreserved `stock` rows -> pinned in Task 3 (`TestDeleteUnreservedDryRunAndReservedRejection`) and Task 5 (`TestAdminProductsUpdateBlocksReservedVariantAndPreservesStockOnValidationFailure`).
- Searching `GET /api/v1/admin/inventory?q=<sku-or-label>` must return matching SKUs even when product name/brand do not contain `q`, while filtering out sibling SKUs of that product that do not match `q` -> pinned in Task 2 (`TestStoreListInventoryItemsMatchesSkuAndVariantLabel`) and Task 5 (`TestAdminInventoryListFiltersVariantBySkuQuery`).
- Opening `ProductDetailPage` when variant `0` is `OUT_OF_STOCK` and variant `1` is `AVAILABLE` must auto-select variant `1` while keeping variant `0`'s pill clickable so users can inspect out-of-stock variants without getting trapped -> pinned in Task 6 (`selects first purchasable variant and allows switching to out-of-stock variant`).

---

## File Structure

- **Contracts, ADR & Gate Tooling (Task 1)**
  - Create: `docs/adr/013-variant-sku-pricing-and-inventory.md`
  - Modify: `contracts/proto/catalog/v1/catalog.proto`, `contracts/proto/inventory/v1/inventory.proto`, `contracts/proto/cart/v1/cart.proto`, `contracts/proto/order/v1/order.proto`
  - Modify: `scripts/verify-service/main.go`, `scripts/verify-service/main_test.go`, `.github/workflows/ci.yml`
  - Regenerate: `contracts/go/**/*.pb.go`, `contracts/java/src/main/generated/**/*.java`
- **Catalog Service (Task 2)**
  - Create: `services/catalog-service/internal/store/migrations/000005_add_variant_sku_and_pricing.up.sql`
  - Modify: `services/catalog-service/internal/domain/product.go`, `services/catalog-service/internal/domain/admin.go`, `services/catalog-service/internal/domain/admin_test.go`, `services/catalog-service/internal/domain/search_test.go`
  - Modify: `services/catalog-service/internal/store/store.go`, `services/catalog-service/internal/store/admin.go`, `services/catalog-service/internal/store/seed.go`, `services/catalog-service/internal/store/seed_data.go`, `services/catalog-service/internal/store/seed_data_test.go`, `services/catalog-service/internal/store/store_test.go`
  - Modify: `services/catalog-service/internal/grpc/server.go`, `services/catalog-service/internal/grpc/admin.go`, `services/catalog-service/internal/grpc/server_test.go`, `services/catalog-service/internal/grpc/admin_test.go`
- **Inventory Service (Task 3)**
  - Create: `services/inventory-service/internal/store/migrations/000002_refactor_stock_to_sku.up.sql`
  - Modify: `services/inventory-service/internal/domain/stock.go`, `services/inventory-service/internal/domain/stock_test.go`
  - Modify: `services/inventory-service/internal/store/store.go`, `services/inventory-service/internal/store/seed.go`, `services/inventory-service/internal/store/store_test.go`
  - Modify: `services/inventory-service/internal/grpc/server.go`, `services/inventory-service/internal/grpc/admin.go`, `services/inventory-service/internal/grpc/server_test.go`, `services/inventory-service/internal/grpc/admin_test.go`
- **Cart Service (Task 4)**
  - Create: `services/cart-service/internal/store/migrations/000003_refactor_cart_items_to_sku.up.sql`
  - Modify: `services/cart-service/internal/domain/cart.go`, `services/cart-service/internal/store/queries/cart.sql`, `services/cart-service/internal/store/db/*` (via `sqlc generate`), `services/cart-service/internal/store/store.go`, `services/cart-service/internal/store/store_test.go`
  - Modify: `services/cart-service/internal/clients/inventory/client.go`, `services/cart-service/internal/clients/inventory/client_test.go`
  - Modify: `services/cart-service/internal/grpc/server.go`, `services/cart-service/internal/grpc/favorites.go`, `services/cart-service/internal/grpc/server_test.go`, `services/cart-service/internal/grpc/favorites_test.go`
- **BFF Service (Task 5)**
  - Modify: `services/bff-service/internal/catalog/client.go`, `services/bff-service/internal/catalog/client_test.go`
  - Modify: `services/bff-service/internal/inventory/client.go`, `services/bff-service/internal/inventory/client_test.go`
  - Modify: `services/bff-service/internal/cart/client.go`, `services/bff-service/internal/cart/client_test.go`
  - Modify: `services/bff-service/internal/http/products.go`, `services/bff-service/internal/http/cart.go`, `services/bff-service/internal/http/favorites.go`, `services/bff-service/internal/http/admin_products.go`, `services/bff-service/internal/http/admin_inventory.go`
  - Modify: `services/bff-service/internal/http/products_test.go`, `services/bff-service/internal/http/cart_test.go`, `services/bff-service/internal/http/favorites_test.go`, `services/bff-service/internal/http/admin_products_test.go`, `services/bff-service/internal/http/admin_inventory_test.go`
- **Storefront Frontend (Task 6)**
  - Modify: `ecommerce-fe/src/lib/types.ts`, `ecommerce-fe/src/lib/api.ts`, `ecommerce-fe/src/lib/product-availability.ts`, `ecommerce-fe/src/lib/product-availability.test.ts`, `ecommerce-fe/src/lib/api.test.ts`
  - Modify: `ecommerce-fe/src/context/cart-context.ts`, `ecommerce-fe/src/context/cart-provider.tsx`, `ecommerce-fe/src/context/cart-provider.test.tsx`
  - Modify: `ecommerce-fe/src/components/product-card.tsx`, `ecommerce-fe/src/pages/product-detail-page.tsx`, `ecommerce-fe/src/pages/product-detail-page.test.tsx`, `ecommerce-fe/src/pages/favorites-page.tsx`, `ecommerce-fe/src/pages/favorites-page.test.tsx`, `ecommerce-fe/src/pages/cart-page.tsx`, `ecommerce-fe/src/pages/cart-page.test.tsx`, `ecommerce-fe/src/pages/order-preview-page.tsx`, `ecommerce-fe/src/pages/order-preview-page.test.tsx`
- **Admin Frontend (Task 7)**
  - Modify: `admin-fe/src/lib/types.ts`, `admin-fe/src/lib/api.ts`, `admin-fe/src/lib/api.test.ts`
  - Modify: `admin-fe/src/pages/product-drawer.tsx`, `admin-fe/src/pages/product-detail-sheet.tsx`, `admin-fe/src/pages/products-page.tsx`, `admin-fe/src/pages/products-page.test.tsx`
  - Modify: `admin-fe/src/pages/stock-dialog.tsx`, `admin-fe/src/pages/inventory-page.tsx`, `admin-fe/src/pages/inventory-page.test.tsx`, `admin-fe/src/pages/overview-page.tsx`, `admin-fe/src/pages/overview-page.test.tsx`
- **Order Saga Docs, System Docs & Repo Gate (Task 8)**
  - Modify: `docs/superpowers/specs/2026-10-28-order-checkout-saga-design.md`, `docs/superpowers/plans/2026-10-28-order-checkout-saga.md`, `docs/instructions/build-and-test.md`, `docs/steering/service-catalog.md`, `services/catalog-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, `services/bff-service/AGENTS.md`

---

### Task 1: Pure-SKU Protobuf Contracts, ADR-013 & Contract Gate Update

**Files:**
- Create: `docs/adr/013-variant-sku-pricing-and-inventory.md`
- Modify: `scripts/verify-service/main.go:97-131`
- Modify: `scripts/verify-service/main_test.go:71-117`
- Modify: `.github/workflows/ci.yml:42-65`
- Modify: `contracts/proto/catalog/v1/catalog.proto`
- Modify: `contracts/proto/inventory/v1/inventory.proto`
- Modify: `contracts/proto/cart/v1/cart.proto`
- Modify: `contracts/proto/order/v1/order.proto`
- Regenerate: `contracts/go/**`, `contracts/java/src/main/generated/**`

**Interfaces:**
- Consumes: `ecommerce.common.v1.Money` (`contracts/proto/common/v1/common.proto`)
- Produces:
  - `catalogv1.ProductVariant{SkuId, Label, ColorHex, Price, OriginalPrice}`; `GetProductResponse.MaxPrice`; `ProductSummary.{MaxPrice, SkuIds}`; `AdminProductSummary.{MaxPrice, SkuIds}`; `InventoryCatalogItem.Variants`; `CreateProductRequest` & `UpdateProductRequest` without product-level `price`/`original_price`.
  - `inventoryv1.InventoryService`: `GetAvailability(sku_id)`, `BatchGetAvailability(sku_ids)`, `GetStock(user_id, sku_id)`, `BatchGetStock(user_id, sku_ids)`, `ListStocks(user_id)`, `SetStock(user_id, sku_id, product_id, quantity)`, `AdjustStock(user_id, sku_id, product_id, delta)`, `DeleteStocks(user_id, sku_ids, dry_run)`; `StockItem{SkuId, ProductId, Quantity, Reserved, AvailableQuantity, UpdatedAt}`; `AvailabilityItem{SkuId, ProductId, AvailableQuantity}`.
  - `cartv1.AddItemRequest{UserId, ProductId, SkuId, Quantity}`; `CartItem.{SkuId, OriginalPrice}`; `FavoriteVariant{SkuId, Label, ColorHex, Price, OriginalPrice}`; `FavoriteItem.MaxPrice`.
  - `orderv1.OrderItem{ProductId, SkuId, Quantity, UnitPrice}`.

- [ ] **Step 1: Update `TestVerifyServiceJavaAndFrontendAndContracts` in `scripts/verify-service/main_test.go` to expect contract verification without `buf breaking`**

```go
	want := []string{
		"mvn --batch-mode --no-transfer-progress -pl services/identity-service -am verify",
		"npm --prefix ecommerce-fe test -- --run",
		"npm --prefix ecommerce-fe run lint",
		"npm --prefix ecommerce-fe run build",
		"npm --prefix admin-fe test -- --run",
		"npm --prefix admin-fe run lint",
		"npm --prefix admin-fe run build",
		"buf format --diff --exit-code",
		"buf lint",
		"buf generate",
		"git diff --exit-code -- contracts/go contracts/java/src/main/generated",
		"git ls-files --others --exclude-standard -- contracts/go contracts/java/src/main/generated",
	}
```

- [ ] **Step 2: Run `scripts/verify-service` test to confirm it fails**

Run: `go test ./scripts/verify-service/...`
Expected: FAIL (`commands = [... git rev-parse ... buf breaking ...]`)

- [ ] **Step 3: Remove `buf breaking` from `scripts/verify-service/main.go` and `.github/workflows/ci.yml`, and write `docs/adr/013-variant-sku-pricing-and-inventory.md`**

Remove `git rev-parse --git-common-dir` and `buf breaking` from `case service == "contracts":` in `scripts/verify-service/main.go`, remove the `Check protobuf breaking changes against main` step in `.github/workflows/ci.yml`, and document the accepted SKU identity, per-variant pricing, and per-SKU stock ownership decision in `docs/adr/013-variant-sku-pricing-and-inventory.md`.

- [ ] **Step 4: Update `catalog.proto`, `inventory.proto`, `cart.proto`, and `order.proto` to the pure-SKU schemas in Spec §5.1, format, clean stale generated files, regenerate, and stage generated contracts**

Run:
```powershell
$gitRoot = Split-Path (Split-Path (Get-Command git).Source -Parent) -Parent
$env:PATH = "$gitRoot\usr\bin;$env:PATH"
buf format -w
Remove-Item -Recurse -Force contracts/java/src/main/generated/*
Get-ChildItem contracts/go -Recurse -Filter *.pb.go | Remove-Item -Force
buf generate
git add contracts/go contracts/java/src/main/generated
```

- [ ] **Step 5: Verify `scripts/verify-service` tests and `make verify SERVICE=contracts` pass**

Run: `go test ./scripts/verify-service/...; go run ./scripts/verify-service -service contracts`
Expected: PASS (`PASSED: contracts`)

- [ ] **Step 6: Commit Task 1**

```powershell
git add docs/adr/013-variant-sku-pricing-and-inventory.md scripts/verify-service/main.go scripts/verify-service/main_test.go .github/workflows/ci.yml contracts/proto contracts/go contracts/java/src/main/generated
git commit -m "feat(contracts)!: switch catalog, inventory, cart, and order contracts to pure SKU model"
```

---

### Task 2: Catalog Service — Variant SKU Identity, Per-Variant Pricing & Representative Price Sync

**Files:**
- Create: `services/catalog-service/internal/store/migrations/000005_add_variant_sku_and_pricing.up.sql`
- Modify: `services/catalog-service/internal/domain/product.go`
- Modify: `services/catalog-service/internal/domain/admin.go`
- Modify: `services/catalog-service/internal/domain/admin_test.go`
- Modify: `services/catalog-service/internal/domain/search_test.go`
- Modify: `services/catalog-service/internal/store/store.go`
- Modify: `services/catalog-service/internal/store/admin.go`
- Modify: `services/catalog-service/internal/store/seed.go`
- Modify: `services/catalog-service/internal/store/seed_data.go`
- Modify: `services/catalog-service/internal/store/seed_data_test.go`
- Modify: `services/catalog-service/internal/store/store_test.go`
- Modify: `services/catalog-service/internal/grpc/server.go`
- Modify: `services/catalog-service/internal/grpc/admin.go`
- Modify: `services/catalog-service/internal/grpc/server_test.go`
- Modify: `services/catalog-service/internal/grpc/admin_test.go`

**Interfaces:**
- Consumes: `catalogv1` pure-SKU generated types from Task 1.
- Produces:
  - `domain.Variant{SkuID string, Label string, ColorHex string, PriceMinor int64, OriginalPriceMinor int64}`
  - `domain.Product.MaxPriceMinor int64`; `domain.ProductSummary.{MaxPriceMinor int64, SkuIDs []string}`; `domain.AdminProductSummary.{MaxPriceMinor int64, SkuIDs []string}`; `domain.InventoryCatalogItem.Variants []Variant`
  - `domain.ErrSkuConflict` -> gRPC `codes.AlreadyExists` with `ErrorInfo{Reason: "SKU_ALREADY_EXISTS", Domain: "ecommerce.catalog.v1"}`
  - `ValidateAndPrepareCreate(p *domain.Product) error` & `ValidateAndPrepareUpdate(id string, p *domain.Product) error` deriving `PriceMinor`, `OriginalPriceMinor`, `MaxPriceMinor` from `p.Variants` and generating `<product_id>-<8-hex>` for empty `SkuID`.

- [ ] **Step 1: Write failing domain, store, seed, and gRPC tests in `catalog-service`**

Add tests covering:
- `internal/domain/admin_test.go`:
  - `TestValidateVariantsDerivesMinAndMaxProductPrices`: variants `[{Label: "512GB", PriceMinor: 20990000, OriginalPriceMinor: 22990000}, {Label: "256GB", PriceMinor: 18990000, OriginalPriceMinor: 21990000}]` sets `p.PriceMinor = 18990000`, `p.OriginalPriceMinor = 21990000`, `p.MaxPriceMinor = 20990000`, and generates valid `^[a-z0-9][a-z0-9-]{1,95}$` `SkuID`s when blank.
  - `TestValidateVariantsRejectsDuplicateLabelsInvalidPricesAndDuplicateSkus`: rejects case-insensitive duplicate labels (`"Đen"` vs `"đen"`), `PriceMinor <= 0`, `OriginalPriceMinor < PriceMinor`, and duplicate `SkuID`s.
- `internal/store/store_test.go`:
  - `TestStoreUpdatePreservesAndReordersVariantSkuIDs`: updating product variants `[v1, v2]` -> `[v2, v1, newVar]` keeps `v1.SkuID` and `v2.SkuID` intact in new positions `1` and `0`, deletes omitted variants, and updates `products.max_price_amount_minor`.
  - `TestStoreUpdateRejectsCrossProductSkuCollision`: passing a `SkuID` owned by product `p1` into `Store.Update` (or `Store.Create`) for `p2` returns `domain.ErrSkuConflict` without altering `p1`.
  - `TestStoreListInventoryItemsMatchesSkuAndVariantLabel`: `ListInventoryItems` with `Query: "dt-1-v3"` or a variant label returns the parent product with all `Variants` populated; `Search` and `ListAdmin` return `SkuIDs` in position order and `MaxPriceMinor`.
- `internal/grpc/admin_test.go` & `server_test.go`:
  - `CreateProduct` / `UpdateProduct` map variant `sku_id`, `price`, `original_price` and product `max_price`, and map `domain.ErrSkuConflict` to `codes.AlreadyExists` (`SKU_ALREADY_EXISTS`).

- [ ] **Step 2: Run `catalog-service` tests to verify they fail**

Run: `go test ./services/catalog-service/...`
Expected: FAIL (missing fields/migrations/methods)

- [ ] **Step 3: Add `000005_add_variant_sku_and_pricing.up.sql` and implement domain, store, seed, and gRPC changes in `catalog-service`**

Implement the exact SQL migration from Spec §5.2, update `domain.Variant`/`Product`/`ProductSummary`/`AdminProductSummary`/`InventoryCatalogItem`, implement variant validation + representative price derivation in `internal/domain/admin.go`, implement `ON CONFLICT (sku_id) DO UPDATE` + cross-product `sku_id` ownership check + `loadSkuIDsFor` + `loadVariantsFor` in `internal/store/admin.go` and `store.go`, assign deterministic `<id>-v1..N` SKUs and tiered configuration prices in `seed_data.go` + `seed.go`, and wire protobuf fields in `internal/grpc/server.go` and `admin.go`.

- [ ] **Step 4: Run `make verify SERVICE=catalog-service` to verify build, boundaries, and PostgreSQL/unit tests pass**

Run: `go run ./scripts/verify-service -service catalog-service`
Expected: PASS (`PASSED: catalog-service`)

- [ ] **Step 5: Commit Task 2**

```powershell
git add services/catalog-service
git commit -m "feat(catalog): add persistent variant SKU IDs and per-variant pricing"
```

---

### Task 3: Inventory Service — Pure SKU Stock Storage, Batch Availability & Safe Variant Deletion

**Files:**
- Create: `services/inventory-service/internal/store/migrations/000002_refactor_stock_to_sku.up.sql`
- Modify: `services/inventory-service/internal/domain/stock.go`
- Modify: `services/inventory-service/internal/domain/stock_test.go`
- Modify: `services/inventory-service/internal/store/store.go`
- Modify: `services/inventory-service/internal/store/seed.go`
- Modify: `services/inventory-service/internal/store/store_test.go`
- Modify: `services/inventory-service/internal/grpc/server.go`
- Modify: `services/inventory-service/internal/grpc/admin.go`
- Modify: `services/inventory-service/internal/grpc/server_test.go`
- Modify: `services/inventory-service/internal/grpc/admin_test.go`

**Interfaces:**
- Consumes: `inventoryv1` pure-SKU generated types from Task 1; Identity authorization gRPC (`inventory.view`, `inventory.update`, `product.update`).
- Produces:
  - `domain.Stock{SkuID, ProductID, Quantity, Reserved, UpdatedAt}`
  - `domain.VariantHasReservedStockError{SkuID string}` (`errors.As` target)
  - `Store.Get(ctx, skuID)`, `Store.BatchGet(ctx, skuIDs)`, `Store.List(ctx)`, `Store.SetQuantity(ctx, skuID, productID, quantity)`, `Store.AdjustQuantity(ctx, skuID, productID, delta)`, `Store.DeleteUnreserved(ctx, skuIDs, dryRun)`
  - `InventoryServiceServer`: `GetAvailability`, `BatchGetAvailability`, `GetStock`, `BatchGetStock`, `ListStocks`, `SetStock`, `AdjustStock`, `DeleteStocks`.

- [ ] **Step 1: Write failing domain, store, and gRPC tests in `inventory-service`**

Add tests covering:
- `internal/store/store_test.go`:
  - `TestStoreSkuSetAdjustAndBatchGetOrder`: `SetQuantity("dt-1-v1", "dt-1", 10)` and `AdjustQuantity("dt-1-v1", "dt-1", -3)` persist independent SKU balances; `BatchGet(ctx, []string{"missing-sku", "dt-1-v1"})` preserves request order with `Quantity: 0` default for missing rows.
  - `TestDeleteUnreservedDryRunAndReservedRejection`: `DeleteUnreserved(ctx, ["sku-free"], true)` succeeds without deleting `"sku-free"`; `DeleteUnreserved(ctx, ["sku-free"], false)` deletes `"sku-free"`; if `"sku-reserved"` has `reserved = 2`, both `dryRun = true` and `dryRun = false` return `*domain.VariantHasReservedStockError{SkuID: "sku-reserved"}` and leave all rows untouched.
- `internal/grpc/server_test.go` & `admin_test.go`:
  - `BatchGetAvailability` returns `AvailabilityItem`s in input order without requiring `user_id`.
  - `DeleteStocks` allows callers with `product.update` (even when `inventory.update` is `false`), denies callers lacking both (`INVENTORY_PERMISSION_DENIED`), and maps `VariantHasReservedStockError` to `codes.FailedPrecondition` with `ErrorInfo{Reason: "VARIANT_HAS_RESERVED_STOCK", Metadata: {"sku_id": "sku-reserved"}}`.

- [ ] **Step 2: Run `inventory-service` tests to verify they fail**

Run: `go test ./services/inventory-service/...`
Expected: FAIL (old `product_id` signatures and missing `DeleteStocks` / `BatchGetAvailability`)

- [ ] **Step 3: Add `000002_refactor_stock_to_sku.up.sql` and implement pure-SKU domain, store, seed, and gRPC handlers**

Implement `000002_refactor_stock_to_sku.up.sql` from Spec §5.3, update `domain.Stock` and `VariantHasReservedStockError`, implement `Get`, `BatchGet`, `List`, `SetQuantity`, `AdjustQuantity`, and `DeleteUnreserved(ctx, skuIDs, dryRun)` (`SELECT sku_id, reserved FROM stock WHERE sku_id = ANY($1) ORDER BY sku_id ASC FOR UPDATE`) in `internal/store/store.go`, update `internal/store/seed.go` with per-SKU initial balances matching `catalog-service` seed SKUs (including mixed in-stock/low-stock/out-of-stock variants on `dt-1` and all-zero on `dt-2`), and implement all 8 pure-SKU gRPC handlers in `internal/grpc/server.go` and `admin.go`.

- [ ] **Step 4: Run `make verify SERVICE=inventory-service` to verify build, boundaries, and tests pass**

Run: `go run ./scripts/verify-service -service inventory-service`
Expected: PASS (`PASSED: inventory-service`)

- [ ] **Step 5: Commit Task 3**

```powershell
git add services/inventory-service
git commit -m "feat(inventory)!: refactor stock storage and gRPC contracts to SKU granularity"
```

---

### Task 4: Cart Service — Pure `(cart_id, sku_id)` Persistence & Batch SKU Enrichment

**Files:**
- Create: `services/cart-service/internal/store/migrations/000003_refactor_cart_items_to_sku.up.sql`
- Modify: `services/cart-service/internal/domain/cart.go`
- Modify: `services/cart-service/internal/store/queries/cart.sql`
- Regenerate: `services/cart-service/internal/store/db/*` (via `sqlc generate`)
- Modify: `services/cart-service/internal/store/store.go`
- Modify: `services/cart-service/internal/store/store_test.go`
- Modify: `services/cart-service/internal/clients/inventory/client.go`
- Modify: `services/cart-service/internal/clients/inventory/client_test.go`
- Modify: `services/cart-service/internal/grpc/server.go`
- Modify: `services/cart-service/internal/grpc/favorites.go`
- Modify: `services/cart-service/internal/grpc/server_test.go`
- Modify: `services/cart-service/internal/grpc/favorites_test.go`

**Interfaces:**
- Consumes: `cartv1`, `catalogv1`, `inventoryv1` from Task 1.
- Produces:
  - `domain.Line{ID string, ProductID string, SkuID string, Quantity int32}`
  - `Store.AddItem(ctx context.Context, cartID, productID, skuID string, quantity int32) error`
  - `inventory.Client.BatchGetAvailability(ctx context.Context, skuIDs []string) ([]*inventoryv1.AvailabilityItem, error)`
  - `CartServiceServer`: `AddItem` keyed by `(product_id, sku_id)`, `GetCart` enriching `SkuId`, `VariantLabel`, `VariantColorHex`, `UnitPrice`, `OriginalPrice`, `AvailableQuantity`, `Status` via one `BatchGetAvailability` call, and `GetFavorites` summing active variant availability + mapping `MaxPrice` and per-variant `SkuId`, `Price`, `OriginalPrice`.

- [ ] **Step 1: Write failing store, client, cart, and favorites tests in `cart-service`**

Add tests covering:
- `internal/store/store_test.go`: adding `(cartID, "dt-1", "dt-1-v1", 1)` and `(cartID, "dt-1", "dt-1-v2", 2)` stores two distinct SKU lines; re-adding `(cartID, "dt-1", "dt-1-v1", 3)` increments `dt-1-v1` to `4`; empty `skuID` fails DB constraint.
- `internal/clients/inventory/client_test.go`: `BatchGetAvailability` sends `sku_ids` and returns `AvailabilityItem` slice.
- `internal/grpc/server_test.go`: `GetCart` with two variants of `dt-1` (`dt-1-v1` price `18990000` avail `5`, `dt-1-v3` price `23990000` avail `0`) makes **one** `BatchGetAvailability` call, returning `AVAILABLE` (`18990000`) for `dt-1-v1` and `OUT_OF_STOCK` (`23990000`) for `dt-1-v3`; if `sku_id` was removed from `product.Variants`, line becomes `UNAVAILABLE`; if Catalog fails transiently, `degraded = true` and `item.SkuId` is still populated.
- `internal/grpc/favorites_test.go`: `GetFavorites` sums availability across active product variants (`dt-1-v1: 2` + `dt-1-v2: 4` -> `AVAILABLE` with `available_quantity: 6`) via one `BatchGetAvailability` call and populates `max_price` and `FavoriteVariant.{SkuId, Price, OriginalPrice}`.

- [ ] **Step 2: Run `cart-service` internal tests to verify they fail**

Run: `go test ./services/cart-service/internal/...`
Expected: FAIL (missing `SkuID` fields and `BatchGetAvailability`)

- [ ] **Step 3: Add `000003_refactor_cart_items_to_sku.up.sql`, run `sqlc generate`, and implement store, inventory client, and gRPC enrichment**

Write `000003_refactor_cart_items_to_sku.up.sql` from Spec §5.4, update `internal/store/queries/cart.sql` (`ListItems` and `AddItem` on `(cart_id, sku_id)`), run `sqlc generate` inside `services/cart-service`, update `internal/store/store.go`, replace `GetAvailability(productID)` with `BatchGetAvailability(skuIDs)` in `internal/clients/inventory/client.go`, and update `internal/grpc/server.go` and `favorites.go` to batch SKU availability lookups.

- [ ] **Step 4: Run `make verify SERVICE=cart-service` to verify build, boundaries, and tests pass**

Run: `go run ./scripts/verify-service -service cart-service`
Expected: PASS (`PASSED: cart-service`)

- [ ] **Step 5: Commit Task 4**

```powershell
git add services/cart-service
git commit -m "feat(cart)!: key cart items by SKU and enrich pricing and availability per SKU"
```

---

### Task 5: BFF Service — Storefront, Cart, Favorites, 2-Phase Admin Variant Removal & SKU Inventory Routes

**Files:**
- Modify: `services/bff-service/internal/catalog/client.go`, `services/bff-service/internal/catalog/client_test.go`
- Modify: `services/bff-service/internal/inventory/client.go`, `services/bff-service/internal/inventory/client_test.go`
- Modify: `services/bff-service/internal/cart/client.go`, `services/bff-service/internal/cart/client_test.go`
- Modify: `services/bff-service/internal/http/products.go`, `services/bff-service/internal/http/products_test.go`
- Modify: `services/bff-service/internal/http/cart.go`, `services/bff-service/internal/http/cart_test.go`
- Modify: `services/bff-service/internal/http/favorites.go`, `services/bff-service/internal/http/favorites_test.go`
- Modify: `services/bff-service/internal/http/admin_products.go`, `services/bff-service/internal/http/admin_products_test.go`
- Modify: `services/bff-service/internal/http/admin_inventory.go`, `services/bff-service/internal/http/admin_inventory_test.go`

**Interfaces:**
- Consumes: `catalogv1`, `inventoryv1`, `cartv1` from Task 1.
- Produces REST JSON contracts (`/api/v1`):
  - `GET /api/v1/storefront/products`: `items[].{price, originalPrice, maxPrice, status, availableQuantity}` aggregated via one `BatchGetAvailability` over `sku_ids`.
  - `GET /api/v1/storefront/products/{id}`: `{..., maxPrice, variants: [{skuId, label, colorHex, price, originalPrice, status, availableQuantity}]}`.
  - `POST /api/v1/cart/items`: body `{productId, skuId, quantity}`; `GET /api/v1/cart`: `items[].{..., skuId, unitPrice, originalPrice}`.
  - `GET /api/v1/favorites`: `items[].{..., maxPrice, variants: [{skuId, label, colorHex, price, originalPrice}]}`.
  - `GET /api/v1/admin/products` & `GET /api/v1/admin/products/{id}`: `maxPrice`, product stock rollup over active `sku_ids`, and per-variant `stock` on detail.
  - `POST /api/v1/admin/products` & `PUT /api/v1/admin/products/{id}`: per-variant `{skuId?, label, colorHex?, price, originalPrice?}` (no product-level price input); `PUT` runs `DeleteStocks(dry_run=true)` pre-check (`409 VARIANT_HAS_RESERVED_STOCK`), then `Catalog.UpdateProduct` (`409 SKU_ALREADY_EXISTS` on `ErrSkuConflict`), then `DeleteStocks(dry_run=false)`.
  - `GET /api/v1/admin/inventory`, `PUT /api/v1/admin/inventory/skus/{skuId}` (`{productId, quantity}`), `POST /api/v1/admin/inventory/skus/{skuId}/adjust` (`{productId, delta}`).

- [ ] **Step 1: Write failing client and HTTP handler tests in `bff-service`**

Add/update tests covering:
- `catalog/client_test.go` & `inventory/client_test.go` & `cart/client_test.go`: `ErrSkuConflict` (`SKU_ALREADY_EXISTS`), `ErrVariantHasReservedStock` (`VARIANT_HAS_RESERVED_STOCK` with `SkuID`), `BatchGetAvailability`, `DeleteStocks(userID, skuIDs, dryRun)`, and `cart.Client.AddItem(ctx, userID, productID, skuID, quantity)`.
- `http/products_test.go`: storefront list calls `BatchGetAvailability` once across page `sku_ids` and returns `maxPrice`; storefront detail returns per-variant `skuId`, `price`, `originalPrice`, `status`, `availableQuantity` plus product-level summed availability.
- `http/cart_test.go` & `http/favorites_test.go`: `POST /api/v1/cart/items` rejects missing `skuId` (`400`) and forwards `skuId`; responses include `skuId`, `originalPrice`, `maxPrice`, and variant pricing.
- `http/admin_products_test.go`:
  - `TestAdminProductsUpdateSkuConflict`: maps `catalog.ErrSkuConflict` to `409 SKU_ALREADY_EXISTS`.
  - `TestAdminProductsUpdateBlocksReservedVariantAndPreservesStockOnValidationFailure`: removing a SKU with `reserved > 0` fails at `DeleteStocks(dryRun=true)` with `409 VARIANT_HAS_RESERVED_STOCK` before `UpdateProduct` is called; if `DeleteStocks(dryRun=true)` passes and `UpdateProduct` returns `InvalidArgument`, `DeleteStocks(dryRun=false)` is never called; when `UpdateProduct` succeeds, `DeleteStocks(dryRun=false)` cleans up removed SKUs.
- `http/admin_inventory_test.go`:
  - `GET /api/v1/admin/inventory` expands products into per-SKU rows with `totalTrackedProducts` and `totalTrackedSkus`.
  - `TestAdminInventoryListFiltersVariantBySkuQuery`: `?q=dt-1-v2` returns only variant `dt-1-v2` (not `dt-1-v1`), while `?q=Apex` returns all variants of `Apex`.
  - `PUT /api/v1/admin/inventory/skus/{skuId}` and `POST /api/v1/admin/inventory/skus/{skuId}/adjust` validate `skuId` belongs to `productId` (`404 SKU_NOT_FOUND` if mismatched) and mutate SKU stock; old `/api/v1/admin/inventory/{productId}` route is gone (`404`/`405`).

- [ ] **Step 2: Run `bff-service` tests to verify they fail**

Run: `go test ./services/bff-service/...`
Expected: FAIL

- [ ] **Step 3: Implement BFF clients and HTTP handlers (`products.go`, `cart.go`, `favorites.go`, `admin_products.go`, `admin_inventory.go`)**

Update `internal/catalog/client.go`, `internal/inventory/client.go`, and `internal/cart/client.go` to pure-SKU methods and sentinel errors; update storefront, cart, favorites, admin products (including 2-phase `DeleteStocks` around `UpdateProduct`), and admin inventory handlers (removing `/api/v1/admin/inventory/{productId}` routes and registering `/api/v1/admin/inventory/skus/{skuId}` routes).

- [ ] **Step 4: Run `make verify SERVICE=bff-service` to verify build, boundaries, and tests pass**

Run: `go run ./scripts/verify-service -service bff-service`
Expected: PASS (`PASSED: bff-service`)

- [ ] **Step 5: Commit Task 5**

```powershell
git add services/bff-service
git commit -m "feat(bff)!: adapt storefront, cart, and admin REST APIs to per-SKU pricing and stock"
```

---

### Task 6: Storefront Frontend (`ecommerce-fe`) — Price Range Cards, Dynamic Variant Selection & SKU Cart Flow

**Files:**
- Modify: `ecommerce-fe/src/lib/types.ts`
- Modify: `ecommerce-fe/src/lib/api.ts`, `ecommerce-fe/src/lib/api.test.ts`
- Modify: `ecommerce-fe/src/lib/product-availability.ts`, `ecommerce-fe/src/lib/product-availability.test.ts`
- Modify: `ecommerce-fe/src/context/cart-context.ts`, `ecommerce-fe/src/context/cart-provider.tsx`, `ecommerce-fe/src/context/cart-provider.test.tsx`
- Modify: `ecommerce-fe/src/components/product-card.tsx`
- Modify: `ecommerce-fe/src/pages/product-detail-page.tsx`, `ecommerce-fe/src/pages/product-detail-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/favorites-page.tsx`, `ecommerce-fe/src/pages/favorites-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/cart-page.tsx`, `ecommerce-fe/src/pages/cart-page.test.tsx`
- Modify: `ecommerce-fe/src/pages/order-preview-page.tsx`, `ecommerce-fe/src/pages/order-preview-page.test.tsx`

**Interfaces:**
- Consumes: BFF REST `/api/v1/storefront/products*`, `/api/v1/cart*`, `/api/v1/favorites*` from Task 5.
- Produces:
  - `resolveVariantAvailability(product: { deleted?: boolean }, variant?: ProductVariant): ProductAvailabilityView` in `src/lib/product-availability.ts`
  - `addCartItem(payload: { productId: string; skuId: string; quantity: number }): Promise<CartData>` in `src/lib/api.ts`
  - `CartContextValue.addItem: (productId: string, skuId: string, quantity: number) => Promise<void>` in `src/context/cart-context.ts`

- [ ] **Step 1: Write failing Vitest unit and component tests in `ecommerce-fe`**

Add/update tests covering:
- `src/lib/product-availability.test.ts`: `resolveVariantAvailability` returns `UNAVAILABLE` when `product.deleted` is `true`, `OUT_OF_STOCK` (`purchasable: false`, `maxQuantity: 0`) when `variant.status === "OUT_OF_STOCK"`, and `LIMITED` (`maxQuantity: 3`) when `variant.availableQuantity === 3`.
- `src/pages/product-detail-page.test.tsx`: `selects first purchasable variant and allows switching to out-of-stock variant` — when `variants[0]` (`18.990.000₫`, `OUT_OF_STOCK`, `availableQuantity: 0`) and `variants[1]` (`20.990.000₫`, `AVAILABLE`, `availableQuantity: 5`) load, `variants[1]` is auto-selected showing `20.990.000`, clicking `Thêm vào giỏ` sends `skuId: variants[1].skuId`, and clicking `variants[0]` updates the price to `18.990.000` and disables `Thêm vào giỏ` (`Tạm hết hàng`) while `variants[1]` remains clickable to switch back.
- `src/context/cart-provider.test.tsx`, `src/pages/favorites-page.test.tsx`, `src/pages/cart-page.test.tsx`, `src/pages/order-preview-page.test.tsx`: `addItem(productId, skuId, quantity)` sends `{ productId, skuId, quantity }`, `ProductCard`/`FavoritesPage` show `Từ` when `maxPrice > price`, and `CartPage`/`OrderPreviewPage` render strikethrough `originalPrice` when `originalPrice > unitPrice`.

- [ ] **Step 2: Run `ecommerce-fe` tests to verify they fail**

Run: `npm --prefix ecommerce-fe test -- --run`
Expected: FAIL

- [ ] **Step 3: Implement TypeScript types, `resolveVariantAvailability`, `CartProvider.addItem`, `ProductCard`, `ProductDetailPage`, `FavoritesPage`, `CartPage`, and `OrderPreviewPage`**

Update `src/lib/types.ts` and `src/lib/api.ts`, implement `resolveVariantAvailability` in `src/lib/product-availability.ts`, update `CartContext` and `CartProvider` to `(productId, skuId, quantity)`, render `Từ` prefix on `ProductCard` and `FavoritesPage` when `maxPrice > price`, update `ProductDetailPage` to auto-select the first purchasable variant while keeping all variant buttons clickable unless `product.deleted` and dynamically binding price/discount/availability/quantity cap to `selectedVariant`, and display `originalPrice` on `CartPage` and `OrderPreviewPage`.

- [ ] **Step 4: Run `make verify SERVICE=ecommerce-fe` to verify tests, ESLint, TypeScript, and Vite build pass**

Run: `go run ./scripts/verify-service -service ecommerce-fe`
Expected: PASS (`PASSED: ecommerce-fe`)

- [ ] **Step 5: Commit Task 6**

```powershell
git add ecommerce-fe
git commit -m "feat(ecommerce-fe)!: support per-variant SKU pricing, availability, and cart actions"
```

---

### Task 7: Admin Frontend (`admin-fe`) — Structured Variant Pricing Drawer & SKU Inventory Console

**Files:**
- Modify: `admin-fe/src/lib/types.ts`
- Modify: `admin-fe/src/lib/api.ts`, `admin-fe/src/lib/api.test.ts`
- Modify: `admin-fe/src/pages/product-drawer.tsx`
- Modify: `admin-fe/src/pages/product-detail-sheet.tsx`
- Modify: `admin-fe/src/pages/products-page.tsx`, `admin-fe/src/pages/products-page.test.tsx`
- Modify: `admin-fe/src/pages/stock-dialog.tsx`
- Modify: `admin-fe/src/pages/inventory-page.tsx`, `admin-fe/src/pages/inventory-page.test.tsx`
- Modify: `admin-fe/src/pages/overview-page.tsx`, `admin-fe/src/pages/overview-page.test.tsx`

**Interfaces:**
- Consumes: BFF REST `/api/v1/admin/products*` and `/api/v1/admin/inventory*` from Task 5.
- Produces:
  - `ProductVariant{skuId?: string; label: string; colorHex?: string; price: number; originalPrice?: number; stock?: AdminStockSnapshot | null}` in `admin-fe/src/lib/types.ts`
  - `AdminProductMutationPayload` (without root `price`/`originalPrice`, carrying `variants: ProductVariant[]`)
  - `setSkuStock(skuId: string, productId: string, quantity: number): Promise<StockMutationResult>` & `adjustSkuStock(skuId: string, productId: string, delta: number): Promise<StockMutationResult>` in `admin-fe/src/lib/api.ts`

- [ ] **Step 1: Write failing Vitest API and page tests in `admin-fe`**

Add/update tests covering:
- `src/lib/api.test.ts`: `setSkuStock("dt-1-v1", "dt-1", 25)` calls `PUT /admin/inventory/skus/dt-1-v1` with `{ productId: "dt-1", quantity: 25 }`; `adjustSkuStock("dt-1-v1", "dt-1", -3)` calls `POST /admin/inventory/skus/dt-1-v1/adjust` with `{ productId: "dt-1", delta: -3 }`.
- `src/pages/products-page.test.tsx`:
  - Creating a multi-variant product in `ProductDrawer` submits per-variant `{ label, colorHex, price, originalPrice }` (no root `price`) and calls `setSkuStock` for each new variant with `initialStock > 0` (preserving multi-SKU retry state if `setSkuStock` fails).
  - Editing an existing product preserves `skuId` on retained variants, disables variant deletion when `variant.stock.reserved > 0`, and `ProductDetailSheet` renders per-SKU price and stock breakdown.
- `src/pages/inventory-page.test.tsx` & `src/pages/overview-page.test.tsx`: renders SKU rows (`skuId`, `variantLabel`, `price`, `availableQuantity`), opens `StockDialog` for a specific SKU, and deduplicates low-stock alerts on `OverviewPage` by `skuId`.

- [ ] **Step 2: Run `admin-fe` tests to verify they fail**

Run: `npm --prefix admin-fe test -- --run`
Expected: FAIL

- [ ] **Step 3: Implement `admin-fe` types, API client, `ProductDrawer`, `ProductDetailSheet`, `ProductsPage`, `StockDialog`, `InventoryPage`, and `OverviewPage`**

Update `src/lib/types.ts` and `src/lib/api.ts` (replacing `setProductStock`/`adjustProductStock` with `setSkuStock`/`adjustSkuStock`), build the structured variant editor with representative price preview and multi-SKU `pendingStockRetry` in `src/pages/product-drawer.tsx`, render range prices on `src/pages/products-page.tsx` and SKU tables in `src/pages/product-detail-sheet.tsx`, and update `src/pages/inventory-page.tsx`, `src/pages/stock-dialog.tsx`, and `src/pages/overview-page.tsx` to operate on `(skuId, productId)`.

- [ ] **Step 4: Run `make verify SERVICE=admin-fe` to verify tests, ESLint, TypeScript, and Vite build pass**

Run: `go run ./scripts/verify-service -service admin-fe`
Expected: PASS (`PASSED: admin-fe`)

- [ ] **Step 5: Commit Task 7**

```powershell
git add admin-fe
git commit -m "feat(admin-fe)!: manage per-variant SKU pricing and SKU-level inventory"
```

---

### Task 8: Order Checkout Saga Spec/Plan Sync, Documentation & Full Repository Verification

**Files:**
- Modify: `docs/superpowers/specs/2026-10-28-order-checkout-saga-design.md`
- Modify: `docs/superpowers/plans/2026-10-28-order-checkout-saga.md`
- Modify: `docs/instructions/build-and-test.md`
- Modify: `docs/steering/service-catalog.md`
- Modify: `services/catalog-service/AGENTS.md`, `services/inventory-service/AGENTS.md`, `services/cart-service/AGENTS.md`, `services/bff-service/AGENTS.md`

**Interfaces:**
- Consumes: Finalized contracts and routes from Tasks 1–7.
- Produces: Synchronized `order-service` checkout saga spec & plan keyed on `sku_id` (`ORDER BY sku_id ASC`, `stock_reservation_items(order_id, sku_id)`, `order_items.sku_id`, per-SKU snapshot validation, `ADR-014`), updated developer/service docs, and passing Java/Go/Frontend/Boundary gates across the monorepo.

- [ ] **Step 1: Update `2026-10-28-order-checkout-saga-design.md` and `2026-10-28-order-checkout-saga.md` to the SKU model in Spec §5.8**

Update `ReservationLine` (`sku_id`, `product_id`, `quantity`), `stock_reservation_items(order_id, sku_id)` PK, `ReserveStock` / `OrderConfirmed` / `ReservationReleaseRequested` locking order (`ORDER BY sku_id ASC` without collapsing distinct variants onto `product_id`), `INVENTORY_INSUFFICIENT` metadata (`sku_id`, `product_id`, `available`, `requested`), `order_items.sku_id`, per-SKU Catalog snapshot price/label verification, and shift the planned saga ADR reference from `ADR-013` to `ADR-014`.

- [ ] **Step 2: Synchronize `docs/instructions/build-and-test.md`, `docs/steering/service-catalog.md`, and service `AGENTS.md` files**

Remove stale `buf breaking` command references in `docs/instructions/build-and-test.md` and update RPC/route/schema summaries in `docs/steering/service-catalog.md` and the four Go service `AGENTS.md` files to reflect pure SKU contracts and `/api/v1/admin/inventory/skus/{skuId}`.

- [ ] **Step 3: Run Java (`order-service`), Contracts, Go, Frontend, and Boundary gates across the repository**

Run:
```powershell
go run ./scripts/check-boundaries
go run ./scripts/verify-service -service contracts
mvn --batch-mode --no-transfer-progress -pl services/order-service -am test
go test ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/internal/...
go run ./scripts/verify-service -service ecommerce-fe
go run ./scripts/verify-service -service admin-fe
```
Expected: Exit code `0` on every command.

- [ ] **Step 4: Commit Task 8 (excluding unrelated pre-existing untracked files such as `docs/superpowers/plans/rv.md`)**

```powershell
git add docs/superpowers/specs/2026-10-28-order-checkout-saga-design.md docs/superpowers/plans/2026-10-28-order-checkout-saga.md docs/instructions/build-and-test.md docs/steering/service-catalog.md services/catalog-service/AGENTS.md services/inventory-service/AGENTS.md services/cart-service/AGENTS.md services/bff-service/AGENTS.md
git commit -m "docs: align checkout saga spec and service docs with SKU pricing and inventory"
```

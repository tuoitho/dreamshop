# ADR-013: Variant SKU identity, per-variant pricing, and SKU-level inventory

## Status

Accepted

## Context

Previously, `ProductVariant` in `catalog-service` only carried display labels (`label`, `color_hex`) without a stable identifier or independent price, `inventory-service` keyed `stock` by `product_id`, and `cart-service` keyed `cart_items` by `(cart_id, product_id, variant_label)`. Consequently, configurations with different capacities or finishes had to share a single product price and draw from one pooled stock row, while editing a product wiped and reinserted `product_variants`.

## Decision

1. **`catalog-service` owns SKU identity and per-variant pricing**:
   - `product_variants` uses `sku_id` as its primary key (`^[a-z0-9][a-z0-9-]{1,95}$`) and stores mandatory `price_amount_minor > 0` and `original_price_amount_minor >= price_amount_minor`.
   - `UpdateProduct` upserts variants in place by `sku_id` (`INSERT ... ON CONFLICT (sku_id) DO UPDATE`) and rejects cross-product `sku_id` collisions with `SKU_ALREADY_EXISTS`.
   - On every catalog write, `catalog-service` derives representative `price_amount_minor`, `original_price_amount_minor` (from the lowest-priced variant), and `max_price_amount_minor` onto `products` in the same transaction so search, price filtering, and sorting stay `JOIN`-free.
2. **`inventory-service` owns SKU-level stock and reservations**:
   - `stock` uses `sku_id` as its primary key (retaining indexed `product_id` metadata) and enforces `CHECK (quantity >= 0)` and `CHECK (reserved >= 0 AND reserved <= quantity)`.
   - All `InventoryService` RPCs operate on `sku_id`, including `BatchGetAvailability` for single-round-trip read enrichment and `DeleteStocks(dry_run)` (`product.update` or `inventory.update`) to block removing variants with `reserved > 0` (`VARIANT_HAS_RESERVED_STOCK`) before catalog mutation and delete unreserved rows afterward.
3. **`cart-service` and downstream commerce contracts key lines by `sku_id`**:
   - `cart_items` enforces `UNIQUE (cart_id, sku_id)` without persisting redundant `variant_label`, enriching label, color, selling/original price, and availability per SKU over private gRPC.
   - `bff-service`, `ecommerce-fe`, `admin-fe`, and `order.v1` operate directly on `sku_id` without legacy `product_id` inventory shims.

## Consequences

- **Benefits**: Independent pricing and physical/reserved stock per variant, single-RPC batch availability lookups per page/cart, preserved SKU links across product edits, and `O(1)` catalog search filtering without runtime `JOIN`s.
- **Costs & trade-offs**:
  - Removing a variant from a product requires a two-step coordination in `bff-service` (`DeleteStocks(dry_run=true)` pre-check -> `CatalogService.UpdateProduct` -> `DeleteStocks(dry_run=false)` cleanup) rather than a foreign-key cascade across separate service databases.
  - Because this pre-production cutover removes legacy `product_id` inventory RPCs and reuses `v1` protobuf wire tags with `sku_id` types, mixed pre-cutover and post-cutover `v1` instances cannot run side-by-side; `catalog-service`, `inventory-service`, `cart-service`, `bff-service`, `ecommerce-fe`, and `admin-fe` must deploy together in a single coordinated cutover alongside their database migrations.
  - `buf breaking` was removed during this cutover branch because `main` still carried the pre-SKU `v1` schemas; once this pure-SKU baseline lands on `main`, subsequent additive changes (such as `ADR-014` Order Checkout Saga) must preserve wire compatibility and may re-enable `buf breaking` against the updated `main` baseline.

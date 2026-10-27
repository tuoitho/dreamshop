# ADR-010: Host customer product favorites inside cart-service

**Status**: Accepted — 2026-10-27

## Context
Authenticated customers need a synchronized product favorites list with live price, discontinuation, and stock enrichment plus `CUSTOMER` RBAC enforcement (`favorite.manage`). `cart-service` already owns per-customer shopping lists and maintains private gRPC clients to Catalog, Inventory, and Identity Authorization.

## Decision
Extend `cart-service` (`cart_db`) with `favorite_items (id, user_id, product_id, created_at)` and additive `CartService` favorite RPCs exposed via `bff-service` at `/api/v1/favorites`. Do not create a separate `favorite-service` or store per-user favorites in `catalog_db`.

## Consequences
+ Reuses existing `BFF -> Cart -> Catalog/Inventory/Identity` private gRPC topology without adding a seventh service, database, or container.
− `cart-service` covers both cart lines and saved favorite products until scale or workflow divergence justifies splitting.
→ Do not move per-user favorites into `catalog-service` or spin up a standalone `favorite-service` without a superseding ADR.

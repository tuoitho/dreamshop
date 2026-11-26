# Admin Dashboard UI Upgrade Design (`admin-fe`)

**Date:** 2026-11-25
**Scope:** Single-surface frontend upgrade (`admin-fe/`). No backend, protobuf, Kong routing, or REST contract changes.

## 1. Intent & Why

The current `admin-fe` console uses a heavy dark-forest-green sidebar (`#0f1f1a`) and flat muted-green cards, and the Overview page omits live Order metrics even though `listAdminOrders` already exposes `statusCounts`, `totalItems`, and recent orders. This upgrade refreshes `admin-fe` into a bright, high-clarity light-mode SaaS operations console inspired by modern dashboards (Zenith / Rhombus references), while preserving 100% real API data, strict RBAC permission gating, and existing workflows.

## 2. Goals, Non-Goals & Anti-Goals

- **Goals:**
  - Upgrade shared design tokens (`index.css`) and UI primitives (`Card`, `Badge`, `Table`, `Button`) so every page in `admin-fe` automatically inherits cleaner contrast, soft elevation, and typography.
  - Redesign `AdminShell` with a bright, collapsible light sidebar, refined topbar with permission-aware Quick Jump navigation, and bottom operator profile footer.
  - Redesign `OverviewPage` with a personalized operator header, richer KPI cards (tinted icon badges + real ratio/progress bars), live **Order Status Distribution** donut chart & **Recent Orders** card (from `listAdminOrders`), upgraded **Category Distribution** bar chart, **Stock Health** breakdown, and the **Low-Stock SKU Watchlist** with inline `StockDialog`.
- **Non-Goals:**
  - Rewriting the internal layouts/toolbars of `/products`, `/inventory`, `/orders`, `/users`, `/rbac`, or `/login` (they inherit the new theme and primitives).
  - Adding backend time-series revenue endpoints or modifying any Go/Java service.
- **Anti-Goals:**
  - Never invent fake/hardcoded revenue trends, sparkline numbers, or mock percentages. Every displayed metric must derive directly from existing API responses.
  - Never weaken RBAC UI gating (`can(...)`) or break partial-failure resilience when one upstream service fails (`Promise.allSettled`).

## 3. Approaches Considered

1. **Approach 1 (Selected): Shared Design System + `AdminShell` + Full `OverviewPage` Redesign**
   - Updates CSS tokens, shared UI primitives, `AdminShell`, `BrandLogo`, and `OverviewPage` (+ tests).
   - *Trade-off:* Delivers immediate visual transformation on the dashboard and lifts all sub-pages consistently through shared primitives, with low regression risk and zero backend changes.
2. **Approach 2 (Deferred): Approach 1 + Full Redesign of All 6 Sub-Pages & Login**
   - *Trade-off:* ~3x larger scope, high churn across CRUD page tests; better done incrementally after locking in the new design system.

## 4. Visual Design System

### Color Tokens
- **Canvas:** `#F6F7F9` (Alabaster Slate)
- **Surface / Card:** `#FFFFFF` with border `#E6E8EC` and soft elevation shadow
- **Sidebar Surface:** `#FCFCFD` with border `#EAECF0`, active pill `#F1F5F9` / text `#0F172A`
- **Primary Action:** `#18181B` (Carbon Ink, hover `#27272A`)
- **Brand / Commerce Accents:**
  - Teal (`#0D9488`, soft `#F0FDFA`) — catalog & healthy inventory
  - Signal Orange (`#EA580C`, soft `#FFF7ED`) — orders & fulfillment
  - Cobalt (`#2563EB`, soft `#EFF6FF`) — active users
  - Violet (`#7C3AED`, soft `#F5F3FF`) — RBAC governance
  - Amber (`#D97706`, soft `#FFFBEB`) — low stock / pending
  - Crimson (`#DC2626`, soft `#FEF2F2`) — out of stock / cancelled / destructive

### Typography
- **Headings:** `Plus Jakarta Sans` (clean geometric sans with Vietnamese support)
- **Body / UI:** `Be Vietnam Pro`
- **Numeric / Codes / SKUs:** `JetBrains Mono` with `tabular-nums`

### Layout & Signature
- **Shell:** Collapsible light sidebar (`256px` / `72px` desktop, drawer on mobile) + sticky `64px` topbar with breadcrumb, permission-filtered Quick Jump search, and contextual primary action button.
- **Overview Signature:** Asymmetric 12-column bento grid combining live multi-domain health (Products, Orders, Inventory, Users/RBAC) with real ratio bars, custom-styled Recharts tooltips, a centered-total Order Status donut, and actionable Recent Orders + Low-Stock SKU Watchlist panels.

## 5. Surface Map & Verification Plan (`fullstack-feature`)

| Surface | Impact Classification | Owner Paths | Gate |
| --- | --- | --- | --- |
| `admin-fe` | Internal UI / presentation & client-side composition | `admin-fe/index.html`, `admin-fe/src/index.css`, `admin-fe/src/components/layout/*`, `admin-fe/src/components/ui/{card,badge,table,button}.tsx`, `admin-fe/src/pages/overview-page.tsx`, `admin-fe/src/pages/overview-page.test.tsx`, `admin-fe/src/components/layout/admin-shell.test.tsx` | `make verify SERVICE=admin-fe` |

- **Dependency Order:** `admin-fe` only (no contract, provider, or consumer changes).
- **Verification Mode Lock:** **Mode 2 (Unit/Component Vitest + `make verify SERVICE=admin-fe`)**. Existing unit/slice tests cover RBAC gating, partial upstream failure (`503`), order status aggregation, SKU deduplication, and `StockDialog` mutation; `make verify SERVICE=admin-fe` verifies boundaries, lint, TypeScript build, and full test suite.
- **Review Requirement:** Independent read-only `reviewing-code` subagent before completion (`reviewing-service-boundaries` not required since only `admin-fe` UI changes without touching contracts, services, persistence, or routing).

## 6. Acceptance Criteria

1. `AdminShell` renders the light collapsible sidebar (desktop collapse toggle + mobile drawer), grouped permission-gated navigation, topbar breadcrumb + Quick Jump route search, and bottom operator profile + logout action.
2. `OverviewPage` displays personalized greeting, permission-gated KPI cards with real ratio indicators, Category bar chart, **Order Status distribution** donut + **Recent Orders** list (when `can("order.admin.view")`), **Stock Health** breakdown, and **Low-Stock SKU Watchlist** with working `StockDialog` adjustment.
3. All numbers come strictly from live API responses; unauthorized endpoints are never called; partial API failures (`503`) preserve fulfilled panels while showing an alert banner.
4. `npm --prefix admin-fe run test`, `lint`, `build`, and `make verify SERVICE=admin-fe` exit `0`.

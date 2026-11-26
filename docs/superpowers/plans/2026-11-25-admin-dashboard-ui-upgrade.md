# Admin Dashboard UI Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade `admin-fe` into a bright, refined light-mode SaaS operations console by updating shared design tokens/primitives, redesigning `AdminShell` (collapsible light sidebar + Quick Jump topbar), and rebuilding `OverviewPage` with live Order insights alongside Catalog, Inventory, Users, and RBAC data.

**Architecture:** Single-surface frontend upgrade (`admin-fe/`). Shared CSS tokens (`index.css`) and UI primitives (`Card`, `Badge`, `Table`, `Button`) automatically elevate all existing routes while `AdminShell` and `OverviewPage` receive full layout, visual hierarchy, and data-visualization upgrades using existing REST APIs (`listAdminProducts`, `listAdminInventory`, `listAdminOrders`, `listAdminUsers`, `listRbacItems`).

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, Radix UI, Recharts, Lucide React, Vitest + Testing Library.

**Spec:** `docs/superpowers/specs/2026-11-25-admin-dashboard-ui-upgrade-design.md`

## Global Constraints

- Single surface: `admin-fe/` only. No backend, protobuf, Kong, or shared contract edits.
- 100% live API data: never fabricate revenue time-series, sparklines, or hardcoded percentages.
- Strict RBAC gating via `can(...)`: unauthorized endpoints must never be called.
- Partial failure resilience: `Promise.allSettled` must preserve fulfilled KPIs/panels if one endpoint returns an error (e.g., `503`).
- Gate: `make verify SERVICE=admin-fe` (boundary check + `oxlint` + `tsc -b && vite build` + `vitest run`).

## Review Focus

1. **Permission-gated order fetch:** Operators without `order.admin.view` must not trigger `listAdminOrders` and must not see broken/empty order cards.
2. **Partial upstream failure (`503`):** If `listAdminOrders` or `listAdminProducts` rejects, fulfilled panels (inventory, users, RBAC) still render alongside an alert banner.
3. **Zero-division & empty-state safety:** When `totalItems === 0`, `trackedSkus === 0`, or `statusCounts` is empty, ratio bars and donut percentages render `0%` / clean empty states without `NaN`.
4. **Sidebar collapse & Quick Jump accessibility:** Collapse toggle updates `aria-expanded` / layout cleanly; Quick Jump filters only permitted routes and navigates via click or Enter.
5. **SKU watchlist deduplication & `StockDialog`:** Multiple low/out-of-stock variants of the same product remain distinct rows by `skuId` and submit `setSkuStock` for the exact clicked SKU.

---

### Task 1: Design System Tokens & Shared UI Primitives

**Files:**
- Modify: `admin-fe/index.html`
- Modify: `admin-fe/src/index.css`
- Modify: `admin-fe/src/components/ui/card.tsx`
- Modify: `admin-fe/src/components/ui/badge.tsx`
- Modify: `admin-fe/src/components/ui/table.tsx`
- Modify: `admin-fe/src/components/ui/button.tsx`

- [ ] Load `Plus Jakarta Sans` alongside `Be Vietnam Pro` and `JetBrains Mono` in `admin-fe/index.html`.
- [ ] Update `:root` and `@theme inline` tokens in `admin-fe/src/index.css` to the light-mode Alabaster Slate (`#F6F7F9`), Porcelain Sidebar (`#FCFCFD`), Carbon Primary (`#18181B`), and refined semantic accents.
- [ ] Polish `Card`, `Badge`, `Table`, and `Button` with crisp borders, subtle elevation, refined spacing, and smooth hover states.

### Task 2: Collapsible Light `AdminShell` & `BrandLogo`

**Files:**
- Create: `admin-fe/src/components/layout/admin-shell.test.tsx`
- Modify: `admin-fe/src/components/layout/brand-logo.tsx`
- Modify: `admin-fe/src/components/layout/admin-shell.tsx`

- [ ] **Step 1 (TDD Red):** Write `admin-shell.test.tsx` verifying:
  - Collapsible desktop sidebar toggle (`Thu gọn menu` / `Mở rộng menu`) toggles collapsed state.
  - Permission-gated navigation hides unauthorized links.
  - Quick Jump search input filters permitted routes and navigates to `/orders` on selection.
  - Operator profile & logout button work from the sidebar footer.
- [ ] **Step 2:** Run `npx --prefix admin-fe vitest run src/components/layout/admin-shell.test.tsx` and confirm failure.
- [ ] **Step 3:** Implement upgraded `BrandLogo` (crisp dark rounded square mark) and light collapsible `AdminShell` with grouped nav, Quick Jump search popover, contextual topbar action, and bottom operator profile card.
- [ ] **Step 4:** Re-run `admin-shell.test.tsx` and confirm pass.

### Task 3: Redesigned `OverviewPage` with Live Order Insights

**Files:**
- Modify: `admin-fe/src/pages/overview-page.test.tsx`
- Modify: `admin-fe/src/pages/overview-page.tsx`

- [ ] **Step 1 (TDD Red):** Extend `overview-page.test.tsx` to assert:
  - Personalized operator greeting (`Xin chào, Điều hành viên`).
  - When `order.admin.view` is granted, `listAdminOrders` is called, total orders KPI, Order Status distribution donut/legend, and Recent Orders table render real data.
  - When `order.admin.view` is absent, `listAdminOrders` is NOT called.
  - When `listAdminOrders` rejects (`503`), fulfilled KPIs still render and error alert is displayed.
  - Existing SKU deduplication and `StockDialog` adjustment continue to pass.
- [ ] **Step 2:** Run `npx --prefix admin-fe vitest run src/pages/overview-page.test.tsx` and confirm failure on new assertions.
- [ ] **Step 3:** Implement the redesigned `OverviewPage`:
  - Greeting header + refresh & quick action buttons.
  - KPI card grid (Products, Orders, Available Stock, Active Users, plus RBAC summary) with tinted icon badges and real proportional progress bars.
  - Asymmetric 12-col analytics bento: Category Distribution rounded bar chart (7 cols) + Order Status Distribution centered-total donut chart (5 cols) + Stock Health segmented bar card.
  - Operational row: Recent Orders card + Low-Stock SKU Watchlist table with inline `StockDialog`.
- [ ] **Step 4:** Re-run `overview-page.test.tsx` and confirm pass.

### Task 4: Full Verification & Independent Code Review

- [ ] Run `make verify SERVICE=admin-fe` (boundary check + lint + build + full Vitest suite) and confirm exit `0`.
- [ ] Dispatch a fresh read-only `reviewing-code` subagent on the diff.
- [ ] Address any blocking/major findings via TDD and re-verify before reporting completion.

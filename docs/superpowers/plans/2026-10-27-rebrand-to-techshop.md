# Rebrand S-Shop to TechShop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebrand the e-commerce customer storefront (`ecommerce-fe`) and operator admin console (`admin-fe`) from "S-Shop" (initial "S") to "TechShop" (initial "T") with associated email placeholders, page titles, and tests.

**Architecture:** Update UI presentation layers, titles, and layout components on both React frontend surfaces while keeping backend services, database schema, and API contracts intact to eliminate cross-service breaking risk.

**Tech Stack:** React 19, TypeScript, Vite 8, Tailwind CSS 4, Vitest, React Testing Library.

**Spec:** `docs/superpowers/specs/2026-10-27-rebrand-to-techshop-design.md`

## Global Constraints
- Target user-facing brand name is verbatim `TechShop` (PascalCase / word-joined).
- Logo badge lettermark is verbatim `T`.
- Support / operator email domain is verbatim `@techshop.vn` (e.g., `hotro@techshop.vn`, `admin@techshop.vn`).
- Backend packages, gRPC contracts, DB tables, and Kong routes must not be renamed or broken.

## Review Focus
- Document title tags in both `index.html` files accurately display `TechShop`.
- Mobile and desktop viewports in `SiteHeader` and `AdminShell` render the `T` lettermark badge and `TechShop` brand text.
- Footer copyright displays current year and `TechShop`.
- Access-denied screen in `RequireAdminAuth` mentions `trung tâm quản trị TechShop`.
- All affected unit tests in `ecommerce-fe` and `admin-fe` pass cleanly.

---

### Task 1: Storefront (`ecommerce-fe`) Rebrand

**Files:**
- Modify: `ecommerce-fe/index.html:7`
- Modify: `ecommerce-fe/src/components/layout/site-header.tsx:112-118`
- Modify: `ecommerce-fe/src/components/layout/site-footer.tsx:32-38,90,97-99`
- Modify: `ecommerce-fe/src/pages/home-page.tsx:146-147`

**Interfaces:**
- Consumes: `SiteHeader`, `SiteFooter`, `HomePage`
- Produces: Updated storefront presentation displaying `TechShop` and `T` logo badge

- [ ] **Step 1: Update Storefront HTML Title**
  In `ecommerce-fe/index.html`, change `<title>S-Shop — Mua sắm trực tuyến chính hãng</title>` to `<title>TechShop — Mua sắm trực tuyến chính hãng</title>`.

- [ ] **Step 2: Update SiteHeader Brand & Lettermark**
  In `ecommerce-fe/src/components/layout/site-header.tsx`:
  - Change the logo badge text from `S` to `T`.
  - Change the header brand label from `S-Shop` to `TechShop`.

- [ ] **Step 3: Update SiteFooter Brand, Email & Copyright**
  In `ecommerce-fe/src/components/layout/site-footer.tsx`:
  - Change the footer logo badge text from `S` to `T`.
  - Change `S-Shop` to `TechShop`.
  - Change `hotro@sshop.vn` to `hotro@techshop.vn`.
  - Change `© {new Date().getFullYear()} S-Shop.` to `© {new Date().getFullYear()} TechShop.`.

- [ ] **Step 4: Update HomePage Hero Fallback Brand**
  In `ecommerce-fe/src/pages/home-page.tsx`:
  - Change fallback `seed` to `techshop-hero`.
  - Change fallback `label` to `TechShop`.

- [ ] **Step 5: Verify Storefront Tests, Lint, and Build**
  Run:
  ```powershell
  npm --prefix ecommerce-fe run test
  npm --prefix ecommerce-fe run lint
  npm --prefix ecommerce-fe run build
  ```
  Expected: All tests pass, lint passes with 0 errors, build succeeds.

---

### Task 2: Admin Console (`admin-fe`) Rebrand

**Files:**
- Modify: `admin-fe/index.html:7`
- Modify: `admin-fe/AGENTS.md:8`
- Modify: `admin-fe/src/pages/login-page.tsx:75,78,100,169`
- Modify: `admin-fe/src/components/layout/admin-shell.tsx:135,138,211`
- Modify: `admin-fe/src/routes/require-admin-auth.tsx:51`
- Modify: `admin-fe/src/pages/login-page.test.tsx:28,39,52,72,97`
- Modify: `admin-fe/src/pages/products-page.test.tsx:18,287`

**Interfaces:**
- Consumes: `LoginPage`, `AdminShell`, `RequireAdminAuth`
- Produces: Updated admin console displaying `TechShop Admin` and `T` logo badge

- [ ] **Step 1: Update Admin HTML Title and AGENTS.md**
  - In `admin-fe/index.html`, change title to `<title>TechShop Admin — Trung tâm điều hành</title>`.
  - In `admin-fe/AGENTS.md`, change `S-Shop` to `TechShop`.

- [ ] **Step 2: Update LoginPage Brand, Lettermark & Email Placeholder**
  In `admin-fe/src/pages/login-page.tsx`:
  - Change the logo badge text from `S` to `T`.
  - Change `S-Shop Admin` to `TechShop Admin`.
  - Change `S-Shop Commerce Platform` to `TechShop Commerce Platform`.
  - Change email placeholder to `admin@techshop.vn`.

- [ ] **Step 3: Update AdminShell Sidebar & Breadcrumbs**
  In `admin-fe/src/components/layout/admin-shell.tsx`:
  - Change sidebar badge lettermark from `S` to `T`.
  - Change sidebar header text from `S-Shop Admin` to `TechShop Admin`.
  - Change breadcrumb root label from `S-Shop Admin` to `TechShop Admin`.

- [ ] **Step 4: Update RequireAdminAuth Access-Denied Text**
  In `admin-fe/src/routes/require-admin-auth.tsx`:
  - Change `trung tâm quản trị S-Shop` to `trung tâm quản trị TechShop`.

- [ ] **Step 5: Update Tests for Admin Console**
  - In `admin-fe/src/pages/login-page.test.tsx`:
    - Update `fullName: "Admin S-Shop"` to `"Admin TechShop"`.
    - Update placeholder routes `<div>Bảng điều khiển S-Shop</div>` to `<div>Bảng điều khiển TechShop</div>`.
    - Update assertion `expect(screen.getByText("Bảng điều khiển S-Shop"))` to `"Bảng điều khiển TechShop"`.
  - In `admin-fe/src/pages/products-page.test.tsx`:
    - Update `fullName: "Admin S-Shop"` to `"Admin TechShop"`.
    - Update brand field typing from `"S-Shop"` to `"TechShop"`.

- [ ] **Step 6: Verify Admin Console Tests, Lint, and Build**
  Run:
  ```powershell
  npm --prefix admin-fe test -- src/pages/login-page.test.tsx
  npm --prefix admin-fe run lint
  npm --prefix admin-fe run build
  ```
  Expected: All tests pass, lint passes with 0 errors, build succeeds.

---

### Task 3: Monorepo Verification & Boundary Check

**Files:**
- N/A (Repository-level validation)

- [ ] **Step 1: Run Boundary Check**
  Run:
  ```powershell
  make check-boundaries
  ```
  Expected: Zero boundary violations across services and frontends.

- [ ] **Step 2: Check Git Status**
  Run:
  ```powershell
  git status -s ecommerce-fe admin-fe
  ```
  Expected: Only the rebrand-related files in `ecommerce-fe` and `admin-fe` are modified.

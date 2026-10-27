# Rebrand S-Shop to TechShop Design Specification

## Intent & Why
The e-commerce platform catalog focuses entirely on electronics and tech gadgets (smartphones, laptops, headphones, smartwatches). Rebranding from the generic "S-Shop" name to "TechShop" gives the platform a clear, professional tech-retail identity across customer storefront and admin console surfaces.

## Goals
- Update brand typography and display names from "S-Shop" to "TechShop" on all frontend UI surfaces.
- Update logo lettermarks / initials from `S` to `T`.
- Update HTML document titles (`<title>`) in `ecommerce-fe` and `admin-fe`.
- Update support/contact email references from `@sshop.vn` to `@techshop.vn`.
- Keep frontend unit tests and test fixtures passing with the updated brand name.

## Non-Goals & Anti-Goals
- Do not rename backend services, Go/Java packages, database names, or table columns.
- Do not modify API routes, proto contracts, or internal service ports.
- Avoid modifying unaffected test mocks where email domain is arbitrary, except where brand string matching or user display names are asserted.

## Approaches Considered
1. **Full-stack renaming (Backend + Frontend + Infrastructure)**:
   - Rename repo identifiers, Docker containers, DB configs, Java/Go packages from S-Shop to TechShop.
   - *Trade-off*: High churn across 6 microservices, potential breaking changes in databases or local deployments with zero added user-facing benefit. *Rejected*.
2. **Frontend presentation rebrand to TechShop (Chosen)**:
   - Target user-facing surfaces (`ecommerce-fe` and `admin-fe`): headers, footers, login screens, lettermark badges (`S` -> `T`), titles, and relevant frontend tests.
   - *Trade-off*: Surgical, zero risk to backend stability, immediately delivers the new branding to shoppers and operators. *Selected*.

## Detailed Changes

### 1. Storefront (`ecommerce-fe`)
- `ecommerce-fe/index.html`: Update title to `TechShop — Mua sắm trực tuyến chính hãng` (or `Thiết bị & đồ công nghệ chính hãng`).
- `ecommerce-fe/src/components/layout/site-header.tsx`:
  - Lettermark badge text: `S` -> `T`.
  - Brand title: `S-Shop` -> `TechShop`.
- `ecommerce-fe/src/components/layout/site-footer.tsx`:
  - Lettermark badge text: `S` -> `T`.
  - Brand title: `S-Shop` -> `TechShop`.
  - Support email: `hotro@sshop.vn` -> `hotro@techshop.vn`.
  - Copyright line: `© {year} TechShop. Tất cả quyền được bảo lưu.`.
- `ecommerce-fe/src/pages/home-page.tsx`:
  - Hero fallback seed / label: `sshop-hero` -> `techshop-hero`, `S-Shop` -> `TechShop`.

### 2. Admin Console (`admin-fe`)
- `admin-fe/index.html`: Update title to `TechShop Admin — Trung tâm điều hành`.
- `admin-fe/src/pages/login-page.tsx`:
  - Header lettermark badge: `S` -> `T`.
  - Brand title: `S-Shop Admin` -> `TechShop Admin`.
  - Subtitle: `S-Shop Commerce Platform` -> `TechShop Commerce Platform`.
  - Email placeholder: `admin@sshop.vn` -> `admin@techshop.vn`.
- `admin-fe/src/components/layout/admin-shell.tsx`:
  - Sidebar logo badge: `S` -> `T`.
  - Sidebar title: `S-Shop Admin` -> `TechShop Admin`.
  - Breadcrumb title: `S-Shop Admin` -> `TechShop Admin`.
- `admin-fe/src/routes/require-admin-auth.tsx`:
  - Access-denied copy: `trung tâm quản trị S-Shop` -> `trung tâm quản trị TechShop`.
- `admin-fe/src/pages/login-page.test.tsx`:
  - Test fixture mock full name: `"Admin S-Shop"` -> `"Admin TechShop"`.
  - Test route placeholder: `"Bảng điều khiển S-Shop"` -> `"Bảng điều khiển TechShop"`.
- `admin-fe/src/pages/products-page.test.tsx`:
  - Test fixture mock full name: `"Admin S-Shop"` -> `"Admin TechShop"`.
  - Brand input mock typing: `"S-Shop"` -> `"TechShop"`.

## Verification
- `npm --prefix ecommerce-fe run test`
- `npm --prefix ecommerce-fe run lint`
- `npm --prefix ecommerce-fe run build`
- `npm --prefix admin-fe test -- src/pages/login-page.test.tsx`
- `npm --prefix admin-fe run lint`
- `npm --prefix admin-fe run build`
- `make check-boundaries`

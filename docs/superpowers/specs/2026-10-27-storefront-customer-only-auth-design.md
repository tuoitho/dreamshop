# Storefront Customer-Only Authentication Design Specification

## Intent & Why

The storefront web application (`ecommerce-fe`, local port `5173`) serves retail shoppers, while operational and administrative workflows live in the dedicated `admin-fe` console (local port `5174`). Previously, `ecommerce-fe` accepted any authenticated Identity account regardless of assigned RBAC roles, and `identity-service` bootstrap logic assigned `CUSTOMER` alongside `SUPER_ADMIN` when provisioning the initial super administrator. As a result, operator accounts could sign into the storefront and create customer-scoped records such as carts and product favorites.

Restricting `ecommerce-fe` authentication strictly to pure `CUSTOMER` accounts—and keeping bootstrapped super-administrator accounts free of the `CUSTOMER` role—enforces a clean separation between shopper and operator sessions.

## Goals

- Allow only pure `CUSTOMER` accounts (`roles` containing `"CUSTOMER"` and no non-customer roles) to authenticate in `ecommerce-fe`.
- Reject non-customer sign-in attempts on `ecommerce-fe` `/login`, immediately revoking the newly created refresh session via `/api/v1/auth/logout`, clearing in-memory credentials, and displaying a clear Vietnamese error message.
- Ignore non-customer sessions during `ecommerce-fe` startup refresh (`/api/v1/auth/refresh`) by clearing local in-memory storefront state without calling `/api/v1/auth/logout`, preserving any concurrent `admin-fe` operator session in the same browser.
- Remove the `Admin` development account preset from `ecommerce-fe` `LoginPage` so storefront dev presets only prefill customer credentials.
- Stop assigning the `CUSTOMER` role when `BootstrapSuperAdminService` provisions a missing `SUPER_ADMIN` account in `identity-service`, and update local development documentation accordingly.
- Clean up existing non-customer cart/favorite data in `cart_db` and redundant `CUSTOMER` role assignment on `admin@example.com` in local `identity_db`.

## Non-Goals & Anti-Goals

- **Non-goal:** Modifying `identity-service` `SUPER_ADMIN` wildcard permission evaluation in `RbacRepository` or changing REST/protobuf contracts.
- **Non-goal:** Altering `admin-fe` authentication or permission-gating behavior.
- **Anti-goal:** Calling `/api/v1/auth/logout` during silent `ecommerce-fe` bootstrap refresh when a non-customer cookie is detected, which would terminate an active `admin-fe` tab sharing `localhost` cookies.

## Constraints

- Preserve all session security rules in `ecommerce-fe/AGENTS.md`: access tokens remain strictly in memory, refresh uses the `HttpOnly` cookie with single-flight coalescing, and race-condition epoch/generation guards remain intact.
- Keep public API and protobuf contracts unchanged and backward compatible.
- Maintain strict service boundaries (`make check-boundaries`).

## Approaches Considered

1. **Approach 1 — Storefront Customer-Only Gate + Clean Operator Role Provisioning (Selected):**
   - Enforce pure-`CUSTOMER` role validation in `ecommerce-fe` (`AuthProvider` login & bootstrap, plus `RequireAuth` defense-in-depth), remove the storefront `Admin` dev preset, update `BootstrapSuperAdminService` in `identity-service` so bootstrapped super admins hold only `SUPER_ADMIN`, and purge existing admin cart/favorite records and redundant `CUSTOMER` assignment in local PostgreSQL.
   - *Trade-off:* Surgical, zero API/contract churn, and safe for side-by-side `localhost:5173` / `localhost:5174` browser sessions; direct `curl` calls using a `SUPER_ADMIN` bearer token still pass backend RBAC wildcard checks. *Selected.*

2. **Approach 2 — Storefront Gate + Backend RBAC Wildcard Exclusion for Shopper Permissions:**
   - Everything in Approach 1 plus changing `RbacRepository` permission queries so `SUPER_ADMIN` no longer wildcards `cart.*`, `favorite.manage`, and `address.*` without an explicit `CUSTOMER` role.
   - *Trade-off:* Blocks direct API usage by `SUPER_ADMIN` tokens, but alters foundational `SUPER_ADMIN` wildcard semantics in `identity-service` and existing RBAC/migration invariants. *Not selected.*

## Detailed Changes

### 1. Storefront (`ecommerce-fe`)

- **Customer role predicate (`ecommerce-fe/src/lib/auth-session.ts` or dedicated auth helper):**
  - `isCustomerOnlyRoleSet(roles: readonly string[]): boolean` returning `roles.length === 1 && roles[0] === "CUSTOMER"`.
- **`AuthProvider` (`ecommerce-fe/src/context/auth-provider.tsx`):**
  - **`login(payload)`:** After `loginRequest` and `fetchCurrentUser`, verify `isCustomerOnlyRoleSet(currentUser.roles)`. When false, best-effort call `logoutRequest()` to revoke the newly issued refresh token cookie, clear in-memory access token and user state (guarded by `authGenerationRef` / `advanceSessionEpoch`), and throw an `ApiClientError(403, "Trang mua sắm chỉ dành cho tài khoản khách hàng.", "CUSTOMER_ROLE_REQUIRED")`.
  - **Bootstrap `useEffect` (`refreshAccessToken` -> `fetchCurrentUser`):** Verify `isCustomerOnlyRoleSet(currentUser.roles)`. When false, clear in-memory access token and keep `token = null`, `user = null` without calling `logoutRequest()`.
- **`RequireAuth` (`ecommerce-fe/src/routes/require-auth.tsx`):**
  - Defense-in-depth check ensuring `user === null || isCustomerOnlyRoleSet(user.roles)` alongside `isAuthenticated`; redirect non-customer contexts to `/login`.
- **`LoginPage` (`ecommerce-fe/src/pages/login-page.tsx`) & `ecommerce-fe/README.md`:**
  - Remove the `Admin` entry from `DEV_ACCOUNTS`, keeping only `Customer` in development mode, and update `ecommerce-fe/README.md` dev-account notes.
- **Frontend tests:**
  - Add unit tests in `ecommerce-fe/src/context/auth-provider.test.tsx`, `ecommerce-fe/src/pages/login-page.test.tsx`, and update legacy `"USER"` role fixture in `ecommerce-fe/src/App.test.tsx` to `"CUSTOMER"`.

### 2. Identity Service (`services/identity-service`) & Docs

- **`BootstrapSuperAdminService` (`services/identity-service/src/main/java/com/ecommerce/identity/auth/application/BootstrapSuperAdminService.java`):**
  - Within the bootstrap transaction, after `registration.register(...)` and `rbac.assignRole(user.userId(), RbacPermissions.SUPER_ADMIN, now)`, revoke the default `CUSTOMER` role (`rbac.revokeRole(user.userId(), RbacPermissions.CUSTOMER)`) so bootstrapped super administrators hold only `SUPER_ADMIN`.
- **Backend tests & docs:**
  - Update `BootstrapSuperAdminServiceTest` and `BootstrapSuperAdminStartupTest` to assert `CUSTOMER` is revoked and only `SUPER_ADMIN` remains assigned.
  - Update the local test accounts table in `docs/instructions/local-development.md` to list `SUPER_ADMIN` (without `CUSTOMER`) for `admin@example.com`.

### 3. Local Database Cleanup (One-off Operational Step via `psql`)

- In `cart_db`: delete `favorite_items` and `carts` (cascading `cart_items`) belonging to `admin@example.com` (`82117c74-0632-473d-999a-024ceaf104cf`) as well as orphan `carts` rows whose `user_id` does not exist in `identity_db.identity_users`, preserving `customer1@gmail.com` (`91db75b1-6205-4195-86a8-dc299e3ac863`).
- In `identity_db`: delete the `CUSTOMER` row in `auth_assignments` for `admin@example.com`, leaving `ADMIN` and `SUPER_ADMIN`.

## What Is Left to the Builder

- Exact placement/naming of `isCustomerOnlyRoleSet` helper within `ecommerce-fe/src/lib/`.
- Exact test case structure in Vitest and JUnit suites.

## Verification

- `npm --prefix ecommerce-fe run test`
- `npm --prefix ecommerce-fe run lint`
- `npm --prefix ecommerce-fe run build`
- `mvn -pl services/identity-service test -Dtest=BootstrapSuperAdminServiceTest,BootstrapSuperAdminStartupTest`
- `make verify SERVICE=ecommerce-fe`
- `psql` verification queries confirming `admin@example.com` has zero rows in `cart_db` (`carts`, `cart_items`, `favorite_items`) and no `CUSTOMER` assignment in `identity_db.auth_assignments`.

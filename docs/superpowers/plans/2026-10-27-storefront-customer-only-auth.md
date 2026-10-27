# Storefront Customer-Only Authentication Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restrict `ecommerce-fe` authentication strictly to pure `CUSTOMER` accounts, prevent `identity-service` bootstrap from assigning `CUSTOMER` to `SUPER_ADMIN`, and purge existing non-customer cart/favorite data and redundant role assignments in local PostgreSQL.

**Architecture:** Add a pure-customer role predicate in `ecommerce-fe` enforced during both interactive login (with silent server-session revocation and clear Vietnamese error feedback) and silent startup refresh (clearing only local in-memory state so concurrent `admin-fe` sessions survive), backed by defense-in-depth checking in `RequireAuth`. Update `BootstrapSuperAdminService` in `identity-service` to revoke the default `CUSTOMER` role inside the bootstrap transaction, then clean existing local database records via `psql`.

**Tech Stack:** React 19, TypeScript 6, Vitest 4, React Testing Library, Java 21, Spring Boot, JUnit 5, Testcontainers, PostgreSQL (`psql`).

**Spec:** `docs/superpowers/specs/2026-10-27-storefront-customer-only-auth-design.md`

## Global Constraints

- Access tokens remain exclusively in memory; refresh tokens use the server-managed `HttpOnly` cookie with single-flight coalescing.
- Only accounts whose `roles` contain `"CUSTOMER"` and no other role (`isCustomerOnlyRoleSet`) may authenticate in `ecommerce-fe`.
- Rejected interactive logins on `ecommerce-fe` `/login` must revoke the server refresh cookie via `/auth/logout` without showing false `"Login successful"` or `"Logout successful"` toasts, clear in-memory auth state, and surface error code `"CUSTOMER_ROLE_REQUIRED"` with message `"Trang mua sắm chỉ dành cho tài khoản khách hàng."`.
- Rejected silent bootstrap refreshes on `ecommerce-fe` startup must clear in-memory storefront state without calling `/auth/logout`.
- Public REST and protobuf contracts remain unchanged; `make check-boundaries` must pass.

## Review Focus

- Dual-role accounts such as `["CUSTOMER", "SUPER_ADMIN"]` or `["ADMIN", "CUSTOMER"]` are rejected on `ecommerce-fe` just like `["SUPER_ADMIN"]` or `[]`.
- Non-customer login rejection does not emit misleading `toast.success` notifications for either `/auth/login` or rollback `/auth/logout`.
- Concurrent logout or re-login while non-customer login rollback is in flight respects `authGenerationRef` / `sessionEpoch` guards without clearing a newer valid customer session.
- Silent bootstrap refresh with an active operator cookie leaves `getAccessToken() === null`, `isAuthenticated === false`, and zero `/auth/logout` calls.
- `BootstrapSuperAdminService` rolls back cleanly on failure and leaves bootstrapped super admins with `SUPER_ADMIN` only (no `CUSTOMER` row in `auth_assignments`).

---

### Task 1: Storefront Customer-Only Session Gate & Login Preset Cleanup (`ecommerce-fe`)

**Files:**
- Modify: `ecommerce-fe/src/lib/auth-session.ts`
- Modify: `ecommerce-fe/src/lib/auth-session.test.ts`
- Modify: `ecommerce-fe/src/lib/api.ts`
- Modify: `ecommerce-fe/src/lib/api.test.ts`
- Modify: `ecommerce-fe/src/context/auth-provider.tsx`
- Modify: `ecommerce-fe/src/context/auth-provider.test.tsx`
- Modify: `ecommerce-fe/src/routes/require-auth.tsx`
- Modify: `ecommerce-fe/src/pages/login-page.tsx`
- Modify: `ecommerce-fe/src/pages/login-page.test.tsx`
- Modify: `ecommerce-fe/src/App.test.tsx`
- Modify: `ecommerce-fe/README.md`

**Interfaces:**
- Consumes: `CurrentUser` (`ecommerce-fe/src/lib/types.ts`), `ApiClientError` (`ecommerce-fe/src/lib/api.ts`)
- Produces:
  - `isCustomerOnlyRoleSet(roles: readonly string[]): boolean` in `ecommerce-fe/src/lib/auth-session.ts`
  - `CUSTOMER_ONLY_ERROR_CODE = "CUSTOMER_ROLE_REQUIRED"` and `CUSTOMER_ONLY_ERROR_MESSAGE = "Trang mua sắm chỉ dành cho tài khoản khách hàng."` in `ecommerce-fe/src/lib/auth-session.ts`
  - `logoutRequest(options?: { silent?: boolean }): Promise<void>` in `ecommerce-fe/src/lib/api.ts`

- [x] **Step 1: Write failing unit tests for `isCustomerOnlyRoleSet`, silent rollback logout/login toast suppression, `AuthProvider` non-customer login & bootstrap rejection, `RequireAuth` role guard, and `LoginPage` customer-only dev presets**
  - In `ecommerce-fe/src/lib/auth-session.test.ts`: assert `isCustomerOnlyRoleSet(["CUSTOMER"]) === true`, and `false` for `[]`, `["ADMIN"]`, `["SUPER_ADMIN"]`, `["CUSTOMER", "SUPER_ADMIN"]`, `["ADMIN", "CUSTOMER"]`.
  - In `ecommerce-fe/src/lib/api.test.ts`: assert `logoutRequest({ silent: true })` sends `POST /auth/logout` without triggering `toast.success` or `toast.error`, and `login()` suppresses premature interceptor `toast.success`.
  - In `ecommerce-fe/src/context/auth-provider.test.tsx`:
    1. Rejects `login()` when `/users/me` returns `["SUPER_ADMIN"]` or `["CUSTOMER", "SUPER_ADMIN"]` or `[]`: calls `/auth/logout`, clears access token & user, emits no `toast.success`, and rejects with `ApiClientError` (`code === "CUSTOMER_ROLE_REQUIRED"`, `message === "Trang mua sắm chỉ dành cho tài khoản khách hàng."`).
    2. Ignores non-customer account during bootstrap `/auth/refresh`: clears in-memory token, keeps `isAuthenticated === false` and `ready === true`, and does **not** call `/auth/logout`.
    3. Does not clobber a newer valid customer login if a rejected non-customer login's `/auth/logout` resolves afterwards.
  - In `ecommerce-fe/src/pages/login-page.test.tsx`: assert development mode prefills `customer1@gmail.com`, renders Customer preset only (no `Admin` preset button), and displays `"Trang mua sắm chỉ dành cho tài khoản khách hàng."` alert when `login` rejects with that error.
  - In `ecommerce-fe/src/App.test.tsx`: update `USER.roles` fixture from legacy `["USER"]` to `["CUSTOMER"]` and add a test verifying `RequireAuth` redirects non-customer `AuthContext` users (`roles: ["ADMIN"]`) to `/login`.

- [x] **Step 2: Run focused Vitest tests to verify RED failure**
  Run:
  ```powershell
  npm --prefix ecommerce-fe run test -- src/lib/auth-session.test.ts src/lib/api.test.ts src/context/auth-provider.test.tsx src/pages/login-page.test.tsx src/App.test.tsx
  ```
  Expected: FAIL because `isCustomerOnlyRoleSet`, silent logout option, non-customer `AuthProvider` checks, and `LoginPage` preset changes are not yet implemented.

- [x] **Step 3: Implement customer-only session gate and `LoginPage` updates**
  - Export `CUSTOMER_ONLY_ERROR_CODE`, `CUSTOMER_ONLY_ERROR_MESSAGE`, and `isCustomerOnlyRoleSet(roles: readonly string[]): boolean` (`roles.length === 1 && roles[0] === "CUSTOMER"`) from `ecommerce-fe/src/lib/auth-session.ts`.
  - In `ecommerce-fe/src/lib/api.ts`: add `_silentToast?: boolean` to `SessionAwareRequestConfig`; skip `toast.success` / `toast.error` when `config?._silentToast` is true and skip interceptor `toast.success` on `/auth/login`; accept `options?: { silent?: boolean }` in `logoutRequest`.
  - In `ecommerce-fe/src/context/auth-provider.tsx`:
    - During bootstrap `useEffect`, if `!isCustomerOnlyRoleSet(currentUser.roles)`, call `clearAccessToken()`, `setToken(null)`, `setUser(null)` without calling `logoutRequest()`.
    - During `login()`, if `!isCustomerOnlyRoleSet(currentUser.roles)`, clear local token/state for that generation, invoke `await logoutRequest({ silent: true }).catch(() => undefined)`, and throw `new ApiClientError(403, CUSTOMER_ONLY_ERROR_MESSAGE, CUSTOMER_ONLY_ERROR_CODE)`. Emit `toast.success` only after customer validation succeeds.
  - In `ecommerce-fe/src/routes/require-auth.tsx`: check `const hasCustomerAccess = isAuthenticated && (user === null || isCustomerOnlyRoleSet(user.roles))` and redirect to `/login` when false.
  - In `ecommerce-fe/src/pages/login-page.tsx` and `ecommerce-fe/README.md`: remove `Admin` from `DEV_ACCOUNTS` and update dev-mode documentation.

- [x] **Step 4: Run `ecommerce-fe` test, lint, and build gates to verify GREEN**
  Run:
  ```powershell
  npm --prefix ecommerce-fe run test
  npm --prefix ecommerce-fe run lint
  npm --prefix ecommerce-fe run build
  ```
  Expected: All Vitest suites pass, `oxlint` reports 0 errors, and TypeScript/Vite build exits 0.

---

### Task 2: Exclude `CUSTOMER` Role from Bootstrapped `SUPER_ADMIN` (`identity-service`)

**Files:**
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/auth/application/BootstrapSuperAdminService.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/auth/application/BootstrapSuperAdminServiceTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/auth/application/BootstrapSuperAdminStartupTest.java`
- Modify: `docs/instructions/local-development.md`

**Interfaces:**
- Consumes: `RbacRepository.revokeRole(UUID userId, String roleName)` (`services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java`)
- Produces: Bootstrapped super-admin accounts provisioned with `SUPER_ADMIN` only (no `CUSTOMER` assignment)

- [x] **Step 1: Write failing tests in `BootstrapSuperAdminServiceTest` and `BootstrapSuperAdminStartupTest`**
  - In `BootstrapSuperAdminServiceTest.createsAndPromotesMissingBootstrapUser`: verify `rbac.revokeRole(USER_ID, "CUSTOMER")` is called after `rbac.assignRole(USER_ID, "SUPER_ADMIN", NOW)`.
  - In `BootstrapSuperAdminStartupTest.seedsSuperAdminDuringApplicationStartup`: assert `auth_assignments` for `bootstrap-root@example.test` contains exact role `["SUPER_ADMIN"]` and zero rows for `'CUSTOMER'`.

- [x] **Step 2: Run Maven test to verify RED failure**
  Run:
  ```powershell
  mvn -pl services/identity-service test "-Dtest=BootstrapSuperAdminServiceTest,BootstrapSuperAdminStartupTest"
  ```
  Expected: FAIL because `BootstrapSuperAdminService` still leaves `CUSTOMER` assigned.

- [x] **Step 3: Implement `CUSTOMER` revocation in `BootstrapSuperAdminService` and update docs**
  - In `BootstrapSuperAdminService.ensureSuperAdmin`, call `rbac.revokeRole(user.userId(), RbacPermissions.CUSTOMER);` immediately after `rbac.assignRole(user.userId(), RbacPermissions.SUPER_ADMIN, now)` inside `transactions.executeWithoutResult`.
  - In `docs/instructions/local-development.md`: update `admin@example.com` role column from `` `CUSTOMER`, `SUPER_ADMIN` `` to `` `SUPER_ADMIN` `` and note that `ecommerce-fe` `/login` prefills the customer account.

- [x] **Step 4: Run Maven tests to verify GREEN**
  Run:
  ```powershell
  mvn -pl services/identity-service test "-Dtest=BootstrapSuperAdminServiceTest,BootstrapSuperAdminStartupTest"
  ```
  Expected: PASS (`BUILD SUCCESS`).

---

### Task 3: Local PostgreSQL Cleanup via `psql` & Repository Gate Verification

**Files:**
- Local PostgreSQL databases: `cart_db`, `identity_db` (operational cleanup, no schema migration changes)

- [x] **Step 1: Delete non-customer cart/favorite records and redundant `CUSTOMER` role assignment via `psql`**
  Run against `ecommerce-postgres`:
  - In `cart_db`: delete `favorite_items` and `carts` (cascading `cart_items`) for `user_id = '82117c74-0632-473d-999a-024ceaf104cf'` (`admin@example.com`), plus orphan `carts` (`00000000-0000-0000-0000-000000000001`, `00000000-0000-0000-0000-000000000002`).
  - In `identity_db`: delete `auth_assignments` row where `user_id = '82117c74-0632-473d-999a-024ceaf104cf'` and `item_name = 'CUSTOMER'`.

- [x] **Step 2: Verify cleaned PostgreSQL state via `psql`**
  Run read-only `SELECT` queries on `identity_db` and `cart_db`.
  Expected:
  - `admin@example.com` has roles `ADMIN, SUPER_ADMIN` (no `CUSTOMER`), `0` rows in `carts`, `0` rows in `cart_items`, `0` rows in `favorite_items`.
  - `customer1@gmail.com` retains role `CUSTOMER` and its existing cart and favorite items intact.

- [x] **Step 3: Run surface & boundary verification gates**
  Run:
  ```powershell
  make verify SERVICE=ecommerce-fe
  make verify SERVICE=identity-service
  make check-boundaries
  ```
  Expected: All commands exit with code 0.

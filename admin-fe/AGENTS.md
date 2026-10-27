# Admin Frontend Instructions

These instructions apply to `admin-fe/` in addition to the root
[`AGENTS.md`](../AGENTS.md).

## Ownership

`admin-fe/` owns the standalone operator console for S-Shop:

- Overview KPIs, stock health breakdown, and low-stock watchlist.
- Catalog product CRUD, soft-delete/restore, and direct browser S3/MinIO media uploads (`IMAGE` & `VIDEO`).
- Inventory stock inspection and atomic `SetStock` / `AdjustStock` operations.
- User search, `ACTIVE` / `DISABLED` status management, and role assignment/revocation.
- RBAC roles, permissions, and parent-child hierarchy administration.

`admin-fe/` does not own backend domain rules or share source imports with
`ecommerce-fe/`, `services/*`, or `contracts/*`.

## API & Storage Boundaries

- Route all REST API requests through Kong (`VITE_API_BASE_URL`, default
  `http://localhost:8000/api/v1` locally and `/api/v1` in production Compose).
- Upload product images and videos directly to MinIO using the presigned `PUT`
  URL returned by `POST /api/v1/admin/products/media/presign` (`ADR-012`). Use
  native `fetch` with only `Content-Type` matching the presigned MIME type —
  never attach `Authorization` headers to S3 presigned requests.
- Keep access tokens strictly in memory (`src/lib/auth-session.ts`), restore
  sessions on reload via HttpOnly cookie refresh (`POST /api/v1/auth/refresh`),
  and coalesce concurrent refreshes via a single-flight promise with session
  epoch protection.
- Gate navigation items, dashboard panels, and mutation controls using the
  operator's effective permissions from `GET /api/v1/users/me/permissions`
  (`can(permission)`). UI gating improves operator UX; backend services enforce
  authoritative RBAC checks.

## Verification

From repository root:

```powershell
npm --prefix admin-fe run test
npm --prefix admin-fe run lint
npm --prefix admin-fe run build
make verify SERVICE=admin-fe
```

# Identity Session and Profile Design

## Summary

This slice completes the local hybrid edge-auth workflow and adds renewable, revocable browser sessions to the first-party Identity service. Kong remains the public ingress and verifies access JWTs for protected Identity routes. Identity persists opaque refresh-token sessions, supports rotation and current-session logout, and exposes the authenticated caller profile.

The React frontend demonstrates the complete browser flow: login, session restoration after reload, profile display and name update, silent access-token refresh, and logout.

## Goals

- Run infrastructure and Kong in Docker while Identity runs directly on Windows; Kong reaches Identity at `host.docker.internal:8080`.
- Keep `GET /api/v1/users/me` protected by Kong JWT verification and trusted caller-context propagation.
- Add `PATCH /api/v1/users/me` for the authenticated user to change only `full_name`.
- Issue a short-lived access JWT plus an opaque refresh token stored only in an `HttpOnly; Secure; SameSite=Lax` cookie.
- Rotate refresh tokens atomically, detect token reuse, and revoke the affected token family.
- Let `POST /api/v1/auth/logout` revoke only the refresh-token session presented by its cookie.
- Restore make targets and smoke checks for the hybrid local workflow.
- Provide an auth/profile frontend demo without changing storefront or cart behavior.

## Non-goals

- Rate limiting, password reset, email verification, MFA, social login, or third-party OIDC.
- Logout from all devices, user session management UI, or administrator session revocation.
- Updating email, password, roles, or account status through `/users/me`.
- Per-service JWT validation, changes to checkout, catalog, cart, or order workflows.

## Architecture

```text
Browser (localhost:5173)
  | Authorization: Bearer <access JWT>, credentials: include
  v
Kong (Docker, localhost:8000)
  | verifies JWT for /api/v1/users/*; injects x-user-id/x-user-roles
  v
Identity (Windows, localhost:8080)
  | identity_db in Docker
  +-- access JWT signing
  +-- refresh-token session store
```

Public authentication endpoints are routed through Kong without the caller-context plugin. Protected user endpoints require the plugin; it verifies RS256 signatures against Identity JWKS, removes caller-provided identity headers, and injects identity values derived from valid claims.

Identity is configured with one canonical local issuer. The issuer value used to sign access tokens and the value configured in Kong must match exactly. Docker connectivity is used only for the Kong-to-Identity upstream and JWKS URI; it does not change the JWT issuer.

## Session Model

### Access token

Identity returns an RS256 access JWT in JSON with the existing `access_token`, `token_type`, and `expires_in` fields. The client keeps it in memory only. The token contains the existing minimum identity claims: `sub`, `roles`, `iss`, `aud`, `exp`, `iat`, `jti`, and `kid`.

### Refresh token

Identity generates a cryptographically random opaque refresh token. The raw value is never persisted, logged, or returned in JSON. Identity stores a one-way SHA-256 digest and places the raw token in a cookie:

```text
Set-Cookie: refresh_token=<opaque-value>; Path=/api/v1/auth; HttpOnly; Secure; SameSite=Lax; Max-Age=<configured TTL>
```

`Secure` is enabled in production. The local profile explicitly disables it because the demo runs over HTTP; this exception is limited to the local environment configuration.

### Persistence

Identity owns a `refresh_token_sessions` table with these fields:

| Column | Purpose |
|---|---|
| `id` UUID | Session identifier |
| `user_id` UUID | Owning Identity user |
| `family_id` UUID | Groups the initial session and every replacement produced by rotation |
| `token_hash` | SHA-256 digest of the opaque refresh token; unique |
| `issued_at`, `expires_at` | Session lifetime |
| `revoked_at` | Null while active |
| `replaced_by_session_id` | Successor created by rotation |
| `revocation_reason` | `ROTATED`, `LOGOUT`, or `REUSE_DETECTED` |

An index supports lookup by `token_hash`, and indexes support revoking active sessions by family and querying a user's expiring sessions. A foreign key may reference the Identity-owned user table.

### Rotation and reuse detection

`POST /auth/refresh` reads the refresh cookie, hashes it, and locks the matching session row in one database transaction.

1. A missing, unknown, expired, or revoked token returns `401 INVALID_REFRESH_TOKEN` and clears the refresh cookie.
2. An active token is revoked with reason `ROTATED`; Identity creates a new active session with the same `family_id`, links it through `replaced_by_session_id`, sets a replacement cookie, and returns a new access JWT.
3. A token already revoked with reason `ROTATED` is a reuse attempt. Identity revokes every still-active session in that family with reason `REUSE_DETECTED`, clears the cookie, and returns `401 INVALID_REFRESH_TOKEN`.

The row lock and unique hash constraint ensure concurrent refresh requests cannot create multiple valid successors.

### Logout

`POST /auth/logout` reads the refresh cookie. If it maps to an active session, Identity revokes only that session with reason `LOGOUT`. Regardless of whether a valid session exists, the endpoint clears the cookie and returns `204 No Content`; logout is intentionally idempotent.

## HTTP Contracts

All endpoints below are exposed by Kong at `/api/v1`. Internal Identity controller paths omit `/api/v1` because Kong strips the public prefix.

| Method | Public path | Authentication | Result |
|---|---|---|---|
| `POST` | `/auth/login` | credentials | JSON access token plus refresh cookie |
| `POST` | `/auth/refresh` | refresh cookie | JSON replacement access token plus rotated refresh cookie |
| `POST` | `/auth/logout` | refresh cookie when present | `204` plus cleared refresh cookie |
| `GET` | `/users/me` | valid access JWT | caller profile |
| `PATCH` | `/users/me` | valid access JWT | caller profile with updated full name |

`PATCH /users/me` accepts only:

```json
{"full_name":"Nguyen Van A"}
```

It applies the same non-empty and maximum-length validation used during registration. The caller identity comes solely from `CallerContextPrincipal`; no request field can select another user.

Refresh failures use the existing API error envelope:

```json
{"code":"INVALID_REFRESH_TOKEN","message":"Refresh token is invalid or expired"}
```

Malformed profile input returns `400 VALIDATION_FAILED`; missing or disabled callers return the established `404 USER_NOT_FOUND` response.

## Gateway, CORS, and Local Operations

Kong routes the Identity service to `http://host.docker.internal:8080`. Its protected route reads JWKS from `http://host.docker.internal:8080/.well-known/jwks.json`. The `caller-context` plugin configuration uses the exact local issuer emitted by Identity.

The global CORS configuration permits the configured frontend origin (default `http://localhost:5173`), allows the `Authorization` header, and sets `credentials: true`. It must not use a wildcard origin when allowing credentials.

Make targets distinguish infrastructure from Windows application processes:

- `infra-up`, `infra-down`, and `infra-logs` manage Docker infrastructure and Kong.
- `identity` starts Identity directly on port `8080` with local key paths and configuration.
- `identity-keys`, `proto`, and `lint` remain available.
- `verify-identity` and `verify-edge-auth` run existing smoke scripts through Kong.

The repository README and frontend README describe the required startup order: generate local signing keys, start infrastructure, start Identity on Windows, then start the Vite dev server.

## Frontend Demo

The frontend auth provider owns the in-memory access token and authenticated profile state. It never writes the access token to local storage or session storage.

1. Login submits credentials with `withCredentials: true`, receives an access JWT, and then loads `/users/me`.
2. On provider initialization, the app calls `/auth/refresh` with credentials. On success it stores the returned access JWT only in memory and loads `/users/me`; on failure it becomes anonymous without surfacing an error.
3. Axios attaches the in-memory access JWT and sends credentials on all API requests. For a protected request that returns `401`, a single shared refresh operation runs once; concurrent failed requests await it and retry exactly once. Authentication endpoints never trigger this interceptor refresh path.
4. The protected profile page shows email, roles, and full name; it submits the name form to `PATCH /users/me` and replaces the displayed profile with the response.
5. Logout calls `/auth/logout`, clears in-memory access token and profile, and redirects to login.

## Testing and Acceptance Criteria

### Identity tests

- Login creates an active hashed refresh-token session and sets the configured cookie attributes.
- Refresh rotates an active token; the old token cannot be reused and reuse revokes the family.
- Logout revokes the presented session, clears the cookie, and remains `204` for missing or invalid cookies.
- `GET /users/me` continues to accept only verified caller context; `PATCH /users/me` changes only the caller's name and validates input.

### Gateway and hybrid smoke tests

- Register and login through Kong; a valid access token fetches `/users/me`.
- Missing or tampered access JWT is rejected with `401`; forged caller headers have no effect.
- Refresh through the cookie returns a new access JWT; logout makes a subsequent refresh fail.
- Kong reaches Windows-hosted Identity through `host.docker.internal`; public auth endpoints and JWKS remain available.

### Frontend tests

- Provider bootstraps from a successful refresh and becomes anonymous when refresh fails.
- The interceptor performs only one refresh for simultaneous `401` responses and never loops on auth endpoints.
- Profile data loads, a name update renders the returned profile, and logout clears UI state.

The slice is complete when the documented local sequence demonstrates login, reload/session restoration, profile update, and current-session logout end-to-end through Kong, with all service, smoke, and frontend tests passing.

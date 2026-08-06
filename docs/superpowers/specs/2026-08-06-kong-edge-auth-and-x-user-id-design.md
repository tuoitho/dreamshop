# Kong Edge Authentication and x-user-id Propagation Design

## 1. Summary

Identity already issues short-lived RSA-signed access JWTs and publishes the matching public JWKS. This design moves JWT verification to the Kong ingress: Kong validates the signature and claims once, then forwards the verified identity to upstream services as `x-user-id` and `x-user-roles` headers. Upstream services consume those headers instead of self-verifying tokens.

The externally demonstrable flow:

```text
Client ──GET /api/v1/users/me  (Authorization: Bearer <RS256 JWT>)──► Kong
  Kong: strip x-user-id/x-user-roles from client, verify JWT via JWKS,
        inject x-user-id (sub) and x-user-roles (roles)
        ──GET /users/me──► Identity ─► identity_db ─► {user_id, email, full_name, roles}
```

This follows the trusted-edge model documented in [ADR-007](../../adr/007-first-party-identity-service.md) and the edge-routing design (Kong strips client identity headers and only forwards verified caller context). This design does **not** adopt a zero-trust model where every service re-verifies the JWT; the trade-off is explicit in [Section 13](#13-security-model-and-trade-offs).

## 2. Goals

- Kong verifies the RS256 access JWT (signature, `iss`, `aud`, `exp`) at the edge using the existing Identity JWKS endpoint, without embedding any PEM key material in committed configuration.
- Kong strips client-supplied `x-user-id` / `x-user-roles` headers and injects values derived only from the verified token.
- Identity exposes `GET /users/me` that reads the caller context from the injected headers (no self-verification) and returns the caller's profile.
- Route split in Kong so public endpoints (`/api/v1/auth/*`) stay open while `/api/v1/users/*` becomes protected.
- End-to-end automated verification: register → login → call `/users/me` through Kong with a token; assert 401 without a token and correct caller identity with a token.
- Establish the reusable pattern (edge verify + header propagation + service-side caller-context consumer) that later slices apply to BFF/order.

## 3. Non-goals

This slice does not implement:

- per-service JWT self-verification libraries for BFF, order, catalog or inventory;
- consuming the injected headers in any service other than Identity;
- refresh tokens, logout, revocation or token families;
- Kong rate limiting, request-size limits or an anonymous consumer;
- runtime key-rotation administration; Kong only reacts to JWKS changes;
- production network policies, service meshes or egress controls that would harden the trusted-edge model;
- RBAC/authorization beyond returning the caller's own profile.

## 4. Architectural decisions

### 4.1 Trusted-edge, not zero-trust

JWT/JWKS enables stateless per-service verification, but this design deliberately uses **edge verification**: Kong is the only place that validates tokens, and services trust the propagated headers. Rationale:

- Each service verifying via JWKS duplicates key-loading, caching, rotation and claim-checking logic.
- Kong is already the sole public ingress (host ports 8000/8001 only); services are reachable only on the private Docker network.
- Kong strips client-supplied identity headers, so external callers cannot forge `x-user-id`.

Consequences and the accepted residual risk are documented in [Section 13](#13-security-model-and-trade-offs).

### 4.2 Custom Kong plugin instead of the built-in `jwt` plugin

The built-in Kong `jwt` plugin requires `rsa_public_key` PEM content inside the credential configuration, and it does not support JWKS URLs (GitHub Kong/kong#7784 remains open). The PEM files are git-ignored (`**/*.pem`), so embedding them in the committed `kong.yaml` is not viable, and a config-generation step would be needed for every key change.

Instead, one custom Kong plugin (`caller-context`) fetches the JWKS from Identity, caches it, verifies RS256 using `lua-resty-openssl` (bundled with the Kong image), and injects/strips the caller headers. No key material is stored in Kong configuration, and key rotation works automatically via `kid`.

### 4.3 Route split

The current `identity-public` route covers both `/api/v1/auth` and `/api/v1/users`. It is split into:

| Route | Paths | Auth |
|---|---|---|
| `identity-auth-public` | `/api/v1/auth` | none |
| `identity-users-protected` | `/api/v1/users` | `caller-context` plugin |

Both route to `http://identity-service:8080` with `strip_path: true`.

### 4.4 Identity consumes headers via a Spring Security filter

Identity adds a `CallerContextFilter` that reads `x-user-id` / `x-user-roles`, validates the UUID format, and populates the `SecurityContext` before authorization runs. `GET /users/me` then requires an authenticated principal. This keeps the "service does not verify tokens" property and mirrors the pattern later services will reuse.

## 5. Edge caller-context contract

### 5.1 Request headers (client → Kong)

| Header | Origin | Semantics |
|---|---|---|
| `Authorization: Bearer <JWT>` | client | the RS256 access token issued by Identity |

### 5.2 Upstream headers (Kong → service)

| Header | Value | Source |
|---|---|---|
| `x-user-id` | `sub` claim (UUID) | verified token |
| `x-user-roles` | `roles` claim joined with `,` | verified token |

Kong **clears** both headers for every request on the protected route before injecting the verified values, so a client-supplied `x-user-id`/`x-user-roles` is never forwarded. The `Authorization` header is forwarded unchanged to the upstream (per the edge-routing allowlist).

### 5.3 Plugin failure responses

All error bodies use the project's JSON envelope shape (`{code, message}`; `trace_id` is added by services, not Kong):

| Condition | Status | Body |
|---|---|---|
| missing/malformed `Authorization` or token | `401` | `{"code":"UNAUTHORIZED","message":"Missing or invalid access token"}` |
| signature/`iss`/`aud`/`exp` check failed or unknown `kid` after refetch | `401` | `{"code":"UNAUTHORIZED","message":"Invalid or expired access token"}` |
| JWKS fetch failed (Identity unreachable) | `503` | `{"code":"AUTHENTICATION_UNAVAILABLE","message":"Authentication is temporarily unavailable"}` |

Preflight `OPTIONS` requests are short-circuited by the global CORS plugin before the `caller-context` plugin runs, so they remain unauthenticated.

## 6. Kong components

### 6.1 Plugin layout

```text
deployments/kong/
├── Dockerfile
├── kong.yaml
└── plugins/caller-context/
    ├── handler.lua
    └── schema.lua
```

### 6.2 `deployments/kong/Dockerfile`

```dockerfile
FROM kong:3.8-ubuntu
COPY plugins/caller-context /usr/local/share/lua/5.1/kong/plugins/caller-context
ENV KONG_PLUGINS=bundled,caller-context
```

### 6.3 Plugin behavior (`handler.lua`)

Runs in the `access` phase with `PRIORITY = 1001` (after CORS, before passthrough):

1. Clear `x-user-id` and `x-user-roles`.
2. Read the bearer token from the `Authorization` header only.
3. Decode the JWT compact form (`header.payload.signature`, three dot-separated parts).
4. Read `kid` from the header; load the JWKS for `config.jwks_uri` from `kong.cache` (TTL `config.cache_ttl_seconds`, default 300) and select the key with that `kid`. On an unknown `kid`, invalidate and refetch once.
5. Verify `alg == "RS256"`, then verify the signature over `<header>.<payload>` using `resty.openssl.pkey.new(jwk_json, {format = "JWK"})` and `pkey:verify`.
6. Check `iss == config.issuer`, `aud` contains `config.audience`, and `exp` is in the future (allowing `config.clock_skew_seconds`).
7. Inject `x-user-id = claims.sub` and `x-user-roles = table.concat(claims.roles, ",")`.
8. Any failure in 3–6 maps to the responses in [Section 5.3](#53-plugin-failure-responses).

### 6.4 Plugin schema (`schema.lua`)

| Field | Type | Default | Required |
|---|---|---|---|
| `jwks_uri` | string | — | yes |
| `issuer` | string | — | yes |
| `audience` | string | — | yes |
| `user_id_header` | string | `x-user-id` | no |
| `roles_header` | string | `x-user-roles` | no |
| `cache_ttl_seconds` | integer | `300` | no |
| `clock_skew_seconds` | integer | `0` | no |

### 6.5 Declarative config (`kong.yaml`)

```yaml
services:
  - name: identity-service
    url: http://identity-service:8080
    routes:
      - name: identity-auth-public
        paths: ["/api/v1/auth"]
        strip_path: true
      - name: identity-users-protected
        paths: ["/api/v1/users"]
        strip_path: true
        plugins:
          - name: caller-context
            config:
              jwks_uri: http://identity-service:8080/.well-known/jwks.json
              issuer: http://identity-service:8080
              audience: ecommerce-api
```

The global CORS plugin and the `bff-service` route remain unchanged.

### 6.6 Compose change

The `kong` service in `deployments/docker/compose.foundation.yml` switches from `image: kong:3.8-ubuntu` to a build of the Dockerfile above.

## 7. Identity `/users/me` contract

Kong routes `/api/v1/users` to Identity with `strip_path: true`, so the upstream path is `/users/me`.

| Capability | Public path | Identity upstream path | Auth |
|---|---|---|---|
| Current caller | `GET /api/v1/users/me` | `GET /users/me` | `caller-context` |

Success `200`:

```json
{
  "user_id": "9c798f79-c122-4c0b-8db4-156b263fcce0",
  "email": "user@example.com",
  "full_name": "Nguyen Van A",
  "roles": ["CUSTOMER"]
}
```

Errors:

| Condition | Status | Body |
|---|---|---|
| missing/malformed `x-user-id` (Kong should have rejected first) | `401` | `{"code":"UNAUTHORIZED",...}` |
| no user with that id, or account disabled | `404` | `{"code":"USER_NOT_FOUND","message":"User not found",...}` |

`MeResponse` mirrors `RegisterResponse` (`user_id`, `email`, `full_name`, `roles`). A disabled account returns `404 USER_NOT_FOUND` to avoid leaking account state, consistent with login behavior.

## 8. Identity components

```text
com.ecommerce.identity
├── user
│   ├── api              MeController, MeResponse
│   └── application      MeService
├── security
│   ├── CallerContextFilter
│   └── CallerContext, CallerContextPrincipal
└── common
    └── error            UserNotFoundException mapping (404 USER_NOT_FOUND)
```

### 8.1 `CallerContextFilter`

- A `OncePerRequestFilter` defined as a `@Bean` and inserted into the security chain via `http.addFilterBefore(callerContextFilter, AuthorizationFilter.class)`, so it populates the `SecurityContext` before authorization runs and is not double-registered as a servlet filter.
- Reads `x-user-id`; if present and parseable as a `UUID`, sets an authenticated `CallerContextPrincipal(userId, roles)` where `roles` is parsed from the comma-separated `x-user-roles` header.
- If the header is absent or malformed, no principal is set (request remains unauthenticated). It never rejects the request itself; authorization is left to the `SecurityFilterChain`.

### 8.2 Security policy

- `/register`, `/login`, `/.well-known/jwks.json` and health/readiness stay `permitAll`.
- `GET /users/me` becomes `.authenticated()`.
- `anyRequest()` stays `denyAll()`.
- The `authenticationEntryPoint` changes from `403 FORBIDDEN` to `401` with `{"code":"UNAUTHORIZED","message":"Missing or invalid access token"}`; the `accessDeniedHandler` remains `403 FORBIDDEN`.

### 8.3 `MeService`

- Loads the user by id with profile and roles (new repository method `findWithProfileAndRolesById(UUID id)` using an `@EntityGraph` over `profile` and `roles`).
- Returns the profile or throws `UserNotFoundException`.

## 9. Error and logging contract

- Kong error bodies follow the envelope shape but do not include `trace_id` (Kong cannot know the identity trace id; services add it).
- Identity's `GlobalExceptionHandler` maps `UserNotFoundException` to `404 USER_NOT_FOUND`.
- No password, token or private-key material is ever logged by Kong or Identity.

## 10. Detailed flows

### 10.1 Happy path

1. Client registers and logs in, obtaining an RS256 JWT.
2. Client calls `GET /api/v1/users/me` with `Authorization: Bearer <JWT>`.
3. Kong route `identity-users-protected` matches; `caller-context` clears the two headers, verifies the JWT via cached JWKS, and injects `x-user-id` / `x-user-roles`.
4. Identity's `CallerContextFilter` populates the security context; `/users/me` loads the profile and returns it.

### 10.2 Forged-header attempt

Client sends a valid token plus a fake `x-user-id` header. Kong clears the fake header before injecting the verified value, so the upstream sees only the verified `sub`.

### 10.3 Expired or tampered token

Kong returns `401 UNAUTHORIZED`; the request never reaches Identity.

### 10.4 Identity unreachable (JWKS fetch failure)

Kong returns `503 AUTHENTICATION_UNAVAILABLE`, distinguishable from `401`.

## 11. Test strategy

### 11.1 Identity (MockMvc + PostgreSQL integration)

- `MeApiTest`: valid `x-user-id` header → `200` with `user_id`/`email`/`full_name`/`roles`; missing header → `401 UNAUTHORIZED`; malformed UUID → `401`; unknown user id → `404 USER_NOT_FOUND`; disabled account → `404`.
- Existing register/login/JWKS tests must continue to pass unchanged.

### 11.2 Edge end-to-end (`scripts/verify-edge-auth.ps1` + Makefile target `verify-edge-auth`)

- register → login → `GET /api/v1/users/me` through Kong with the token → assert `200` and the returned `user_id` equals the `sub` from the verified JWT.
- same request without a token → assert `401`.
- same request with a valid token and a forged `x-user-id` header → assert the returned `user_id` is the real one, not the forged one.

### 11.3 Kong plugin

The Lua verification logic is exercised through the end-to-end script rather than a separate Lua test harness; the RSA path is the same verified in `scripts/verify-identity.ps1`.

## 12. Operational behavior

- JWKS is cached for 300 s, matching Identity's `Cache-Control: max-age=300` on the JWKS endpoint. Key rotation therefore takes effect within at most ~5 minutes (or immediately on the next unknown-`kid` refetch).
- Kong resolves `identity-service` on the private `ecommerce-net` network; the JWKS fetch bypasses Kong itself.
- `make up` rebuilds the Kong image from the new Dockerfile, so plugin changes take effect on restart.

## 13. Security model and trade-offs

- **Accepted**: services trust `x-user-id` / `x-user-roles` from Kong. An attacker who can reach a service directly on the Docker network (e.g., from another compromised container) can forge these headers. This is accepted for the dev/MVP environment.
- **Mitigations in place**: only Kong is exposed to the host (8000/8001); Kong strips client-supplied identity headers; the private key never leaves Identity; JWKS is public-by-design.
- **Production hardening (future)**: separate service networks with egress rules, or move to per-service verification if the threat model demands zero-trust.

## 14. Delivery boundaries and follow-on slices

This slice delivers the Kong plugin, route split, Identity `/users/me`, and end-to-end verification. Later slices can reuse the same pattern for BFF (Go) and order (Spring): consume `x-user-id` / `x-user-roles`, keep services on the private network, and enforce authorization per endpoint.

## 15. Acceptance criteria

- `GET /api/v1/users/me` with a valid token through Kong returns `200` with the correct caller profile.
- The same call without a token returns `401`.
- A forged `x-user-id` header is stripped and never influences the response.
- Public endpoints (`/register`, `/login`, JWKS, health) remain accessible without a token.
- No PEM key material appears in committed Kong configuration.
- `make verify-edge-auth` passes; Identity register/login/JWKS tests pass.

## 16. References

- [ADR-007: First-party Identity Service](../../adr/007-first-party-identity-service.md)
- [Hybrid REST/BFF/gRPC edge-routing design](../2026-07-30-hybrid-rest-grpc-edge-routing-design.md)
- [Identity A: Register, Login and JWKS design](../2026-08-03-identity-register-login-jwks-design.md)

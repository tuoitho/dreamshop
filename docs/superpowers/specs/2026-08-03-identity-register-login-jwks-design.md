# Identity A: Register, Login and JWKS Design

## 1. Summary

This design defines the first functional vertical slice of the Identity Service. It delivers user registration, password login, access-token issuance and a public JWKS endpoint. The slice is intentionally smaller than the complete Identity roadmap so credential storage and JWT signing can be implemented and verified before refresh-session lifecycle and protected profile APIs are added.

The externally demonstrable flow is:

```text
Client ──POST /api/v1/auth/register──► Kong ─► Identity ─► identity_db
Client ──POST /api/v1/auth/login─────► Kong ─► Identity ─► signed access JWT
Client ──GET  /api/v1/auth/.well-known/jwks.json ────────► public JWK Set
Verifier ─────────────────────────────► verifies JWT signature and claims
```

This design follows [ADR-006](../../adr/006-error-model-and-idempotency.md) for safe REST errors and [ADR-007](../../adr/007-first-party-identity-service.md) for the first-party Identity boundary.

## 2. Goals

- Register a first-party customer with a unique normalized email, an Argon2id password hash, a profile and the default `CUSTOMER` role.
- Authenticate email/password credentials without revealing whether an account exists.
- Issue a short-lived RSA-signed access JWT containing stable identity and authorization claims.
- Publish only the corresponding public key as a standards-compatible JWK Set.
- Exercise the complete path through validation, application logic, PostgreSQL, key loading, Kong routing and automated tests.
- Establish contracts that can be extended by refresh/logout and profile slices without changing the meaning of existing endpoints or claims.

## 3. Non-goals

This slice does not implement:

- refresh tokens, token families, rotation, revocation or logout;
- `GET/PATCH /api/v1/users/me`;
- password reset, email verification, MFA, social login or third-party OAuth/OIDC clients;
- a generic OAuth/OIDC authorization server or Spring Authorization Server;
- runtime key-rotation operations or an administration API;
- Kong JWT enforcement, BFF authentication middleware or Order authorization;
- login/register rate limiting or account lockout;
- browser cookie sessions or CSRF-based authentication.

Those items remain separate slices. In particular, refresh-session tables must not be created speculatively in this slice.

## 4. Architectural decisions

### 4.1 Application shape

Identity remains a Spring Boot service and uses:

- Spring MVC for the three public HTTP endpoints;
- Spring Security for stateless endpoint policy and password encoding;
- Spring Data JPA for the small account aggregate;
- Flyway as the sole owner of database schema migrations;
- PostgreSQL `identity_db` as the source of truth;
- Spring Security's Nimbus-based JWT encoder for RSA signing.

The service is not an OAuth authorization server. It exposes a deliberately small first-party REST contract.

Suggested package boundaries:

```text
com.ecommerce.identity
├── auth
│   ├── api             request/response DTOs and AuthController
│   ├── application     RegisterUser and Login use cases
│   └── domain          account status and normalization policies
├── user
│   └── persistence     JPA entities and repositories
├── security
│   ├── PasswordConfig
│   ├── JwtKeyProvider
│   ├── JwtTokenService
│   └── JwksController
└── common
    ├── config          validated typed configuration
    └── error           exception mapping and trace-id provider
```

HTTP DTOs, JPA entities and JWT claims are separate representations. Controllers do not call repositories or cryptographic APIs directly.

### 4.2 Transaction boundaries

Registration is one database transaction that creates the account, profile and `CUSTOMER` membership. A failure in any write rolls back the complete registration.

Login is read-only with respect to persistent state in this slice. It loads credentials and roles, verifies the password and signs a token. Login audit/session persistence is deferred rather than adding a partial refresh-session model.

## 5. Public HTTP contract

Kong currently routes `/api/v1/auth` to Identity with `strip_path: true`. Therefore the public and upstream paths are:

| Capability | Public path | Identity upstream path |
|---|---|---|
| Register | `POST /api/v1/auth/register` | `POST /register` |
| Login | `POST /api/v1/auth/login` | `POST /login` |
| JWKS | `GET /api/v1/auth/.well-known/jwks.json` | `GET /.well-known/jwks.json` |

### 5.1 Register

Request:

```json
{
  "email": "user@example.com",
  "password": "a sufficiently long password",
  "full_name": "Nguyen Van A"
}
```

Validation and normalization:

- `email` is required, trimmed, syntactically validated and limited to 254 characters;
- the canonical lookup value is `trim(email).toLowerCase(Locale.ROOT)`;
- `password` is required, accepts Unicode, is 12–128 characters and has no composition rule;
- `full_name` is trimmed, required and 1–100 characters;
- raw request values and validation failures must never log the password.

Success returns HTTP `201 Created`:

```json
{
  "user_id": "9c798f79-c122-4c0b-8db4-156b263fcce0",
  "email": "user@example.com",
  "full_name": "Nguyen Van A",
  "roles": ["CUSTOMER"]
}
```

Registration does not issue a token. This keeps account creation and authentication independently testable.

### 5.2 Login

Request:

```json
{
  "email": "user@example.com",
  "password": "a sufficiently long password"
}
```

Success returns HTTP `200 OK`:

```json
{
  "access_token": "<compact-jwt>",
  "token_type": "Bearer",
  "expires_in": 900
}
```

The access token is returned only in the response body. It is not written to a cookie, database or application log.

Unknown email, wrong password and a disabled account all return the same `401 INVALID_CREDENTIALS` status, code and safe message. For an unknown email, the service verifies the supplied password against a valid dummy Argon2id hash created once at startup. This reduces, but does not claim to eliminate, timing-based account enumeration.

### 5.3 JWKS

Success returns HTTP `200 OK` with `Content-Type: application/json` and a standard JWK Set:

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "alg": "RS256",
      "kid": "identity-dev-2026-01",
      "n": "<base64url-modulus>",
      "e": "AQAB"
    }
  ]
}
```

Only public-key material is serialized. The endpoint returns `Cache-Control: public, max-age=300`. The first slice publishes one active key; the JWK Set shape and `kid` lookup allow a future rotation window to publish old and new public keys together.

## 6. Persistence model

Flyway owns an initial migration with these tables:

### `identity_users`

| Column | Type and constraint |
|---|---|
| `id` | `uuid primary key` |
| `email` | `varchar(254) not null` |
| `normalized_email` | `varchar(254) not null unique` |
| `password_hash` | `varchar(255) not null` |
| `status` | `varchar(32) not null`, initially `ACTIVE` or `DISABLED` |
| `created_at` | `timestamptz not null` |
| `updated_at` | `timestamptz not null` |

### `identity_profiles`

| Column | Type and constraint |
|---|---|
| `user_id` | `uuid primary key` and FK to `identity_users(id)` |
| `full_name` | `varchar(100) not null` |

### `roles`

| Column | Type and constraint |
|---|---|
| `id` | small generated numeric primary key |
| `code` | `varchar(32) not null unique` |

The migration seeds exactly one initial role, `CUSTOMER`.

### `user_roles`

| Column | Type and constraint |
|---|---|
| `user_id` | FK to `identity_users(id)` |
| `role_id` | FK to `roles(id)` |

The composite primary key is `(user_id, role_id)`. There is no cross-service foreign key. The database unique constraint on `normalized_email`, rather than an application pre-check, is authoritative under concurrent registration.

## 7. Password storage

- Passwords use Argon2id with a unique random salt embedded in the encoded hash.
- The initial parameters are salt length 16 bytes, hash length 32 bytes, memory 19 MiB, two iterations and parallelism one. They are explicit configuration rather than an implicit library default.
- Before production use, the parameters must be benchmarked on the actual deployment target and raised when feasible without creating an authentication denial-of-service risk.
- Stored values use Spring Security's delegating password format with an algorithm identifier so a later login can upgrade an older encoding.
- Password hashes are never decrypted, returned, logged or included in events.

The parameter baseline follows the OWASP Password Storage Cheat Sheet. Spring Security's `Argon2PasswordEncoder` requires Bouncy Castle, which is an explicit runtime dependency.

## 8. Access-token contract

The service signs access tokens using RSA and `RS256` through Spring Security's `NimbusJwtEncoder`. It does not implement JWS signing manually.

Required JOSE headers:

| Header | Value |
|---|---|
| `alg` | `RS256` |
| `typ` | `JWT` |
| `kid` | configured active key identifier |

Required claims:

| Claim | Meaning |
|---|---|
| `sub` | Identity user UUID as a string |
| `roles` | array of stable role codes, initially `CUSTOMER` |
| `iss` | exact configured issuer |
| `aud` | contains the configured audience, initially `ecommerce-api` |
| `iat` | issue instant |
| `exp` | issue instant plus configured access TTL, default 15 minutes |
| `jti` | cryptographically unpredictable token UUID |

The token deliberately excludes email, full name, password data and infrastructure details. Verifiers must validate signature, accepted algorithm, `kid`, issuer, audience and expiry; decoding claims without validation is never sufficient.

## 9. Key and configuration lifecycle

Identity reads an existing RSA key pair. It never silently creates a new persistent key during normal startup because that would invalidate every token after a restart.

Required configuration:

```text
IDENTITY_DB_URL
IDENTITY_DB_USERNAME
IDENTITY_DB_PASSWORD
IDENTITY_JWT_ISSUER
IDENTITY_JWT_AUDIENCE
IDENTITY_JWT_PRIVATE_KEY_PATH
IDENTITY_JWT_PUBLIC_KEY_PATH
IDENTITY_JWT_KEY_ID
IDENTITY_JWT_ACCESS_TTL
```

Startup validation fails fast when:

- either key file is absent or unreadable;
- the public key does not match the private key;
- the RSA key is smaller than 2048 bits;
- issuer, audience or `kid` is blank;
- the access TTL is non-positive.

Local development uses an idempotent PowerShell helper that creates an RSA development key pair in a git-ignored directory when none exists. Compose mounts the files read-only. Tests generate an ephemeral key pair inside the test process and never depend on a developer key. `.env.example` documents paths and non-secret identifiers but contains no private key.

## 10. Endpoint security policy

Spring Security uses stateless request handling:

- permit `/register`, `/login`, `/.well-known/jwks.json` and health/readiness endpoints;
- deny every other request by default;
- do not create an HTTP session;
- disable form login, HTTP Basic and logout handlers;
- disable CSRF for these bearer-token JSON APIs because this slice does not authenticate with cookies.

This slice issues tokens but does not yet configure Identity as a resource server. Protected profile endpoints will add JWT validation in the later profile slice.

## 11. Error and logging contract

Every error uses the accepted REST envelope:

```json
{
  "code": "INVALID_CREDENTIALS",
  "message": "Email or password is invalid.",
  "trace_id": "<trace identifier>"
}
```

| Condition | HTTP | Stable code |
|---|---:|---|
| malformed or invalid input | 400 | `VALIDATION_FAILED` |
| normalized email already exists | 409 | `EMAIL_ALREADY_REGISTERED` |
| unknown email, wrong password or disabled account | 401 | `INVALID_CREDENTIALS` |
| signing/key service unexpectedly unavailable | 503 | `AUTHENTICATION_UNAVAILABLE` |
| unclassified server failure | 500 | `INTERNAL_ERROR` |

The global exception mapper handles both an application-level duplicate check and the database unique-constraint race as the same `409` result. Internal exception messages, SQL, hostnames and key paths never appear in client responses.

The `VALIDATION_FAILED` response may additionally include an `errors` map whose keys are Java bean property names and whose values are the corresponding validation messages, e.g. `{"code":"VALIDATION_FAILED","message":"Request validation failed","errors":{"password":"size must be between 12 and 128"},"trace_id":"<trace identifier>"}`. Field names and validation messages are safe to expose; rejected values, SQL and internal exception text still never appear. Malformed request bodies return the envelope without the `errors` key.

Logs are structured and may include `trace_id`, `user_id` after successful identification, outcome and stable error code. They must not include email, password, password hash, complete JWT, private key or request body. Until OpenTelemetry is installed, a request-scoped trace-id provider supplies the envelope/log identifier; it must be replaceable by the active span trace ID without changing the API.

## 12. Detailed flows

### 12.1 Register

1. Kong strips `/api/v1/auth` and forwards `/register`.
2. Identity validates the DTO and normalizes email/full name.
3. The application service hashes the password before constructing persistence data.
4. In one transaction, it inserts `identity_users`, `identity_profiles` and the `CUSTOMER` membership.
5. A concurrent unique violation is translated to `EMAIL_ALREADY_REGISTERED`.
6. The service returns the safe user representation with HTTP 201.

### 12.2 Login

1. Kong forwards `/login`.
2. Identity normalizes the email and fetches the account with roles.
3. Missing accounts take the dummy-hash verification path.
4. Existing active accounts verify the stored Argon2id encoding.
5. Failed verification or non-active status returns the same generic error.
6. The token service builds headers/claims, signs with the active private key and returns the token metadata.

### 12.3 Verify with JWKS

1. A verifier fetches the public JWK Set and caches it according to response headers.
2. It selects a key by the token's `kid` and only accepts the configured RSA algorithm.
3. It verifies the signature, issuer, audience and time claims.
4. Only after successful validation does it use `sub` and `roles` to build caller context.

## 13. Test strategy

### Unit tests

- email/full-name normalization and input boundaries;
- account status policy;
- Argon2id encode/match and delegating encoding identifier;
- JWT headers, claims and TTL calculation;
- error mapping and safe messages.

### PostgreSQL integration tests

Use Testcontainers PostgreSQL so migrations and database constraints are exercised against the same database family as local deployment:

- Flyway migrates an empty `identity_db` and seeds `CUSTOMER`;
- registration persists account/profile/role atomically;
- raw passwords never appear in persisted columns;
- the same password produces different encoded hashes for different accounts;
- normalized duplicate and concurrent duplicate registration create at most one account;
- a forced profile/role failure rolls back the account row.

### HTTP/security integration tests

- valid register returns 201 and the safe schema;
- malformed register/login input returns the stable error envelope;
- correct credentials return an RS256 access token;
- unknown email, wrong password and disabled user are indistinguishable at the HTTP-contract level;
- unauthorized/unmapped endpoints are denied;
- JWKS exposes only public parameters;
- the JWKS key matching `kid` verifies the login token;
- wrong issuer, audience, signature and expired tokens fail verifier tests.

### End-to-end smoke test

With Compose running, a PowerShell smoke test calls register and login through Kong, fetches JWKS through Kong, and verifies the returned JWT. The script must not print the password or full token. Re-running with the same email must observe the documented 409 response.

## 14. Operational behavior

- Readiness is DOWN when database migration/connectivity or required key loading is unavailable; liveness does not depend on PostgreSQL.
- A signing failure is fail-closed: login does not return an unsigned token or alternate weak algorithm.
- Database errors are not retried inside an HTTP request except through normal connection-pool behavior.
- The service does not cache credentials, password hashes or account existence in Redis.
- JWT access TTL is configurable from 1 through 30 minutes; the default is 15 minutes. Values outside that range fail startup validation.

## 15. Delivery boundaries and follow-on slices

The next Identity slice adds refresh-token sessions, token-family rotation/revocation, reuse detection and logout. It consumes the existing `user_id`, issuer/audience and access-token contract but adds new tables and endpoints.

The following slice adds `GET/PATCH /api/v1/users/me`, configures Identity/BFF as JWT resource servers, and wires ownership checks. Kong JWT enforcement and rate limiting are integrated only after token verification behavior is proven in the service and test client.

## 16. Acceptance criteria

- Register through Kong creates exactly one active user, profile and `CUSTOMER` membership.
- Passwords are stored only as configured Argon2id encodings.
- Login through Kong returns a 15-minute RSA-signed access JWT with every required header and claim.
- Unknown email, wrong password and disabled account share one public failure contract.
- JWKS through Kong verifies the login token without access to private-key material.
- Restarting Identity with the same mounted key preserves token verifiability.
- Startup fails on missing, mismatched or undersized keys.
- Unit, PostgreSQL integration, HTTP/security integration and end-to-end smoke tests pass.
- No secret, private key, password, password hash or complete access token is committed or logged.
- Refresh/logout/profile behavior and tables remain outside this implementation.

## 17. References

- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [Spring Security password storage](https://docs.spring.io/spring-security/reference/features/authentication/password-storage.html)
- [Spring Security `NimbusJwtEncoder`](https://docs.spring.io/spring-security/reference/api/java/org/springframework/security/oauth2/jwt/NimbusJwtEncoder.html)
- [RFC 7517: JSON Web Key](https://datatracker.ietf.org/doc/html/rfc7517)
- [RFC 7519: JSON Web Token](https://datatracker.ietf.org/doc/html/rfc7519)

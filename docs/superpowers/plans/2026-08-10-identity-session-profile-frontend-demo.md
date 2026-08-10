# Identity Session, Profile, and Frontend Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Deliver renewable, revocable current-browser sessions and a profile demo through Kong, with Identity on Windows and infrastructure in Docker.

**Architecture:** Identity returns an access JWT in JSON and an opaque refresh token only through an HttpOnly cookie. Refresh sessions store SHA-256 token hashes and rotate under a pessimistic lock; using an already-rotated token strictly revokes its family. React keeps access state only in memory, bootstraps from the refresh cookie, and retries protected 401 responses once through one shared refresh operation.

**Tech Stack:** Java 21, Spring Boot, Spring Security, JPA, Flyway, PostgreSQL, Kong, PowerShell, React, TypeScript, Axios, Vitest, React Testing Library.

---

## File map

| Path | Responsibility |
|---|---|
| services/identity-service/src/main/resources/db/migration/V2__create_refresh_token_sessions.sql | Refresh-session schema. |
| services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentitySessionProperties.java | Refresh TTL and cookie configuration. |
| services/identity-service/src/main/java/com/ecommerce/identity/auth/persistence/ | Session state and locking repository. |
| services/identity-service/src/main/java/com/ecommerce/identity/auth/application/RefreshTokenService.java | Issue, rotate, strict reuse, logout. |
| services/identity-service/src/main/java/com/ecommerce/identity/user/api/UpdateMeRequest.java | Profile update contract. |
| deployments/kong/kong.yaml, Makefile, scripts/verify-edge-auth.ps1 | Hybrid gateway and smoke verification. |
| ecommerce-fe/src/lib/auth-session.ts | In-memory token plus refresh single-flight. |
| ecommerce-fe/src/lib/api.ts | Credentialed API and one-shot retry. |
| ecommerce-fe/src/context/auth-provider.tsx | Bootstrap and profile state. |
| ecommerce-fe/src/pages/profile-page.tsx | Profile demo. |

### Task 1: Add refresh-session schema, config, and persistence

**Files:**
- Create: services/identity-service/src/main/resources/db/migration/V2__create_refresh_token_sessions.sql
- Create: services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentitySessionProperties.java
- Create: services/identity-service/src/main/java/com/ecommerce/identity/auth/persistence/RefreshTokenRevocationReason.java
- Create: services/identity-service/src/main/java/com/ecommerce/identity/auth/persistence/RefreshTokenSessionEntity.java
- Create: services/identity-service/src/main/java/com/ecommerce/identity/auth/persistence/RefreshTokenSessionRepository.java
- Create: services/identity-service/src/test/java/com/ecommerce/identity/auth/persistence/RefreshTokenSessionPersistenceTest.java
- Modify: services/identity-service/src/main/resources/application.yml
- Modify: services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java
- Modify: services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java

- [ ] **Step 1: Write failing migration tests.**

Create a Testcontainers test that asserts the table and unique token-hash constraint exist:

~~~java
@Test
void createsRefreshSessionsWithUniqueTokenHash() {
    assertThat(jdbc.sql("""
        select table_name from information_schema.tables where table_schema = 'public'
        """).query(String.class).list()).contains("refresh_token_sessions");
    assertThat(jdbc.sql("""
        select conname from pg_constraint
        where conrelid = 'refresh_token_sessions'::regclass and contype = 'u'
        """).query(String.class).list()).contains("uq_refresh_token_sessions_token_hash");
}
~~~

Run:

~~~powershell
mvn -q -pl services/identity-service -am test -Dtest=IdentityDatabaseMigrationTest,RefreshTokenSessionPersistenceTest -Dsurefire.failIfNoSpecifiedTests=false
~~~

Expected: FAIL because the table is absent.

- [ ] **Step 2: Add the migration and configuration.**

Create this migration:

~~~sql
create table refresh_token_sessions (
    id uuid primary key,
    user_id uuid not null references identity_users(id) on delete cascade,
    family_id uuid not null,
    token_hash varchar(64) not null,
    issued_at timestamptz not null,
    expires_at timestamptz not null,
    revoked_at timestamptz,
    replaced_by_session_id uuid references refresh_token_sessions(id),
    revocation_reason varchar(32),
    constraint uq_refresh_token_sessions_token_hash unique (token_hash),
    constraint ck_refresh_token_sessions_revocation check (
      (revoked_at is null and revocation_reason is null)
      or (revoked_at is not null and revocation_reason in ('ROTATED','LOGOUT','REUSE_DETECTED'))
    )
);
create index ix_refresh_token_sessions_family_active
  on refresh_token_sessions (family_id) where revoked_at is null;
create index ix_refresh_token_sessions_user_expires
  on refresh_token_sessions (user_id, expires_at);
~~~

Add the validated configuration record:

~~~java
@ConfigurationProperties("identity.session")
public record IdentitySessionProperties(Duration refreshTtl, String cookieName,
        String cookiePath, boolean cookieSecure) {
    public IdentitySessionProperties {
        if (refreshTtl == null || refreshTtl.isNegative() || refreshTtl.isZero()) {
            throw new IllegalArgumentException("Refresh TTL must be positive");
        }
        if (cookieName == null || cookieName.isBlank()
                || cookiePath == null || cookiePath.isBlank()) {
            throw new IllegalArgumentException("Refresh cookie name and path are required");
        }
    }
}
~~~

Configure a 30-day TTL, cookie name refresh_token, path /api/v1/auth, and false secure flag in local/test configuration.

- [ ] **Step 3: Implement entity and locked repository.**

Use revocation enum values ROTATED, LOGOUT, and REUSE_DETECTED. The entity factory receives ids, hash, and times. It must expose activeAt(now), rotate(now, replacementId), and revoke(now, reason).

Use exact repository operations:

~~~java
@Lock(LockModeType.PESSIMISTIC_WRITE)
@Query("select s from RefreshTokenSessionEntity s where s.tokenHash = :tokenHash")
Optional<RefreshTokenSessionEntity> findForUpdateByTokenHash(String tokenHash);

@Modifying
@Query("""
    update RefreshTokenSessionEntity s
    set s.revokedAt = :now, s.revocationReason = :reason
    where s.familyId = :familyId and s.revokedAt is null
    """)
int revokeActiveFamily(UUID familyId, Instant now, RefreshTokenRevocationReason reason);
~~~

- [ ] **Step 4: Verify and commit.**

~~~powershell
mvn -q -pl services/identity-service -am test -Dtest=IdentityDatabaseMigrationTest,RefreshTokenSessionPersistenceTest -Dsurefire.failIfNoSpecifiedTests=false
git add services/identity-service/src/main/resources/db/migration/V2__create_refresh_token_sessions.sql services/identity-service/src/main/resources/application.yml services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentitySessionProperties.java services/identity-service/src/main/java/com/ecommerce/identity/auth/persistence services/identity-service/src/test/java/com/ecommerce/identity/auth/persistence services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java
git commit -m "feat(identity): persist refresh token sessions"
~~~

Expected: tests pass.

### Task 2: Implement issuance, rotation, strict reuse detection, and logout

**Files:**
- Create: services/identity-service/src/main/java/com/ecommerce/identity/auth/domain/InvalidRefreshTokenException.java
- Create: services/identity-service/src/main/java/com/ecommerce/identity/auth/application/RefreshTokenGenerator.java
- Create: services/identity-service/src/main/java/com/ecommerce/identity/auth/application/AuthenticatedSession.java
- Create: services/identity-service/src/main/java/com/ecommerce/identity/auth/application/RefreshTokenService.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/auth/application/LoginService.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java
- Create test: services/identity-service/src/test/java/com/ecommerce/identity/auth/application/RefreshTokenServiceTest.java
- Modify test: services/identity-service/src/test/java/com/ecommerce/identity/auth/application/LoginServiceTest.java

- [ ] **Step 1: Write failing lifecycle tests.**

Require login to produce both values:

~~~java
public record AuthenticatedSession(AccessTokenResult accessToken, String rawRefreshToken) {}
~~~

Require rotation to retain family and reuse to revoke all active sessions:

~~~java
AuthenticatedSession replacement = service.rotate(rawToken);
assertThat(replacement.accessToken()).isEqualTo(new AccessTokenResult("new-jwt", 900));
assertThat(sessionByHash(rawToken).revocationReason()).isEqualTo(ROTATED);
assertThat(sessionByHash(replacement.rawRefreshToken()).familyId()).isEqualTo(original.familyId());

assertThatThrownBy(() -> service.rotate(rawToken))
    .isInstanceOf(InvalidRefreshTokenException.class);
assertThat(activeSessions(original.familyId())).isEmpty();
~~~

Run:

~~~powershell
mvn -q -pl services/identity-service -am test -Dtest=LoginServiceTest,RefreshTokenServiceTest -Dsurefire.failIfNoSpecifiedTests=false
~~~

Expected: FAIL because lifecycle types do not exist.

- [ ] **Step 2: Implement secure opaque tokens and lifecycle.**

Generate 32 random bytes with SecureRandom, Base64URL encode without padding, and persist only SHA-256 hex:

~~~java
byte[] bytes = new byte[32];
secureRandom.nextBytes(bytes);
String raw = Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
String hash = HexFormat.of().formatHex(
    MessageDigest.getInstance("SHA-256").digest(raw.getBytes(StandardCharsets.UTF_8)));
~~~

Login issues a JWT and creates an initial session in one transaction. Rotation locks by hash and follows this exact policy:

1. Missing, expired, LOGOUT, or reuse-revoked rows throw InvalidRefreshTokenException.
2. An active row becomes ROTATED, a successor in the same family is saved, then a new JWT is issued.
3. A ROTATED row calls revokeActiveFamily with REUSE_DETECTED and throws InvalidRefreshTokenException.

Logout revokes only an active matching row with LOGOUT and returns normally for every other input. Map invalid refresh to 401 INVALID_REFRESH_TOKEN with message Refresh token is invalid or expired.

- [ ] **Step 3: Add concurrency test.**

Use two TransactionTemplate executions and a CountDownLatch. Begin B before A commits; A rotates and commits; B gets the lock and rotates. Assert B fails, the replacement is REUSE_DETECTED, and no active family session remains. Do not add a grace interval.

- [ ] **Step 4: Verify and commit.**

~~~powershell
mvn -q -pl services/identity-service -am test -Dtest=LoginServiceTest,RefreshTokenServiceTest -Dsurefire.failIfNoSpecifiedTests=false
git add services/identity-service/src/main/java/com/ecommerce/identity/auth services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/auth/application
git commit -m "feat(identity): rotate refresh sessions and detect reuse"
~~~

Expected: tests pass.

### Task 3: Expose refresh/logout cookies and PATCH users/me

**Files:**
- Create: services/identity-service/src/main/java/com/ecommerce/identity/user/api/UpdateMeRequest.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/auth/api/AuthController.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/user/api/MeController.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/user/application/MeService.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserEntity.java
- Modify: services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityProfileEntity.java
- Create test: services/identity-service/src/test/java/com/ecommerce/identity/auth/api/RefreshApiTest.java
- Modify test: services/identity-service/src/test/java/com/ecommerce/identity/user/api/MeApiTest.java

- [ ] **Step 1: Write failing MockMvc tests.**

Test login Set-Cookie includes HttpOnly, SameSite=Lax, Path=/api/v1/auth, and configured secure flag. Test refresh returns a JWT and replacement cookie; absent, expired, and reused cookies return 401 INVALID_REFRESH_TOKEN plus Max-Age=0. Test logout always returns 204 and clear cookie.

Add PATCH tests:

~~~java
mockMvc.perform(patch("/users/me").header("X-User-Id", userId)
        .contentType(MediaType.APPLICATION_JSON)
        .content("{\"full_name\":\"Updated Name\"}"))
    .andExpect(status().isOk())
    .andExpect(jsonPath("$.user_id").value(userId))
    .andExpect(jsonPath("$.full_name").value("Updated Name"));

mockMvc.perform(patch("/users/me").header("X-User-Id", userId)
        .contentType(MediaType.APPLICATION_JSON)
        .content("{\"full_name\":\"   \"}"))
    .andExpect(status().isBadRequest())
    .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"));
~~~

Run:

~~~powershell
mvn -q -pl services/identity-service -am test -Dtest=RefreshApiTest,MeApiTest,LoginApiTest -Dsurefire.failIfNoSpecifiedTests=false
~~~

Expected: FAIL because routes are absent.

- [ ] **Step 2: Implement endpoint and profile behavior.**

Build response cookies from configuration:

~~~java
ResponseCookie.from(properties.cookieName(), rawToken)
    .httpOnly(true).secure(properties.cookieSecure()).sameSite("Lax")
    .path(properties.cookiePath()).maxAge(properties.refreshTtl()).build();
~~~

A clear cookie has empty value and maxAge(Duration.ZERO). Login sets the first cookie; refresh reads the configured cookie, rotates, and sets replacement; logout always clears cookie and returns 204.

Create this request:

~~~java
public record UpdateMeRequest(
    @JsonProperty("full_name") @NotBlank @Size(max = 100) String fullName) {
    public UpdateMeRequest { fullName = fullName == null ? null : fullName.trim(); }
}
~~~

Add rename to IdentityProfileEntity, updateFullName to IdentityUserEntity, and update(UUID, String) to MeService using Clock.instant. PATCH must use CallerContextPrincipal alone for identity. Permit POST refresh/logout and PATCH users/me in SecurityConfiguration.

- [ ] **Step 3: Verify and commit.**

~~~powershell
mvn -q -pl services/identity-service -am test -Dtest=RefreshApiTest,MeApiTest,LoginApiTest -Dsurefire.failIfNoSpecifiedTests=false
git add services/identity-service/src/main/java/com/ecommerce/identity/auth/api services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java services/identity-service/src/main/java/com/ecommerce/identity/user services/identity-service/src/test/java/com/ecommerce/identity/auth/api services/identity-service/src/test/java/com/ecommerce/identity/user/api
git commit -m "feat(identity): expose refresh logout and profile update"
~~~

Expected: tests pass.

### Task 4: Restore hybrid operations and extend Kong smoke verification

**Files:**
- Modify: deployments/docker/compose.foundation.yml
- Modify: deployments/kong/kong.yaml
- Modify: Makefile
- Modify: .env.example
- Modify: scripts/verify-identity.ps1
- Modify: scripts/verify-edge-auth.ps1
- Modify: README.md
- Modify: ecommerce-fe/README.md

- [ ] **Step 1: Extend smoke assertions first.**

Retain login cookie; POST it to /auth/refresh and require a new access JWT. PATCH /users/me with that JWT and full_name Updated Edge Auth Smoke Test. POST /auth/logout with latest cookie and require 204. A final refresh using it must return 401 INVALID_REFRESH_TOKEN.

~~~powershell
pwsh -NoProfile -File scripts/verify-edge-auth.ps1
~~~

Expected: current implementation fails new assertions.

- [ ] **Step 2: Normalize local configuration.**

Compose retains only PostgreSQL, Redis, RabbitMQ, and Kong; remove commented application service blocks. Kong config uses exactly:

~~~yaml
url: http://host.docker.internal:8080
jwks_uri: http://host.docker.internal:8080/.well-known/jwks.json
issuer: http://localhost:8080
~~~

Keep credentialed CORS for http://localhost:5173 and Authorization header. Do not add rate limiting.

Make targets are infra-up, infra-down, infra-logs, identity, order, identity-keys, proto, lint, verify, verify-identity, verify-edge-auth, up, down, help. Use up: infra-up and down: infra-down. Identity calls pwsh -NoProfile -File services/identity-service/start-local.ps1. Local environment issuer is http://localhost:8080 and refresh-cookie Secure is false.

- [ ] **Step 3: Verify and commit.**

Start in separate terminals: make identity-keys, make infra-up, make identity. Then run:

~~~powershell
make verify-identity
make verify-edge-auth
~~~

Expected: both success messages print and Kong reaches Windows Identity through host.docker.internal.

~~~powershell
git add deployments/docker/compose.foundation.yml deployments/kong/kong.yaml Makefile .env.example scripts/verify-identity.ps1 scripts/verify-edge-auth.ps1 README.md ecommerce-fe/README.md services/identity-service/start-local.ps1
git commit -m "chore(local): restore hybrid identity verification workflow"
~~~

### Task 5: Replace browser local storage with memory-only auth and single-flight refresh

**Files:**
- Create: ecommerce-fe/src/lib/auth-session.ts
- Create test: ecommerce-fe/src/lib/auth-session.test.ts
- Modify: ecommerce-fe/src/lib/api.ts
- Modify: ecommerce-fe/src/lib/api.test.ts
- Modify: ecommerce-fe/src/lib/types.ts
- Delete: ecommerce-fe/src/lib/token-storage.ts
- Delete: ecommerce-fe/src/lib/token-storage.test.ts

- [ ] **Step 1: Write failing session and interceptor tests.**

Require getAccessToken, setAccessToken, clearAccessToken, runSingleFlightRefresh. Prove no localStorage writes and only one refresh occurs:

~~~ts
const refresh = vi.fn().mockResolvedValue("new-jwt")
const [first, second] = await Promise.all([
  runSingleFlightRefresh(refresh), runSingleFlightRefresh(refresh),
])
expect(first).toBe("new-jwt")
expect(second).toBe("new-jwt")
expect(refresh).toHaveBeenCalledTimes(1)
~~~

Require Axios withCredentials true, Bearer injection from memory, one protected retry after 401, and no refresh after a 401 from /auth/login.

~~~powershell
Set-Location ecommerce-fe
npm run test -- src/lib/auth-session.test.ts src/lib/api.test.ts
~~~

Expected: FAIL because local storage and no retry interceptor are used.

- [ ] **Step 2: Implement frontend API contract.**

Add:

~~~ts
export interface CurrentUser { user_id: string; email: string; full_name: string; roles: string[] }
export interface UpdateProfileRequest { full_name: string }
~~~

auth-session owns module-scoped accessToken and refreshInFlight; refreshInFlight resets in finally. API uses Axios withCredentials true. A separate client without response interceptor posts /auth/refresh and updates memory. Retry only when status is 401, URL does not start /auth/, and private _retried is false; await the shared refresh, update Authorization, resubmit once. If refresh fails, clear memory and rethrow original error.

Export fetchCurrentUser, updateCurrentUser, logoutRequest using existing ApiClientError mapping, then delete both storage files.

- [ ] **Step 3: Verify and commit.**

~~~powershell
Set-Location ecommerce-fe
npm run test -- src/lib/auth-session.test.ts src/lib/api.test.ts
git add src/lib
git commit -m "feat(frontend): refresh in-memory access sessions"
~~~

Expected: client tests pass.

### Task 6: Implement bootstrap and protected profile demo

**Files:**
- Modify: ecommerce-fe/src/context/auth-context.ts
- Modify: ecommerce-fe/src/context/auth-provider.tsx
- Modify: ecommerce-fe/src/routes/require-auth.tsx
- Create: ecommerce-fe/src/pages/profile-page.tsx
- Create test: ecommerce-fe/src/pages/profile-page.test.tsx
- Modify: ecommerce-fe/src/App.tsx
- Modify: ecommerce-fe/src/pages/login-page.tsx
- Modify: ecommerce-fe/src/pages/login-page.test.tsx
- Delete: ecommerce-fe/src/pages/dashboard-page.tsx

- [ ] **Step 1: Write failing provider and profile tests.**

Mock refresh, profile fetch, profile update, and logout. Require provider initially ready=false, authenticated after refresh/profile success, anonymous after refresh failure with no alert. Require profile page to render name/email/roles, submit full_name Updated Name, render returned value, and await logout.

~~~powershell
Set-Location ecommerce-fe
npm run test -- src/pages/profile-page.test.tsx src/pages/login-page.test.tsx
~~~

Expected: FAIL because profile state and page do not exist.

- [ ] **Step 2: Implement state, routes, and UI.**

Context contract:

~~~ts
interface AuthContextValue {
  token: string | null
  user: CurrentUser | null
  ready: boolean
  isAuthenticated: boolean
  login(payload: LoginRequest): Promise<void>
  register(payload: RegisterRequest): Promise<void>
  updateProfile(payload: UpdateProfileRequest): Promise<void>
  logout(): Promise<void>
}
~~~

Use a useRef guard around bootstrap to handle StrictMode; refresh then fetch current user. Any bootstrap error clears memory/user; finally set ready. Login stores JWT then fetches profile. Logout calls endpoint and clears state in finally.

RequireAuth shows Đang khôi phục phiên... while not ready, redirects anonymous users, then renders children. Profile page must not render a JWT. Route /profile is protected; login default becomes /profile; /dashboard redirects to /profile. Delete health-check dashboard.

- [ ] **Step 3: Verify and commit.**

~~~powershell
Set-Location ecommerce-fe
npm run test
npm run build
git add src/context src/routes src/pages src/App.tsx
git commit -m "feat(frontend): demo renewable profile session"
~~~

Expected: tests and build pass.

### Task 7: Complete regression checks and the manual demo

**Files:**
- Modify: README.md
- Modify: ecommerce-fe/README.md

- [ ] **Step 1: Run regression suites.**

~~~powershell
mvn -q -pl services/identity-service -am test
Set-Location ecommerce-fe
npm run lint
npm run test
npm run build
~~~

Expected: BUILD SUCCESS; frontend lint, tests, and build pass.

- [ ] **Step 2: Perform end-to-end demo.**

Start infra and Identity as Task 4. Start Vite, register a fresh account, login, reload /profile, update the name, logout, and reload /profile. Expected: session restores after login; logout reload redirects to login.

- [ ] **Step 3: Update documentation and commit.**

Document refresh/logout and GET/PATCH users/me, local cookie security configuration, strict concurrent-refresh behavior, and hybrid startup sequence.

~~~powershell
git add README.md ecommerce-fe/README.md
git commit -m "docs: describe identity session profile demo"
~~~

## Plan self-review

- **Spec coverage:** Tasks 1–3 cover persisted hashed sessions, strict rotation/reuse, cookies, logout, and profile update. Task 4 covers Kong, CORS, hybrid commands, and smoke verification. Tasks 5–6 cover memory-only frontend auth, retry, bootstrap, profile UI, and logout. Task 7 verifies every acceptance criterion.
- **Scope:** Rate limiting, all-device logout, email/password/role changes, checkout, and storefront changes are excluded.
- **Type consistency:** Backend uses AuthenticatedSession, AccessTokenResult, CurrentUser, UpdateMeRequest, and RefreshTokenRevocationReason. Frontend uses CurrentUser, UpdateProfileRequest, ready, and refresh single-flight consistently.


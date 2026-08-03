# Identity Register, Login and JWKS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a production-shaped Identity vertical slice that registers customers, authenticates passwords, issues short-lived RSA access JWTs and publishes the matching public JWKS through Kong.

**Architecture:** Keep HTTP DTOs, application use cases, persistence entities and cryptographic configuration separate inside `identity-service`. PostgreSQL and Flyway own account state, Spring Security owns password/JWT primitives, and Kong remains a path-routing edge without JWT enforcement in this slice. Register is one write transaction; login is read-only and fails closed if signing is unavailable.

**Tech Stack:** Java 21, Spring Boot 4.1, Spring MVC/Security/Data JPA, Flyway, PostgreSQL 16, Argon2id/Bouncy Castle, Spring Security Nimbus JOSE, Testcontainers 2, JUnit 5, MockMvc, Docker Compose, Kong and PowerShell 7.

---

## Execution precondition

The current main worktree contains staged Phase-0/BFF changes, including the Spring Boot 4.1 upgrade, `/api/v1/*` Kong routes and Identity local port `8090`. Preserve and land that work before executing this plan. Execute the plan from a dedicated feature worktree created from the updated `main`; do not mix Identity commits into the current dirty worktree.

The approved design is [`docs/superpowers/specs/2026-08-03-identity-register-login-jwks-design.md`](../specs/2026-08-03-identity-register-login-jwks-design.md). A contract change requires updating and re-reviewing that design before continuing.

## File map

Existing files to modify:

- `services/identity-service/pom.xml`: runtime/test dependencies.
- `services/identity-service/src/main/java/com/ecommerce/identity/IdentityApplication.java`: property scanning.
- `services/identity-service/src/main/resources/application.yml`: datasource, Flyway, JWT and health configuration.
- `services/identity-service/src/test/java/com/ecommerce/identity/IdentityApplicationTest.java`: real-PostgreSQL context test.
- `.env.example`, `Makefile`, `deployments/docker/compose.foundation.yml`, `README.md`: local operations and docs.

New production areas:

- `services/identity-service/src/main/resources/db/migration/V1__create_identity_accounts.sql`.
- `services/identity-service/src/main/java/com/ecommerce/identity/common/{config,error}/`.
- `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/`.
- `services/identity-service/src/main/java/com/ecommerce/identity/auth/{api,application,domain}/`.
- `services/identity-service/src/main/java/com/ecommerce/identity/security/`.
- `services/identity-service/src/test/java/com/ecommerce/identity/{support,user,auth,security}/`.
- `scripts/generate-identity-dev-keys.ps1` and `scripts/verify-identity.ps1`.

### Task 1: Add typed JWT configuration and security libraries

**Files:**

- Modify: `services/identity-service/pom.xml`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/IdentityApplication.java`
- Modify: `services/identity-service/src/main/resources/application.yml`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentityJwtProperties.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/common/config/IdentityJwtPropertiesTest.java`

- [ ] **Step 1: Write the failing property test**

```java
package com.ecommerce.identity.common.config;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.file.Path;
import java.time.Duration;
import org.junit.jupiter.api.Test;

class IdentityJwtPropertiesTest {
    private static final Path PRIVATE_KEY = Path.of("private.pem");
    private static final Path PUBLIC_KEY = Path.of("public.pem");

    @Test
    void rejectsAccessTtlOutsideOneToThirtyMinutes() {
        assertThatThrownBy(() -> properties(Duration.ofSeconds(59)))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("1 through 30 minutes");
        assertThatThrownBy(() -> properties(Duration.ofMinutes(31)))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("1 through 30 minutes");
    }

    @Test
    void rejectsBlankIssuerAudienceAndKeyId() {
        assertThatThrownBy(() -> new IdentityJwtProperties(
                " ", "ecommerce-api", PRIVATE_KEY, PUBLIC_KEY, "key-1", Duration.ofMinutes(15)))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> new IdentityJwtProperties(
                "identity", " ", PRIVATE_KEY, PUBLIC_KEY, "key-1", Duration.ofMinutes(15)))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> new IdentityJwtProperties(
                "identity", "ecommerce-api", PRIVATE_KEY, PUBLIC_KEY, " ", Duration.ofMinutes(15)))
                .isInstanceOf(IllegalArgumentException.class);
    }

    private IdentityJwtProperties properties(Duration ttl) {
        return new IdentityJwtProperties(
                "http://localhost:8090", "ecommerce-api", PRIVATE_KEY, PUBLIC_KEY, "key-1", ttl);
    }
}
```

- [ ] **Step 2: Prove the type is missing**

Run: `mvn -pl services/identity-service -Dtest=IdentityJwtPropertiesTest test`

Expected: FAIL because `IdentityJwtProperties` does not exist.

- [ ] **Step 3: Add dependencies controlled by the Spring Boot 4.1 BOM**

```xml
<dependency>
    <groupId>org.springframework.security</groupId>
    <artifactId>spring-security-oauth2-jose</artifactId>
</dependency>
<dependency>
    <groupId>org.springframework.security</groupId>
    <artifactId>spring-security-crypto</artifactId>
</dependency>
<dependency>
    <groupId>org.bouncycastle</groupId>
    <artifactId>bcprov-jdk18on</artifactId>
    <version>1.85</version>
</dependency>
<dependency>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-configuration-processor</artifactId>
    <optional>true</optional>
</dependency>
<dependency>
    <groupId>org.springframework.security</groupId>
    <artifactId>spring-security-test</artifactId>
    <scope>test</scope>
</dependency>
```

- [ ] **Step 4: Implement the property record**

```java
package com.ecommerce.identity.common.config;

import java.nio.file.Path;
import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties("identity.jwt")
public record IdentityJwtProperties(
        String issuer, String audience, Path privateKeyPath, Path publicKeyPath,
        String keyId, Duration accessTtl) {
    public IdentityJwtProperties {
        requireText(issuer, "issuer");
        requireText(audience, "audience");
        requireText(keyId, "key-id");
        if (privateKeyPath == null || publicKeyPath == null) {
            throw new IllegalArgumentException("JWT key paths are required");
        }
        if (accessTtl == null || accessTtl.compareTo(Duration.ofMinutes(1)) < 0
                || accessTtl.compareTo(Duration.ofMinutes(30)) > 0) {
            throw new IllegalArgumentException("JWT access TTL must be 1 through 30 minutes");
        }
    }

    private static void requireText(String value, String name) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException("JWT " + name + " is required");
        }
    }
}
```

Add `@ConfigurationPropertiesScan` to `IdentityApplication`. Replace `application.yml` with:

```yaml
server:
  port: ${SERVER_PORT:8090}
spring:
  datasource:
    url: ${IDENTITY_DB_URL:jdbc:postgresql://localhost:5432/identity_db}
    username: ${IDENTITY_DB_USERNAME:ecommerce}
    password: ${IDENTITY_DB_PASSWORD:change-me-local-only}
  jpa:
    open-in-view: false
    hibernate:
      ddl-auto: validate
  flyway:
    enabled: true
identity:
  jwt:
    issuer: ${IDENTITY_JWT_ISSUER:http://localhost:8090}
    audience: ${IDENTITY_JWT_AUDIENCE:ecommerce-api}
    private-key-path: ${IDENTITY_JWT_PRIVATE_KEY_PATH:deployments/secrets/identity/private.pem}
    public-key-path: ${IDENTITY_JWT_PUBLIC_KEY_PATH:deployments/secrets/identity/public.pem}
    key-id: ${IDENTITY_JWT_KEY_ID:identity-dev-2026-01}
    access-ttl: ${IDENTITY_JWT_ACCESS_TTL:15m}
management:
  endpoints:
    web:
      exposure:
        include: health,info
  endpoint:
    health:
      probes:
        enabled: true
      validate-group-membership: false
      group:
        readiness:
          include: readinessState,db
```

- [ ] **Step 5: Verify and commit**

Run: `mvn -pl services/identity-service -Dtest=IdentityJwtPropertiesTest,IdentityApplicationTest test`

Expected: PASS; DB/key loaders are not active yet.

```powershell
git add services/identity-service/pom.xml services/identity-service/src/main/java/com/ecommerce/identity/IdentityApplication.java services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentityJwtProperties.java services/identity-service/src/main/resources/application.yml services/identity-service/src/test/java/com/ecommerce/identity/common/config/IdentityJwtPropertiesTest.java
git commit -m "feat(identity): add validated auth configuration"
```

### Task 2: Create the Flyway schema and PostgreSQL test foundation

**Files:**

- Modify: `services/identity-service/pom.xml`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/IdentityApplicationTest.java`
- Create: `services/identity-service/src/main/resources/db/migration/V1__create_identity_accounts.sql`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`

- [ ] **Step 1: Add the database dependencies**

```xml
<dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-data-jpa</artifactId></dependency>
<dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-flyway</artifactId></dependency>
<dependency><groupId>org.flywaydb</groupId><artifactId>flyway-database-postgresql</artifactId></dependency>
<dependency><groupId>org.postgresql</groupId><artifactId>postgresql</artifactId><scope>runtime</scope></dependency>
<dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-testcontainers</artifactId><scope>test</scope></dependency>
<dependency><groupId>org.testcontainers</groupId><artifactId>testcontainers-junit-jupiter</artifactId><scope>test</scope></dependency>
<dependency><groupId>org.testcontainers</groupId><artifactId>testcontainers-postgresql</artifactId><scope>test</scope></dependency>
```

- [ ] **Step 2: Write the shared container and failing migration test**

```java
@Testcontainers
@SpringBootTest
public abstract class PostgresIntegrationTestSupport {
    @Container
    @ServiceConnection
    protected static final PostgreSQLContainer<?> POSTGRES =
            new PostgreSQLContainer<>("postgres:16-alpine")
                    .withDatabaseName("identity_test")
                    .withUsername("identity")
                    .withPassword("identity");
}
```

Import `PostgreSQLContainer` from the Testcontainers 2 package `org.testcontainers.postgresql.PostgreSQLContainer`, not the deprecated 1.x compatibility package `org.testcontainers.containers`.

```java
class IdentityDatabaseMigrationTest extends PostgresIntegrationTestSupport {
    @Autowired JdbcClient jdbc;

    @Test
    void createsAccountTablesAndSeedsCustomerRole() {
        List<String> tables = jdbc.sql("""
                select table_name from information_schema.tables where table_schema = 'public'
                """).query(String.class).list();
        assertThat(tables).contains("identity_users", "identity_profiles", "roles", "user_roles");
        assertThat(jdbc.sql("select code from roles").query(String.class).list())
                .containsExactly("CUSTOMER");
    }
}
```

Run: `mvn -pl services/identity-service -Dtest=IdentityDatabaseMigrationTest test`

Expected: FAIL because the migration/tables do not exist.

- [ ] **Step 3: Add the complete migration**

```sql
create table identity_users (
    id uuid primary key,
    email varchar(254) not null,
    normalized_email varchar(254) not null,
    password_hash varchar(255) not null,
    status varchar(32) not null,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    constraint uq_identity_users_normalized_email unique (normalized_email),
    constraint ck_identity_users_status check (status in ('ACTIVE', 'DISABLED'))
);
create table identity_profiles (
    user_id uuid primary key references identity_users(id) on delete cascade,
    full_name varchar(100) not null
);
create table roles (
    id smallint primary key,
    code varchar(32) not null,
    constraint uq_roles_code unique (code)
);
create table user_roles (
    user_id uuid not null references identity_users(id) on delete cascade,
    role_id smallint not null references roles(id),
    primary key (user_id, role_id)
);
insert into roles (id, code) values (1, 'CUSTOMER');
```

- [ ] **Step 4: Move the context test to PostgreSQL, verify and commit**

Make `IdentityApplicationTest extends PostgresIntegrationTestSupport`, preserving its `contextLoads()` test.

Run: `mvn -pl services/identity-service -Dtest=IdentityDatabaseMigrationTest,IdentityApplicationTest test`

Expected: PASS with one successful Flyway migration.

```powershell
git add services/identity-service/pom.xml services/identity-service/src/main/resources/db/migration services/identity-service/src/test/java/com/ecommerce/identity/IdentityApplicationTest.java services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java
git commit -m "feat(identity): add account schema migrations"
```

### Task 3: Map the account aggregate and prove persistence invariants

**Files:**

- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/domain/AccountStatus.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/{IdentityUserEntity,IdentityProfileEntity,RoleEntity,IdentityUserRepository,RoleRepository}.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityPersistenceTest.java`

- [ ] **Step 1: Write the failing repository tests**

Test one aggregate save/reload and one duplicate normalized email:

```java
@Transactional
class IdentityPersistenceTest extends PostgresIntegrationTestSupport {
    @Autowired IdentityUserRepository users;
    @Autowired RoleRepository roles;

    @Test
    void savesProfileAndCustomerRoleWithAccount() {
        RoleEntity customer = roles.findByCode("CUSTOMER").orElseThrow();
        var user = IdentityUserEntity.create(UUID.randomUUID(), "User@example.com",
                "user@example.com", "{argon2id}hash", AccountStatus.ACTIVE,
                Instant.parse("2026-08-03T00:00:00Z"), "Nguyen Van A", Set.of(customer));
        users.saveAndFlush(user);
        var reloaded = users.findWithRolesByNormalizedEmail("user@example.com").orElseThrow();
        assertThat(reloaded.fullName()).isEqualTo("Nguyen Van A");
        assertThat(reloaded.roleCodes()).containsExactly("CUSTOMER");
    }

    @Test
    void databaseRejectsDuplicateNormalizedEmail() {
        RoleEntity customer = roles.findByCode("CUSTOMER").orElseThrow();
        users.saveAndFlush(user("A@example.com", "same@example.com", customer));
        assertThatThrownBy(() -> users.saveAndFlush(user("B@example.com", "same@example.com", customer)))
                .isInstanceOf(DataIntegrityViolationException.class);
    }
}
```

Run: `mvn -pl services/identity-service -Dtest=IdentityPersistenceTest test`

Expected: FAIL because entities and repositories are missing.

- [ ] **Step 2: Implement exact mappings**

`AccountStatus` contains only `ACTIVE` and `DISABLED`. `IdentityUserEntity` maps `identity_users` and provides:

```java
public static IdentityUserEntity create(UUID id, String email, String normalizedEmail,
        String passwordHash, AccountStatus status, Instant now, String fullName, Set<RoleEntity> roles)
public UUID id()
public String email()
public String normalizedEmail()
public String passwordHash()
public AccountStatus status()
public String fullName()
public Set<String> roleCodes()
public void disable(Instant now)
```

Use `@Enumerated(EnumType.STRING)`, a cascading/orphan-removing one-to-one profile with `@MapsId`, and a non-cascading many-to-many role mapping through `user_roles`. Use protected JPA constructors, reject null factory arguments, store supplied instants, and return a sorted immutable role-code set.

Repositories are:

```java
public interface IdentityUserRepository extends JpaRepository<IdentityUserEntity, UUID> {
    @EntityGraph(attributePaths = {"profile", "roles"})
    Optional<IdentityUserEntity> findWithRolesByNormalizedEmail(String normalizedEmail);

    long countByNormalizedEmail(String normalizedEmail);
}
```

```java
public interface RoleRepository extends JpaRepository<RoleEntity, Short> {
    Optional<RoleEntity> findByCode(String code);
}
```

- [ ] **Step 3: Verify and commit persistence**

Run: `mvn -pl services/identity-service -Dtest=IdentityPersistenceTest test`

Expected: PASS; Hibernate validation matches Flyway and PostgreSQL rejects the duplicate.

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/auth/domain/AccountStatus.java services/identity-service/src/main/java/com/ecommerce/identity/user/persistence services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityPersistenceTest.java
git commit -m "feat(identity): map customer account persistence"
```

### Task 4: Implement transactional registration and Argon2id

**Files:**

- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/domain/{EmailNormalizer,EmailAlreadyRegisteredException}.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/application/{RegisterCommand,RegisteredUser,RegisterUserService}.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/security/PasswordConfiguration.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/auth/application/RegisterUserServiceTest.java`

- [ ] **Step 1: Write failing registration tests**

Use real PostgreSQL and assert:

```java
RegisteredUser result = service.register(new RegisterCommand(
        " User@Example.com ", "correct horse battery staple", " Nguyen Van A "));
var stored = users.findWithRolesByNormalizedEmail("user@example.com").orElseThrow();
assertThat(result.email()).isEqualTo("user@example.com");
assertThat(result.roles()).containsExactly("CUSTOMER");
assertThat(stored.passwordHash()).startsWith("{argon2id}");
assertThat(stored.passwordHash()).doesNotContain("correct horse battery staple");
assertThat(passwordEncoder.matches("correct horse battery staple", stored.passwordHash())).isTrue();
```

Register `USER@example.com` after `user@example.com` and assert `EmailAlreadyRegisteredException`. Add a test that forces missing `CUSTOMER` role and asserts no user/profile row commits.

Also add:

- two registrations with the same raw password and assert their stored Argon2id strings differ while both verify;
- a two-thread test synchronized by `CyclicBarrier` that submits the same normalized email, then asserts one success, one `EmailAlreadyRegisteredException`, and exactly one matching database row.

Run: `mvn -pl services/identity-service -Dtest=RegisterUserServiceTest test`

Expected: FAIL at compilation.

- [ ] **Step 2: Configure password encoding and clock**

```java
@Configuration
public class PasswordConfiguration {
    @Bean
    PasswordEncoder passwordEncoder() {
        PasswordEncoder argon2id = new Argon2PasswordEncoder(16, 32, 1, 19_456, 2);
        return new DelegatingPasswordEncoder("argon2id", Map.of("argon2id", argon2id));
    }

    @Bean
    Clock clock() {
        return Clock.systemUTC();
    }
}
```

- [ ] **Step 3: Implement registration**

Use these immutable boundaries:

```java
public record RegisterCommand(String email, String password, String fullName) {}
public record RegisteredUser(UUID userId, String email, String fullName, Set<String> roles) {}
```

`EmailNormalizer.normalize` returns `email.trim().toLowerCase(Locale.ROOT)`. `RegisterUserService.register`, annotated `@Transactional`, loads `CUSTOMER`, encodes once, creates the UUID aggregate using injected `Clock`, calls `saveAndFlush`, translates the normalized-email constraint violation to a parameterless `EmailAlreadyRegisteredException`, and returns no credential material. Do not translate unrelated integrity violations.

- [ ] **Step 4: Verify and commit registration**

Run: `mvn -pl services/identity-service -Dtest=RegisterUserServiceTest,IdentityPersistenceTest test`

Expected: PASS; duplicate normalization creates at most one account.

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/auth services/identity-service/src/main/java/com/ecommerce/identity/security/PasswordConfiguration.java services/identity-service/src/test/java/com/ecommerce/identity/auth/application/RegisterUserServiceTest.java
git commit -m "feat(identity): register customer accounts"
```

### Task 5: Expose register with validation and safe errors

**Files:**

- Modify: `services/identity-service/pom.xml`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/api/{AuthController,RegisterRequest,RegisterResponse}.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/{ApiError,TraceIdProvider,TraceIdFilter,GlobalExceptionHandler}.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/auth/api/RegisterApiTest.java`

- [ ] **Step 1: Write failing MockMvc tests**

Extend `PostgresIntegrationTestSupport`, use `@AutoConfigureMockMvc`, and assert valid register returns 201, normalized email and `CUSTOMER` without a password field. Assert invalid input returns `400/VALIDATION_FAILED/trace_id`; duplicate returns `409/EMAIL_ALREADY_REGISTERED` without SQL or exception names.

Run: `mvn -pl services/identity-service -Dtest=RegisterApiTest test`

Expected: FAIL with 401/404.

- [ ] **Step 2: Implement DTOs and controller**

```java
public record RegisterRequest(
        @NotBlank @Email @Size(max = 254) String email,
        @NotBlank @Size(min = 12, max = 128) String password,
        @JsonProperty("full_name") @NotBlank @Size(max = 100) String fullName) {}
```

```java
public record RegisterResponse(
        @JsonProperty("user_id") UUID userId, String email,
        @JsonProperty("full_name") String fullName, Set<String> roles) {
    static RegisterResponse from(RegisteredUser user) {
        return new RegisterResponse(user.userId(), user.email(), user.fullName(), user.roles());
    }
}
```

`AuthController` maps `POST /register`, validates with `@Valid`, calls `RegisterUserService` and returns HTTP 201.

- [ ] **Step 3: Implement trace and exception contracts**

```java
public record ApiError(String code, String message, @JsonProperty("trace_id") String traceId) {}
```

`TraceIdFilter` extends `OncePerRequestFilter`, creates a lowercase 32-hex UUID, stores it in MDC key `trace_id`, sets `X-Trace-Id`, and removes MDC in `finally`. `TraceIdProvider.current()` reads MDC with a generated fallback for non-HTTP tests.

`GlobalExceptionHandler` maps invalid/unreadable input to `400 VALIDATION_FAILED`, duplicate email to `409 EMAIL_ALREADY_REGISTERED`, and unclassified failure to `500 INTERNAL_ERROR`. Use fixed safe messages; never serialize `BindingResult`, rejected values or exception messages.

- [ ] **Step 4: Add deny-by-default stateless security**

Add the servlet security starter now that its explicit public-path policy is implemented in the same task:

```xml
<dependency>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-security</artifactId>
</dependency>
```

```java
@Bean
SecurityFilterChain securityFilterChain(HttpSecurity http) throws Exception {
    return http
            .csrf(AbstractHttpConfigurer::disable)
            .sessionManagement(s -> s.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
            .formLogin(AbstractHttpConfigurer::disable)
            .httpBasic(AbstractHttpConfigurer::disable)
            .logout(AbstractHttpConfigurer::disable)
            .authorizeHttpRequests(auth -> auth
                    .requestMatchers(HttpMethod.POST, "/register", "/login").permitAll()
                    .requestMatchers(HttpMethod.GET, "/.well-known/jwks.json",
                            "/actuator/health", "/actuator/health/**").permitAll()
                    .anyRequest().denyAll())
            .build();
}
```

- [ ] **Step 5: Verify and commit register HTTP**

Run: `mvn -pl services/identity-service -Dtest=RegisterApiTest,RegisterUserServiceTest test`

Expected: PASS; response bodies never contain raw password values.

```powershell
git add services/identity-service/pom.xml services/identity-service/src/main/java/com/ecommerce/identity/auth/api services/identity-service/src/main/java/com/ecommerce/identity/common/error services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java services/identity-service/src/test/java/com/ecommerce/identity/auth/api/RegisterApiTest.java
git commit -m "feat(identity): expose customer registration API"
```

### Task 6: Load RSA keys and issue standards-shaped access tokens

**Files:**

- Create: `services/identity-service/src/main/java/com/ecommerce/identity/security/{SigningKeyMaterial,PemRsaKeyLoader,JwtConfiguration,JwtTokenService}.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/application/AccessTokenResult.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/domain/AuthenticationUnavailableException.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/support/TestRsaKeyFiles.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/security/{PemRsaKeyLoaderTest,JwtTokenServiceTest}.java`

- [ ] **Step 1: Create ephemeral test keys and failing loader tests**

`TestRsaKeyFiles.create(int bits)` uses `KeyPairGenerator.getInstance("RSA")`, writes PKCS#8 private PEM and X.509 public PEM under `Files.createTempDirectory`, and returns both paths plus the key pair. It never writes under the repository.

Test matching 2048-bit load, mismatched key rejection, 1024-bit rejection and missing-file rejection. Expect `IllegalStateException`; messages may name a path but must not contain PEM content.

Run: `mvn -pl services/identity-service -Dtest=PemRsaKeyLoaderTest test`

Expected: FAIL because the loader is missing.

- [ ] **Step 2: Implement validated key material**

```java
public record SigningKeyMaterial(
        RSAPublicKey publicKey,
        RSAPrivateKey privateKey,
        RSAKey publicJwk) {}
```

`PemRsaKeyLoader.load(IdentityJwtProperties)` must remove only expected PEM markers/whitespace, Base64-decode, build public X.509 and private PKCS#8 keys with `KeyFactory.getInstance("RSA")`, compare moduli, reject fewer than 2048 bits, and build:

```java
RSAKey publicJwk = new RSAKey.Builder(publicKey)
        .keyUse(KeyUse.SIGNATURE)
        .algorithm(JWSAlgorithm.RS256)
        .keyID(properties.keyId())
        .build();
```

- [ ] **Step 3: Configure Spring Security's encoder**

```java
@Configuration
public class JwtConfiguration {
    @Bean
    SigningKeyMaterial signingKeyMaterial(PemRsaKeyLoader loader, IdentityJwtProperties properties) {
        return loader.load(properties);
    }

    @Bean
    JwtEncoder jwtEncoder(SigningKeyMaterial keys, IdentityJwtProperties properties) {
        return NimbusJwtEncoder.withKeyPair(keys.publicKey(), keys.privateKey())
                .algorithm(SignatureAlgorithm.RS256)
                .jwkPostProcessor(jwk -> jwk.keyID(properties.keyId()))
                .build();
    }
}
```

- [ ] **Step 4: Write the failing token test and implement issuance**

With a fixed clock, assert RS256, `typ=JWT`, configured `kid`, UUID subject, sorted `CUSTOMER` roles, issuer, single audience, exact `iat/exp`, nonblank `jti`, and 900 seconds.

Add a second test with a `JwtEncoder` stub that throws `JwtEncodingException`; assert the service throws `AuthenticationUnavailableException` and exposes no encoder message.

```java
public record AccessTokenResult(String token, long expiresInSeconds) {}
```

`JwtTokenService.issue(UUID, Set<String>)` builds `JwsHeader` and `JwtClaimsSet`, signs through `JwtEncoder`, and returns the compact value plus TTL seconds. It must not log the compact token. Catch `JwtEncodingException` and throw a parameterless `AuthenticationUnavailableException`; map that exception to `503 AUTHENTICATION_UNAVAILABLE` with the safe message `Authentication is temporarily unavailable.`. Do not fall back to an unsigned token, symmetric key or another algorithm.

- [ ] **Step 5: Give integration contexts ephemeral key paths**

Add one static `TestRsaKeyFiles` fixture and these `@DynamicPropertySource` values to `PostgresIntegrationTestSupport`:

```java
registry.add("identity.jwt.private-key-path", () -> KEYS.privateKeyPath().toString());
registry.add("identity.jwt.public-key-path", () -> KEYS.publicKeyPath().toString());
registry.add("identity.jwt.key-id", () -> "identity-test-key");
registry.add("identity.jwt.issuer", () -> "identity-test");
registry.add("identity.jwt.audience", () -> "ecommerce-api");
registry.add("identity.jwt.access-ttl", () -> "15m");
```

- [ ] **Step 6: Verify and commit signing**

Run: `mvn -pl services/identity-service -Dtest=PemRsaKeyLoaderTest,JwtTokenServiceTest,IdentityApplicationTest test`

Expected: PASS; undersized/mismatched keys fail closed, while tests use only temp keys.

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/security services/identity-service/src/main/java/com/ecommerce/identity/auth/application/AccessTokenResult.java services/identity-service/src/main/java/com/ecommerce/identity/auth/domain/AuthenticationUnavailableException.java services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/security services/identity-service/src/test/java/com/ecommerce/identity/support
git commit -m "feat(identity): sign short lived access tokens"
```

### Task 7: Implement generic-failure login and expose `/login`

**Files:**

- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/domain/InvalidCredentialsException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/application/{LoginCommand,LoginService}.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/auth/api/{LoginRequest,LoginResponse}.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/auth/api/AuthController.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/auth/application/LoginServiceTest.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/auth/api/LoginApiTest.java`

- [ ] **Step 1: Write failing service tests**

Cover correct credentials, wrong password, unknown email and disabled account. Mock only `JwtTokenService`; assert it is invoked only for success. Spy `PasswordEncoder` and assert `matches` runs once for known and unknown email, preserving the dummy-hash path.

Run: `mvn -pl services/identity-service -Dtest=LoginServiceTest test`

Expected: FAIL because login types are missing.

- [ ] **Step 2: Implement login**

```java
public record LoginCommand(String email, String password) {}
```

`LoginService` creates one dummy encoded password in its constructor. `login` normalizes email, loads user/profile/roles, calls `matches` with the stored or dummy hash, rejects missing/disabled/mismatch through one parameterless `InvalidCredentialsException`, and calls `JwtTokenService.issue` only for an active match. Do not distinguish failure cases in logs.

- [ ] **Step 3: Write failing HTTP tests**

After registering a customer, assert:

```java
mockMvc.perform(post("/login")
        .contentType(MediaType.APPLICATION_JSON)
        .content("""{"email":"user@example.com","password":"correct horse battery staple"}"""))
    .andExpect(status().isOk())
    .andExpect(jsonPath("$.access_token").isNotEmpty())
    .andExpect(jsonPath("$.token_type").value("Bearer"))
    .andExpect(jsonPath("$.expires_in").value(900));
```

Unknown email, wrong password and disabled account must each return 401, `INVALID_CREDENTIALS`, exactly `Email or password is invalid.`, and a non-empty trace ID.

Mock `JwtTokenService` to throw `AuthenticationUnavailableException` for otherwise valid credentials and assert `503 AUTHENTICATION_UNAVAILABLE`; the response must contain neither key path nor encoder exception detail.
/
- [ ] **Step 4: Implement login DTO and mapping**

```java
public record LoginRequest(
        @NotBlank @Email @Size(max = 254) String email,
        @NotBlank @Size(max = 128) String password) {}
```

```java
public record LoginResponse(
        @JsonProperty("access_token") String accessToken,
        @JsonProperty("token_type") String tokenType,
        @JsonProperty("expires_in") long expiresIn) {
    static LoginResponse from(AccessTokenResult token) {
        return new LoginResponse(token.token(), "Bearer", token.expiresInSeconds());
    }
}
```

Add `POST /login` to `AuthController`; map `InvalidCredentialsException` to the fixed 401 contract. Never echo supplied email/password.

- [ ] **Step 5: Verify and commit login**

Run: `mvn -pl services/identity-service -Dtest=LoginServiceTest,LoginApiTest,RegisterApiTest test`

Expected: PASS.

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/auth services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/auth
git commit -m "feat(identity): authenticate customer credentials"
```

### Task 8: Publish JWKS and verify the login token

**Files:**

- Create: `services/identity-service/src/main/java/com/ecommerce/identity/security/JwksController.java`
- Test: `services/identity-service/src/test/java/com/ecommerce/identity/security/JwksApiTest.java`

- [ ] **Step 1: Write the failing JWKS test**

Register/login through MockMvc, fetch `/.well-known/jwks.json`, and assert status 200, public five-minute caching, `kty=RSA`, `use=sig`, `alg=RS256`, configured `kid`, nonempty `n/e`, and absent `d/p/q/dp/dq/qi`.

Parse the JWK Set with Nimbus, choose by token `kid`, configure a decoder with timestamp, issuer and audience validators, then assert verified `sub` and `CUSTOMER`. Assert modified signature, wrong issuer, wrong audience and expired fixtures fail.

Run: `mvn -pl services/identity-service -Dtest=JwksApiTest test`

Expected: FAIL with 404.

- [ ] **Step 2: Implement public-key-only JWKS**

```java
@RestController
public class JwksController {
    private final SigningKeyMaterial keys;

    public JwksController(SigningKeyMaterial keys) {
        this.keys = keys;
    }

    @GetMapping("/.well-known/jwks.json")
    ResponseEntity<Map<String, Object>> jwks() {
        Map<String, Object> body = new JWKSet(keys.publicJwk().toPublicJWK()).toJSONObject();
        return ResponseEntity.ok()
                .cacheControl(CacheControl.maxAge(Duration.ofMinutes(5)).cachePublic())
                .body(body);
    }
}
```

- [ ] **Step 3: Verify and commit JWKS**

Run: `mvn -pl services/identity-service -Dtest=RegisterApiTest,LoginApiTest,JwksApiTest,PemRsaKeyLoaderTest,JwtTokenServiceTest test`

Expected: PASS and no private RSA fields.

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/security/JwksController.java services/identity-service/src/test/java/com/ecommerce/identity/security/JwksApiTest.java
git commit -m "feat(identity): publish public jwks"
```

### Task 9: Generate local keys and wire Identity Compose dependencies

**Files:**

- Modify: `.env.example`
- Modify: `Makefile`
- Modify: `deployments/docker/compose.foundation.yml`
- Create: `scripts/generate-identity-dev-keys.ps1`

- [ ] **Step 1: Prove the key helper is missing**

Run: `pwsh -NoProfile -File scripts/generate-identity-dev-keys.ps1`

Expected: FAIL because the script does not exist.

- [ ] **Step 2: Implement idempotent .NET RSA generation**

```powershell
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$keyDir = Join-Path $repoRoot 'deployments/secrets/identity'
$privatePath = Join-Path $keyDir 'private.pem'
$publicPath = Join-Path $keyDir 'public.pem'

if ((Test-Path -LiteralPath $privatePath) -xor (Test-Path -LiteralPath $publicPath)) {
    throw 'Identity key pair is incomplete; remove the remaining file deliberately and rerun.'
}
if ((Test-Path -LiteralPath $privatePath) -and (Test-Path -LiteralPath $publicPath)) {
    Write-Host 'Identity development key pair already exists.'
    exit 0
}

New-Item -ItemType Directory -Force -Path $keyDir | Out-Null
$rsa = [System.Security.Cryptography.RSA]::Create(2048)
try {
    [System.IO.File]::WriteAllText($privatePath, $rsa.ExportPkcs8PrivateKeyPem())
    [System.IO.File]::WriteAllText($publicPath, $rsa.ExportSubjectPublicKeyInfoPem())
} finally {
    $rsa.Dispose()
}
Write-Host 'Created untracked Identity development key pair.'
```

- [ ] **Step 3: Add safe `.env.example` values**

```dotenv
IDENTITY_DB_URL=jdbc:postgresql://localhost:5432/identity_db
IDENTITY_DB_USERNAME=ecommerce
IDENTITY_DB_PASSWORD=change-me-local-only
IDENTITY_JWT_PRIVATE_KEY_PATH=deployments/secrets/identity/private.pem
IDENTITY_JWT_PUBLIC_KEY_PATH=deployments/secrets/identity/public.pem
IDENTITY_JWT_KEY_ID=identity-dev-2026-01
IDENTITY_JWT_ACCESS_TTL=15m
```

Keep issuer/audience; add no PEM content.

- [ ] **Step 4: Wire Make and Compose**

Add `identity-keys` to `.PHONY`, make `up` depend on it, and add:

```makefile
identity-keys:
	pwsh -NoProfile -File scripts/generate-identity-dev-keys.ps1
```

Replace the Identity Compose block:

```yaml
  identity-service:
    build:
      context: ../..
      dockerfile: services/identity-service/Dockerfile
    container_name: identity-service
    environment:
      SERVER_PORT: "8080"
      IDENTITY_DB_URL: jdbc:postgresql://postgres:5432/identity_db
      IDENTITY_DB_USERNAME: ${POSTGRES_USER:-ecommerce}
      IDENTITY_DB_PASSWORD: ${POSTGRES_PASSWORD:-change-me-local-only}
      IDENTITY_JWT_ISSUER: ${IDENTITY_JWT_ISSUER:-http://identity-service:8080}
      IDENTITY_JWT_AUDIENCE: ${IDENTITY_JWT_AUDIENCE:-ecommerce-api}
      IDENTITY_JWT_PRIVATE_KEY_PATH: /run/secrets/identity/private.pem
      IDENTITY_JWT_PUBLIC_KEY_PATH: /run/secrets/identity/public.pem
      IDENTITY_JWT_KEY_ID: ${IDENTITY_JWT_KEY_ID:-identity-dev-2026-01}
      IDENTITY_JWT_ACCESS_TTL: ${IDENTITY_JWT_ACCESS_TTL:-15m}
    volumes:
      - ../secrets/identity:/run/secrets/identity:ro
    networks:
      - ecommerce-net
    depends_on:
      postgres:
        condition: service_healthy
```

Do not publish an Identity host port.

- [ ] **Step 5: Verify idempotency/ignore/Compose and commit**

Run:

```powershell
pwsh -NoProfile -File scripts/generate-identity-dev-keys.ps1
pwsh -NoProfile -File scripts/generate-identity-dev-keys.ps1
git check-ignore deployments/secrets/identity/private.pem deployments/secrets/identity/public.pem
docker compose -f deployments/docker/compose.foundation.yml config --quiet
```

Expected: both runs exit 0, the second preserves the pair, both PEM files are ignored, Compose validates.

```powershell
git add .env.example Makefile deployments/docker/compose.foundation.yml scripts/generate-identity-dev-keys.ps1
git commit -m "chore(identity): wire local database and signing keys"
```

Check `git status --short` before commit; no PEM may be staged.

### Task 10: Add a Kong smoke verifier and runnable docs

**Files:**

- Create: `scripts/verify-identity.ps1`
- Modify: `Makefile`
- Modify: `README.md`

- [ ] **Step 1: Add a failing Make target**

```makefile
verify-identity:
	pwsh -NoProfile -File scripts/verify-identity.ps1
```

Run: `make verify-identity`

Expected: FAIL because the script is missing.

- [ ] **Step 2: Implement the non-leaking smoke verifier**

The script accepts `-BaseUrl` default `http://localhost:8000/api/v1/auth` and `-ExpectedIssuer` default `http://identity-service:8080`. It generates a unique `@example.test` email and local-only password, registers (201), logs in (200), fetches JWKS, selects by `kid`, verifies RS256 using .NET `RSA.VerifyData`, checks `sub`, `roles`, issuer, audience and future expiry, then repeats registration and checks `409 EMAIL_ALREADY_REGISTERED`.

Use:

```powershell
function ConvertFrom-Base64Url([string] $Value) {
    $padded = $Value.Replace('-', '+').Replace('_', '/')
    switch ($padded.Length % 4) {
        2 { $padded += '==' }
        3 { $padded += '=' }
        1 { throw 'Invalid Base64URL value.' }
    }
    return [Convert]::FromBase64String($padded)
}
```

Construct `RSAParameters` from decoded `n/e`, verify ASCII bytes of `<header>.<payload>` with SHA-256 and PKCS#1 padding, and use `Invoke-WebRequest -SkipHttpErrorCheck` for expected 409. Print only `Identity register/login/JWKS smoke checks passed.`; never print request password/body or token.

- [ ] **Step 3: Document exact commands and limitations**

Add README section `Identity A: register, login and JWKS` with `make identity-keys`, `make up`, `make verify-identity`; the three URLs; fake example bodies; local 8090 versus container 8080; 15-minute default TTL; local-only PEM warning; and explicit absence of refresh/logout/profile/Kong JWT enforcement.

- [ ] **Step 4: Run Compose smoke and commit**

```powershell
make up
docker compose -f deployments/docker/compose.foundation.yml ps
make verify-identity
```

Expected: PostgreSQL, Identity and Kong run; verifier prints its one success line.

```powershell
git add Makefile README.md scripts/verify-identity.ps1
git commit -m "test(identity): verify auth flow through Kong"
```

### Task 11: Complete repository verification and handoff

**Files:**

- Modify only files required to correct failures found below.

- [ ] **Step 1: Run clean Identity tests**

Run: `mvn -pl services/identity-service -am clean test`

Expected: BUILD SUCCESS; migration, persistence, register, login, key, JWT and JWKS tests pass with Docker available.

- [ ] **Step 2: Run foundation regression tests**

```powershell
mvn test
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
pwsh -NoProfile -File scripts/verify-foundation.ps1
```

Expected: all exit 0. Mockito agent warnings are not failures.

- [ ] **Step 3: Rebuild and run both smoke suites**

```powershell
make up
make verify
make verify-identity
```

Expected: foundation and Identity routes pass through Kong.

- [ ] **Step 4: Prove secret and scope safety**

```powershell
git status --short
git ls-files "*.pem" "*.key" ".env"
git diff --check
git diff --name-only main...HEAD
```

Expected: no secret files tracked; no whitespace errors; no refresh table/endpoint, profile endpoint, Kong JWT plugin or public Identity host port.

- [ ] **Step 5: Commit only a required verification correction**

If Steps 1–4 required a correction, stage only corrected files and run:

```powershell
git commit -m "fix(identity): complete auth slice verification"
```

If nothing changed, do not create an empty commit.

## Definition of done

- Register through Kong creates one active account/profile/`CUSTOMER` membership and returns no credential material.
- Login through Kong returns RS256 access JWT only for active valid credentials.
- Unknown email, wrong password and disabled account expose the same 401 contract.
- JWKS publishes only the public key matching token `kid`.
- Smoke verifies signature, issuer, audience, expiry, subject and role without logging token/password.
- Flyway exclusively owns schema; Hibernate uses `ddl-auto: validate`.
- Argon2id parameters are explicit and hashes use `{argon2id}` prefix.
- Tests use PostgreSQL 16 Testcontainers and ephemeral RSA files outside the repository.
- Local/Compose startup fails closed for missing, mismatched or undersized RSA keys.
- Refresh/logout/profile and their persistence are absent.

## Primary references

- [Approved Identity A design](../specs/2026-08-03-identity-register-login-jwks-design.md)
- [Spring Boot Testcontainers](https://docs.spring.io/spring-boot/reference/testing/testcontainers.html)
- [Spring Boot Flyway initialization](https://docs.spring.io/spring-boot/how-to/data-initialization.html)
- [Spring Security password storage](https://docs.spring.io/spring-security/reference/features/authentication/password-storage.html)
- [Spring Security NimbusJwtEncoder](https://docs.spring.io/spring-security/reference/api/java/org/springframework/security/oauth2/jwt/NimbusJwtEncoder.html)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)
- [RFC 7517: JWK](https://datatracker.ietf.org/doc/html/rfc7517)
- [RFC 7519: JWT](https://datatracker.ietf.org/doc/html/rfc7519)

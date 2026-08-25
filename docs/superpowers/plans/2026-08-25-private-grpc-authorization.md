# Private gRPC Authorization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a current-state private Identity authorization API and use it to protect Cart `AddItem` with `cart.item.add`, returning `403` for denial and `503` when authorization cannot be evaluated.

**Architecture:** Identity serves additive unary gRPC authorization RPCs backed by its private RBAC graph and no decision cache. Cart owns a generated-contract client and enforces `cart.item.add` before any Cart mutation; BFF maps stable Cart gRPC error details to the public REST envelope.

**Tech Stack:** Protobuf/Buf, Java 21, Spring Boot 4.1, grpc-java 1.65.1, PostgreSQL 16/Flyway/JdbcClient, Go 1.24, grpc-go 1.65.0, JUnit 5/Mockito/AssertJ/Testcontainers, Go testing.

**Spec:** `docs/superpowers/specs/2026-08-25-private-grpc-authorization-design.md`

## Global Constraints

- Identity owns RBAC items, hierarchy, assignments, account status, and decision evaluation; Cart owns the meaning and enforcement point of `cart.item.add`.
- No service may read Identity tables, import Identity implementation code, or route this internal call through Kong or BFF.
- JWT remains the authentication and caller-ID mechanism; do not add permissions to JWT claims.
- Read current RBAC state on every authorization request; do not cache decisions or retry a failed authorization call.
- Protect only `CartService.AddItem` in this increment. Do not change the authorization behavior of the other Cart RPCs.
- Identity uses private gRPC port `50054`; Cart uses a positive, configurable timeout with local default `500ms`.
- Identity `allowed=false` becomes Cart `PERMISSION_DENIED`; Identity dependency failure becomes Cart `UNAVAILABLE`; invalid public Cart input remains `INVALID_ARGUMENT` before authorization.
- Protobuf sources are authoritative. Run `buf generate` and never edit generated Java or Go sources manually.
- Preserve all unrelated work already present in the working tree. Before each path-limited commit, inspect `git diff` and `git diff --cached`; do not include pre-existing changes in an implementation commit.
- Follow Red-Green-Refactor for handwritten production behavior. Generated code and declarative configuration are verified by their contract/config tests.

---

## File Structure

| Surface | Files | Responsibility |
| --- | --- | --- |
| Contract | `contracts/proto/identity/v1/authorization.proto`; generated files under `contracts/go/identity/v1/` and `contracts/java/src/main/generated/com/ecommerce/contracts/identity/v1/` | Additive single and batch authorization transport contract. |
| Identity RBAC | `RbacRepository.java`, `RbacAuthorizationService.java`, their tests, RBAC test data, `V6__add_cart_item_add_permission.sql`, migration test | Current-state single/batch decisions and permission provisioning. |
| Identity gRPC | `IdentityGrpcProperties.java`, `AuthorizationGrpcService.java`, `GrpcServerLifecycle.java`, tests, Identity `pom.xml` and `application.yml` | Input validation, status mapping, listener lifecycle, and runtime configuration. |
| Cart client | `internal/authorization/client.go`, `client_test.go`, config files | Generated Identity client adaptation and per-call timeout. |
| Cart enforcement | `internal/grpc/server.go`, `server_test.go`, `cmd/server/main.go` | Pre-mutation `cart.item.add` guard and stable Cart error reasons. |
| BFF mapping | `internal/cart/client.go`, `client_test.go`, `internal/http/cart.go`, `cart_test.go`, `go.mod`, `go.sum` | Decode stable Cart errors and expose `403`/`503` REST envelopes. |
| Operations | service start scripts, Identity Dockerfile, `AGENTS.md`, `contracts/README.md`, root/local-operation/topology docs | Port, environment, ownership, rollout, and troubleshooting truth. |

---

### Task 1: Add and Generate the Authorization Contract

**Files:**
- Create: `contracts/proto/identity/v1/authorization.proto`
- Generate: `contracts/go/identity/v1/authorization.pb.go`
- Generate: `contracts/go/identity/v1/authorization_grpc.pb.go`
- Generate: `contracts/java/src/main/generated/com/ecommerce/contracts/identity/v1/AuthorizationServiceGrpc.java`
- Generate: the Java message and `OrBuilder` classes for all request, response, and decision messages

**Interfaces:**
- Produces: `AuthorizationService.CheckPermission(CheckPermissionRequest) -> CheckPermissionResponse`
- Produces: `AuthorizationService.CheckPermissions(CheckPermissionsRequest) -> CheckPermissionsResponse`
- Compatibility: additive file in existing package `ecommerce.identity.v1`; no existing field, RPC, or service changes

- [ ] **Step 1: Create the protobuf source**

Create the file with the exact contract approved in the spec:

```proto
syntax = "proto3";

package ecommerce.identity.v1;

option go_package = "ecommerce.local/contracts/go/identity/v1;identityv1";
option java_multiple_files = true;
option java_package = "com.ecommerce.contracts.identity.v1";

service AuthorizationService {
  rpc CheckPermission(CheckPermissionRequest) returns (CheckPermissionResponse);
  rpc CheckPermissions(CheckPermissionsRequest) returns (CheckPermissionsResponse);
}

message CheckPermissionRequest {
  string user_id = 1;
  string permission = 2;
}

message CheckPermissionResponse {
  bool allowed = 1;
}

message CheckPermissionsRequest {
  string user_id = 1;
  repeated string permissions = 2;
}

message CheckPermissionsResponse {
  repeated PermissionDecision decisions = 1;
}

message PermissionDecision {
  string permission = 1;
  bool allowed = 2;
}
```

- [ ] **Step 2: Verify source formatting and lint**

Run from the repository root:

```powershell
$gitRoot = Split-Path (Split-Path (Get-Command git).Source -Parent) -Parent
$env:PATH = "$gitRoot\usr\bin;$env:PATH"
buf format -w contracts/proto/identity/v1/authorization.proto
buf format --diff --exit-code
buf lint
```

Expected: all commands exit `0`; formatting produces no remaining diff.

- [ ] **Step 3: Generate both language modules**

Run:

```powershell
buf generate
git status --short -- contracts/go/identity/v1 contracts/java/src/main/generated/com/ecommerce/contracts/identity/v1
```

Expected: only generated authorization artifacts are new; existing Identity generated files are unchanged.

- [ ] **Step 4: Compile the generated contract modules**

Run:

```powershell
mvn -pl contracts/java test
go test ./contracts/go/...
```

Expected: both commands pass.

- [ ] **Step 5: Commit the additive contract**

After verifying no unrelated staged paths are included:

```powershell
git add -- contracts/proto/identity/v1/authorization.proto contracts/go/identity/v1 contracts/java/src/main/generated/com/ecommerce/contracts/identity/v1
git commit --only -m "feat(contracts): add identity authorization grpc api" -- contracts/proto/identity/v1/authorization.proto contracts/go/identity/v1 contracts/java/src/main/generated/com/ecommerce/contracts/identity/v1
```

---

### Task 2: Provision the Permission and Add Current-State Batch Evaluation

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V6__add_cart_item_add_permission.sql`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/application/RbacAuthorizationService.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/application/RbacAuthorizationServiceTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/support/RbacTestData.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`

**Interfaces:**
- Produces: `Set<String> RbacRepository.findAllowedPermissions(UUID userId, Collection<String> permissions)`
- Produces: `boolean RbacAuthorizationService.can(UUID userId, String permission)`
- Produces: `Set<String> RbacAuthorizationService.allowedPermissions(UUID userId, List<String> permissions)`
- Preserves: `boolean can(Authentication authentication, String permission)` for existing Spring method authorization

- [ ] **Step 1: Write failing application tests**

Extend `RbacAuthorizationServiceTest` with tests that define delegation and preserve the existing authentication adapter:

```java
@Test
void checksPermissionForExplicitUserId() {
    when(repository.findAllowedPermissions(userId, List.of("cart.item.add")))
            .thenReturn(Set.of("cart.item.add"));

    assertThat(service.can(userId, "cart.item.add")).isTrue();
}

@Test
void returnsAllowedSubsetForBatchWithoutLosingInputContract() {
    List<String> requested = List.of("cart.item.add", "cart.item.remove");
    when(repository.findAllowedPermissions(userId, requested))
            .thenReturn(Set.of("cart.item.add"));

    assertThat(service.allowedPermissions(userId, requested))
            .containsExactly("cart.item.add");
}
```

Update the existing `delegatesPermissionCheckToCurrentDatabaseState` stub to target `findAllowedPermissions` so both authentication and UUID entry points share one decision path.

- [ ] **Step 2: Write failing PostgreSQL and migration tests**

Add repository cases that prove one batch result, inactive-user denial, unknown permissions, `SUPER_ADMIN`, and immediate revocation:

```java
@Test
@Transactional
void resolvesCurrentAllowedSubsetAndReflectsRevocationImmediately() {
    repository.assignRole(USER_ID, "CUSTOMER", Instant.now());

    assertThat(repository.findAllowedPermissions(USER_ID,
            List.of("cart.item.add", "rbac.manage", "missing.permission")))
            .containsExactly("cart.item.add");

    jdbcTemplate.update("delete from auth_item_children where parent_name = 'CUSTOMER' and child_name = 'cart.item.add'");

    assertThat(repository.findAllowedPermissions(USER_ID, List.of("cart.item.add")))
            .isEmpty();
}
```

In `IdentityDatabaseMigrationTest`, extend the exact permission assertion with `cart.item.add` and assert:

```java
assertThat(jdbc.sql("""
        select count(*) from auth_item_children
        where parent_name = 'CUSTOMER' and child_name = 'cart.item.add'
        """).query(Long.class).single()).isEqualTo(1L);
```

Add an upgraded-schema test that creates `migration_target` through the existing
V5 helper, executes the V6 resource through a new `v6MigrationSql()` helper, and
asserts that `migration_target.auth_items` contains `cart.item.add` as a protected
`PERMISSION` plus exactly one `CUSTOMER -> cart.item.add` edge. This is distinct
from the fresh-schema Flyway assertion and proves provider-first rollout over an
already migrated Identity database.

- [ ] **Step 3: Run the focused tests and confirm RED**

Run from `services/identity-service`:

```powershell
mvn test '-Dtest=RbacAuthorizationServiceTest,RbacPersistenceTest,IdentityDatabaseMigrationTest'
```

Expected: compilation or assertions fail because the batch API and V6 seed do not exist. If Testcontainers is unavailable, record that environment blocker and still verify `RbacAuthorizationServiceTest` fails for the missing API.

- [ ] **Step 4: Add the immutable V6 migration and align test fixtures**

Create the migration with exact idempotent seed semantics:

```sql
insert into auth_items (name, type, description, system_item, created_at, updated_at)
values ('cart.item.add', 'PERMISSION', 'Add products to the current user cart', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values ('CUSTOMER', 'cart.item.add')
on conflict do nothing;
```

Add the same protected item and `CUSTOMER` edge to `RbacTestData.resetAuthorizationData`, because that helper intentionally rebuilds the RBAC registry after Flyway has run. Do not add a `CART_ITEM_ADD` constant to Identity's `RbacPermissions`.

- [ ] **Step 5: Implement one-query current-state evaluation**

Add `findAllowedPermissions` using one recursive query. Bind the requested collection through a named `IN (:permissions)` parameter and return an insertion-ordered set:

```java
public Set<String> findAllowedPermissions(UUID userId, Collection<String> permissions) {
    if (permissions.isEmpty()) {
        return Set.of();
    }
    List<String> allowed = jdbc.sql("""
            with recursive reachable(name) as (
                select a.item_name
                from auth_assignments a
                join identity_users u on u.id = a.user_id and u.status = 'ACTIVE'
                where a.user_id = :userId
                union
                select c.child_name
                from auth_item_children c
                join reachable r on r.name = c.parent_name
            )
            select target.name
            from auth_items target
            where target.type = 'PERMISSION'
              and target.name in (:permissions)
              and (
                  exists (select 1 from reachable where name = 'SUPER_ADMIN')
                  or exists (select 1 from reachable where name = target.name)
              )
            order by target.name
            """)
            .param("userId", userId)
            .param("permissions", permissions)
            .query(String.class)
            .list();
    return Collections.unmodifiableSet(new LinkedHashSet<>(allowed));
}
```

Make `hasPermission` delegate to this method with `List.of(permission)`. In the application service, make both public entry points share the same repository method:

```java
public boolean can(UUID userId, String permission) {
    return userId != null
            && permission != null
            && !permission.isBlank()
            && allowedPermissions(userId, List.of(permission)).contains(permission);
}

public Set<String> allowedPermissions(UUID userId, List<String> permissions) {
    if (userId == null || permissions == null || permissions.isEmpty()) {
        return Set.of();
    }
    return repository.findAllowedPermissions(userId, permissions);
}
```

Keep `can(Authentication, String)` as a type/blank guard that delegates to `can(caller.userId(), permission)`.

- [ ] **Step 6: Run focused and Identity regression tests**

Run:

```powershell
mvn test '-Dtest=RbacAuthorizationServiceTest'
mvn test '-Dtest=RbacPersistenceTest,IdentityDatabaseMigrationTest'
```

Expected: both commands pass; the integration test demonstrates a grant and revocation without process restart.

- [ ] **Step 7: Commit Identity RBAC behavior**

```powershell
git add -- services/identity-service/src/main/resources/db/migration/V6__add_cart_item_add_permission.sql services/identity-service/src/main/java/com/ecommerce/identity/rbac services/identity-service/src/test/java/com/ecommerce/identity/rbac services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java
git commit --only -m "feat(identity): evaluate current rbac permissions" -- services/identity-service/src/main/resources/db/migration/V6__add_cart_item_add_permission.sql services/identity-service/src/main/java/com/ecommerce/identity/rbac services/identity-service/src/test/java/com/ecommerce/identity/rbac services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java
```

---

### Task 3: Serve the Private Identity Authorization API

**Files:**
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentityGrpcProperties.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/grpc/AuthorizationGrpcService.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/grpc/GrpcServerLifecycle.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/common/config/IdentityGrpcPropertiesTest.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/grpc/AuthorizationGrpcServiceTest.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/grpc/AuthorizationGrpcIntegrationTest.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java`
- Modify: `services/identity-service/pom.xml`
- Modify: `services/identity-service/src/main/resources/application.yml`

**Interfaces:**
- Consumes: generated `AuthorizationServiceGrpc.AuthorizationServiceImplBase`
- Consumes: `RbacAuthorizationService.can(UUID, String)` and `allowedPermissions(UUID, List<String>)`
- Produces: private listener configured by `identity.grpc.host`, `identity.grpc.port`, and `identity.grpc.shutdown-grace`
- Produces: gRPC `INVALID_ARGUMENT` for malformed inputs and `INTERNAL` for safe unexpected failures

- [ ] **Step 1: Write failing configuration tests**

Create constructor-level tests for exact validation:

```java
@Test
void acceptsPrivateListenerConfiguration() {
    IdentityGrpcProperties properties =
            new IdentityGrpcProperties("127.0.0.1", 50054, Duration.ofSeconds(5));

    assertThat(properties.port()).isEqualTo(50054);
}

@Test
void rejectsBlankHostInvalidPortAndNonPositiveGrace() {
    assertThatThrownBy(() -> new IdentityGrpcProperties(" ", 50054, Duration.ofSeconds(5)))
            .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new IdentityGrpcProperties("127.0.0.1", -1, Duration.ofSeconds(5)))
            .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new IdentityGrpcProperties("127.0.0.1", 50054, Duration.ZERO))
            .isInstanceOf(IllegalArgumentException.class);
}
```

Port `0` is valid for integration tests; production uses `50054`.

- [ ] **Step 2: Write failing transport-unit tests**

Create a recording `StreamObserver<T>` test utility in `AuthorizationGrpcServiceTest` and cover:

- valid single request delegates the parsed UUID and exact permission;
- valid batch returns one `PermissionDecision` per input in input order, including duplicates;
- malformed UUID, blank/trimmed permission, empty batch, and 101-item batch return `INVALID_ARGUMENT` without calling RBAC;
- an unexpected application exception returns `INTERNAL` with the client-safe message `authorization check failed`.

The valid batch assertion must be exact:

```java
assertThat(observer.value().getDecisionsList())
        .extracting(PermissionDecision::getPermission, PermissionDecision::getAllowed)
        .containsExactly(
                tuple("cart.item.add", true),
                tuple("cart.item.remove", false),
                tuple("cart.item.add", true));
```

- [ ] **Step 3: Write a failing real-listener integration test**

Extend `PostgresIntegrationTestSupport` dynamic properties with:

```java
registry.add("identity.grpc.host", () -> "127.0.0.1");
registry.add("identity.grpc.port", () -> "0");
registry.add("identity.grpc.shutdown-grace", () -> "2s");
```

Create `AuthorizationGrpcIntegrationTest` that autowires `GrpcServerLifecycle`, inserts an active user with `CUSTOMER`, opens a plaintext channel to `lifecycle.localPort()`, and uses the generated blocking stub. Assert `cart.item.add` is true, delete the hierarchy edge, and assert the next RPC is false using the same channel.

- [ ] **Step 4: Run the new tests and confirm RED**

Run:

```powershell
mvn test '-Dtest=IdentityGrpcPropertiesTest,AuthorizationGrpcServiceTest,AuthorizationGrpcIntegrationTest'
```

Expected: compilation fails because the configuration, transport adapter, and listener do not exist.

- [ ] **Step 5: Add grpc-java runtime dependency and configuration**

Add `io.grpc:grpc-netty-shaded:1.65.1` to the Identity service. The generated contract dependency already supplies `grpc-stub` and `grpc-protobuf`; do not add a Spring gRPC framework or duplicate generated types.

Merge these keys into the existing `identity` section in `application.yml`, preserving all current user changes in that file:

```yaml
identity:
  grpc:
    host: ${IDENTITY_GRPC_HOST:127.0.0.1}
    port: ${IDENTITY_GRPC_PORT:50054}
    shutdown-grace: ${IDENTITY_GRPC_SHUTDOWN_GRACE:5s}
```

Implement the configuration record:

```java
@ConfigurationProperties("identity.grpc")
public record IdentityGrpcProperties(String host, int port, Duration shutdownGrace) {
    public IdentityGrpcProperties {
        if (host == null || host.isBlank()) {
            throw new IllegalArgumentException("Identity gRPC host is required");
        }
        if (port < 0 || port > 65535) {
            throw new IllegalArgumentException("Identity gRPC port must be 0 through 65535");
        }
        if (shutdownGrace == null || shutdownGrace.isZero() || shutdownGrace.isNegative()) {
            throw new IllegalArgumentException("Identity gRPC shutdown grace must be positive");
        }
    }
}
```

- [ ] **Step 6: Implement the gRPC adapter**

`AuthorizationGrpcService` extends the generated base and has one validation path. Use these exact limits and status mappings:

```java
private static final int MAX_BATCH_SIZE = 100;

private UUID requireUserId(String rawUserId) {
    try {
        return UUID.fromString(rawUserId);
    } catch (RuntimeException exception) {
        throw Status.INVALID_ARGUMENT.withDescription("user_id must be a UUID")
                .asRuntimeException();
    }
}

private String requirePermission(String permission) {
    if (permission == null || permission.isBlank() || !permission.equals(permission.trim())) {
        throw Status.INVALID_ARGUMENT.withDescription("permission must be non-blank and trimmed")
                .asRuntimeException();
    }
    return permission;
}
```

For `checkPermission`, validate, call `rbac.can`, and emit one response. For `checkPermissions`, reject sizes outside `1..100`, validate every entry before invoking RBAC, call `allowedPermissions` once, then build decisions by iterating the original list. Catch `StatusRuntimeException` and pass it unchanged; catch other runtime exceptions and return `Status.INTERNAL.withDescription("authorization check failed")` without the exception text.

Use an SLF4J logger in the adapter: log decision category, user ID, permission or
batch size, and any available correlation context; never log tokens. Keep normal
allow/deny decisions at debug level and log unexpected provider failures at warn
level with the server-side exception while returning only the safe client
description.

- [ ] **Step 7: Implement the managed private listener**

Implement `GrpcServerLifecycle` as a Spring `SmartLifecycle`. Build the listener with:

```java
server = NettyServerBuilder
        .forAddress(new InetSocketAddress(properties.host(), properties.port()))
        .addService(authorizationService)
        .build()
        .start();
```

`start()` converts `IOException` to an actionable `IllegalStateException`. `stop(Runnable)` calls `shutdown()`, waits at most `shutdownGrace`, calls `shutdownNow()` if necessary, preserves interruption, clears running state, and always invokes the callback. `localPort()` returns `server.getPort()` only while running so the integration test can connect to an ephemeral port.

- [ ] **Step 8: Run transport and Identity reactor tests**

Run:

```powershell
mvn test '-Dtest=IdentityGrpcPropertiesTest,AuthorizationGrpcServiceTest,AuthorizationGrpcIntegrationTest'
mvn -pl services/identity-service -am test
```

Expected: all tests pass and repeated test contexts use ephemeral gRPC ports without conflict.

- [ ] **Step 9: Commit the Identity provider without absorbing unrelated config edits**

Commit clean new/source/test paths first. Include `application.yml` only if its pre-existing changes are already owned or can be staged as an isolated patch; otherwise leave that overlapping file for the final handoff:

```powershell
git add -- services/identity-service/pom.xml services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentityGrpcProperties.java services/identity-service/src/main/java/com/ecommerce/identity/rbac/grpc services/identity-service/src/test/java/com/ecommerce/identity/common/config/IdentityGrpcPropertiesTest.java services/identity-service/src/test/java/com/ecommerce/identity/rbac/grpc services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java
git commit --only -m "feat(identity): serve private authorization grpc" -- services/identity-service/pom.xml services/identity-service/src/main/java/com/ecommerce/identity/common/config/IdentityGrpcProperties.java services/identity-service/src/main/java/com/ecommerce/identity/rbac/grpc services/identity-service/src/test/java/com/ecommerce/identity/common/config/IdentityGrpcPropertiesTest.java services/identity-service/src/test/java/com/ecommerce/identity/rbac/grpc services/identity-service/src/test/java/com/ecommerce/identity/support/PostgresIntegrationTestSupport.java
```

---

### Task 4: Add the Cart-Owned Authorization Client and Configuration

**Files:**
- Create: `services/cart-service/internal/authorization/client.go`
- Create: `services/cart-service/internal/authorization/client_test.go`
- Modify: `services/cart-service/internal/config/config.go`
- Modify: `services/cart-service/internal/config/config_test.go`

**Interfaces:**
- Consumes: generated `identityv1.AuthorizationServiceClient`
- Produces: `CheckPermission(context.Context, string, string) (bool, error)`
- Produces: `Close() error`
- Produces config: `IdentityGRPCAddr string`, `IdentityGRPCTimeout time.Duration`

- [ ] **Step 1: Write failing client tests against a real test server**

Use a TCP listener on `127.0.0.1:0`, a generated recording authorization server, and the production client. Cover exact forwarding, allowed/denied response values, status preservation, and a blocking RPC bounded by `20ms`:

```go
allowed, err := client.CheckPermission(context.Background(), "user-1", "cart.item.add")
if err != nil || !allowed {
	t.Fatalf("CheckPermission() = %t, %v; want true, nil", allowed, err)
}
if server.request.GetUserId() != "user-1" || server.request.GetPermission() != "cart.item.add" {
	t.Fatalf("request = %+v", server.request)
}
```

For the blocking case, assert `status.Code(err) == codes.DeadlineExceeded` and elapsed time stays below one second.

- [ ] **Step 2: Write failing config tests**

Extend `clearConfigEnvironment` with `IDENTITY_GRPC_ADDR` and `IDENTITY_GRPC_TIMEOUT`. Add defaults and overrides:

```go
if cfg.IdentityGRPCAddr != "127.0.0.1:50054" {
	t.Fatalf("IdentityGRPCAddr = %q", cfg.IdentityGRPCAddr)
}
if cfg.IdentityGRPCTimeout != 500*time.Millisecond {
	t.Fatalf("IdentityGRPCTimeout = %s", cfg.IdentityGRPCTimeout)
}
```

Add table cases for `IDENTITY_GRPC_TIMEOUT=invalid`, `0s`, and `-1s`; each must return an error naming the variable.

- [ ] **Step 3: Run focused tests and confirm RED**

Run:

```powershell
go test ./services/cart-service/internal/authorization ./services/cart-service/internal/config
```

Expected: compilation fails because the client package and config fields do not exist.

- [ ] **Step 4: Implement the client and config**

Follow the existing Cart Catalog/Inventory adapter pattern:

```go
type Client interface {
	CheckPermission(context.Context, string, string) (bool, error)
}

type client struct {
	conn    *grpc.ClientConn
	service identityv1.AuthorizationServiceClient
	timeout time.Duration
}

func (c *client) CheckPermission(ctx context.Context, userID, permission string) (bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.service.CheckPermission(callCtx, &identityv1.CheckPermissionRequest{
		UserId: userID, Permission: permission,
	})
	if err != nil {
		return false, err
	}
	return response.GetAllowed(), nil
}
```

`NewClient` uses `grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))`; `Close` closes the connection. Configuration parses the Identity timeout through the existing positive `loadDuration` function and defaults to `127.0.0.1:50054` and `500ms`.

- [ ] **Step 5: Run focused Cart adapter tests**

Run:

```powershell
go test ./services/cart-service/internal/authorization ./services/cart-service/internal/config
```

Expected: both packages pass.

- [ ] **Step 6: Commit the Cart client**

```powershell
git add -- services/cart-service/internal/authorization services/cart-service/internal/config
git commit --only -m "feat(cart): add identity authorization client" -- services/cart-service/internal/authorization services/cart-service/internal/config
```

---

### Task 5: Enforce `cart.item.add` Before Cart Mutation

**Files:**
- Modify: `services/cart-service/internal/grpc/server.go`
- Modify: `services/cart-service/internal/grpc/server_test.go`
- Modify: `services/cart-service/cmd/server/main.go`

**Interfaces:**
- Consumes: Cart-local `authorizationClient.CheckPermission(ctx, userID, permission)`
- Produces: `codes.PermissionDenied` plus `google.rpc.ErrorInfo{Reason: "CART_PERMISSION_DENIED", Domain: "ecommerce.cart.v1"}`
- Produces: `codes.Unavailable` plus `google.rpc.ErrorInfo{Reason: "AUTHORIZATION_UNAVAILABLE", Domain: "ecommerce.cart.v1"}`
- Preserves: validation before authorization and all non-AddItem behavior

- [ ] **Step 1: Add failing authorization-guard tests**

Add a fake to `server_test.go`:

```go
type fakeAuthorization struct {
	allowed    bool
	err        error
	userID     string
	permission string
	calls      int
}

func (f *fakeAuthorization) CheckPermission(_ context.Context, userID, permission string) (bool, error) {
	f.calls++
	f.userID = userID
	f.permission = permission
	return f.allowed, f.err
}
```

Add three focused tests:

1. Invalid AddItem input leaves `authorization.calls == 0`.
2. `allowed=false` returns `PermissionDenied` with reason `CART_PERMISSION_DENIED`, records `user-1` and `cart.item.add`, and leaves `store.ensureUserID`, `store.addArgs`, Catalog calls, and Inventory calls empty.
3. An authorization `Unavailable` error returns Cart `Unavailable` with reason `AUTHORIZATION_UNAVAILABLE` and the same no-side-effect assertions.

Add a fourth assertion to the existing successful mutation test: the allowed fake was called once before `store.EnsureCart` and existing success behavior remains intact.

- [ ] **Step 2: Run the Cart gRPC test and confirm RED**

Run:

```powershell
go test ./services/cart-service/internal/grpc -run 'TestAddItem|TestMutationsValidate' -count=1
```

Expected: compilation or assertions fail because `Server` has no authorization dependency or guard.

- [ ] **Step 3: Implement the guard and stable errors**

Add the interface, field, constructor parameter, and Cart-owned constants:

```go
const (
	cartItemAddPermission          = "cart.item.add"
	grpcErrorDomain                = "ecommerce.cart.v1"
	reasonCartPermissionDenied     = "CART_PERMISSION_DENIED"
	reasonAuthorizationUnavailable = "AUTHORIZATION_UNAVAILABLE"
)

type authorizationClient interface {
	CheckPermission(context.Context, string, string) (bool, error)
}
```

After all existing AddItem field validation and before `EnsureCart`, run:

```go
allowed, err := s.authorization.CheckPermission(ctx, userID, cartItemAddPermission)
if err != nil {
	if ctx.Err() != nil {
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	return nil, statusWithReason(codes.Unavailable,
		"authorization service is unavailable", reasonAuthorizationUnavailable)
}
if !allowed {
	return nil, statusWithReason(codes.PermissionDenied,
		"cart permission denied", reasonCartPermissionDenied)
}
```

Build stable details with `status.New(...).WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: grpcErrorDomain})`. If detail construction fails, return the same status code and safe message without details; never allow the mutation because detail annotation failed.

Use `log/slog` to record denial and dependency-failure categories with the
permission and user ID, using `WarnContext` for dependency failures and
`DebugContext` for denials. Do not log request payloads, JWTs, or credentials.

Update all test `NewServer` calls to pass an allowed fake by default. Keep explicit denied/failing fakes local to the new tests.

- [ ] **Step 4: Wire the production authorization client**

In `buildProductionService`, construct the Identity client after the existing Catalog and Inventory clients. On construction failure, close the earlier clients and pool. Pass it into `cartgrpc.NewServer` and close it in the returned cleanup function:

```go
authorizationClient, err := authorization.NewClient(cfg.IdentityGRPCAddr, cfg.IdentityGRPCTimeout)
if err != nil {
	_ = inventoryClient.Close()
	_ = catalogClient.Close()
	cleanupPool()
	return nil, nil, errors.New("failed to create identity authorization client")
}
```

The generated client is lazy, so Identity reachability remains a request-time fail-closed condition.

- [ ] **Step 5: Run Cart regression checks**

Run:

```powershell
go test ./services/cart-service/internal/...
go build ./services/cart-service/...
```

Expected: all internal package tests pass and the complete Cart service builds. On Windows, do not claim `go test ./services/cart-service/...` passed unless the known Unix-only signal test has been run in Linux/CI.

- [ ] **Step 6: Commit Cart enforcement**

```powershell
git add -- services/cart-service/internal/grpc services/cart-service/cmd/server/main.go
git commit --only -m "feat(cart): authorize add item mutations" -- services/cart-service/internal/grpc services/cart-service/cmd/server/main.go
```

---

### Task 6: Map Authorization Outcomes in BFF

**Files:**
- Modify: `services/bff-service/internal/cart/client.go`
- Modify: `services/bff-service/internal/cart/client_test.go`
- Modify: `services/bff-service/internal/http/cart.go`
- Modify: `services/bff-service/internal/http/cart_test.go`
- Modify: `services/bff-service/go.mod`
- Modify: `services/bff-service/go.sum`

**Interfaces:**
- Produces sentinels: `ErrPermissionDenied`, `ErrAuthorizationUnavailable`
- Consumes stable `google.rpc.ErrorInfo` domain/reason from Cart
- Produces REST `403 FORBIDDEN` and `503 AUTHORIZATION_SERVICE_UNAVAILABLE`
- Preserves REST `502 CART_UNAVAILABLE` for unclassified Cart outages

- [ ] **Step 1: Write failing Cart-client error tests**

Extend the recording Cart server with an `addErr` field and return it from `AddItem`. Build status errors in the test with `errdetails.ErrorInfo` and add cases asserting:

```go
if !errors.Is(err, ErrPermissionDenied) {
	t.Fatalf("error = %v, want ErrPermissionDenied", err)
}
```

and:

```go
if !errors.Is(err, ErrAuthorizationUnavailable) {
	t.Fatalf("error = %v, want ErrAuthorizationUnavailable", err)
}
```

Also return bare `codes.Unavailable` without `ErrorInfo` and assert neither sentinel matches, preserving the error's gRPC code.

- [ ] **Step 2: Write failing HTTP mapping tests**

In `cart_test.go`, make the fake AddItem client return each sentinel. Assert exact status and code:

```go
if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "FORBIDDEN" {
	t.Fatalf("denied response = %d %s", rec.Code, rec.Body.String())
}
```

```go
if rec.Code != http.StatusServiceUnavailable || decodeBody(t, rec)["code"] != "AUTHORIZATION_SERVICE_UNAVAILABLE" {
	t.Fatalf("unavailable response = %d %s", rec.Code, rec.Body.String())
}
```

Keep the existing `TestCartUnavailableMapsToBadGateway` unchanged as the regression for bare Cart outage.

- [ ] **Step 3: Run focused BFF tests and confirm RED**

Run:

```powershell
go test ./services/bff-service/internal/cart ./services/bff-service/internal/http -run 'Authorization|Permission|CartUnavailable' -count=1
```

Expected: compilation fails because the sentinels and mapping do not exist.

- [ ] **Step 4: Implement stable detail decoding and sentinel mapping**

Make `google.golang.org/genproto/googleapis/rpc` a direct dependency. Add:

```go
var (
	ErrCartItemNotFound       = errors.New("cart: item not found")
	ErrPermissionDenied       = errors.New("cart: permission denied")
	ErrAuthorizationUnavailable = errors.New("cart: authorization unavailable")
)
```

Decode only `ErrorInfo` with domain `ecommerce.cart.v1`. Map `CART_PERMISSION_DENIED` to `ErrPermissionDenied` and `AUTHORIZATION_UNAVAILABLE` to `ErrAuthorizationUnavailable`; preserve all unknown codes/reasons unchanged. Run this mapping before the existing NotFound mapping so behavior is explicit.

Update `writeCartError` in this order:

1. `ErrPermissionDenied` -> `403`, code `FORBIDDEN`, message `You do not have permission to add products to the cart.`
2. `ErrAuthorizationUnavailable` -> `503`, code `AUTHORIZATION_SERVICE_UNAVAILABLE`, message `Authorization service is unavailable.`
3. existing cart-item NotFound mapping;
4. existing InvalidArgument mapping;
5. existing `502 CART_UNAVAILABLE` fallback.

- [ ] **Step 5: Run BFF regression tests**

Run:

```powershell
go test ./services/bff-service/internal/cart ./services/bff-service/internal/http
go test ./services/bff-service/...
```

Expected: all tests pass, including the pre-existing bare Cart outage `502` case.

- [ ] **Step 6: Commit BFF mapping**

```powershell
git add -- services/bff-service/internal/cart services/bff-service/internal/http/cart.go services/bff-service/internal/http/cart_test.go services/bff-service/go.mod services/bff-service/go.sum
git commit --only -m "feat(bff): expose cart authorization outcomes" -- services/bff-service/internal/cart services/bff-service/internal/http/cart.go services/bff-service/internal/http/cart_test.go services/bff-service/go.mod services/bff-service/go.sum
```

---

### Task 7: Align Runtime Configuration and Architecture Documentation

**Files:**
- Modify: `services/identity-service/start-local.ps1`
- Modify: `services/identity-service/start-local.sh`
- Modify: `services/identity-service/Dockerfile`
- Modify: `services/cart-service/start-local.ps1`
- Modify: `services/cart-service/start-local.sh`
- Modify: `services/identity-service/AGENTS.md`
- Modify: `AGENTS.md`
- Modify: `contracts/README.md`
- Modify: `README.md`
- Modify: `docs/steering/service-catalog.md`
- Modify: `docs/instructions/local-development.md`
- Modify: `docs/instructions/operations.md`

**Interfaces:**
- Documents: Identity HTTP `8080`, private gRPC `50054`
- Documents: Cart outbound Identity `127.0.0.1:50054`, timeout `500ms`
- Preserves: no Kong route for Identity authorization gRPC

- [ ] **Step 1: Add local runtime defaults**

Set `IDENTITY_GRPC_HOST=127.0.0.1`, `IDENTITY_GRPC_PORT=50054`, and `IDENTITY_GRPC_SHUTDOWN_GRACE=5s` in both Identity start scripts without overwriting an explicit caller value. Set `IDENTITY_GRPC_ADDR=127.0.0.1:50054` and `IDENTITY_GRPC_TIMEOUT=500ms` in both Cart start scripts and export them in the shell script.

Expose `50054` beside `8080` in the Identity Dockerfile. Do not add a Kong route.

- [ ] **Step 2: Update ownership and contract truth**

Update Identity's local instructions so they state that Identity evaluates current RBAC assignment/hierarchy decisions but does not own the business meaning or resource rules of commerce permissions. Update `contracts/README.md` so `AuthorizationService` is described as implemented while the older `IdentityService.GetUser` remains unwired.

- [ ] **Step 3: Update topology and runbooks**

Make these facts consistent across the root instructions, root README, service catalog, local-development guide, and operations guide:

- Identity private gRPC is `127.0.0.1:50054` locally.
- Cart calls Identity directly for authorization in addition to Catalog and Inventory.
- Start Identity before Cart.
- Identity authorization outage causes add-item `503` and no Cart mutation.
- Permission denial causes `403`.
- Diagnose port `50054` and `IDENTITY_GRPC_ADDR` directly; never check Kong for this RPC.
- Provider-first rollout order is Identity, Cart, then BFF.

- [ ] **Step 4: Verify documentation and scripts**

Run:

```powershell
rg -n "50054|IDENTITY_GRPC_ADDR|IDENTITY_GRPC_TIMEOUT|cart.item.add|AUTHORIZATION_SERVICE_UNAVAILABLE" AGENTS.md README.md contracts/README.md docs services/identity-service services/cart-service
git diff --check
```

Expected: every runtime reference agrees on `50054` and `500ms`; no Kong configuration contains an authorization gRPC route.

- [ ] **Step 5: Commit only documentation/script changes that do not absorb pre-existing work**

Several documentation and configuration paths may already contain user changes. Inspect each diff and use isolated staging or leave overlapping files uncommitted for handoff. Commit only paths whose complete diff belongs to this feature:

```powershell
git diff -- services/identity-service/start-local.ps1 services/identity-service/start-local.sh services/identity-service/Dockerfile services/cart-service/start-local.ps1 services/cart-service/start-local.sh services/identity-service/AGENTS.md AGENTS.md contracts/README.md README.md docs/steering/service-catalog.md docs/instructions/local-development.md docs/instructions/operations.md
```

Use commit message `docs: document private authorization grpc` for the clean subset.

---

### Task 8: Run Cross-Surface Verification and Smoke Checks

**Files:**
- Verify only; change owning files only if a gate reveals a feature regression

**Interfaces:**
- Verifies: protobuf source/generated synchronization
- Verifies: provider, consumer, and public error contracts
- Verifies: immediate revocation and fail-closed behavior

- [ ] **Step 1: Verify generated output is reproducible**

Run after all contract files are committed or staged:

```powershell
$gitRoot = Split-Path (Split-Path (Get-Command git).Source -Parent) -Parent
$env:PATH = "$gitRoot\usr\bin;$env:PATH"
buf format --diff --exit-code
buf lint
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
git ls-files --others --exclude-standard -- contracts/go contracts/java/src/main/generated
```

Expected: all commands exit `0`, and the last command prints nothing.

- [ ] **Step 2: Verify affected Java modules**

Run:

```powershell
mvn -pl services/identity-service -am verify
```

Expected: contracts and Identity compile; unit, gRPC boundary, migration, and PostgreSQL integration tests pass.

- [ ] **Step 3: Verify affected Go modules**

Run:

```powershell
go test ./services/cart-service/internal/...
go build ./services/cart-service/...
go test ./services/bff-service/...
go build ./services/bff-service/...
```

Expected: all four commands pass. Run `go test ./services/cart-service/...` on Linux/CI; on Windows, record the existing Unix-signal test limitation rather than representing the narrower check as a full Cart test pass.

- [ ] **Step 4: Run repository-level gates proportional to the change**

Run:

```powershell
mvn --batch-mode --no-transfer-progress verify
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
go build ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...
```

Expected: all commands pass. Frontend checks are not required because no frontend contract or behavior changes.

- [ ] **Step 5: Smoke-test grant, immediate revocation, and outage**

Start infrastructure and long-running services in background sessions, recording PIDs/log paths and checking ports before launch. Start Identity first, then Catalog, Inventory, Cart, and BFF. Use an authenticated customer and perform this sequence:

1. Add a valid item through `http://localhost:8000/api/v1/cart/items`; expect `200`.
2. Record the Cart row state.
3. Remove `CUSTOMER -> cart.item.add` through the existing RBAC admin API or a local Identity-owned `psql` operation; repeat the same request and expect `403 FORBIDDEN` with no Cart row change.
4. Restore the edge; repeat and expect the operation to be eligible again.
5. Stop only Identity; repeat and expect `503 AUTHORIZATION_SERVICE_UNAVAILABLE` with no Cart row change.
6. Restart Identity and verify its HTTP health plus private gRPC listener before stopping all application sessions created for the smoke test.

Do not delete volumes or modify another service's database during this check.

- [ ] **Step 6: Review final scope and evidence**

Run:

```powershell
git status --short
git diff --check
git diff --name-only
```

Confirm every acceptance criterion has a corresponding passing automated test or smoke-test record, generated files are synchronized, no Identity implementation was imported by Cart/BFF, no Kong route was added, and all pre-existing working-tree changes remain preserved and identified in the handoff.

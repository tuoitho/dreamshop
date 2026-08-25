# Identity Yii2-Style RBAC and Shipping Addresses Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add database-backed hierarchical RBAC, administrator user/RBAC APIs, and permission-protected customer shipping-address APIs to Identity Service.

**Architecture:** Identity replaces its fixed role join tables with a Yii2-style authorization-item graph and resolves effective permissions from PostgreSQL on every protected operation. Admin APIs mutate items, hierarchy edges, assignments, and account state; shipping-address APIs always derive ownership from the authenticated principal and enforce one transactional default per user. Kong keeps Identity as the owner of admin user/RBAC paths while narrowing the development BFF product-admin route.

**Tech Stack:** Java 21, Spring Boot 4.1, Spring Security method authorization, Spring Data JPA, Spring `JdbcClient`, Flyway, PostgreSQL 16, JUnit 5, MockMvc, Testcontainers, Kong declarative configuration, Go 1.24 configuration tests.

**Spec:** `docs/superpowers/specs/2026-08-25-identity-yii2-style-rbac-and-shipping-addresses-design.md`

## Global Constraints

- Identity Service owns authorization items, assignments, account state, and shipping addresses; no other service reads its tables.
- Browser traffic enters through Kong. `X-User-Id` and `X-User-Roles` are trusted only after the caller-context plugin has validated and replaced them.
- JWT keeps the existing `roles` claim; permissions are never embedded in the token.
- Permission decisions call current PostgreSQL state and must deny inactive users.
- Only roles are directly assignable to users. Supported hierarchy edges are role-to-role, role-to-permission, and permission-to-permission.
- `SUPER_ADMIN` implicitly has every current and future item. Built-in item names and existence are protected.
- Personal profile address fields remain unchanged and are never copied into shipping addresses.
- New REST success and error responses follow the repository envelopes and stable error codes in the design spec.
- Applied Flyway migrations are immutable. Add `V5`; never edit `V1` through `V4`.
- The worktree already contains unrelated staged changes. Never use `git add -A`, `git commit -a`, or repository-wide pathspecs. Stage only the task-owned files/directories shown in that task and inspect the staged name list before every commit.
- If a task must touch a path already modified before execution, preserve the existing content and leave that overlapping file uncommitted for explicit user review.

---

## File Structure

| Path | Responsibility |
| --- | --- |
| `services/identity-service/src/main/resources/db/migration/V5__add_rbac_and_shipping_addresses.sql` | Migrates role data, seeds the RBAC graph, and creates shipping-address storage. |
| `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/` | Item types, public permission constants, RBAC views, and stable domain failures. |
| `services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java` | PostgreSQL graph queries and atomic item/edge/assignment persistence. |
| `services/identity-service/src/main/java/com/ecommerce/identity/rbac/application/` | Permission evaluation and protected RBAC mutations. |
| `services/identity-service/src/main/java/com/ecommerce/identity/rbac/api/` | `/admin/rbac/items` transport and DTOs. |
| `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/` | Admin user query, status, and role-assignment use cases and REST transport. |
| `services/identity-service/src/main/java/com/ecommerce/identity/address/` | Shipping-address entity, repository, application service, validation, API, and errors. |
| `services/identity-service/src/main/java/com/ecommerce/identity/security/` | Caller authorities, method security, URL admission, and permission bridge. |
| `deployments/kong/kong.yaml` | Non-shadowing local routes for BFF product admin and Identity admin APIs. |
| `services/identity-service/RBAC.md` | Bootstrap `SUPER_ADMIN` procedure and authorization model operations. |

## Task 1: Migrate Fixed Roles to the RBAC Graph Without Breaking Authentication

**Files:**
- Create: `services/identity-service/src/main/resources/db/migration/V5__add_rbac_and_shipping_addresses.sql`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/AuthItemType.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/RbacPermissions.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/persistence/RbacPersistenceTest.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/support/RbacTestData.java`
- Delete: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/RoleEntity.java`
- Delete: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/RoleRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserEntity.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/auth/application/RegisterUserService.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/auth/application/LoginService.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/auth/application/RefreshTokenService.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/application/MeService.java`
- Modify: `services/identity-service/src/test/java/com/ecommerce/identity/user/persistence/IdentityDatabaseMigrationTest.java`
- Modify: role-dependent tests returned by `rg -l "RoleRepository|RoleEntity|user_roles|insert into roles|delete from roles" services/identity-service/src/test/java`

**Interfaces:**
- Produces `RbacRepository.assignedRoles(UUID): Set<String>` and `RbacRepository.hasPermission(UUID, String): boolean`.
- Produces `RbacRepository.assignRole(UUID, String, Instant): boolean` for registration and later admin assignment.
- Keeps `RegisteredUser.roles`, `CurrentUser.roles`, and JWT `roles` behavior unchanged.

- [ ] **Step 1: Write failing final-schema and data-copy tests**

Change `IdentityDatabaseMigrationTest.createsAccountTablesAndSeedsCustomerRole` to require `auth_items`, `auth_item_children`, `auth_assignments`, and `shipping_addresses`, and to reject superseded tables:

```java
assertThat(tables)
        .contains("identity_users", "identity_profiles", "auth_items",
                "auth_item_children", "auth_assignments", "shipping_addresses")
        .doesNotContain("roles", "user_roles");
assertThat(jdbc.sql("select name from auth_items where system_item order by name")
        .query(String.class).list())
        .contains("CUSTOMER", "ADMIN", "SUPER_ADMIN", "address.create", "rbac.manage");
```

Add an isolated-schema test that creates the V1 role tables, inserts one custom role and two assignments, runs the V5 SQL, then asserts both role names and both user-to-role assignments survived. Read V5 with `ClassPathResource` exactly as the existing V4 test does.

- [ ] **Step 2: Run the migration test and confirm red**

Run from `services/identity-service`:

```powershell
mvn test '-Dtest=IdentityDatabaseMigrationTest#createsAccountTablesAndSeedsCustomerRole'
```

Expected: FAIL because V5 and the new tables do not exist.

- [ ] **Step 3: Add the V5 migration**

Create the tables with these exact constraints and seed semantics:

```sql
create table auth_items (
    name varchar(128) primary key,
    type varchar(16) not null,
    description varchar(255),
    system_item boolean not null default false,
    created_at timestamptz not null,
    updated_at timestamptz not null,
    constraint ck_auth_items_type check (type in ('ROLE', 'PERMISSION'))
);

create table auth_item_children (
    parent_name varchar(128) not null references auth_items(name)
        on update cascade on delete cascade,
    child_name varchar(128) not null references auth_items(name)
        on update cascade on delete cascade,
    primary key (parent_name, child_name),
    constraint ck_auth_item_children_not_self check (parent_name <> child_name)
);

create table auth_assignments (
    user_id uuid not null references identity_users(id) on delete cascade,
    item_name varchar(128) not null references auth_items(name)
        on update cascade on delete cascade,
    created_at timestamptz not null,
    primary key (user_id, item_name)
);

insert into auth_items (name, type, description, system_item, created_at, updated_at)
select code, 'ROLE', code || ' role', code in ('CUSTOMER', 'ADMIN', 'SUPER_ADMIN'), now(), now()
from roles;

insert into auth_assignments (user_id, item_name, created_at)
select ur.user_id, r.code, now()
from user_roles ur join roles r on r.id = ur.role_id;
```

Use `insert ... on conflict (name) do update` to seed the three built-in roles and the ten permission names from the spec, marking them `system_item = true`. Seed `CUSTOMER -> address.*` and `ADMIN -> user.* / rbac.*` with `on conflict do nothing`, then drop `user_roles` before `roles`.

Create `shipping_addresses` and its partial index in the same migration:

```sql
create table shipping_addresses (
    id uuid primary key,
    user_id uuid not null references identity_users(id) on delete cascade,
    recipient_name varchar(100) not null,
    phone varchar(20) not null,
    province_code varchar(20) not null,
    ward_code varchar(20) not null,
    address_line varchar(255) not null,
    label varchar(50),
    is_default boolean not null default false,
    created_at timestamptz not null,
    updated_at timestamptz not null
);
create unique index uq_shipping_addresses_default_per_user
    on shipping_addresses (user_id) where is_default;
create index ix_shipping_addresses_user_created
    on shipping_addresses (user_id, created_at desc, id);
```

- [ ] **Step 4: Add RBAC read/assignment persistence tests**

In `RbacPersistenceTest`, seed an active user, assign `CUSTOMER`, add
`CUSTOMER -> address.manage -> address.create`, and assert:

```java
assertThat(repository.assignedRoles(userId)).containsExactly("CUSTOMER");
assertThat(repository.hasPermission(userId, "address.create")).isTrue();
assertThat(repository.hasPermission(userId, "rbac.manage")).isFalse();
```

Disable the user and assert `hasPermission` is false. Assign `SUPER_ADMIN`, create a custom permission after the assignment, and assert the custom permission is immediately allowed.

- [ ] **Step 5: Implement the initial RBAC repository**

Create `AuthItemType` with `ROLE` and `PERMISSION`. Create `RbacPermissions` constants for every permission in the spec and built-in role constants.

Implement `RbacRepository` with `JdbcClient`. Use this recursive query for normal access and an explicit `SUPER_ADMIN` branch:

```sql
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
select exists (
    select 1 from reachable where name = 'SUPER_ADMIN'
) or exists (
    select 1 from reachable r
    join auth_items i on i.name = r.name and i.type = 'PERMISSION'
    where r.name = :permission
)
```

`assignRole` executes an `insert ... select` restricted to `type = 'ROLE'` with
`on conflict do nothing` and returns whether a row was inserted.

- [ ] **Step 6: Refactor authentication and profile reads to assignments**

Remove the `roles` relationship and `roleCodes()` from `IdentityUserEntity`; change `create(...)` to omit the role set. Replace repository entity graphs with `profile` only.

Use these application flows:

```java
IdentityUserEntity user = users.saveAndFlush(IdentityUserEntity.create(
        UUID.randomUUID(), normalizedEmail, normalizedEmail, passwordHash,
        AccountStatus.ACTIVE, now, command.fullName()));
rbac.assignRole(user.id(), RbacPermissions.CUSTOMER, now);
return new RegisteredUser(user.id(), user.normalizedEmail(), user.fullName(),
        rbac.assignedRoles(user.id()));
```

Login, refresh, and `MeService` call `rbac.assignedRoles(user.id())`. Refresh must also reject a missing or disabled user before rotating into a new authenticated session.

- [ ] **Step 7: Update legacy test setup without weakening assertions**

Create `RbacTestData.restoreBuiltIns(JdbcTemplate)` that upserts the built-in items and hierarchy edges from V5. Replace direct `roles`/`user_roles` cleanup in auth, JWKS, Me, and persistence tests with:

```java
jdbcTemplate.update("delete from auth_assignments");
RbacTestData.restoreBuiltIns(jdbcTemplate);
```

Replace `RoleRepository` fixtures with `RbacRepository.assignRole`. Preserve tests asserting registration/login/refresh/JWT roles.

- [ ] **Step 8: Run focused and module tests**

```powershell
mvn test -Dtest=IdentityDatabaseMigrationTest,RbacPersistenceTest
Set-Location ..\..
mvn -pl services/identity-service -am test
```

Expected: PASS; existing auth/profile/JWKS behavior remains green.

- [ ] **Step 9: Commit only Task 1 paths**

```powershell
git add services/identity-service/src/main/resources/db/migration/V5__add_rbac_and_shipping_addresses.sql services/identity-service/src/main/java/com/ecommerce/identity/rbac services/identity-service/src/main/java/com/ecommerce/identity/user/persistence services/identity-service/src/main/java/com/ecommerce/identity/auth/application services/identity-service/src/main/java/com/ecommerce/identity/user/application/MeService.java services/identity-service/src/test/java/com/ecommerce/identity
git commit -m "feat(identity): migrate authorization to RBAC graph" -- services/identity-service
```

Before committing, inspect `git diff --cached --name-only`; it must contain only Identity paths changed by this task.

## Task 2: Bridge Database Permissions into Spring Method Security

**Files:**
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/application/RbacAuthorizationService.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/application/RbacAuthorizationServiceTest.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/security/CallerContextPrincipalTest.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/security/CallerContextPrincipal.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/security/SecurityConfiguration.java`

**Interfaces:**
- Produces bean `rbac` with `boolean can(Authentication authentication, String permission)` for every protected controller.
- Produces `ROLE_<name>` authorities without using them as permission truth.

- [ ] **Step 1: Write failing authority and permission tests**

```java
@Test
void exposesRolesAsSpringAuthorities() {
    var principal = new CallerContextPrincipal(userId, Set.of("CUSTOMER", "ADMIN"));
    assertThat(principal.getAuthorities()).extracting(GrantedAuthority::getAuthority)
            .containsExactlyInAnyOrder("ROLE_CUSTOMER", "ROLE_ADMIN");
}

@Test
void delegatesPermissionCheckToCurrentDatabaseState() {
    when(repository.hasPermission(userId, "address.create")).thenReturn(true);
    assertThat(service.can(new CallerContextPrincipal(userId, Set.of("CUSTOMER")),
            "address.create")).isTrue();
}
```

Also assert false for null authentication, a non-`CallerContextPrincipal` principal, and blank permission.

- [ ] **Step 2: Run both classes and confirm red**

```powershell
mvn test -Dtest=CallerContextPrincipalTest,RbacAuthorizationServiceTest
```

Expected: FAIL because authorities are empty and the service does not exist.

- [ ] **Step 3: Implement the permission bridge and method security**

Defensively copy roles in the record compact constructor and map sorted roles:

```java
public CallerContextPrincipal {
    roles = Set.copyOf(roles);
}

@Override
public Collection<? extends GrantedAuthority> getAuthorities() {
    return roles.stream()
            .sorted()
            .map(role -> new SimpleGrantedAuthority("ROLE_" + role))
            .toList();
}
```

Implement:

```java
@Service("rbac")
@RequiredArgsConstructor
public class RbacAuthorizationService {
    private final RbacRepository repository;

    public boolean can(Authentication authentication, String permission) {
        return authentication instanceof CallerContextPrincipal caller
                && permission != null && !permission.isBlank()
                && repository.hasPermission(caller.userId(), permission);
    }
}
```

Add `@EnableMethodSecurity` to `SecurityConfiguration`. Admit authenticated requests to `/users/me/addresses/**`, `/admin/users/**`, and `/admin/rbac/**`; method annotations in later tasks decide permission. Preserve permit-all and deny-all behavior for every existing route.

- [ ] **Step 4: Run focused and security regression tests**

```powershell
mvn test -Dtest=CallerContextPrincipalTest,RbacAuthorizationServiceTest,MeApiTest,LocationApiTest
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/security services/identity-service/src/main/java/com/ecommerce/identity/rbac/application/RbacAuthorizationService.java services/identity-service/src/test/java/com/ecommerce/identity/security services/identity-service/src/test/java/com/ecommerce/identity/rbac/application
git commit -m "feat(identity): authorize methods with current RBAC state" -- services/identity-service/src/main/java/com/ecommerce/identity/security services/identity-service/src/main/java/com/ecommerce/identity/rbac/application/RbacAuthorizationService.java services/identity-service/src/test/java/com/ecommerce/identity/security services/identity-service/src/test/java/com/ecommerce/identity/rbac/application
```

## Task 3: Add Protected RBAC Administration APIs

**Files:**
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/RbacItem.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/RbacItemNotFoundException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/RbacItemExistsException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/RbacHierarchyConflictException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/RbacSystemItemProtectedException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/domain/InvalidRbacItemException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/application/RbacAdminService.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/application/RbacItemView.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/api/RbacAdminController.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/api/CreateRbacItemRequest.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/api/UpdateRbacItemRequest.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/api/RbacItemResponse.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/rbac/api/RbacAdminApiTest.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`

**Interfaces:**
- Produces the seven `/admin/rbac/items` operations from the spec.
- Produces transactional item/edge methods consumed by Task 4 assignment management.

- [ ] **Step 1: Write failing API authorization and hierarchy tests**

`RbacAdminApiTest` must bootstrap users by direct assignment and cover:

```java
mockMvc.perform(post("/admin/rbac/items")
        .header("X-User-Id", customerId)
        .contentType(APPLICATION_JSON)
        .content("""{"name":"SUPPORT","type":"ROLE","description":"Support"}"""))
        .andExpect(status().isForbidden())
        .andExpect(jsonPath("$.code").value("FORBIDDEN"));
```

With `SUPER_ADMIN`, create `SUPPORT` and `ticket.view`, add both
`SUPPORT -> ticket.view` and `ADMIN -> SUPPORT`, then verify item detail returns direct children. Attempt `ticket.view -> SUPPORT` and assert `409 RBAC_HIERARCHY_CONFLICT`. Assert duplicate `PUT` and missing `DELETE` edges are successful and idempotent. Assert built-in rename/delete returns `409 RBAC_SYSTEM_ITEM_PROTECTED`.

- [ ] **Step 2: Run the new API class and confirm red**

```powershell
mvn test -Dtest=RbacAdminApiTest
```

Expected: FAIL with unmapped/denied routes.

- [ ] **Step 3: Extend persistence with atomic graph mutations**

Add these repository signatures:

```java
List<RbacItem> findItems(AuthItemType type);
Optional<RbacItem> findItem(String name);
Set<String> findDirectChildren(String name);
boolean createItem(RbacItem item);
boolean renameItem(String currentName, String newName, String description, Instant now);
boolean updateDescription(String name, String description, Instant now);
boolean deleteCustomItem(String name);
boolean addChild(String parent, String child);
boolean removeChild(String parent, String child);
boolean isSuperAdmin(UUID userId);
long activeSuperAdminCount();
boolean revokeRole(UUID userId, String roleName);
```

Serialize edge mutations with `select pg_advisory_xact_lock(7242221)`. Before insert, load both item types and reject `PERMISSION -> ROLE`; run a recursive descendants query from `child` and reject when `parent` is reachable. Use `on conflict do nothing` for idempotent addition. Keep cycle/type decisions in `RbacAdminService`; repository returns data and mutation outcomes.

- [ ] **Step 4: Implement the application service**

Use these public methods:

```java
List<RbacItemView> list(AuthItemType type);
RbacItemView get(String name);
RbacItemView create(String name, AuthItemType type, String description);
RbacItemView update(String currentName, String name, String description);
void delete(String name);
void addChild(String parent, String child);
void removeChild(String parent, String child);
```

Define `RbacItemView` as:

```java
public record RbacItemView(
        String name,
        AuthItemType type,
        String description,
        boolean systemItem,
        Set<String> directChildren,
        Instant createdAt,
        Instant updatedAt) {}
```

Validate role names with `^[A-Z][A-Z0-9_]{1,127}$` and permission names with
`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$`. Reject system item rename/delete, unknown items, permission-to-role edges, self-edges, and cycles with the stable failures from the spec. Treat an already-present/missing edge as success.

- [ ] **Step 5: Add DTOs, controller, and error mappings**

Request records:

```java
public record CreateRbacItemRequest(
        @NotBlank @Size(max = 128) String name,
        @NotNull AuthItemType type,
        @Size(max = 255) String description) {}

public record UpdateRbacItemRequest(
        @NotBlank @Size(max = 128) String name,
        @Size(max = 255) String description) {}
```

Annotate read methods with `@PreAuthorize("@rbac.can(authentication, 'rbac.view')")` and mutations with `rbac.manage`. Return `ApiResponse` with messages such as `"RBAC item created."`, `"RBAC hierarchy updated."`, and `data: null` for edge/deletion success.

Map domain failures to `RBAC_ITEM_NOT_FOUND`, `RBAC_ITEM_EXISTS`, `RBAC_HIERARCHY_CONFLICT`, and `RBAC_SYSTEM_ITEM_PROTECTED` with the statuses in the spec.

- [ ] **Step 6: Run RBAC API and persistence tests**

```powershell
mvn test -Dtest=RbacPersistenceTest,RbacAdminApiTest,GlobalExceptionHandlerTest
```

Expected: PASS.

- [ ] **Step 7: Commit Task 3**

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/rbac services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/rbac services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java
git commit -m "feat(identity): add RBAC administration APIs" -- services/identity-service/src/main/java/com/ecommerce/identity/rbac services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/rbac services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java
```

## Task 4: Add Admin User Queries, Account Status, and Role Assignments

**Files:**
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/application/AdminUserService.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/application/AdminUserView.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/application/AdminUserPage.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/domain/SelfDisableNotAllowedException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/domain/LastSuperAdminRequiredException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/domain/SuperAdminAssignmentForbiddenException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/api/AdminUserController.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/api/UpdateUserStatusRequest.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/api/AdminUserResponse.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/user/admin/api/AdminUserPageResponse.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/user/admin/api/AdminUserApiTest.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserEntity.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`

**Interfaces:**
- Produces paginated `/admin/users`, user detail, status change, and idempotent role assignment/revocation.
- Uses Task 3 repository methods and preserves the last active super administrator.

- [ ] **Step 1: Write failing admin-user API tests**

Cover exact permission boundaries: `user.view` cannot update status or roles; `user.status.update` can change status; `user.role.assign` can assign/revoke normal roles. Verify list shape:

```java
.andExpect(jsonPath("$.data.items[0].userId").exists())
.andExpect(jsonPath("$.data.page").value(1))
.andExpect(jsonPath("$.data.pageSize").value(20))
.andExpect(jsonPath("$.data.totalItems").isNumber())
.andExpect(jsonPath("$.data.totalPages").isNumber());
```

Also test email/name search, stable ordering, self-disable conflict, disabled-user permission denial, non-super-admin attempt to grant `SUPER_ADMIN`, final-super-admin revoke/disable conflict, and repeated normal assignment/revocation success.

- [ ] **Step 2: Run the new class and confirm red**

```powershell
mvn test -Dtest=AdminUserApiTest
```

Expected: FAIL because `/admin/users` is not implemented.

- [ ] **Step 3: Add user persistence operations**

Make `IdentityUserRepository` extend `JpaSpecificationExecutor<IdentityUserEntity>` and add a pessimistic lock:

```java
@Lock(LockModeType.PESSIMISTIC_WRITE)
@Query("select u from IdentityUserEntity u where u.id = :id")
Optional<IdentityUserEntity> findForUpdateById(UUID id);
```

Add `IdentityUserEntity.changeStatus(AccountStatus status, Instant now)`. Build the list `Specification` from optional normalized email/full-name search, and page with `Sort.by(desc("createdAt"), asc("id"))`.

Add `RbacRepository.assignedRoles(Collection<UUID>)` returning a map so list responses do not perform one query per user.

- [ ] **Step 4: Implement lockout-safe application methods**

```java
AdminUserPage list(String query, int page, int pageSize);
AdminUserView get(UUID userId);
AdminUserView changeStatus(UUID actorId, UUID userId, AccountStatus status);
void assignRole(UUID actorId, UUID userId, String roleName);
void revokeRole(UUID actorId, UUID userId, String roleName);
```

Define `AdminUserView` with `UUID userId`, `String email`, `String fullName`,
`AccountStatus status`, `Set<String> roles`, `Instant createdAt`, and
`Instant updatedAt`. Define `AdminUserPage` with `List<AdminUserView> items`,
one-based `int page`, `int pageSize`, `long totalItems`, and `int totalPages`.

Use one-based public pages and zero-based `PageRequest`. Clamp validation with `@Min(1)` and `@Max(100)` rather than silently changing inputs. In a transaction, forbid `actorId.equals(userId) && status == DISABLED`; forbid disabling or revoking the final active `SUPER_ADMIN`; require the actor to be `SUPER_ADMIN` before changing a `SUPER_ADMIN` assignment.

- [ ] **Step 5: Implement REST mapping and stable errors**

Use `PATCH /admin/users/{userId}/status` body:

```java
public record UpdateUserStatusRequest(@NotNull AccountStatus status) {}
```

`AdminUserResponse` exposes `userId`, `email`, `fullName`, `status`, sorted `roles`, `createdAt`, and `updatedAt`. It never exposes hashes or sessions. Return the standard pagination object and meaningful mutation messages. Map `SELF_DISABLE_NOT_ALLOWED` and `LAST_SUPER_ADMIN_REQUIRED` to `409`.

- [ ] **Step 6: Run focused admin and auth regressions**

```powershell
mvn test -Dtest=AdminUserApiTest,LoginApiTest,RefreshApiTest,MeApiTest
```

Expected: PASS; a disabled user cannot login, refresh, or pass a permission check.

- [ ] **Step 7: Commit Task 4**

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/user/admin services/identity-service/src/main/java/com/ecommerce/identity/user/persistence services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/user/admin services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java
git commit -m "feat(identity): add admin user management" -- services/identity-service/src/main/java/com/ecommerce/identity/user/admin services/identity-service/src/main/java/com/ecommerce/identity/user/persistence services/identity-service/src/main/java/com/ecommerce/identity/rbac/persistence/RbacRepository.java services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/user/admin services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java
```

## Task 5: Implement Transactional Shipping-Address Ownership and Defaults

**Files:**
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/domain/ShippingAddressPatch.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/domain/ShippingAddressNotFoundException.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/application/CreateShippingAddressCommand.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/application/UpdateShippingAddressCommand.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/application/ShippingAddressView.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/application/ShippingAddressService.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/persistence/ShippingAddressEntity.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/persistence/ShippingAddressRepository.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/address/application/ShippingAddressServiceTest.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserRepository.java`

**Interfaces:**
- Produces caller-owned list/create/update/delete/set-default operations for Task 6.
- Guarantees at most one default through the V5 partial unique index plus user-row locking.

- [ ] **Step 1: Write failing service integration tests**

Test these state transitions against PostgreSQL:

1. First address becomes default.
2. Second address does not replace the default.
3. `setDefault` clears the previous default atomically and is idempotent.
4. Deleting the default promotes `createdAt DESC, id ASC` first.
5. Deleting the final address leaves no default.
6. Updating/deleting another user's address throws `ShippingAddressNotFoundException`.
7. Unknown province and mismatched ward produce `InvalidProfileLocationException` with the same field errors as profile validation.

Add a two-thread test using `ExecutorService` and a `CountDownLatch`; concurrently create the first two addresses for one user and assert exactly one persisted row has `is_default = true`.

- [ ] **Step 2: Run the service test and confirm red**

```powershell
mvn test -Dtest=ShippingAddressServiceTest
```

Expected: FAIL because address classes do not exist.

- [ ] **Step 3: Implement the entity and repository**

`ShippingAddressEntity` maps every V5 column and exposes focused methods:

```java
static ShippingAddressEntity create(UUID id, UUID userId, CreateShippingAddressCommand command,
        boolean defaultAddress, Instant now);
void apply(UpdateShippingAddressCommand command, Instant now);
void markDefault(Instant now);
void clearDefault(Instant now);
```

Repository signatures:

```java
List<ShippingAddressEntity> findAllByUserIdOrderByIsDefaultDescCreatedAtDescIdAsc(UUID userId);
Optional<ShippingAddressEntity> findByIdAndUserId(UUID id, UUID userId);
Optional<ShippingAddressEntity> findFirstByUserIdAndIsDefaultTrue(UUID userId);
Optional<ShippingAddressEntity> findFirstByUserIdOrderByCreatedAtDescIdAsc(UUID userId);
boolean existsByUserId(UUID userId);
```

Add `IdentityUserRepository.lockById(UUID)` with `PESSIMISTIC_WRITE`; the service separately rejects a missing/inactive owner.

- [ ] **Step 4: Implement service transactions and validation**

Public signatures:

```java
List<ShippingAddressView> list(UUID userId);
ShippingAddressView create(UUID userId, CreateShippingAddressCommand command);
ShippingAddressView update(UUID userId, UUID addressId, UpdateShippingAddressCommand command);
void delete(UUID userId, UUID addressId);
ShippingAddressView setDefault(UUID userId, UUID addressId);
```

Every mutation first locks the active owner row. Create decides `isDefault` only after the lock. Default switch clears the current default and flushes before marking the target to avoid transient partial-index conflicts. Delete flushes removal before promoting the deterministic replacement. Patch values distinguish omitted, set, and cleared; required address fields reject clear. Validate the resolved province/ward pair through `LocationCatalog.wardBelongsToProvince`.

Use these command shapes:

```java
public record CreateShippingAddressCommand(
        String recipientName, String phone, String provinceCode,
        String wardCode, String addressLine, String label) {}

public record UpdateShippingAddressCommand(
        ShippingAddressPatch<String> recipientName,
        ShippingAddressPatch<String> phone,
        ShippingAddressPatch<String> provinceCode,
        ShippingAddressPatch<String> wardCode,
        ShippingAddressPatch<String> addressLine,
        ShippingAddressPatch<String> label) {}
```

`ShippingAddressPatch<T>` has `KEEP`, `SET`, and `CLEAR` actions plus static
`keep()`, `set(T)`, and `clear()` factories; `set` rejects null.

- [ ] **Step 5: Run focused service and persistence tests**

```powershell
mvn test -Dtest=ShippingAddressServiceTest,IdentityDatabaseMigrationTest
```

Expected: PASS, including the concurrency invariant.

- [ ] **Step 6: Commit Task 5**

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/address services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserRepository.java services/identity-service/src/test/java/com/ecommerce/identity/address
git commit -m "feat(identity): persist customer shipping addresses" -- services/identity-service/src/main/java/com/ecommerce/identity/address services/identity-service/src/main/java/com/ecommerce/identity/user/persistence/IdentityUserRepository.java services/identity-service/src/test/java/com/ecommerce/identity/address
```

## Task 6: Expose Permission-Protected Shipping-Address REST APIs

**Files:**
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/api/CreateShippingAddressRequest.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/api/UpdateShippingAddressRequest.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/api/ShippingAddressResponse.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/address/api/ShippingAddressController.java`
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/address/api/ShippingAddressApiTest.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`

**Interfaces:**
- Produces all five `/users/me/addresses` endpoints with exact `address.*` permission checks.
- Never accepts an address owner ID from a request.

- [ ] **Step 1: Write failing MockMvc permission and contract tests**

For each operation, remove only its hierarchy edge from `CUSTOMER` and assert the corresponding endpoint returns `403 FORBIDDEN`; verify a neighboring permission does not authorize it. Restore the edge and assert success. Include:

```java
mockMvc.perform(post("/users/me/addresses")
        .header("X-User-Id", userId)
        .contentType(APPLICATION_JSON)
        .content("""
            {"recipientName":"Nguyen Van A","phone":"0901234567",
             "provinceCode":"79","wardCode":"26740",
             "addressLine":"12 Le Loi","label":"Nhà riêng"}
            """))
        .andExpect(status().isCreated())
        .andExpect(jsonPath("$.data.isDefault").value(true))
        .andExpect(jsonPath("$.message").value("Shipping address created."));
```

Test list ordering, partial update, idempotent default selection, default promotion, validation errors, and cross-owner `404 ADDRESS_NOT_FOUND`. Assert missing caller context is `401`, not `403`.

- [ ] **Step 2: Run the API test and confirm red**

```powershell
mvn test -Dtest=ShippingAddressApiTest
```

Expected: FAIL because the controller does not exist.

- [ ] **Step 3: Implement validated request mapping**

Create request requires nonblank `recipientName`, `phone`, `provinceCode`, `wardCode`, and `addressLine`; use the existing Vietnamese phone regex and spec maximum lengths. `label` is nullable but, when supplied, must not be blank and must be at most 50 characters.

Update request uses nullable fields plus `@JsonSetter` presence flags so omitted differs from explicit null. Explicit null/blank for required fields returns `VALIDATION_FAILED`; explicit null or blank label clears it. Map those states into `ShippingAddressPatch.keep/set/clear` in `UpdateShippingAddressCommand`.

- [ ] **Step 4: Implement controller and permission annotations**

Use exact annotations:

```java
@PreAuthorize("@rbac.can(authentication, 'address.view')")
@GetMapping("/users/me/addresses")

@PreAuthorize("@rbac.can(authentication, 'address.create')")
@PostMapping("/users/me/addresses")

@PreAuthorize("@rbac.can(authentication, 'address.update')")
@PatchMapping("/users/me/addresses/{addressId}")

@PreAuthorize("@rbac.can(authentication, 'address.delete')")
@DeleteMapping("/users/me/addresses/{addressId}")

@PreAuthorize("@rbac.can(authentication, 'address.set-default')")
@PutMapping("/users/me/addresses/{addressId}/default")
```

Every method takes `@AuthenticationPrincipal CallerContextPrincipal caller` and calls the service with `caller.userId()`. Return `201` on create, `200` on other operations, and `{data:null}` for delete. Map `ShippingAddressNotFoundException` to `404 ADDRESS_NOT_FOUND`.

- [ ] **Step 5: Run address and complete Identity tests**

```powershell
mvn test -Dtest=ShippingAddressApiTest,ShippingAddressServiceTest
Set-Location ..\..
mvn -pl services/identity-service -am test
```

Expected: PASS.

- [ ] **Step 6: Commit Task 6**

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/address/api services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/address/api services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java
git commit -m "feat(identity): expose protected shipping address APIs" -- services/identity-service/src/main/java/com/ecommerce/identity/address/api services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/address/api services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java
```

## Task 7: Separate Kong Admin Routes and Document Super-Admin Bootstrap

**Files:**
- Modify: `deployments/kong/kong.yaml`
- Modify: `services/bff-service/internal/config/config_test.go`
- Create: `services/identity-service/RBAC.md`

**Interfaces:**
- `/api/v1/admin/products/**` continues reaching the BFF as `/products/**`.
- `/api/v1/admin/users/**` and `/api/v1/admin/rbac/**` reach Identity through its protected `/api/v1` route.
- Documents the only bootstrap operation that bypasses the admin API.

- [ ] **Step 1: Write a failing local Kong routing test**

Refactor the anonymous YAML test structs into named `kongConfig`, `kongService`,
and `kongRoute` types without changing the production-route assertions. Add
`TestDevelopmentKongSeparatesProductAndIdentityAdminRoutes`, parse
`deployments/kong/kong.yaml`, locate service `bff-admin-products`, and assert:

```go
if adminProductsService.URL != "http://host.docker.internal:8081/products" {
    t.Fatalf("admin products upstream = %q", adminProductsService.URL)
}
if len(adminProductsRoute.Paths) != 1 || adminProductsRoute.Paths[0] != "/api/v1/admin/products" || !adminProductsRoute.StripPath {
    t.Fatalf("admin products route does not preserve /products mapping")
}
for _, service := range config.Services {
    for _, route := range service.Routes {
        for _, path := range route.Paths {
            if path == "/api/v1/admin" {
                t.Fatalf("route %s shadows all Identity admin APIs", route.Name)
            }
        }
    }
}
```

Also assert no route claims the broad `/api/v1/admin` prefix.

- [ ] **Step 2: Run the Go test and confirm red**

```powershell
go test ./services/bff-service/internal/config -run TestDevelopmentKongSeparatesProductAndIdentityAdminRoutes -count=1
```

Expected: FAIL because the current route claims `/api/v1/admin`.

- [ ] **Step 3: Split the development-only BFF admin service entry**

Remove `admin-products-api` from the shared `bff-service` routes and add:

```yaml
  - name: bff-admin-products
    url: http://host.docker.internal:8081/products
    routes:
      - name: admin-products-api
        paths: ["/api/v1/admin/products"]
        strip_path: true
```

Do not add it to `kong-production.yaml`; its existing test requires production to exclude this development-only route. The existing protected Identity `/api/v1` route receives the remaining admin paths and applies caller-context.

- [ ] **Step 4: Document the bootstrap and recovery commands**

Create `services/identity-service/RBAC.md` with this local bootstrap statement:

```sql
insert into auth_assignments (user_id, item_name, created_at)
select '<USER_UUID>'::uuid, name, now()
from auth_items
where name = 'SUPER_ADMIN' and type = 'ROLE'
on conflict (user_id, item_name) do nothing;
```

Document verification with a read-only select, state that normal changes must use admin APIs, and describe recovery through a database backup or forward Flyway migration. Do not include credentials, tokens, or a real UUID.

- [ ] **Step 5: Run routing/config checks**

```powershell
go test ./services/bff-service/internal/config -count=1
docker compose -f deployments/docker/compose.foundation.yml config
```

Expected: PASS; production still excludes the development product-admin route.

- [ ] **Step 6: Commit Task 7**

```powershell
git add deployments/kong/kong.yaml services/bff-service/internal/config/config_test.go services/identity-service/RBAC.md
git commit -m "feat(edge): separate identity and product admin routes" -- deployments/kong/kong.yaml services/bff-service/internal/config/config_test.go services/identity-service/RBAC.md
```

## Task 8: Run Cross-Surface Verification and Security Smoke Checks

**Files:**
- Modify only when a verification failure proves a defect in a task-owned file.

**Interfaces:**
- Verifies migration safety, RBAC revocation immediacy, admin protection, shipping-address ownership/defaults, and Kong route selection.

- [ ] **Step 1: Run focused security and migration gates**

```powershell
Set-Location services\identity-service
mvn test -Dtest=IdentityDatabaseMigrationTest,RbacPersistenceTest,RbacAuthorizationServiceTest,RbacAdminApiTest,AdminUserApiTest,ShippingAddressServiceTest,ShippingAddressApiTest
```

Expected: PASS with no skipped Testcontainers tests.

- [ ] **Step 2: Run the complete affected Java reactor**

```powershell
Set-Location ..\..
mvn -pl services/identity-service -am test
```

Expected: PASS for contracts and Identity modules.

- [ ] **Step 3: Run the affected Go and declarative-config checks**

```powershell
go test ./services/bff-service/internal/config -count=1
docker compose -f deployments/docker/compose.foundation.yml config
```

Expected: PASS.

- [ ] **Step 4: Perform a bounded local HTTP smoke test when foundation is available**

Start foundation in the background, start Identity with the documented Windows command/session mechanism, wait for `/actuator/health`, and verify through Kong:

1. Register customer and bootstrap that UUID as `SUPER_ADMIN` using the documented SQL.
2. Login and create a custom role containing `address.view` but not `address.create`.
3. Assign that role to a second customer and revoke `CUSTOMER`; with the same unexpired access token, `GET /users/me/addresses` returns `200` and `POST` returns `403`.
4. Add `address.create`; retry with the same token and observe `201` immediately.
5. Create two addresses, mark the second default, delete it, and verify the first is promoted.
6. Attempt to update another user's address ID and verify `404 ADDRESS_NOT_FOUND`.
7. Verify `/api/v1/admin/products/...` still reaches BFF while `/api/v1/admin/users` reaches Identity.

Stop the Identity process after the bounded check and retain logs only in a temporary location.

- [ ] **Step 5: Inspect final repository state**

```powershell
git diff --check
git status --short
git log --oneline -10
```

Expected: no whitespace errors; feature commits contain only their explicit paths; unrelated pre-existing staged changes remain untouched.

- [ ] **Step 6: Prepare the evidence handoff**

Report exact commands and results, migration/route changes, tests skipped with reasons, the bootstrap procedure location, and any residual operational risk. Do not claim the smoke test passed unless the complete bounded flow ran in the current environment.

# Identity RBAC and Shipping Address Design

## Summary

Identity Service provides database-backed hierarchical RBAC modeled after Yii2
and a separate shipping-address book for authenticated customers. Administrators
manage users, authorization items, hierarchy edges, and role assignments.
Authorization checks use current database state so grants and revocations take
effect immediately without waiting for access-token renewal.

Personal address fields in `identity_profiles` remain independent from shipping
addresses. No profile data is copied into the shipping-address table.

## Scope

### Included

- General hierarchical RBAC with role and permission items.
- Role-to-role, role-to-permission, and permission-to-permission inheritance.
- Role assignments to users.
- User listing, user detail, account activation/deactivation, and role assignment
  administration.
- RBAC item and hierarchy administration.
- Customer shipping-address CRUD and default-address selection.
- Identity REST, persistence, security, migration, and focused Kong routing
  changes.

### Excluded

- Executable or database-defined authorization rules.
- Direct permission assignments to users.
- Admin editing of customer profiles or shipping addresses.
- User deletion.
- Frontend administration screens or shipping-address UI.
- Permission propagation to other services.
- Copying personal profile address data into shipping addresses.

## Ownership and Boundaries

Identity Service owns users, account status, roles, permissions, assignments,
shipping addresses, and all related invariants. The new behavior does not add a
cross-service data dependency.

Kong remains the public ingress and establishes the trusted caller context. The
Identity application uses the authenticated user ID as the subject of every
permission check. Request bodies and path parameters never select the owner of a
customer shipping address.

The existing JWT `roles` claim remains backward compatible. Permissions are not
added to the token. Identity resolves effective permissions from its database on
every protected operation, which avoids stale grants and revocations.

## Authorization Model

### Authorization items

An authorization item has:

- `name`: stable unique identifier such as `CUSTOMER` or `address.create`.
- `type`: `ROLE` or `PERMISSION`.
- `description`: administrator-facing description.
- `system_item`: whether rename and deletion are prohibited.
- `created_at` and `updated_at`.

Names use uppercase snake case for built-in roles and lower-case dot notation for
permissions. Custom items follow the same convention for their type.

### Hierarchy

`parent -> child` means the parent contains the child. Supported edges are:

- role -> role;
- role -> permission;
- permission -> permission.

A permission cannot contain a role. Self-edges and edges that create a direct or
indirect cycle are rejected; the primary key prevents duplicate stored edges.
Effective permissions are all
permission descendants reachable from the roles assigned to a user. A directly
assigned role is also an effective role.

### Assignments

Only roles can be assigned directly to users. An assignment is unique by
`(user_id, item_name)`. Assigning an existing assignment and revoking a missing
assignment are idempotent operations.

### Built-in items

The migration seeds these roles:

- `CUSTOMER`;
- `ADMIN`;
- `SUPER_ADMIN`.

The migration seeds these permissions:

- `address.view`;
- `address.create`;
- `address.update`;
- `address.delete`;
- `address.set-default`;
- `user.view`;
- `user.status.update`;
- `user.role.assign`;
- `rbac.view`;
- `rbac.manage`.

`CUSTOMER` initially contains all `address.*` permissions. Administrators may
change its hierarchy edges, so removing `address.create` makes address creation
return `403` for users who do not inherit that permission elsewhere.

`ADMIN` initially contains the user-management and RBAC-management permissions.
Its hierarchy edges may be changed. `SUPER_ADMIN` is immutable and is treated as
having every authorization item, including items created after deployment.

All built-in roles and permissions are system items: their names and existence
are protected, while the hierarchy of `CUSTOMER` and `ADMIN` remains editable.

### Bootstrap and lockout protection

The first administrator registers through the normal user flow and receives the
`SUPER_ADMIN` assignment through an explicit local `psql` operation using that
user's UUID. No fixed administrator identity or credential is stored in source
control.

The authorization application service enforces these invariants:

- only a `SUPER_ADMIN` may grant or revoke `SUPER_ADMIN`;
- the final `SUPER_ADMIN` assignment cannot be revoked;
- the last active user assigned `SUPER_ADMIN` cannot be disabled;
- `SUPER_ADMIN` cannot be renamed, deleted, or have its implicit inheritance
  changed;
- an administrator cannot disable their own account;
- a disabled user has no effective permissions.

### Spring Security integration

`CallerContextPrincipal.getAuthorities()` exposes caller roles as
`ROLE_<ROLE_NAME>` authorities for conventional `hasRole` interoperability.
Permission-protected operations use a dedicated evaluator:

```java
@PreAuthorize("@rbac.can(authentication, 'address.create')")
```

`RbacAuthorizationService.can(userId, permission)` is the source of truth. It
verifies that the user is active, resolves assigned roles and recursive item
children, and returns false for unknown permissions. Phase one deliberately does
not cache authorization results.

## Persistence

### RBAC tables

`auth_items`:

- `name varchar(128)` primary key;
- `type varchar(16)` with a `ROLE`/`PERMISSION` check constraint;
- `description varchar(255)`;
- `system_item boolean not null`;
- `created_at timestamptz not null`;
- `updated_at timestamptz not null`.

`auth_item_children`:

- `parent_name` references `auth_items(name)`;
- `child_name` references `auth_items(name)`;
- primary key `(parent_name, child_name)`;
- check constraint preventing a self-edge.

Application logic validates type compatibility and performs cycle detection in
the same transaction that inserts an edge. The transaction serializes hierarchy
mutations so concurrent inserts cannot jointly create a cycle.

`auth_assignments`:

- `user_id` references `identity_users(id)` with cascade delete;
- `item_name` references `auth_items(name)`;
- `created_at timestamptz not null`;
- primary key `(user_id, item_name)`.

Application logic permits assignment only when the referenced item is a role.

### Migration of existing authorization data

A new Flyway migration creates the RBAC tables, copies each existing `roles` row
as a role item, copies `user_roles` rows as assignments, seeds missing built-in
items and hierarchy edges, and then removes the superseded tables. Built-in
items are marked as system items; any other migrated role remains editable.
Existing `CUSTOMER` assignments are preserved. The migration is Identity-private
and does not affect another service's schema.

### Shipping-address table

`shipping_addresses` contains:

- `id uuid` primary key;
- `user_id` referencing `identity_users(id)` with cascade delete;
- `recipient_name varchar(100)`;
- `phone varchar(20)`;
- `province_code varchar(20)`;
- `ward_code varchar(20)`;
- `address_line varchar(255)`;
- `label varchar(50)` nullable;
- `is_default boolean not null`;
- `created_at timestamptz not null`;
- `updated_at timestamptz not null`.

A partial unique index on `user_id WHERE is_default` guarantees at most one
default address per user. Mutations that can affect the default address lock the
owning `identity_users` row, making first-address creation, default switching,
and default deletion deterministic under concurrency.

The first shipping address becomes the default. Setting another address as
default clears the previous default in the same transaction. Deleting the
default promotes the most recently created remaining address, with `id` as a
stable tie-breaker. Deleting the final address leaves no default address.

## REST API

All new success responses use `{ "data": ..., "message": "..." }`. Errors use
the repository error envelope with a stable code and trace ID.

### Customer shipping addresses

| Method | Path | Permission | Behavior |
| --- | --- | --- | --- |
| `GET` | `/users/me/addresses` | `address.view` | Lists the caller's addresses, default first. |
| `POST` | `/users/me/addresses` | `address.create` | Creates an address; the first becomes default. |
| `PATCH` | `/users/me/addresses/{addressId}` | `address.update` | Partially updates an address owned by the caller. |
| `DELETE` | `/users/me/addresses/{addressId}` | `address.delete` | Deletes an owned address and promotes a replacement default when required. |
| `PUT` | `/users/me/addresses/{addressId}/default` | `address.set-default` | Idempotently marks an owned address as default. |

Create and update payloads use `recipientName`, `phone`, `provinceCode`,
`wardCode`, `addressLine`, and optional `label`. Responses add `id`, `isDefault`,
`createdAt`, and `updatedAt`. Province and ward pairs use the existing Identity
location catalog validation. Existing profile endpoints and fields do not
change.

### Admin user management

| Method | Path | Permission | Behavior |
| --- | --- | --- | --- |
| `GET` | `/admin/users` | `user.view` | Returns paginated users and supports email/name search. |
| `GET` | `/admin/users/{userId}` | `user.view` | Returns user status, profile summary, and assigned roles. |
| `PATCH` | `/admin/users/{userId}/status` | `user.status.update` | Changes status between `ACTIVE` and `DISABLED`. |
| `PUT` | `/admin/users/{userId}/roles/{roleName}` | `user.role.assign` | Idempotently assigns a role. |
| `DELETE` | `/admin/users/{userId}/roles/{roleName}` | `user.role.assign` | Idempotently revokes a role subject to lockout protection. |

The list endpoint uses the standard pagination object under `data` and applies a
stable `createdAt DESC, userId ASC` order. Search is case-insensitive and matches
normalized email or full name. Password hashes, refresh sessions, tokens, and
other secrets are never returned.

### Admin RBAC management

| Method | Path | Permission | Behavior |
| --- | --- | --- | --- |
| `GET` | `/admin/rbac/items` | `rbac.view` | Lists items, optionally filtered by type. |
| `POST` | `/admin/rbac/items` | `rbac.manage` | Creates a custom role or permission. |
| `GET` | `/admin/rbac/items/{name}` | `rbac.view` | Returns an item and its direct children. |
| `PATCH` | `/admin/rbac/items/{name}` | `rbac.manage` | Updates a custom item's name or description. |
| `DELETE` | `/admin/rbac/items/{name}` | `rbac.manage` | Deletes a custom item and its edges/assignments. |
| `PUT` | `/admin/rbac/items/{parent}/children/{child}` | `rbac.manage` | Idempotently adds a valid hierarchy edge. |
| `DELETE` | `/admin/rbac/items/{parent}/children/{child}` | `rbac.manage` | Idempotently removes an editable hierarchy edge. |

Path item names are URL-encoded. Renaming a custom item cascades to its hierarchy
edges and assignments in one transaction. System items may update their
descriptions but cannot be renamed or deleted.

## Error Semantics

- Missing or malformed authenticated caller context: `401 UNAUTHORIZED`.
- Authenticated caller without the required permission: `403 FORBIDDEN`.
- Missing address or an address owned by another user: `404 ADDRESS_NOT_FOUND`.
- Missing user: `404 USER_NOT_FOUND`.
- Missing authorization item: `404 RBAC_ITEM_NOT_FOUND`.
- Invalid request fields or location pair: `400 VALIDATION_FAILED`.
- Duplicate item name: `409 RBAC_ITEM_EXISTS`.
- Invalid type edge or hierarchy cycle: `409 RBAC_HIERARCHY_CONFLICT`.
- Protected system-item mutation: `409 RBAC_SYSTEM_ITEM_PROTECTED`.
- Attempt to remove the last super administrator:
  `409 LAST_SUPER_ADMIN_REQUIRED`.
- Attempt to disable the current administrator:
  `409 SELF_DISABLE_NOT_ALLOWED`.
- Attempt to disable the final active super administrator:
  `409 LAST_SUPER_ADMIN_REQUIRED`.

An idempotent assignment, revocation, edge addition, edge removal, or default
selection returns success when the requested end state already exists.

## Kong Routing

The development Kong configuration currently routes all `/api/v1/admin` traffic
to the BFF admin-products handler. It is narrowed to
`/api/v1/admin/products/**`, with its upstream path mapped to `/products/**` so
the frontend URL remains unchanged.

The Identity `/api/v1` protected route then receives
`/api/v1/admin/users/**` and `/api/v1/admin/rbac/**`. The caller-context plugin
continues validating the JWT and replacing caller headers before forwarding.
Production currently has no BFF admin-products route, so its Identity protected
route already covers the new admin paths.

## Verification Strategy

### Unit tests

- Permission traversal across nested roles and permissions.
- Unknown item and inactive-user denial.
- Edge type validation and direct/indirect cycle detection.
- Protected-item and last-super-admin invariants.
- Shipping-address default selection and promotion rules.

### Focused Spring tests

- Each endpoint returns `401` without caller context.
- Each endpoint returns `403` when its exact permission is absent.
- Each endpoint succeeds when permission is inherited through the hierarchy.
- Address ownership returns `404` without leaking another user's address.
- Admin user and RBAC mutations return the documented envelopes and stable error
  codes.

### PostgreSQL integration tests

- Flyway migrates existing roles and assignments without loss.
- Hierarchy persistence and recursive permission resolution.
- Concurrent first-address creation and default switching preserve the unique
  default invariant.
- Deleting a default address promotes the deterministic replacement.

### Surface and routing verification

- Run focused Identity tests during red-green development.
- Run `mvn -pl services/identity-service -am test` from the repository root.
- Run affected BFF configuration tests after the development Kong route change.
- Validate Kong declarative configuration and perform bounded HTTP checks when
  the local foundation is available.

## Rollout and Recovery

The Identity migration must complete before deploying code that reads the new
RBAC tables. Existing JWTs remain valid because the token contract is unchanged.
Authorization changes take effect immediately from database state.

Before production rollout, an operator registers or identifies the bootstrap
administrator and executes the documented assignment statement using that UUID.
The assignment is verified before exposing RBAC administration endpoints.

The migration replaces Identity-private authorization tables after copying their
data. Recovery uses the pre-deployment database backup or a forward migration;
the applied Flyway migration is never edited. Shipping-address data is additive
and has no profile-data rollback dependency.

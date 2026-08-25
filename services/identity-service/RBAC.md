# Identity RBAC Operations

Identity stores hierarchical authorization items in PostgreSQL. Roles are
assigned to users through `auth_assignments`; permissions are inherited through
`auth_item_children`. Normal changes must use the protected `/admin/rbac` and
`/admin/users` APIs so the service can enforce hierarchy and lockout invariants.

## Bootstrap the first super administrator

Register the administrator through the normal registration endpoint, record the
returned `userId`, and execute this statement against the Identity database:

```sql
insert into auth_assignments (user_id, item_name, created_at)
select '<USER_UUID>'::uuid, name, now()
from auth_items
where name = 'SUPER_ADMIN' and type = 'ROLE'
on conflict (user_id, item_name) do nothing;
```

Replace `<USER_UUID>` with the registered user's UUID. Verify the assignment
without exposing credentials or tokens:

```sql
select user_id, item_name, created_at
from auth_assignments
where user_id = '<USER_UUID>'::uuid
  and item_name = 'SUPER_ADMIN';
```

After verification, use the admin APIs for all subsequent role, permission,
hierarchy, user-status, and assignment changes. The final active
`SUPER_ADMIN` assignment cannot be revoked, and a super administrator cannot be
disabled when it is the last active assignment.

## Opt-in startup seed

For local development, Identity can create a `SUPER_ADMIN` during application
startup when all three variables below are supplied:

```powershell
$env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL = "root@example.test"
$env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD = "SecurePassword123!"
$env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME = "Local Root"
```

The equivalent shell variables are
`IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL`,
`IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD`, and
`IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME`. The feature is disabled when any
variable is missing. If the email already exists, startup leaves that account's
password, status, and roles unchanged. Credentials are read from environment
or a secret manager and are never logged or committed.

This seed is intended for explicit local/bootstrap use. Do not enable it in
production unless the values come from a controlled secret manager and the
resulting account is reviewed.

## Permission changes

The customer address permissions are independent:

- `address.view`
- `address.create`
- `address.update`
- `address.delete`
- `address.set-default`

Removing an edge such as `CUSTOMER -> address.create` immediately makes the
corresponding endpoint return `403 FORBIDDEN`; existing access tokens do not need
to be reissued. The Identity database is the source of truth for each permission
check.

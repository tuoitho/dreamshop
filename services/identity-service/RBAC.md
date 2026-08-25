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

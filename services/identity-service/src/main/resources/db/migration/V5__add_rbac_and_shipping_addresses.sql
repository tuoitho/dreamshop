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
select code, 'ROLE', code || ' role', false, now(), now()
from roles;

insert into auth_items (name, type, description, system_item, created_at, updated_at)
values
    ('ADMIN', 'ROLE', 'Administrator role', true, now(), now()),
    ('SUPER_ADMIN', 'ROLE', 'Super administrator role', true, now(), now()),
    ('address.view', 'PERMISSION', 'View shipping addresses', true, now(), now()),
    ('address.create', 'PERMISSION', 'Create shipping addresses', true, now(), now()),
    ('address.update', 'PERMISSION', 'Update shipping addresses', true, now(), now()),
    ('address.delete', 'PERMISSION', 'Delete shipping addresses', true, now(), now()),
    ('address.set-default', 'PERMISSION', 'Set the default shipping address', true, now(), now()),
    ('user.view', 'PERMISSION', 'View users', true, now(), now()),
    ('user.status.update', 'PERMISSION', 'Change user account status', true, now(), now()),
    ('user.role.assign', 'PERMISSION', 'Assign and revoke user roles', true, now(), now()),
    ('rbac.view', 'PERMISSION', 'View RBAC items', true, now(), now()),
    ('rbac.manage', 'PERMISSION', 'Manage RBAC items and hierarchy', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

update auth_items
set system_item = true,
    updated_at = now()
where name in ('CUSTOMER', 'ADMIN', 'SUPER_ADMIN');

insert into auth_assignments (user_id, item_name, created_at)
select user_roles.user_id, roles.code, now()
from user_roles
join roles on roles.id = user_roles.role_id;

insert into auth_item_children (parent_name, child_name)
values
    ('CUSTOMER', 'address.view'),
    ('CUSTOMER', 'address.create'),
    ('CUSTOMER', 'address.update'),
    ('CUSTOMER', 'address.delete'),
    ('CUSTOMER', 'address.set-default'),
    ('ADMIN', 'user.view'),
    ('ADMIN', 'user.status.update'),
    ('ADMIN', 'user.role.assign'),
    ('ADMIN', 'rbac.view'),
    ('ADMIN', 'rbac.manage')
on conflict do nothing;

drop table user_roles;
drop table roles;

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

insert into auth_items (name, type, description, system_item, created_at, updated_at)
values ('favorite.manage', 'PERMISSION', 'Manage product favorites for the current user', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values ('CUSTOMER', 'favorite.manage')
on conflict do nothing;

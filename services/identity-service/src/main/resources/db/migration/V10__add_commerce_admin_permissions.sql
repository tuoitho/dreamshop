insert into auth_items (name, type, description, system_item, created_at, updated_at)
values
    ('product.view', 'PERMISSION', 'View all catalog products including soft-deleted items', true, now(), now()),
    ('product.create', 'PERMISSION', 'Create catalog products', true, now(), now()),
    ('product.update', 'PERMISSION', 'Update catalog products', true, now(), now()),
    ('inventory.view', 'PERMISSION', 'View inventory stock levels', true, now(), now()),
    ('inventory.update', 'PERMISSION', 'Adjust and set inventory stock quantities', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values
    ('ADMIN', 'product.view'),
    ('ADMIN', 'product.create'),
    ('ADMIN', 'product.update'),
    ('ADMIN', 'inventory.view'),
    ('ADMIN', 'inventory.update')
on conflict do nothing;

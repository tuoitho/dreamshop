insert into auth_items (name, type, description, system_item, created_at, updated_at)
values
    ('order.admin.view', 'PERMISSION', 'View all orders in the admin console', true, now(), now()),
    ('order.admin.update', 'PERMISSION', 'Update order fulfillment status in the admin console', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values
    ('ADMIN', 'order.admin.view'),
    ('ADMIN', 'order.admin.update')
on conflict do nothing;

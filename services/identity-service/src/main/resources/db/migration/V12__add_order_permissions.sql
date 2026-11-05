insert into auth_items (name, type, description, system_item, created_at, updated_at)
values
    ('order.create', 'PERMISSION', 'Create orders from the current user checkout', true, now(), now()),
    ('order.view', 'PERMISSION', 'View the current user orders', true, now(), now()),
    ('order.cancel', 'PERMISSION', 'Cancel eligible orders owned by the current user', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values
    ('CUSTOMER', 'order.create'),
    ('CUSTOMER', 'order.view'),
    ('CUSTOMER', 'order.cancel')
on conflict do nothing;

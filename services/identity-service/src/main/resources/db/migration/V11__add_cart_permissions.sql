insert into auth_items (name, type, description, system_item, created_at, updated_at)
values
    ('cart.view', 'PERMISSION', 'View the current user cart', true, now(), now()),
    ('cart.item.update', 'PERMISSION', 'Update item quantities in the current user cart', true, now(), now()),
    ('cart.item.remove', 'PERMISSION', 'Remove items from the current user cart', true, now(), now()),
    ('cart.clear', 'PERMISSION', 'Clear the current user cart', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values
    ('CUSTOMER', 'cart.view'),
    ('CUSTOMER', 'cart.item.update'),
    ('CUSTOMER', 'cart.item.remove'),
    ('CUSTOMER', 'cart.clear')
on conflict do nothing;

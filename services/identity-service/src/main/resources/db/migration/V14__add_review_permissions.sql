insert into auth_items (name, type, description, system_item, created_at, updated_at)
values
    ('review.create', 'PERMISSION', 'Create, update, and delete product reviews for delivered orders', true, now(), now()),
    ('review.admin.view', 'PERMISSION', 'View all product reviews in the admin console', true, now(), now()),
    ('review.admin.moderate', 'PERMISSION', 'Hide, restore, and reply to product reviews in the admin console', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values
    ('CUSTOMER', 'review.create'),
    ('ADMIN', 'review.admin.view'),
    ('ADMIN', 'review.admin.moderate')
on conflict do nothing;

insert into auth_items (name, type, description, system_item, created_at, updated_at)
values ('product.delete', 'PERMISSION', 'Soft delete or restore catalog products', true, now(), now())
on conflict (name) do update set
    type = excluded.type,
    description = excluded.description,
    system_item = true,
    updated_at = now();

insert into auth_item_children (parent_name, child_name)
values ('ADMIN', 'product.delete')
on conflict do nothing;

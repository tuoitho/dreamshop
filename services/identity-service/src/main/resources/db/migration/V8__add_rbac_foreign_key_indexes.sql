create index if not exists ix_auth_assignments_item_name
    on auth_assignments (item_name);

create index if not exists ix_auth_item_children_child_name
    on auth_item_children (child_name);

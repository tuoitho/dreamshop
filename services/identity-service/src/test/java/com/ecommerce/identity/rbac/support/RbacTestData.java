package com.ecommerce.identity.rbac.support;

import org.springframework.jdbc.core.JdbcTemplate;

public final class RbacTestData {
    private RbacTestData() {
    }

    public static void resetAuthorizationData(JdbcTemplate jdbc) {
        jdbc.update("delete from auth_assignments");
        jdbc.update("delete from auth_item_children");
        jdbc.update("delete from auth_items");
        jdbc.update("""
                insert into auth_items (name, type, description, system_item, created_at, updated_at)
                values
                    ('CUSTOMER', 'ROLE', 'Customer role', true, now(), now()),
                    ('ADMIN', 'ROLE', 'Administrator role', true, now(), now()),
                    ('SUPER_ADMIN', 'ROLE', 'Super administrator role', true, now(), now()),
                    ('address.view', 'PERMISSION', 'View shipping addresses', true, now(), now()),
                    ('address.create', 'PERMISSION', 'Create shipping addresses', true, now(), now()),
                    ('address.update', 'PERMISSION', 'Update shipping addresses', true, now(), now()),
                    ('address.delete', 'PERMISSION', 'Delete shipping addresses', true, now(), now()),
                    ('address.set-default', 'PERMISSION', 'Set default shipping address', true, now(), now()),
                    ('cart.item.add', 'PERMISSION', 'Add products to the current user cart', true, now(), now()),
                    ('product.delete', 'PERMISSION', 'Soft delete or restore catalog products', true, now(), now()),
                    ('user.view', 'PERMISSION', 'View users', true, now(), now()),
                    ('user.status.update', 'PERMISSION', 'Update user status', true, now(), now()),
                    ('user.role.assign', 'PERMISSION', 'Assign user roles', true, now(), now()),
                    ('rbac.view', 'PERMISSION', 'View RBAC items', true, now(), now()),
                    ('rbac.manage', 'PERMISSION', 'Manage RBAC items', true, now(), now())
                """);
        jdbc.update("""
                insert into auth_item_children (parent_name, child_name)
                values
                    ('CUSTOMER', 'address.view'),
                    ('CUSTOMER', 'address.create'),
                    ('CUSTOMER', 'address.update'),
                    ('CUSTOMER', 'address.delete'),
                    ('CUSTOMER', 'address.set-default'),
                    ('CUSTOMER', 'cart.item.add'),
                    ('ADMIN', 'user.view'),
                    ('ADMIN', 'user.status.update'),
                    ('ADMIN', 'user.role.assign'),
                    ('ADMIN', 'rbac.view'),
                    ('ADMIN', 'rbac.manage'),
                    ('ADMIN', 'product.delete')
                """);
    }

    public static void resetDatabase(JdbcTemplate jdbc) {
        jdbc.update("delete from refresh_token_sessions");
        jdbc.update("delete from identity_profiles");
        jdbc.update("delete from identity_users");
        resetAuthorizationData(jdbc);
    }
}

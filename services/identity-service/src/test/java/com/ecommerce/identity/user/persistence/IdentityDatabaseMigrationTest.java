package com.ecommerce.identity.user.persistence;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.transaction.annotation.Transactional;

import static org.assertj.core.api.Assertions.assertThat;

class IdentityDatabaseMigrationTest extends PostgresIntegrationTestSupport {
    @Autowired
    JdbcClient jdbc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @Test
    void createsAccountTablesAndSeedsCustomerRole() {
        List<String> tables = jdbc.sql("""
                select table_name from information_schema.tables where table_schema = 'public'
                """).query(String.class).list();

        assertThat(tables)
                .contains("identity_users", "identity_profiles", "auth_items",
                        "auth_item_children", "auth_assignments", "shipping_addresses")
                .doesNotContain("roles", "user_roles");
        assertThat(jdbc.sql("""
                select column_name || ':' || data_type
                from information_schema.columns
                where table_schema = 'public' and table_name = 'identity_users'
                """).query(String.class).list())
                .contains(
                        "id:uuid",
                        "email:character varying",
                        "normalized_email:character varying",
                        "password_hash:character varying",
                        "status:character varying",
                        "created_at:timestamp with time zone",
                        "updated_at:timestamp with time zone");
        assertThat(jdbc.sql("""
                select conname
                from pg_constraint
                where conrelid = 'identity_users'::regclass
                  and contype = 'u'
                  and pg_get_constraintdef(oid) = 'UNIQUE (normalized_email)'
                """).query(String.class).list())
                .containsExactly("uq_identity_users_normalized_email");
        assertThat(jdbc.sql("select name from auth_items where type = 'ROLE' order by name")
                .query(String.class).list())
                .contains("ADMIN", "CUSTOMER", "SUPER_ADMIN");
        assertThat(jdbc.sql("select name from auth_items where type = 'PERMISSION' order by name")
                .query(String.class).list())
                .containsExactly("address.create", "address.delete", "address.set-default",
                        "address.update", "address.view", "cart.clear", "cart.item.add",
                        "cart.item.remove", "cart.item.update", "cart.view", "favorite.manage",
                        "inventory.update", "inventory.view", "order.admin.update",
                        "order.admin.view", "order.cancel", "order.create",
                        "order.view", "product.create", "product.delete",
                        "product.update", "product.view", "rbac.manage", "rbac.view",
                        "user.role.assign", "user.status.update", "user.view");
        assertThat(jdbc.sql("""
                select child_name from auth_item_children
                where parent_name = 'ADMIN'
                order by child_name
                """).query(String.class).list())
                .contains("inventory.update", "inventory.view", "product.create",
                        "product.delete", "product.update", "product.view")
                .doesNotContain("cart.clear", "cart.item.add", "cart.item.remove",
                        "cart.item.update", "cart.view", "order.cancel", "order.create",
                        "order.view");
        assertThat(jdbc.sql("""
                select child_name from auth_item_children
                where parent_name = 'CUSTOMER' and child_name like 'cart.%'
                order by child_name
                """).query(String.class).list())
                .containsExactly("cart.clear", "cart.item.add", "cart.item.remove",
                        "cart.item.update", "cart.view");
        assertThat(jdbc.sql("""
                select child_name from auth_item_children
                where parent_name = 'CUSTOMER' and child_name like 'order.%'
                order by child_name
                """).query(String.class).list())
                .containsExactly("order.cancel", "order.create", "order.view");
        assertThat(jdbc.sql("""
                select child_name from auth_item_children
                where parent_name = 'ADMIN' and child_name like 'order.%'
                order by child_name
                """).query(String.class).list())
                .containsExactly("order.admin.update", "order.admin.view");
        assertThat(jdbc.sql("""
                select count(*) from auth_item_children
                where parent_name = 'CUSTOMER' and child_name = 'favorite.manage'
                """).query(Long.class).single()).isEqualTo(1L);
        assertThat(jdbc.sql("""
                select count(*) from auth_item_children
                where parent_name = 'ADMIN' and child_name = 'favorite.manage'
                """).query(Long.class).single()).isZero();
        assertThat(jdbc.sql("""
                select count(*) from auth_item_children
                where parent_name = 'ADMIN' and child_name = 'product.delete'
                """).query(Long.class).single()).isEqualTo(1L);
        assertThat(jdbc.sql("""
                select count(*) from auth_item_children
                where parent_name = 'CUSTOMER' and child_name = 'product.delete'
                """).query(Long.class).single()).isZero();
    }

    @Test
    @Transactional
    void migratesLegacyRolesAndAssignmentsToRbacTables() throws Exception {
        jdbcTemplate.execute("create schema if not exists migration_target");
        jdbcTemplate.execute("""
                create table migration_target.identity_users (
                    id uuid primary key,
                    email varchar(254) not null,
                    normalized_email varchar(254) not null,
                    password_hash varchar(255) not null,
                    status varchar(32) not null,
                    created_at timestamptz not null,
                    updated_at timestamptz not null
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.roles (
                    id smallint primary key,
                    code varchar(32) not null unique
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.user_roles (
                    user_id uuid not null,
                    role_id smallint not null,
                    primary key (user_id, role_id)
                )
                """);
        jdbcTemplate.execute("""
                insert into migration_target.identity_users
                    (id, email, normalized_email, password_hash, status, created_at, updated_at)
                values ('00000000-0000-0000-0000-000000000001', 'legacy@example.com',
                        'legacy@example.com', 'hash', 'ACTIVE', now(), now())
                """);
        jdbcTemplate.execute("insert into migration_target.roles (id, code) values (1, 'CUSTOMER'), (2, 'SUPPORT')");
        jdbcTemplate.execute("""
                insert into migration_target.user_roles (user_id, role_id)
                values ('00000000-0000-0000-0000-000000000001', 1),
                       ('00000000-0000-0000-0000-000000000001', 2)
                """);

        jdbcTemplate.execute("set local search_path to migration_target");
        jdbcTemplate.execute(v5MigrationSql());

        assertThat(jdbc.sql("select name from migration_target.auth_items where type = 'ROLE' order by name")
                .query(String.class).list())
                .contains("CUSTOMER", "SUPPORT", "ADMIN", "SUPER_ADMIN");
        assertThat(jdbc.sql("""
                select item_name from migration_target.auth_assignments
                where user_id = '00000000-0000-0000-0000-000000000001'
                order by item_name
                """).query(String.class).list())
                .containsExactly("CUSTOMER", "SUPPORT");
    }

    @Test
    @Transactional
    void upgradesV5SchemaWithProtectedCartPermission() throws Exception {
        jdbcTemplate.execute("create schema migration_target");
        jdbcTemplate.execute("""
                create table migration_target.identity_users (
                    id uuid primary key,
                    email varchar(254) not null,
                    normalized_email varchar(254) not null,
                    password_hash varchar(255) not null,
                    status varchar(32) not null,
                    created_at timestamptz not null,
                    updated_at timestamptz not null
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.roles (
                    id smallint primary key,
                    code varchar(32) not null unique
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.user_roles (
                    user_id uuid not null,
                    role_id smallint not null,
                    primary key (user_id, role_id)
                )
                """);
        jdbcTemplate.execute("insert into migration_target.roles (id, code) values (1, 'CUSTOMER')");

        jdbcTemplate.execute("set local search_path to migration_target");
        jdbcTemplate.execute(v5MigrationSql());
        jdbcTemplate.execute(v6MigrationSql());

        assertThat(jdbc.sql("""
                select name || ':' || type || ':' || system_item
                from migration_target.auth_items
                where name = 'cart.item.add'
                """).query(String.class).single()).isEqualTo("cart.item.add:PERMISSION:true");
        assertThat(jdbc.sql("""
                select count(*) from migration_target.auth_item_children
                where parent_name = 'CUSTOMER' and child_name = 'cart.item.add'
                """).query(Long.class).single()).isEqualTo(1L);
    }

    @Test
    @Transactional
    void upgradesV6SchemaWithProtectedProductDeletePermission() throws Exception {
        jdbcTemplate.execute("create schema migration_target");
        jdbcTemplate.execute("""
                create table migration_target.identity_users (
                    id uuid primary key,
                    email varchar(254) not null,
                    normalized_email varchar(254) not null,
                    password_hash varchar(255) not null,
                    status varchar(32) not null,
                    created_at timestamptz not null,
                    updated_at timestamptz not null
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.roles (
                    id smallint primary key,
                    code varchar(32) not null unique
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.user_roles (
                    user_id uuid not null,
                    role_id smallint not null,
                    primary key (user_id, role_id)
                )
                """);
        jdbcTemplate.execute("insert into migration_target.roles (id, code) values (1, 'CUSTOMER')");

        jdbcTemplate.execute("set local search_path to migration_target");
        jdbcTemplate.execute(v5MigrationSql());
        jdbcTemplate.execute(v6MigrationSql());
        jdbcTemplate.execute(v7MigrationSql());

        assertThat(jdbc.sql("""
                select name || ':' || type || ':' || system_item
                from migration_target.auth_items
                where name = 'product.delete'
                """).query(String.class).single()).isEqualTo("product.delete:PERMISSION:true");
        assertThat(jdbc.sql("""
                select count(*) from migration_target.auth_item_children
                where parent_name = 'ADMIN' and child_name = 'product.delete'
                """).query(Long.class).single()).isEqualTo(1L);
    }

    @Test
    @Transactional
    void upgradesV8SchemaWithProtectedFavoriteManagePermission() throws Exception {
        jdbcTemplate.execute("create schema migration_target");
        jdbcTemplate.execute("""
                create table migration_target.identity_users (
                    id uuid primary key,
                    email varchar(254) not null,
                    normalized_email varchar(254) not null,
                    password_hash varchar(255) not null,
                    status varchar(32) not null,
                    created_at timestamptz not null,
                    updated_at timestamptz not null
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.roles (
                    id smallint primary key,
                    code varchar(32) not null unique
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.user_roles (
                    user_id uuid not null,
                    role_id smallint not null,
                    primary key (user_id, role_id)
                )
                """);
        jdbcTemplate.execute("insert into migration_target.roles (id, code) values (1, 'CUSTOMER')");

        jdbcTemplate.execute("set local search_path to migration_target");
        jdbcTemplate.execute(v5MigrationSql());
        jdbcTemplate.execute(v6MigrationSql());
        jdbcTemplate.execute(v7MigrationSql());
        jdbcTemplate.execute(v8MigrationSql());
        jdbcTemplate.execute(v9MigrationSql());

        assertThat(jdbc.sql("""
                select name || ':' || type || ':' || system_item
                from migration_target.auth_items
                where name = 'favorite.manage'
                """).query(String.class).single()).isEqualTo("favorite.manage:PERMISSION:true");
        assertThat(jdbc.sql("""
                select count(*) from migration_target.auth_item_children
                where parent_name = 'CUSTOMER' and child_name = 'favorite.manage'
                """).query(Long.class).single()).isEqualTo(1L);
        assertThat(jdbc.sql("""
                select count(*) from migration_target.auth_item_children
                where parent_name = 'ADMIN' and child_name = 'favorite.manage'
                """).query(Long.class).single()).isZero();
    }

    @Test
    @Transactional
    void upgradesV10SchemaWithProtectedCartPermissions() throws Exception {
        jdbcTemplate.execute("create schema migration_target");
        jdbcTemplate.execute("""
                create table migration_target.identity_users (
                    id uuid primary key,
                    email varchar(254) not null,
                    normalized_email varchar(254) not null,
                    password_hash varchar(255) not null,
                    status varchar(32) not null,
                    created_at timestamptz not null,
                    updated_at timestamptz not null
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.roles (
                    id smallint primary key,
                    code varchar(32) not null unique
                )
                """);
        jdbcTemplate.execute("""
                create table migration_target.user_roles (
                    user_id uuid not null,
                    role_id smallint not null,
                    primary key (user_id, role_id)
                )
                """);
        jdbcTemplate.execute("insert into migration_target.roles (id, code) values (1, 'CUSTOMER')");

        jdbcTemplate.execute("set local search_path to migration_target");
        jdbcTemplate.execute(v5MigrationSql());
        jdbcTemplate.execute(v6MigrationSql());
        jdbcTemplate.execute(v7MigrationSql());
        jdbcTemplate.execute(v8MigrationSql());
        jdbcTemplate.execute(v9MigrationSql());
        jdbcTemplate.execute(v10MigrationSql());
        jdbcTemplate.execute(v11MigrationSql());

        assertThat(jdbc.sql("""
                select name || ':' || type || ':' || system_item
                from migration_target.auth_items
                where name in ('cart.view', 'cart.item.add', 'cart.item.update', 'cart.item.remove', 'cart.clear')
                order by name
                """).query(String.class).list())
                .containsExactly(
                        "cart.clear:PERMISSION:true",
                        "cart.item.add:PERMISSION:true",
                        "cart.item.remove:PERMISSION:true",
                        "cart.item.update:PERMISSION:true",
                        "cart.view:PERMISSION:true");
        assertThat(jdbc.sql("""
                select child_name from migration_target.auth_item_children
                where parent_name = 'CUSTOMER' and child_name like 'cart.%'
                order by child_name
                """).query(String.class).list())
                .containsExactly("cart.clear", "cart.item.add", "cart.item.remove",
                        "cart.item.update", "cart.view");
        assertThat(jdbc.sql("""
                select count(*) from migration_target.auth_item_children
                where parent_name = 'ADMIN' and child_name like 'cart.%'
                """).query(Long.class).single()).isZero();
    }

    @Test
    void createsNullableProfileContactColumns() {
        assertThat(jdbc.sql("""
                select column_name || ':' || data_type
                        || ':' || coalesce(character_maximum_length::text, '')
                        || ':' || is_nullable
                from information_schema.columns
                where table_schema = 'public' and table_name = 'identity_profiles'
                """).query(String.class).list())
                .contains(
                        "full_name:character varying:100:NO",
                        "phone:character varying:20:YES",
                        "date_of_birth:date::YES",
                        "gender:character varying:16:YES",
                        "province_code:character varying:10:YES",
                        "ward_code:character varying:10:YES",
                        "address_line:character varying:255:YES")
                .doesNotContain(
                        "district:character varying:100:YES",
                        "ward:character varying:100:YES");
    }

    @Test
    @Transactional
    void migratesLegacyAddressColumnsToTwoLevelCodes() throws Exception {
        jdbcTemplate.execute("create schema if not exists migration_target");
        jdbcTemplate.execute("""
                create table migration_target.identity_profiles (
                    user_id uuid primary key,
                    full_name varchar(100) not null,
                    phone varchar(20),
                    date_of_birth date,
                    gender varchar(16),
                    province_code varchar(10),
                    district varchar(100),
                    ward varchar(100),
                    address_line varchar(255)
                )
                """);
        jdbcTemplate.execute("""
                insert into migration_target.identity_profiles
                    (user_id, full_name, province_code, district, ward, address_line)
                values ('00000000-0000-0000-0000-000000000001', 'Nguyen Van A',
                        '79', 'Quan 1', 'Phuong Ben Nghe', '12 Le Loi')
                """);
        jdbcTemplate.execute("set search_path to migration_target");
        jdbcTemplate.execute(v4MigrationSql());

        assertThat(jdbc.sql("""
                select column_name || ':' || data_type
                        || ':' || coalesce(character_maximum_length::text, '')
                        || ':' || is_nullable
                from information_schema.columns
                where table_schema = 'migration_target' and table_name = 'identity_profiles'
                """).query(String.class).list())
                .contains("ward_code:character varying:10:YES")
                .doesNotContain(
                        "district:character varying:100:YES",
                        "ward:character varying:100:YES");
        assertThat(jdbc.sql("""
                select province_code from migration_target.identity_profiles
                """).query((rs, rowNum) -> rs.getString(1)).list())
                .hasSize(1).containsOnlyNulls();
        assertThat(jdbc.sql("""
                select ward_code from migration_target.identity_profiles
                """).query((rs, rowNum) -> rs.getString(1)).list())
                .hasSize(1).containsOnlyNulls();
    }

    private String v4MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V4__migrate_profile_address_to_two_levels.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private String v5MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V5__add_rbac_and_shipping_addresses.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private String v6MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V6__add_cart_item_add_permission.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private String v7MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V7__add_product_delete_permission.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private String v8MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V8__add_rbac_foreign_key_indexes.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private String v9MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V9__add_favorite_manage_permission.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private String v10MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V10__add_commerce_admin_permissions.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    private String v11MigrationSql() throws Exception {
        return new ClassPathResource("db/migration/V11__add_cart_permissions.sql")
                .getContentAsString(java.nio.charset.StandardCharsets.UTF_8);
    }

    @Test
    void createsRefreshSessionsWithUniqueTokenHash() {
        assertThat(jdbc.sql("""
                select table_name from information_schema.tables where table_schema = 'public'
                """).query(String.class).list()).contains("refresh_token_sessions");
        assertThat(jdbc.sql("""
                select conname from pg_constraint
                where conrelid = 'refresh_token_sessions'::regclass and contype = 'u'
                """).query(String.class).list()).contains("uq_refresh_token_sessions_token_hash");
    }

    @Test
    void createsShippingAddressColumnsAndOneDefaultConstraint() {
        assertThat(jdbc.sql("""
                select column_name || ':' || data_type || ':'
                        || coalesce(character_maximum_length::text, '') || ':' || is_nullable
                from information_schema.columns
                where table_schema = 'public' and table_name = 'shipping_addresses'
                """).query(String.class).list())
                .contains(
                        "id:uuid::NO",
                        "user_id:uuid::NO",
                        "recipient_name:character varying:100:NO",
                        "phone:character varying:20:NO",
                        "province_code:character varying:20:NO",
                        "ward_code:character varying:20:NO",
                        "address_line:character varying:255:NO",
                        "label:character varying:50:YES",
                        "is_default:boolean::NO");
        assertThat(jdbc.sql("""
                select indexname from pg_indexes
                where schemaname = 'public' and tablename = 'shipping_addresses'
                """).query(String.class).list())
                .contains("uq_shipping_addresses_default_per_user");
    }
}

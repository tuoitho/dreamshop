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
                        "address.update", "address.view", "rbac.manage", "rbac.view",
                        "user.role.assign", "user.status.update", "user.view");
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
}

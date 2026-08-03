package com.ecommerce.identity.user.persistence;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.simple.JdbcClient;

import static org.assertj.core.api.Assertions.assertThat;

class IdentityDatabaseMigrationTest extends PostgresIntegrationTestSupport {
    @Autowired
    JdbcClient jdbc;

    @Test
    void createsAccountTablesAndSeedsCustomerRole() {
        List<String> tables = jdbc.sql("""
                select table_name from information_schema.tables where table_schema = 'public'
                """).query(String.class).list();

        assertThat(tables).contains("identity_users", "identity_profiles", "roles", "user_roles");
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
        assertThat(jdbc.sql("select code from roles").query(String.class).list())
                .containsExactly("CUSTOMER");
    }
}

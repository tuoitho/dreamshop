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
        assertThat(jdbc.sql("select code from roles").query(String.class).list())
                .containsExactly("CUSTOMER");
    }
}

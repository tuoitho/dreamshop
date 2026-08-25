package com.ecommerce.identity.auth.application;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.simple.JdbcClient;

@SpringBootTest(properties = {
        "identity.bootstrap.super-admin.email=bootstrap-root@example.test",
        "identity.bootstrap.super-admin.password=SecurePassword123!",
        "identity.bootstrap.super-admin.full-name=Bootstrap Root"
})
class BootstrapSuperAdminStartupTest extends PostgresIntegrationTestSupport {
    @Autowired
    JdbcClient jdbc;

    @Test
    void seedsSuperAdminDuringApplicationStartup() {
        UUID userId = jdbc.sql("select id from identity_users where normalized_email = :email")
                .param("email", "bootstrap-root@example.test")
                .query(UUID.class)
                .single();

        assertThat(jdbc.sql("""
                select count(*)
                from auth_assignments
                where user_id = :userId and item_name = 'SUPER_ADMIN'
                """)
                .param("userId", userId)
                .query(Long.class)
                .single()).isEqualTo(1L);
    }
}

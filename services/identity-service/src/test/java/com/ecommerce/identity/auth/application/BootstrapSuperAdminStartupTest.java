package com.ecommerce.identity.auth.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.ecommerce.identity.common.config.IdentityBootstrapSuperAdminProperties;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import javax.sql.DataSource;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.simple.JdbcClient;
import org.springframework.transaction.support.TransactionOperations;

@SpringBootTest(properties = {
        "identity.bootstrap.super-admin.email=bootstrap-root@example.test",
        "identity.bootstrap.super-admin.password=SecurePassword123!",
        "identity.bootstrap.super-admin.full-name=Bootstrap Root"
})
class BootstrapSuperAdminStartupTest extends PostgresIntegrationTestSupport {
    @Autowired
    JdbcClient jdbc;

    @Autowired
    DataSource dataSource;

    @Autowired
    BootstrapSuperAdminService bootstrapSuperAdminService;

    @Autowired
    IdentityUserRepository users;

    @Autowired
    RegisterUserService registration;

    @Autowired
    Clock clock;

    @Autowired
    TransactionOperations transactions;

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

    @Test
    void concurrentBootstrapCollisionReturnsCleanlyWithoutUnexpectedRollbackException() throws Exception {
        String email = "concurrent-bootstrap-root@example.test";
        IdentityBootstrapSuperAdminProperties concurrentProps = new IdentityBootstrapSuperAdminProperties(
                email, "SecurePassword123!", "Concurrent Bootstrap Root");

        try (Connection competitor = dataSource.getConnection()) {
            competitor.setAutoCommit(false);
            UUID winnerId = UUID.randomUUID();
            try (PreparedStatement insertUser = competitor.prepareStatement("""
                    insert into identity_users
                        (id, email, normalized_email, password_hash, status, created_at, updated_at)
                    values (?, ?, ?, 'hash', 'ACTIVE', now(), now())
                    """)) {
                insertUser.setObject(1, winnerId);
                insertUser.setString(2, email);
                insertUser.setString(3, email);
                insertUser.executeUpdate();
            }

            CompletableFuture<Void> contender = CompletableFuture.runAsync(
                    () -> bootstrapSuperAdminService.ensureSuperAdmin(concurrentProps));

            waitForBlockedInsert();
            competitor.commit();

            assertThatCode(() -> contender.get(10, TimeUnit.SECONDS))
                    .doesNotThrowAnyException();
        }
    }

    @Test
    void rollsBackCreatedUserWhenSuperAdminAssignmentFails() {
        String email = "rollback-bootstrap-root@example.test";
        RbacRepository failingRbac = mock(RbacRepository.class);
        when(failingRbac.assignRole(any(UUID.class), any(String.class), any(Instant.class)))
                .thenReturn(false);
        BootstrapSuperAdminService serviceWithFailingRole = new BootstrapSuperAdminService(
                users, registration, failingRbac, clock, transactions);

        assertThatThrownBy(() -> serviceWithFailingRole.ensureSuperAdmin(
                new IdentityBootstrapSuperAdminProperties(email, "SecurePassword123!", "Rollback Root")))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("SUPER_ADMIN");

        assertThat(jdbc.sql("select count(*) from identity_users where normalized_email = :email")
                .param("email", email)
                .query(Long.class)
                .single()).isZero();
    }

    private void waitForBlockedInsert() throws InterruptedException {
        Instant deadline = Instant.now().plus(Duration.ofSeconds(10));
        while (Instant.now().isBefore(deadline)) {
            Long waiting = jdbc.sql("""
                    select count(*)
                    from pg_stat_activity
                    where wait_event_type = 'Lock'
                      and state = 'active'
                    """)
                    .query(Long.class)
                    .single();
            if (waiting != null && waiting > 0) {
                return;
            }
            Thread.sleep(20);
        }
        throw new AssertionError("Contender insert did not reach unique-constraint lock before timeout");
    }
}

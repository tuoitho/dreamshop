package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import com.ecommerce.identity.user.persistence.RoleRepository;
import java.util.List;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.password.PasswordEncoder;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class RegisterUserServiceTest extends PostgresIntegrationTestSupport {
    @Autowired
    RegisterUserService service;

    @Autowired
    IdentityUserRepository users;

    @Autowired
    RoleRepository roles;

    @Autowired
    PasswordEncoder passwordEncoder;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        jdbcTemplate.update("delete from user_roles");
        jdbcTemplate.update("delete from identity_profiles");
        jdbcTemplate.update("delete from identity_users");
        jdbcTemplate.update("delete from roles");
        jdbcTemplate.update("insert into roles (id, code) values (1, 'CUSTOMER')");
    }

    @Test
    void registersNormalizedCustomerAccountWithArgon2idPassword() {
        RegisteredUser result = service.register(new RegisterCommand(
                " User@Example.com ", "correct horse battery staple", " Nguyen Van A "));

        var stored = users.findWithRolesByNormalizedEmail("user@example.com").orElseThrow();

        assertThat(result.email()).isEqualTo("user@example.com");
        assertThat(result.roles()).containsExactly("CUSTOMER");
        assertThat(stored.passwordHash()).startsWith("{argon2id}");
        assertThat(stored.passwordHash()).doesNotContain("correct horse battery staple");
        assertThat(passwordEncoder.matches("correct horse battery staple", stored.passwordHash())).isTrue();
    }

    @Test
    void rejectsEmailAlreadyRegisteredAfterNormalization() {
        service.register(new RegisterCommand("user@example.com", "password", "Nguyen Van A"));

        assertThatThrownBy(() -> service.register(
                new RegisterCommand("USER@example.com", "password", "Nguyen Van B")))
                .isInstanceOf(EmailAlreadyRegisteredException.class);
    }

    @Test
    void rejectsExistingEmailBeforeLookingUpCustomerRole() {
        service.register(new RegisterCommand("user@example.com", "password", "Nguyen Van A"));
        jdbcTemplate.update("delete from user_roles");
        jdbcTemplate.update("delete from roles");

        assertThatThrownBy(() -> service.register(
                new RegisterCommand("USER@example.com", "password", "Nguyen Van B")))
                .isInstanceOf(EmailAlreadyRegisteredException.class);
    }

    @Test
    void doesNotCommitUserOrProfileWhenCustomerRoleIsMissing() {
        roles.delete(roles.findByCode("CUSTOMER").orElseThrow());
        roles.flush();

        assertThatThrownBy(() -> service.register(
                new RegisterCommand("missing-role@example.com", "password", "Nguyen Van A")))
                .isInstanceOf(IllegalStateException.class);

        assertThat(users.countByNormalizedEmail("missing-role@example.com")).isZero();
        assertThat(jdbcTemplate.queryForObject("select count(*) from identity_profiles", Long.class)).isZero();
    }

    @Test
    void saltsIdenticalPasswordsDifferentlyWhileBothHashesVerify() {
        service.register(new RegisterCommand("first@example.com", "same password", "First User"));
        service.register(new RegisterCommand("second@example.com", "same password", "Second User"));

        String firstHash = users.findWithRolesByNormalizedEmail("first@example.com").orElseThrow().passwordHash();
        String secondHash = users.findWithRolesByNormalizedEmail("second@example.com").orElseThrow().passwordHash();

        assertThat(firstHash).isNotEqualTo(secondHash);
        assertThat(passwordEncoder.matches("same password", firstHash)).isTrue();
        assertThat(passwordEncoder.matches("same password", secondHash)).isTrue();
    }

    @Test
    void concurrentDuplicateRegistrationCreatesExactlyOneAccount() throws Exception {
        CyclicBarrier barrier = new CyclicBarrier(2);
        try (ExecutorService executor = Executors.newFixedThreadPool(2)) {
            List<Future<Throwable>> attempts = List.of(
                    executor.submit(() -> registerAfterBarrier(barrier)),
                    executor.submit(() -> registerAfterBarrier(barrier)));

            List<Throwable> results = attempts.stream().map(this::get).toList();

            assertThat(results).filteredOn(java.util.Objects::isNull).hasSize(1);
            assertThat(results).filteredOn(EmailAlreadyRegisteredException.class::isInstance).hasSize(1);
        }

        assertThat(users.countByNormalizedEmail("racer@example.com")).isEqualTo(1);
    }

    private Throwable registerAfterBarrier(CyclicBarrier barrier) {
        try {
            barrier.await();
            service.register(new RegisterCommand(" Racer@Example.com ", "password", "Race User"));
            return null;
        } catch (Throwable throwable) {
            return throwable;
        }
    }

    private Throwable get(Future<Throwable> future) {
        try {
            return future.get();
        } catch (Exception exception) {
            return exception;
        }
    }
}

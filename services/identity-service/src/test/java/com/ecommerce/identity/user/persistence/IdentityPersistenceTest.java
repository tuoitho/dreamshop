package com.ecommerce.identity.user.persistence;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.time.Instant;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.transaction.annotation.Transactional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

@Transactional
class IdentityPersistenceTest extends PostgresIntegrationTestSupport {
    @Autowired
    IdentityUserRepository users;

    @Autowired
    RoleRepository roles;

    @Test
    void savesProfileAndCustomerRoleWithAccount() {
        RoleEntity customer = roles.findByCode("CUSTOMER").orElseThrow();
        var user = IdentityUserEntity.create(UUID.randomUUID(), "User@example.com",
                "user@example.com", "{argon2id}hash", AccountStatus.ACTIVE,
                Instant.parse("2026-08-03T00:00:00Z"), "Nguyen Van A", Set.of(customer));
        users.saveAndFlush(user);

        var reloaded = users.findWithRolesByNormalizedEmail("user@example.com").orElseThrow();

        assertThat(reloaded.fullName()).isEqualTo("Nguyen Van A");
        assertThat(reloaded.roleCodes()).containsExactly("CUSTOMER");
    }

    @Test
    void databaseRejectsDuplicateNormalizedEmail() {
        RoleEntity customer = roles.findByCode("CUSTOMER").orElseThrow();
        users.saveAndFlush(user("A@example.com", "same@example.com", customer));

        assertThatThrownBy(() -> users.saveAndFlush(user("B@example.com", "same@example.com", customer)))
                .isInstanceOf(DataIntegrityViolationException.class);
    }

    @Test
    void normalizesEmailForAggregateAndExistenceLookup() {
        RoleEntity customer = roles.findByCode("CUSTOMER").orElseThrow();
        var user = user("User@example.com", " User@Example.com ", customer);

        assertThat(user.normalizedEmail()).isEqualTo("user@example.com");
        users.saveAndFlush(user);

        assertThat(users.existsByNormalizedEmail("user@example.com")).isTrue();
    }

    private IdentityUserEntity user(String email, String normalizedEmail, RoleEntity customer) {
        return IdentityUserEntity.create(UUID.randomUUID(), email, normalizedEmail, "{argon2id}hash",
                AccountStatus.ACTIVE, Instant.parse("2026-08-03T00:00:00Z"), "Nguyen Van A",
                Set.of(customer));
    }
}

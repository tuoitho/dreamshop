package com.ecommerce.identity.user.persistence;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.time.Instant;
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
    RbacRepository rbac;

    @Test
    void savesProfileAndCustomerRoleWithAccount() {
        var user = IdentityUserEntity.create(UUID.randomUUID(), "User@example.com",
                "user@example.com", "{argon2id}hash", AccountStatus.ACTIVE,
                Instant.parse("2026-08-03T00:00:00Z"), "Nguyen Van A");
        users.saveAndFlush(user);
        rbac.assignRole(user.id(), "CUSTOMER", Instant.parse("2026-08-03T00:00:00Z"));

        var reloaded = users.findWithProfileByNormalizedEmail("user@example.com").orElseThrow();

        assertThat(reloaded.fullName()).isEqualTo("Nguyen Van A");
        assertThat(rbac.assignedRoles(reloaded.id())).containsExactly("CUSTOMER");
    }

    @Test
    void databaseRejectsDuplicateNormalizedEmail() {
        users.saveAndFlush(user("A@example.com", "same@example.com"));

        assertThatThrownBy(() -> users.saveAndFlush(user("B@example.com", "same@example.com")))
                .isInstanceOf(DataIntegrityViolationException.class);
    }

    @Test
    void normalizesEmailForAggregateAndExistenceLookup() {
        var user = user("User@example.com", " User@Example.com ");

        assertThat(user.normalizedEmail()).isEqualTo("user@example.com");
        users.saveAndFlush(user);

        assertThat(users.existsByNormalizedEmail("user@example.com")).isTrue();
    }

    private IdentityUserEntity user(String email, String normalizedEmail) {
        return IdentityUserEntity.create(UUID.randomUUID(), email, normalizedEmail, "{argon2id}hash",
                AccountStatus.ACTIVE, Instant.parse("2026-08-03T00:00:00Z"), "Nguyen Van A");
    }
}

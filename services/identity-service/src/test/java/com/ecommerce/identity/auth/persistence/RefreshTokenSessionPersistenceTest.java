package com.ecommerce.identity.auth.persistence;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import jakarta.persistence.EntityManager;
import jakarta.persistence.PersistenceContext;
import java.time.Instant;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.transaction.annotation.Transactional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

@Transactional
class RefreshTokenSessionPersistenceTest extends PostgresIntegrationTestSupport {
    private static final Instant ISSUED = Instant.parse("2026-08-03T00:00:00Z");
    private static final Instant EXPIRES = Instant.parse("2026-09-02T00:00:00Z");

    @Autowired
    RefreshTokenSessionRepository sessions;

    @Autowired
    IdentityUserRepository users;

    @PersistenceContext
    EntityManager entityManager;

    @Test
    void savesAndReloadsSessionByTokenHash() {
        UUID userId = registerUser();
        UUID familyId = UUID.randomUUID();
        RefreshTokenSessionEntity session = RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, familyId, "hash-1", ISSUED, EXPIRES);

        sessions.saveAndFlush(session);

        RefreshTokenSessionEntity reloaded = sessions.findForUpdateByTokenHash("hash-1").orElseThrow();
        assertThat(reloaded.familyId()).isEqualTo(familyId);
        assertThat(reloaded.userId()).isEqualTo(userId);
        assertThat(reloaded.revokedAt()).isNull();
        assertThat(reloaded.revocationReason()).isNull();
    }

    @Test
    void activeAtIsTrueBeforeExpiryAndFalseAfter() {
        UUID userId = registerUser();
        RefreshTokenSessionEntity session = sessions.saveAndFlush(RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, UUID.randomUUID(), "hash-2", ISSUED, EXPIRES));

        assertThat(session.activeAt(Instant.parse("2026-08-10T00:00:00Z"))).isTrue();
        assertThat(session.activeAt(EXPIRES)).isFalse();
        assertThat(session.activeAt(Instant.parse("2026-09-03T00:00:00Z"))).isFalse();
    }

    @Test
    void rotateMarksSessionRotatedWithReplacement() {
        UUID userId = registerUser();
        RefreshTokenSessionEntity session = RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, UUID.randomUUID(), "hash-3", ISSUED, EXPIRES);
        sessions.saveAndFlush(session);
        RefreshTokenSessionEntity replacement = sessions.saveAndFlush(RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, UUID.randomUUID(), "hash-3b", ISSUED, EXPIRES));
        UUID replacementId = replacement.id();

        session.rotate(Instant.parse("2026-08-10T00:00:00Z"), replacementId);
        sessions.saveAndFlush(session);

        RefreshTokenSessionEntity reloaded = sessions.findForUpdateByTokenHash("hash-3").orElseThrow();
        assertThat(reloaded.revocationReason()).isEqualTo(RefreshTokenRevocationReason.ROTATED);
        assertThat(reloaded.replacedBySessionId()).isEqualTo(replacementId);
        assertThat(reloaded.activeAt(Instant.parse("2026-08-11T00:00:00Z"))).isFalse();
    }

    @Test
    void revokeActiveFamilyRevokesOnlyActiveRowsInFamily() {
        UUID userId = registerUser();
        UUID familyId = UUID.randomUUID();
        sessions.saveAndFlush(RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, familyId, "hash-4", ISSUED, EXPIRES));
        sessions.saveAndFlush(RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, familyId, "hash-5", ISSUED, EXPIRES));
        UUID otherFamily = UUID.randomUUID();
        sessions.saveAndFlush(RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, otherFamily, "hash-6", ISSUED, EXPIRES));

        int updated = sessions.revokeActiveFamily(familyId, Instant.parse("2026-08-10T00:00:00Z"),
                RefreshTokenRevocationReason.REUSE_DETECTED);
        entityManager.clear();

        assertThat(updated).isEqualTo(2);
        assertThat(sessions.findForUpdateByTokenHash("hash-4").orElseThrow().revocationReason())
                .isEqualTo(RefreshTokenRevocationReason.REUSE_DETECTED);
        assertThat(sessions.findForUpdateByTokenHash("hash-5").orElseThrow().revocationReason())
                .isEqualTo(RefreshTokenRevocationReason.REUSE_DETECTED);
        assertThat(sessions.findForUpdateByTokenHash("hash-6").orElseThrow().revocationReason())
                .isNull();
    }

    @Test
    void rejectsDuplicateTokenHash() {
        UUID userId = registerUser();
        RefreshTokenSessionEntity session = RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, UUID.randomUUID(), "hash-7", ISSUED, EXPIRES);
        sessions.saveAndFlush(session);

        assertThatThrownBy(() -> sessions.saveAndFlush(RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, UUID.randomUUID(), "hash-7", ISSUED, EXPIRES)))
                .isInstanceOf(DataIntegrityViolationException.class);
    }

    private UUID registerUser() {
        IdentityUserEntity user = IdentityUserEntity.create(UUID.randomUUID(),
                "session@example.com", "session@example.com", "{argon2id}hash",
                AccountStatus.ACTIVE, ISSUED, "Session User");
        users.saveAndFlush(user);
        return user.id();
    }
}

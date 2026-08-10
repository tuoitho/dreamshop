package com.ecommerce.identity.auth.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import java.time.Instant;
import java.util.Objects;
import java.util.UUID;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.experimental.Accessors;

@Entity
@Table(name = "refresh_token_sessions")
@Getter
@Accessors(fluent = true)
@NoArgsConstructor(access = AccessLevel.PROTECTED)
public class RefreshTokenSessionEntity {
    @Id
    @Column(nullable = false, updatable = false)
    private UUID id;

    @Column(name = "user_id", nullable = false)
    private UUID userId;

    @Column(name = "family_id", nullable = false)
    private UUID familyId;

    @Column(name = "token_hash", nullable = false, length = 64)
    private String tokenHash;

    @Column(name = "issued_at", nullable = false)
    private Instant issuedAt;

    @Column(name = "expires_at", nullable = false)
    private Instant expiresAt;

    @Column(name = "revoked_at")
    private Instant revokedAt;

    @Column(name = "replaced_by_session_id")
    private UUID replacedBySessionId;

    @Enumerated(EnumType.STRING)
    @Column(name = "revocation_reason", length = 32)
    private RefreshTokenRevocationReason revocationReason;

    public static RefreshTokenSessionEntity create(UUID id, UUID userId, UUID familyId,
            String tokenHash, Instant issuedAt, Instant expiresAt) {
        RefreshTokenSessionEntity session = new RefreshTokenSessionEntity();
        session.id = Objects.requireNonNull(id, "id");
        session.userId = Objects.requireNonNull(userId, "userId");
        session.familyId = Objects.requireNonNull(familyId, "familyId");
        session.tokenHash = Objects.requireNonNull(tokenHash, "tokenHash");
        session.issuedAt = Objects.requireNonNull(issuedAt, "issuedAt");
        session.expiresAt = Objects.requireNonNull(expiresAt, "expiresAt");
        return session;
    }

    public boolean activeAt(Instant now) {
        return revokedAt == null && now.isBefore(expiresAt);
    }

    public void rotate(Instant now, UUID replacementId) {
        revokedAt = Objects.requireNonNull(now, "now");
        replacedBySessionId = Objects.requireNonNull(replacementId, "replacementId");
        revocationReason = RefreshTokenRevocationReason.ROTATED;
    }

    public void revoke(Instant now, RefreshTokenRevocationReason reason) {
        revokedAt = Objects.requireNonNull(now, "now");
        revocationReason = Objects.requireNonNull(reason, "reason");
    }
}

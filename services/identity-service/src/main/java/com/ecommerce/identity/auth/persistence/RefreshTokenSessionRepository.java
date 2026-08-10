package com.ecommerce.identity.auth.persistence;

import jakarta.persistence.LockModeType;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

public interface RefreshTokenSessionRepository extends JpaRepository<RefreshTokenSessionEntity, UUID> {
    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select s from RefreshTokenSessionEntity s where s.tokenHash = :tokenHash")
    Optional<RefreshTokenSessionEntity> findForUpdateByTokenHash(@Param("tokenHash") String tokenHash);

    @Modifying
    @Query("""
            update RefreshTokenSessionEntity s
            set s.revokedAt = :now, s.revocationReason = :reason
            where s.familyId = :familyId and s.revokedAt is null
            """)
    int revokeActiveFamily(@Param("familyId") UUID familyId, @Param("now") Instant now,
            @Param("reason") RefreshTokenRevocationReason reason);
}

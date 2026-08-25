package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.auth.domain.InvalidRefreshTokenException;
import com.ecommerce.identity.auth.persistence.RefreshTokenRevocationReason;
import com.ecommerce.identity.auth.persistence.RefreshTokenSessionEntity;
import com.ecommerce.identity.auth.persistence.RefreshTokenSessionRepository;
import com.ecommerce.identity.common.config.IdentitySessionProperties;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.security.JwtTokenService;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import java.util.Set;
import java.util.UUID;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

@org.springframework.stereotype.Service
public class RefreshTokenService {
    private final RefreshTokenSessionRepository sessions;
    private final IdentityUserRepository users;
    private final RefreshTokenGenerator tokenGenerator;
    private final JwtTokenService jwtTokenService;
    private final RbacRepository rbac;
    private final IdentitySessionProperties properties;
    private final Clock clock;
    private final TransactionTemplate revocationTransaction;

    public RefreshTokenService(RefreshTokenSessionRepository sessions, IdentityUserRepository users,
            RefreshTokenGenerator tokenGenerator, JwtTokenService jwtTokenService,
            RbacRepository rbac, IdentitySessionProperties properties, Clock clock,
            PlatformTransactionManager transactionManager) {
        this.sessions = sessions;
        this.users = users;
        this.tokenGenerator = tokenGenerator;
        this.jwtTokenService = jwtTokenService;
        this.rbac = rbac;
        this.properties = properties;
        this.clock = clock;
        this.revocationTransaction = new TransactionTemplate(transactionManager);
        this.revocationTransaction.setPropagationBehavior(
                org.springframework.transaction.TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    }

    @Transactional
    public String issue(UUID userId, Instant now) {
        RefreshTokenGenerator.RawToken token = tokenGenerator.generate();
        sessions.saveAndFlush(RefreshTokenSessionEntity.create(
                UUID.randomUUID(), userId, UUID.randomUUID(), token.hash(), now,
                now.plus(properties.refreshTtl())));
        return token.raw();
    }

    @Transactional
    public AuthenticatedSession rotate(String rawToken) {
        String tokenHash = RefreshTokenGenerator.hash(rawToken);
        RefreshTokenSessionEntity current = sessions.findForUpdateByTokenHash(tokenHash)
                .orElseThrow(InvalidRefreshTokenException::new);
        Instant now = clock.instant();

        if (current.revocationReason() == RefreshTokenRevocationReason.ROTATED) {
            revocationTransaction.executeWithoutResult(status -> sessions.revokeActiveFamily(
                    current.familyId(), now, RefreshTokenRevocationReason.REUSE_DETECTED));
            throw new InvalidRefreshTokenException();
        }
        if (!current.activeAt(now)) {
            throw new InvalidRefreshTokenException();
        }

        RefreshTokenGenerator.RawToken successor = tokenGenerator.generate();
        RefreshTokenSessionEntity replacement = RefreshTokenSessionEntity.create(
                UUID.randomUUID(), current.userId(), current.familyId(), successor.hash(), now,
                now.plus(properties.refreshTtl()));
        sessions.saveAndFlush(replacement);
        current.rotate(now, replacement.id());
        sessions.saveAndFlush(current);

        IdentityUserEntity user = users.findWithProfileById(current.userId())
                .filter(candidate -> candidate.status() == AccountStatus.ACTIVE)
                .orElseThrow(InvalidRefreshTokenException::new);
        Set<String> roles = rbac.assignedRoles(user.id());
        return new AuthenticatedSession(jwtTokenService.issue(current.userId(), roles), successor.raw());
    }

    @Transactional
    public void logout(String rawToken) {
        sessions.findForUpdateByTokenHash(RefreshTokenGenerator.hash(rawToken)).ifPresent(session -> {
            if (session.activeAt(clock.instant())) {
                session.revoke(clock.instant(), RefreshTokenRevocationReason.LOGOUT);
                sessions.saveAndFlush(session);
            }
        });
    }
}

package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.InvalidRefreshTokenException;
import com.ecommerce.identity.auth.persistence.RefreshTokenRevocationReason;
import com.ecommerce.identity.auth.persistence.RefreshTokenSessionEntity;
import com.ecommerce.identity.auth.persistence.RefreshTokenSessionRepository;
import com.ecommerce.identity.rbac.support.RbacTestData;
import com.ecommerce.identity.security.jwt.JwtTokenService;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.UnexpectedRollbackException;
import org.springframework.transaction.support.TransactionTemplate;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;

class RefreshTokenServiceTest extends PostgresIntegrationTestSupport {
    @Autowired
    RefreshTokenService service;

    @Autowired
    RegisterUserService registerUserService;

    @Autowired
    LoginService loginService;

    @Autowired
    RefreshTokenSessionRepository sessions;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @Autowired
    PlatformTransactionManager transactionManager;

    @MockitoBean
    JwtTokenService jwtTokenService;

    TransactionTemplate transactionTemplate;

    @BeforeEach
    void resetDatabase() {
        transactionTemplate = new TransactionTemplate(transactionManager);
        RbacTestData.resetDatabase(jdbcTemplate);
        reset(jwtTokenService);
    }

    @Test
    void loginIssuesAccessTokenAndPersistsRefreshSession() {
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("jwt", 900));

        AuthenticatedSession session = login();

        assertThat(session.accessToken()).isEqualTo(new AccessTokenResult("jwt", 900));
        assertThat(session.rawRefreshToken()).isNotBlank();
        RefreshTokenSessionEntity persisted = sessionByHash(session.rawRefreshToken());
        assertThat(persisted).isNotNull();
        assertThat(persisted.revokedAt()).isNull();
        assertThat(persisted.revocationReason()).isNull();
    }

    @Test
    void rotateRetainsFamilyAndRevokesOriginal() {
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("jwt", 900));
        AuthenticatedSession original = login();
        RefreshTokenSessionEntity originalSession = sessionByHash(original.rawRefreshToken());
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("new-jwt", 900));

        AuthenticatedSession replacement = service.rotate(original.rawRefreshToken());

        assertThat(replacement.accessToken()).isEqualTo(new AccessTokenResult("new-jwt", 900));
        assertThat(sessionByHash(original.rawRefreshToken()).revocationReason())
                .isEqualTo(RefreshTokenRevocationReason.ROTATED);
        assertThat(sessionByHash(replacement.rawRefreshToken()).familyId())
                .isEqualTo(originalSession.familyId());
    }

    @Test
    void reusingRotatedTokenRevokesEntireFamily() {
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("jwt", 900));
        AuthenticatedSession original = login();
        RefreshTokenSessionEntity originalSession = sessionByHash(original.rawRefreshToken());
        service.rotate(original.rawRefreshToken());

        assertThatThrownBy(() -> service.rotate(original.rawRefreshToken()))
                .isInstanceOf(InvalidRefreshTokenException.class);

        assertThat(activeSessions(originalSession.familyId())).isEmpty();
    }

    @Test
    void rejectsExpiredSession() {
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("jwt", 900));
        AuthenticatedSession original = login();
        jdbcTemplate.update("""
                update refresh_token_sessions
                set expires_at = now() - interval '1 hour'
                where token_hash = ?
                """, RefreshTokenGenerator.hash(original.rawRefreshToken()));

        assertThatThrownBy(() -> service.rotate(original.rawRefreshToken()))
                .isInstanceOf(InvalidRefreshTokenException.class);
    }

    @Test
    void rejectsUnknownToken() {
        assertThatThrownBy(() -> service.rotate("definitely-not-a-valid-refresh-token"))
                .isInstanceOf(InvalidRefreshTokenException.class);
    }

    @Test
    void logoutRevokesOnlyActiveSessionAndToleratesUnknownTokens() {
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("jwt", 900));
        AuthenticatedSession original = login();
        RefreshTokenSessionEntity originalSession = sessionByHash(original.rawRefreshToken());

        service.logout(original.rawRefreshToken());

        RefreshTokenSessionEntity reloaded = sessionByHash(original.rawRefreshToken());
        assertThat(reloaded.revocationReason()).isEqualTo(RefreshTokenRevocationReason.LOGOUT);
        assertThat(reloaded.revokedAt()).isNotNull();
        assertThatThrownBy(() -> service.rotate(original.rawRefreshToken()))
                .isInstanceOf(InvalidRefreshTokenException.class);
        service.logout("unknown-token");
    }

    @Test
    void concurrentRotationLeavesOneWinnerAndRevokesFamily() throws Exception {
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("new-jwt", 900));
        AuthenticatedSession original = login();
        String rawToken = original.rawRefreshToken();
        UUID familyId = sessionByHash(rawToken).familyId();
        AtomicReference<InvalidRefreshTokenException> loserFailure = new AtomicReference<>();
        AtomicReference<AuthenticatedSession> winner = new AtomicReference<>();
        CountDownLatch bStarted = new CountDownLatch(1);
        CountDownLatch aCommitted = new CountDownLatch(1);

        ExecutorService executor = Executors.newFixedThreadPool(2);
        try {
            Future<?> b = executor.submit(() -> {
                try {
                    transactionTemplate.executeWithoutResult(status -> {
                        bStarted.countDown();
                        try {
                            aCommitted.await(30, TimeUnit.SECONDS);
                        } catch (InterruptedException e) {
                            Thread.currentThread().interrupt();
                        }
                        try {
                            service.rotate(rawToken);
                        } catch (InvalidRefreshTokenException expected) {
                            loserFailure.set(expected);
                        }
                    });
                } catch (UnexpectedRollbackException expected) {
                    // the reuse detection transaction was rolled back; that is expected
                }
            });

            Future<AuthenticatedSession> a = executor.submit(() -> {
                try {
                    if (!bStarted.await(30, TimeUnit.SECONDS)) {
                        throw new AssertionError("thread B did not start");
                    }
                    AuthenticatedSession replacement = transactionTemplate.execute(status -> service.rotate(rawToken));
                    aCommitted.countDown();
                    return replacement;
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    throw new AssertionError(e);
                }
            });

            winner.set(a.get(30, TimeUnit.SECONDS));
            b.get(30, TimeUnit.SECONDS);
        } finally {
            executor.shutdownNow();
        }

        assertThat(loserFailure.get()).isInstanceOf(InvalidRefreshTokenException.class);
        RefreshTokenSessionEntity replacement = sessionByHash(winner.get().rawRefreshToken());
        assertThat(replacement.revocationReason()).isEqualTo(RefreshTokenRevocationReason.REUSE_DETECTED);
        assertThat(activeSessions(familyId)).isEmpty();
    }

    private AuthenticatedSession login() {
        registerUserService.register(new RegisterCommand(
                "user@example.com", "correct horse battery staple", "Refresh User"));
        return loginService.login(new LoginCommand("user@example.com", "correct horse battery staple"));
    }

    private RefreshTokenSessionEntity sessionByHash(String rawRefreshToken) {
        String hash = RefreshTokenGenerator.hash(rawRefreshToken);
        return transactionTemplate.execute(status ->
                sessions.findForUpdateByTokenHash(hash).orElse(null));
    }

    private List<UUID> activeSessions(UUID familyId) {
        return transactionTemplate.execute(status -> sessions.findAll().stream()
                .filter(session -> session.familyId().equals(familyId)
                        && session.revokedAt() == null)
                .map(RefreshTokenSessionEntity::id)
                .toList());
    }
}

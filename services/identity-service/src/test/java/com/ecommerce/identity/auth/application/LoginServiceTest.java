package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.auth.domain.InvalidCredentialsException;
import com.ecommerce.identity.rbac.support.RbacTestData;
import com.ecommerce.identity.security.JwtTokenService;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Instant;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.context.bean.override.mockito.MockitoSpyBean;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.clearInvocations;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class LoginServiceTest extends PostgresIntegrationTestSupport {
    @Autowired
    LoginService service;

    @Autowired
    RegisterUserService registerUserService;

    @Autowired
    IdentityUserRepository users;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @MockitoSpyBean
    PasswordEncoder passwordEncoder;

    @MockitoBean
    JwtTokenService jwtTokenService;

    @BeforeEach
    void resetDatabase() {
        RbacTestData.resetDatabase(jdbcTemplate);
        clearInvocations(passwordEncoder, jwtTokenService);
    }

    @Test
    void issuesTokenForCorrectActiveCredentials() {
        registerUserService.register(new RegisterCommand(" User@Example.com ", "correct password", "User"));
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("token", 900));
        clearInvocations(passwordEncoder, jwtTokenService);

        AuthenticatedSession session = service.login(new LoginCommand("USER@example.com", "correct password"));

        assertThat(session.accessToken()).isEqualTo(new AccessTokenResult("token", 900));
        assertThat(session.rawRefreshToken()).isNotBlank();
        verify(passwordEncoder).matches("correct password", users.findWithProfileByNormalizedEmail("user@example.com")
                .orElseThrow().passwordHash());
        verify(jwtTokenService).issue(any(), any());
    }

    @Test
    void rejectsWrongPasswordWithoutIssuingToken() {
        registerUserService.register(new RegisterCommand("user@example.com", "correct password", "User"));
        clearInvocations(passwordEncoder, jwtTokenService);

        assertThatThrownBy(() -> service.login(new LoginCommand("user@example.com", "wrong password")))
                .isInstanceOf(InvalidCredentialsException.class);

        verify(passwordEncoder).matches("wrong password", users.findWithProfileByNormalizedEmail("user@example.com")
                .orElseThrow().passwordHash());
        verify(jwtTokenService, never()).issue(any(), any());
    }

    @Test
    void rejectsUnknownEmailAfterOnePasswordComparison() {
        clearInvocations(passwordEncoder, jwtTokenService);

        assertThatThrownBy(() -> service.login(new LoginCommand("missing@example.com", "password")))
                .isInstanceOf(InvalidCredentialsException.class);

        verify(passwordEncoder).matches(any(), any());
        verify(jwtTokenService, never()).issue(any(), any());
    }

    @Test
    void rejectsDisabledAccountWithoutIssuingToken() {
        registerUserService.register(new RegisterCommand("user@example.com", "correct password", "User"));
        var user = users.findWithProfileByNormalizedEmail("user@example.com").orElseThrow();
        user.disable(Instant.now());
        users.saveAndFlush(user);
        clearInvocations(passwordEncoder, jwtTokenService);

        assertThatThrownBy(() -> service.login(new LoginCommand("user@example.com", "correct password")))
                .isInstanceOf(InvalidCredentialsException.class);

        verify(passwordEncoder).matches("correct password", user.passwordHash());
        verify(jwtTokenService, never()).issue(any(), any());
    }
}

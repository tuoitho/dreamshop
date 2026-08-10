package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.auth.domain.EmailNormalizer;
import com.ecommerce.identity.auth.domain.InvalidCredentialsException;
import com.ecommerce.identity.security.JwtTokenService;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import java.util.Optional;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class LoginService {
    private final IdentityUserRepository users;
    private final PasswordEncoder passwordEncoder;
    private final JwtTokenService jwtTokenService;
    private final RefreshTokenService refreshTokenService;
    private final Clock clock;
    private final String dummyPasswordHash;

    public LoginService(IdentityUserRepository users, PasswordEncoder passwordEncoder,
            JwtTokenService jwtTokenService, RefreshTokenService refreshTokenService, Clock clock) {
        this.users = users;
        this.passwordEncoder = passwordEncoder;
        this.jwtTokenService = jwtTokenService;
        this.refreshTokenService = refreshTokenService;
        this.clock = clock;
        this.dummyPasswordHash = passwordEncoder.encode("dummy-password-for-login-timing");
    }

    @Transactional
    public AuthenticatedSession login(LoginCommand command) {
        Optional<IdentityUserEntity> user = users.findWithRolesByNormalizedEmail(
                EmailNormalizer.normalize(command.email()));
        String passwordHash = user.map(IdentityUserEntity::passwordHash).orElse(dummyPasswordHash);
        boolean passwordMatches = passwordEncoder.matches(command.password(), passwordHash);

        if (user.isEmpty() || user.get().status() != AccountStatus.ACTIVE || !passwordMatches) {
            throw new InvalidCredentialsException();
        }

        IdentityUserEntity authenticatedUser = user.get();
        Instant now = clock.instant();
        String rawRefreshToken = refreshTokenService.issue(authenticatedUser.id(), now);
        AccessTokenResult accessToken = jwtTokenService.issue(authenticatedUser.id(), authenticatedUser.roleCodes());
        return new AuthenticatedSession(accessToken, rawRefreshToken);
    }
}

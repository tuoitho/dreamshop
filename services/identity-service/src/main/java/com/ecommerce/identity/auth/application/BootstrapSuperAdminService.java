package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.auth.domain.EmailNormalizer;
import com.ecommerce.identity.common.config.IdentityBootstrapSuperAdminProperties;
import com.ecommerce.identity.rbac.domain.RbacPermissions;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class BootstrapSuperAdminService {
    private final IdentityUserRepository users;
    private final RegisterUserService registration;
    private final RbacRepository rbac;
    private final Clock clock;

    @Transactional
    public void ensureSuperAdmin(IdentityBootstrapSuperAdminProperties properties) {
        if (!properties.configured()) {
            return;
        }

        String normalizedEmail = EmailNormalizer.normalize(properties.email());
        if (users.existsByNormalizedEmail(normalizedEmail)) {
            return;
        }

        RegisteredUser user;
        try {
            user = registration.register(new RegisterCommand(
                    normalizedEmail, properties.password(), properties.fullName()));
        } catch (EmailAlreadyRegisteredException exception) {
            return;
        }

        Instant now = clock.instant();
        if (!rbac.assignRole(user.userId(), RbacPermissions.SUPER_ADMIN, now)) {
            throw new IllegalStateException("SUPER_ADMIN role is not configured");
        }
    }
}

package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.auth.domain.EmailNormalizer;
import com.ecommerce.identity.common.config.IdentityBootstrapSuperAdminProperties;
import com.ecommerce.identity.rbac.domain.RbacPermissions;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionOperations;

@Service
public class BootstrapSuperAdminService {
    private final IdentityUserRepository users;
    private final RegisterUserService registration;
    private final RbacRepository rbac;
    private final Clock clock;
    private final TransactionOperations transactions;

    @Autowired
    public BootstrapSuperAdminService(
            IdentityUserRepository users,
            RegisterUserService registration,
            RbacRepository rbac,
            Clock clock,
            TransactionOperations transactions) {
        this.users = users;
        this.registration = registration;
        this.rbac = rbac;
        this.clock = clock;
        this.transactions = transactions;
    }

    BootstrapSuperAdminService(
            IdentityUserRepository users,
            RegisterUserService registration,
            RbacRepository rbac,
            Clock clock) {
        this(users, registration, rbac, clock, TransactionOperations.withoutTransaction());
    }

    public void ensureSuperAdmin(IdentityBootstrapSuperAdminProperties properties) {
        if (!properties.configured()) {
            return;
        }

        String normalizedEmail = EmailNormalizer.normalize(properties.email());
        if (users.existsByNormalizedEmail(normalizedEmail)) {
            return;
        }

        try {
            transactions.executeWithoutResult(status -> {
                RegisteredUser user = registration.register(new RegisterCommand(
                        normalizedEmail, properties.password(), properties.fullName()));
                Instant now = clock.instant();
                if (!rbac.assignRole(user.userId(), RbacPermissions.SUPER_ADMIN, now)) {
                    throw new IllegalStateException("SUPER_ADMIN role is not configured");
                }
            });
        } catch (EmailAlreadyRegisteredException exception) {
            return;
        }
    }
}

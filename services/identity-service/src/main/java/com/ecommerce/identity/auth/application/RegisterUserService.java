package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.auth.domain.EmailNormalizer;
import com.ecommerce.identity.rbac.domain.RbacPermissions;
import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.time.Instant;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.hibernate.exception.ConstraintViolationException;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
@RequiredArgsConstructor
public class RegisterUserService {
    private static final String CUSTOMER_ROLE = RbacPermissions.CUSTOMER;
    private static final String NORMALIZED_EMAIL_CONSTRAINT = "uq_identity_users_normalized_email";

    private final IdentityUserRepository users;
    private final RbacRepository rbac;
    private final PasswordEncoder passwordEncoder;
    private final Clock clock;

    @Transactional
    public RegisteredUser register(RegisterCommand command) {
        String normalizedEmail = EmailNormalizer.normalize(command.email());
        if (users.existsByNormalizedEmail(normalizedEmail)) {
            throw new EmailAlreadyRegisteredException();
        }
        if (!rbac.roleExists(CUSTOMER_ROLE)) {
            throw new IllegalStateException("CUSTOMER role is not configured");
        }
        String passwordHash = passwordEncoder.encode(command.password());
        Instant now = clock.instant();
        IdentityUserEntity user = IdentityUserEntity.create(UUID.randomUUID(), normalizedEmail, normalizedEmail,
                passwordHash, AccountStatus.ACTIVE, now, command.fullName());

        try {
            users.saveAndFlush(user);
        } catch (DataIntegrityViolationException exception) {
            if (violatesNormalizedEmailConstraint(exception)) {
                throw new EmailAlreadyRegisteredException();
            }
            throw exception;
        }

        rbac.assignRole(user.id(), CUSTOMER_ROLE, now);
        return new RegisteredUser(user.id(), user.normalizedEmail(), user.fullName(),
                rbac.assignedRoles(user.id()));
    }

    private boolean violatesNormalizedEmailConstraint(DataIntegrityViolationException exception) {
        Throwable cause = exception;
        while (cause != null) {
            if (cause instanceof ConstraintViolationException constraintViolation
                    && NORMALIZED_EMAIL_CONSTRAINT.equals(constraintViolation.getConstraintName())) {
                return true;
            }
            cause = cause.getCause();
        }
        return false;
    }
}

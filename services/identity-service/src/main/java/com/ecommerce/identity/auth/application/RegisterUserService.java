package com.ecommerce.identity.auth.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.auth.domain.EmailAlreadyRegisteredException;
import com.ecommerce.identity.auth.domain.EmailNormalizer;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import com.ecommerce.identity.user.persistence.RoleEntity;
import com.ecommerce.identity.user.persistence.RoleRepository;
import java.time.Clock;
import java.time.Instant;
import java.util.Set;
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
    private static final String CUSTOMER_ROLE = "CUSTOMER";
    private static final String NORMALIZED_EMAIL_CONSTRAINT = "uq_identity_users_normalized_email";

    private final IdentityUserRepository users;
    private final RoleRepository roles;
    private final PasswordEncoder passwordEncoder;
    private final Clock clock;

    @Transactional
    public RegisteredUser register(RegisterCommand command) {
        String normalizedEmail = EmailNormalizer.normalize(command.email());
        if (users.existsByNormalizedEmail(normalizedEmail)) {
            throw new EmailAlreadyRegisteredException();
        }
        RoleEntity customer = roles.findByCode(CUSTOMER_ROLE)
                .orElseThrow(() -> new IllegalStateException("CUSTOMER role is not configured"));
        String passwordHash = passwordEncoder.encode(command.password());
        Instant now = clock.instant();
        IdentityUserEntity user = IdentityUserEntity.create(UUID.randomUUID(), normalizedEmail, normalizedEmail,
                passwordHash, AccountStatus.ACTIVE, now, command.fullName(), Set.of(customer));

        try {
            users.saveAndFlush(user);
        } catch (DataIntegrityViolationException exception) {
            if (violatesNormalizedEmailConstraint(exception)) {
                throw new EmailAlreadyRegisteredException();
            }
            throw exception;
        }

        return new RegisteredUser(user.id(), user.normalizedEmail(), user.fullName(), user.roleCodes());
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

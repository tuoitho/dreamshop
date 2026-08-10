package com.ecommerce.identity.user.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.time.Clock;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class MeService {
    private final IdentityUserRepository repository;
    private final Clock clock;

    public MeService(IdentityUserRepository repository, Clock clock) {
        this.repository = repository;
        this.clock = clock;
    }

    public CurrentUser current(UUID userId) {
        IdentityUserEntity user = repository.findWithProfileAndRolesById(userId)
                .filter(candidate -> candidate.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
        return new CurrentUser(user.id(), user.email(), user.fullName(), user.roleCodes());
    }

    @Transactional
    public CurrentUser update(UUID userId, String fullName) {
        IdentityUserEntity user = repository.findWithProfileAndRolesById(userId)
                .filter(candidate -> candidate.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
        user.updateFullName(fullName, clock.instant());
        return new CurrentUser(user.id(), user.email(), user.fullName(), user.roleCodes());
    }
}

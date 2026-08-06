package com.ecommerce.identity.user.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.user.domain.UserNotFoundException;
import com.ecommerce.identity.user.persistence.IdentityUserEntity;
import com.ecommerce.identity.user.persistence.IdentityUserRepository;
import java.util.UUID;
import org.springframework.stereotype.Service;

@Service
public class MeService {
    private final IdentityUserRepository repository;

    public MeService(IdentityUserRepository repository) {
        this.repository = repository;
    }

    public CurrentUser current(UUID userId) {
        IdentityUserEntity user = repository.findWithProfileAndRolesById(userId)
                .filter(candidate -> candidate.status() == AccountStatus.ACTIVE)
                .orElseThrow(UserNotFoundException::new);
        return new CurrentUser(user.id(), user.email(), user.fullName(), user.roleCodes());
    }
}

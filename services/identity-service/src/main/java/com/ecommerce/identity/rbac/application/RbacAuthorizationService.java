package com.ecommerce.identity.rbac.application;

import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.security.CallerContextPrincipal;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import org.springframework.security.core.Authentication;
import org.springframework.stereotype.Service;

@Service("rbac")
@RequiredArgsConstructor
public class RbacAuthorizationService {
    private final RbacRepository repository;

    public boolean can(Authentication authentication, String permission) {
        return authentication instanceof CallerContextPrincipal caller
                && permission != null
                && !permission.isBlank()
                && can(caller.userId(), permission);
    }

    public boolean can(UUID userId, String permission) {
        return userId != null
                && permission != null
                && !permission.isBlank()
                && allowedPermissions(userId, List.of(permission)).contains(permission);
    }

    public Set<String> allowedPermissions(UUID userId, List<String> permissions) {
        if (userId == null || permissions == null || permissions.isEmpty()) {
            return Set.of();
        }
        return repository.findAllowedPermissions(userId, permissions);
    }
}

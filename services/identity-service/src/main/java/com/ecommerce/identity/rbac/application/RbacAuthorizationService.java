package com.ecommerce.identity.rbac.application;

import com.ecommerce.identity.rbac.persistence.RbacRepository;
import com.ecommerce.identity.security.CallerContextPrincipal;
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
                && repository.hasPermission(caller.userId(), permission);
    }
}

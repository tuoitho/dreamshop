package com.ecommerce.identity.user.admin.api;

import com.ecommerce.identity.auth.domain.AccountStatus;
import com.ecommerce.identity.user.admin.application.AdminUserView;
import java.time.Instant;
import java.util.Set;
import java.util.UUID;

public record AdminUserResponse(
        UUID userId,
        String email,
        String fullName,
        AccountStatus status,
        Set<String> roles,
        Instant createdAt,
        Instant updatedAt) {
    public static AdminUserResponse from(AdminUserView view) {
        return new AdminUserResponse(view.userId(), view.email(), view.fullName(), view.status(),
                view.roles(), view.createdAt(), view.updatedAt());
    }
}

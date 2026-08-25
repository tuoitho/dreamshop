package com.ecommerce.identity.user.admin.application;

import com.ecommerce.identity.auth.domain.AccountStatus;
import java.time.Instant;
import java.util.Set;
import java.util.UUID;

public record AdminUserView(
        UUID userId,
        String email,
        String fullName,
        AccountStatus status,
        Set<String> roles,
        Instant createdAt,
        Instant updatedAt) {
}

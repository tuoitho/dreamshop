package com.ecommerce.identity.rbac.application;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import java.time.Instant;
import java.util.Set;

public record RbacItemView(
        String name,
        AuthItemType type,
        String description,
        boolean systemItem,
        Set<String> directChildren,
        Instant createdAt,
        Instant updatedAt) {
}

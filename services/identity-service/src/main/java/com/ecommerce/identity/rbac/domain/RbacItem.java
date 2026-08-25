package com.ecommerce.identity.rbac.domain;

import java.time.Instant;

public record RbacItem(
        String name,
        AuthItemType type,
        String description,
        boolean systemItem,
        Instant createdAt,
        Instant updatedAt) {
}

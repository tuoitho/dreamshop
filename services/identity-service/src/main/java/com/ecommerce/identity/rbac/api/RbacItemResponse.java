package com.ecommerce.identity.rbac.api;

import com.ecommerce.identity.rbac.application.RbacItemView;
import com.ecommerce.identity.rbac.domain.AuthItemType;
import java.time.Instant;
import java.util.Set;

public record RbacItemResponse(
        String name,
        AuthItemType type,
        String description,
        boolean systemItem,
        Set<String> directChildren,
        Instant createdAt,
        Instant updatedAt) {
    public static RbacItemResponse from(RbacItemView view) {
        return new RbacItemResponse(view.name(), view.type(), view.description(), view.systemItem(),
                view.directChildren(), view.createdAt(), view.updatedAt());
    }
}

package com.ecommerce.identity.rbac.api;

import com.ecommerce.identity.rbac.domain.AuthItemType;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

public record CreateRbacItemRequest(
        @NotBlank @Size(max = 128) String name,
        @NotNull AuthItemType type,
        @Size(max = 255) String description) {
}

package com.ecommerce.identity.rbac.api;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record UpdateRbacItemRequest(
        @NotBlank @Size(max = 128) String name,
        @Size(max = 255) String description) {
}

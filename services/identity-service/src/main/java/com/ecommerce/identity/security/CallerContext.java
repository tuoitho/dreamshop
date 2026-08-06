package com.ecommerce.identity.security;

import java.util.Set;
import java.util.UUID;

public record CallerContext(UUID userId, Set<String> roles) {
}

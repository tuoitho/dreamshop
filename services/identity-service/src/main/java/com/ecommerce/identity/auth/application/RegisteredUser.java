package com.ecommerce.identity.auth.application;

import java.util.Set;
import java.util.UUID;

public record RegisteredUser(UUID userId, String email, String fullName, Set<String> roles) {
}

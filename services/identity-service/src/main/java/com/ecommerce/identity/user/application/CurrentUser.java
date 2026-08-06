package com.ecommerce.identity.user.application;

import java.util.Set;
import java.util.UUID;

public record CurrentUser(UUID userId, String email, String fullName, Set<String> roles) {
}

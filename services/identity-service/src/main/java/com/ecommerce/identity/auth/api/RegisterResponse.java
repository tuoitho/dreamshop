package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.RegisteredUser;
import java.util.Set;
import java.util.UUID;

public record RegisterResponse(
        UUID userId,
        String email,
        String fullName,
        Set<String> roles) {
    static RegisterResponse from(RegisteredUser user) {
        return new RegisterResponse(user.userId(), user.email(), user.fullName(), user.roles());
    }
}

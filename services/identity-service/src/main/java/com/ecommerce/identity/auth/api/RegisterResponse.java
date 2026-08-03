package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.RegisteredUser;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Set;
import java.util.UUID;

public record RegisterResponse(
        @JsonProperty("user_id") UUID userId,
        String email,
        @JsonProperty("full_name") String fullName,
        Set<String> roles) {
    static RegisterResponse from(RegisteredUser user) {
        return new RegisterResponse(user.userId(), user.email(), user.fullName(), user.roles());
    }
}

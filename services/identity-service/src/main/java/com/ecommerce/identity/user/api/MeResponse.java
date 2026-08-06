package com.ecommerce.identity.user.api;

import com.ecommerce.identity.user.application.CurrentUser;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Set;
import java.util.UUID;

public record MeResponse(
        @JsonProperty("user_id") UUID userId,
        String email,
        @JsonProperty("full_name") String fullName,
        Set<String> roles) {
    static MeResponse from(CurrentUser user) {
        return new MeResponse(user.userId(), user.email(), user.fullName(), user.roles());
    }
}

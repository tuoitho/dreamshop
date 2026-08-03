package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.AccessTokenResult;
import com.fasterxml.jackson.annotation.JsonProperty;

public record LoginResponse(
        @JsonProperty("access_token") String accessToken,
        @JsonProperty("token_type") String tokenType,
        @JsonProperty("expires_in") long expiresIn) {
    static LoginResponse from(AccessTokenResult token) {
        return new LoginResponse(token.token(), "Bearer", token.expiresInSeconds());
    }
}

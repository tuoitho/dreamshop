package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.AccessTokenResult;

public record LoginResponse(
        String accessToken,
        String tokenType,
        long expiresIn) {
    static LoginResponse from(AccessTokenResult token) {
        return new LoginResponse(token.token(), "Bearer", token.expiresInSeconds());
    }
}

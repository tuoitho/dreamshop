package com.ecommerce.identity.auth.application;

public record AuthenticatedSession(AccessTokenResult accessToken, String rawRefreshToken) {
}

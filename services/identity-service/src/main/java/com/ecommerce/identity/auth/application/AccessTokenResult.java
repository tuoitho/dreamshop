package com.ecommerce.identity.auth.application;

public record AccessTokenResult(String token, long expiresInSeconds) {
}

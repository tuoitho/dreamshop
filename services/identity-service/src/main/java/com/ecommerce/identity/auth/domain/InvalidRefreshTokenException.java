package com.ecommerce.identity.auth.domain;

public class InvalidRefreshTokenException extends RuntimeException {
    public InvalidRefreshTokenException() {
        super("Refresh token is invalid or expired");
    }
}

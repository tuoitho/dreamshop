package com.ecommerce.identity.auth.persistence;

public enum RefreshTokenRevocationReason {
    ROTATED, LOGOUT, REUSE_DETECTED
}

package com.ecommerce.identity.common.config;

import java.nio.file.Path;
import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties("identity.jwt")
public record IdentityJwtProperties(
        String issuer, String audience, Path privateKeyPath, Path publicKeyPath,
        String keyId, Duration accessTtl) {
    public IdentityJwtProperties {
        requireText(issuer, "issuer");
        requireText(audience, "audience");
        requireText(keyId, "key-id");
        if (privateKeyPath == null || publicKeyPath == null) {
            throw new IllegalArgumentException("JWT key paths are required");
        }
        if (accessTtl == null || accessTtl.compareTo(Duration.ofMinutes(1)) < 0
                || accessTtl.compareTo(Duration.ofMinutes(30)) > 0) {
            throw new IllegalArgumentException("JWT access TTL must be 1 through 30 minutes");
        }
    }

    private static void requireText(String value, String name) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException("JWT " + name + " is required");
        }
    }
}

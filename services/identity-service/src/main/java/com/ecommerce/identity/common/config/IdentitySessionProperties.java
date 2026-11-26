package com.ecommerce.identity.common.config;

import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties("identity.session")
public record IdentitySessionProperties(Duration refreshTtl, String cookieName,
        String cookiePath, String adminCookieName, String adminCookiePath, boolean cookieSecure) {
    public IdentitySessionProperties {
        if (refreshTtl == null || refreshTtl.isNegative() || refreshTtl.isZero()) {
            throw new IllegalArgumentException("Refresh TTL must be positive");
        }
        if (cookieName == null || cookieName.isBlank()
                || cookiePath == null || cookiePath.isBlank()) {
            throw new IllegalArgumentException("Refresh cookie name and path are required");
        }
        if (adminCookieName == null || adminCookieName.isBlank()
                || adminCookiePath == null || adminCookiePath.isBlank()) {
            throw new IllegalArgumentException("Admin refresh cookie name and path are required");
        }
        if (cookieName.equals(adminCookieName)) {
            throw new IllegalArgumentException("Admin refresh cookie name must differ from storefront cookie name");
        }
    }
}

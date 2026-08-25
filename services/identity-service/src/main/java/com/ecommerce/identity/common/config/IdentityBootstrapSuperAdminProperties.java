package com.ecommerce.identity.common.config;

import java.util.regex.Pattern;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties("identity.bootstrap.super-admin")
public record IdentityBootstrapSuperAdminProperties(String email, String password, String fullName) {
    private static final Pattern EMAIL = Pattern.compile("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$");

    public IdentityBootstrapSuperAdminProperties {
        email = trim(email);
        password = password == null ? null : password;
        fullName = trim(fullName);

        boolean anyConfigured = notBlank(email) || notBlank(password) || notBlank(fullName);
        boolean fullyConfigured = notBlank(email) && notBlank(password) && notBlank(fullName);
        if (!anyConfigured) {
            email = null;
            password = null;
            fullName = null;
        } else {
            if (!fullyConfigured) {
                throw new IllegalArgumentException(
                        "Identity bootstrap SUPER_ADMIN values must be supplied together");
            }
            if (email.length() > 254 || !EMAIL.matcher(email).matches()) {
                throw new IllegalArgumentException("Identity bootstrap SUPER_ADMIN email is invalid");
            }
            if (password.length() < 12 || password.length() > 128) {
                throw new IllegalArgumentException(
                        "Identity bootstrap SUPER_ADMIN password must be 12 through 128 characters");
            }
            if (fullName.length() > 100) {
                throw new IllegalArgumentException(
                        "Identity bootstrap SUPER_ADMIN full name must be at most 100 characters");
            }
        }
    }

    public boolean configured() {
        return email != null;
    }

    private static boolean notBlank(String value) {
        return value != null && !value.isBlank();
    }

    private static String trim(String value) {
        return value == null ? null : value.trim();
    }
}

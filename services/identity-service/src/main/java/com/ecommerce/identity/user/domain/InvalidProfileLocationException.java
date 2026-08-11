package com.ecommerce.identity.user.domain;

import java.util.Map;

/**
 * Thrown when a profile update supplies an invalid province/ward location pair.
 * Mapped to {@code 400 VALIDATION_FAILED} with per-field messages.
 */
public class InvalidProfileLocationException extends RuntimeException {
    private final Map<String, String> errors;

    public InvalidProfileLocationException(Map<String, String> errors) {
        super(String.join("; ", errors.values()));
        this.errors = Map.copyOf(errors);
    }

    public Map<String, String> errors() {
        return errors;
    }
}

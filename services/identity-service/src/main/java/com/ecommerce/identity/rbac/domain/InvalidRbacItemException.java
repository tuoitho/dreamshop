package com.ecommerce.identity.rbac.domain;

import java.util.Map;

public class InvalidRbacItemException extends RuntimeException {
    private final Map<String, String> errors;

    public InvalidRbacItemException(Map<String, String> errors) {
        super("RBAC item validation failed");
        this.errors = Map.copyOf(errors);
    }

    public Map<String, String> errors() {
        return errors;
    }
}

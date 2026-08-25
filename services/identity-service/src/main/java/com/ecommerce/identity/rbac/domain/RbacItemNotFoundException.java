package com.ecommerce.identity.rbac.domain;

public class RbacItemNotFoundException extends RuntimeException {
    public RbacItemNotFoundException() {
        super("RBAC item not found");
    }
}

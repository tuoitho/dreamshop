package com.ecommerce.identity.rbac.domain;

public class RbacItemExistsException extends RuntimeException {
    public RbacItemExistsException() {
        super("RBAC item already exists");
    }
}

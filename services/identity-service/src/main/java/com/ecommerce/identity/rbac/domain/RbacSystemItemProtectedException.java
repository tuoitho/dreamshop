package com.ecommerce.identity.rbac.domain;

public class RbacSystemItemProtectedException extends RuntimeException {
    public RbacSystemItemProtectedException() {
        super("RBAC system item is protected");
    }
}

package com.ecommerce.identity.rbac.domain;

public class RbacHierarchyConflictException extends RuntimeException {
    public RbacHierarchyConflictException() {
        super("RBAC hierarchy change is invalid");
    }
}

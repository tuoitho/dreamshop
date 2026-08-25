package com.ecommerce.identity.user.admin.domain;

public class LastSuperAdminRequiredException extends RuntimeException {
    public LastSuperAdminRequiredException() {
        super("At least one active super administrator is required");
    }
}

package com.ecommerce.identity.user.admin.domain;

public class SuperAdminAssignmentForbiddenException extends RuntimeException {
    public SuperAdminAssignmentForbiddenException() {
        super("Only a super administrator can change super administrator assignments");
    }
}

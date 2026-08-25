package com.ecommerce.identity.user.admin.domain;

public class SelfDisableNotAllowedException extends RuntimeException {
    public SelfDisableNotAllowedException() {
        super("An administrator cannot disable their own account");
    }
}

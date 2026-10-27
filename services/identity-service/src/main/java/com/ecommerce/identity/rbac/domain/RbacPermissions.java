package com.ecommerce.identity.rbac.domain;

public final class RbacPermissions {
    public static final String CUSTOMER = "CUSTOMER";
    public static final String ADMIN = "ADMIN";
    public static final String SUPER_ADMIN = "SUPER_ADMIN";

    public static final String ADDRESS_VIEW = "address.view";
    public static final String ADDRESS_CREATE = "address.create";
    public static final String ADDRESS_UPDATE = "address.update";
    public static final String ADDRESS_DELETE = "address.delete";
    public static final String ADDRESS_SET_DEFAULT = "address.set-default";
    public static final String USER_VIEW = "user.view";
    public static final String USER_STATUS_UPDATE = "user.status.update";
    public static final String USER_ROLE_ASSIGN = "user.role.assign";
    public static final String RBAC_VIEW = "rbac.view";
    public static final String RBAC_MANAGE = "rbac.manage";
    public static final String PRODUCT_VIEW = "product.view";
    public static final String PRODUCT_CREATE = "product.create";
    public static final String PRODUCT_UPDATE = "product.update";
    public static final String PRODUCT_DELETE = "product.delete";
    public static final String INVENTORY_VIEW = "inventory.view";
    public static final String INVENTORY_UPDATE = "inventory.update";

    private RbacPermissions() {
    }
}

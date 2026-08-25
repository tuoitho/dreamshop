package com.ecommerce.identity.address.domain;

public class ShippingAddressNotFoundException extends RuntimeException {
    public ShippingAddressNotFoundException() {
        super("Shipping address not found");
    }
}

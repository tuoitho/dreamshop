package com.ecommerce.identity.address.application;

public record CreateShippingAddressCommand(
        String recipientName,
        String phone,
        String provinceCode,
        String wardCode,
        String addressLine,
        String label) {
}

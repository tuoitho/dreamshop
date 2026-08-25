package com.ecommerce.identity.address.application;

import java.time.Instant;
import java.util.UUID;

public record ShippingAddressView(
        UUID id,
        String recipientName,
        String phone,
        String provinceCode,
        String wardCode,
        String addressLine,
        String label,
        boolean isDefault,
        Instant createdAt,
        Instant updatedAt) {
}

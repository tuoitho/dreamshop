package com.ecommerce.identity.address.api;

import com.ecommerce.identity.address.application.ShippingAddressView;
import java.time.Instant;
import java.util.UUID;

public record ShippingAddressResponse(
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
    public static ShippingAddressResponse from(ShippingAddressView view) {
        return new ShippingAddressResponse(view.id(), view.recipientName(), view.phone(), view.provinceCode(),
                view.wardCode(), view.addressLine(), view.label(), view.isDefault(), view.createdAt(), view.updatedAt());
    }
}

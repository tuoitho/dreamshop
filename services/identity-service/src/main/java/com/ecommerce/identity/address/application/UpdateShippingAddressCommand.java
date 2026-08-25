package com.ecommerce.identity.address.application;

import com.ecommerce.identity.address.domain.ShippingAddressPatch;

public record UpdateShippingAddressCommand(
        ShippingAddressPatch<String> recipientName,
        ShippingAddressPatch<String> phone,
        ShippingAddressPatch<String> provinceCode,
        ShippingAddressPatch<String> wardCode,
        ShippingAddressPatch<String> addressLine,
        ShippingAddressPatch<String> label) {
    public static UpdateShippingAddressCommand keepAll() {
        return new UpdateShippingAddressCommand(
                ShippingAddressPatch.keep(), ShippingAddressPatch.keep(), ShippingAddressPatch.keep(),
                ShippingAddressPatch.keep(), ShippingAddressPatch.keep(), ShippingAddressPatch.keep());
    }
}

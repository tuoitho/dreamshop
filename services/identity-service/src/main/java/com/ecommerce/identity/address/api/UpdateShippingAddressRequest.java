package com.ecommerce.identity.address.api;

import com.ecommerce.identity.address.application.UpdateShippingAddressCommand;
import com.ecommerce.identity.address.domain.ShippingAddressPatch;
import com.ecommerce.identity.user.api.NullOrNotBlank;
import com.fasterxml.jackson.annotation.JsonSetter;
import com.fasterxml.jackson.annotation.Nulls;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;
import lombok.Getter;
import lombok.experimental.Accessors;

@Getter
@Accessors(fluent = true)
public class UpdateShippingAddressRequest {
    @NullOrNotBlank
    @Size(max = 100)
    private String recipientName;
    private boolean recipientNamePresent;

    @NullOrNotBlank
    @Size(max = 20)
    @Pattern(regexp = "^(?:(?:\\+84|0)(?:3|5|7|8|9)\\d{8})?$",
            message = "must be a valid Vietnamese mobile number")
    private String phone;
    private boolean phonePresent;

    @NullOrNotBlank
    @Size(max = 20)
    private String provinceCode;
    private boolean provinceCodePresent;

    @NullOrNotBlank
    @Size(max = 20)
    private String wardCode;
    private boolean wardCodePresent;

    @NullOrNotBlank
    @Size(max = 255)
    private String addressLine;
    private boolean addressLinePresent;

    @NullOrNotBlank
    @Size(max = 50)
    private String label;
    private boolean labelPresent;

    @JsonSetter(value = "recipientName", nulls = Nulls.SET)
    public void recipientName(String value) {
        recipientNamePresent = true;
        recipientName = value;
    }

    @JsonSetter(value = "phone", nulls = Nulls.SET)
    public void phone(String value) {
        phonePresent = true;
        phone = value;
    }

    @JsonSetter(value = "provinceCode", nulls = Nulls.SET)
    public void provinceCode(String value) {
        provinceCodePresent = true;
        provinceCode = value;
    }

    @JsonSetter(value = "wardCode", nulls = Nulls.SET)
    public void wardCode(String value) {
        wardCodePresent = true;
        wardCode = value;
    }

    @JsonSetter(value = "addressLine", nulls = Nulls.SET)
    public void addressLine(String value) {
        addressLinePresent = true;
        addressLine = value;
    }

    @JsonSetter(value = "label", nulls = Nulls.SET)
    public void label(String value) {
        labelPresent = true;
        label = value;
    }

    public UpdateShippingAddressCommand command() {
        return new UpdateShippingAddressCommand(
                patch(recipientNamePresent, recipientName, false),
                patch(phonePresent, phone, false),
                patch(provinceCodePresent, provinceCode, false),
                patch(wardCodePresent, wardCode, false),
                patch(addressLinePresent, addressLine, false),
                patch(labelPresent, label, true));
    }

    private static ShippingAddressPatch<String> patch(boolean present, String value, boolean optional) {
        if (!present) {
            return ShippingAddressPatch.keep();
        }
        if (optional && (value == null || value.isBlank())) {
            return ShippingAddressPatch.clear();
        }
        if (value == null) {
            return ShippingAddressPatch.clear();
        }
        return ShippingAddressPatch.set(value.trim());
    }
}

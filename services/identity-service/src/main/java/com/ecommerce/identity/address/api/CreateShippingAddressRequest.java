package com.ecommerce.identity.address.api;

import com.ecommerce.identity.user.api.NullOrNotBlank;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Size;

public record CreateShippingAddressRequest(
        @NotBlank @Size(max = 100) String recipientName,
        @NotBlank @Size(max = 20)
        @Pattern(regexp = "^(?:\\+84|0)(?:3|5|7|8|9)\\d{8}$",
                message = "must be a valid Vietnamese mobile number")
        String phone,
        @NotBlank @Size(max = 20) String provinceCode,
        @NotBlank @Size(max = 20) String wardCode,
        @NotBlank @Size(max = 255) String addressLine,
        @NullOrNotBlank @Size(max = 50) String label) {
}

package com.ecommerce.identity.auth.api;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record RegisterRequest(
        @NotBlank @Email @Size(max = 254) String email,
        @NotBlank @Size(min = 12, max = 128) String password,
        @JsonProperty("full_name") @NotBlank @Size(max = 100) String fullName) {
    public RegisterRequest {
        email = trim(email);
        fullName = trim(fullName);
    }

    private static String trim(String value) {
        return value == null ? null : value.trim();
    }
}

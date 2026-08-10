package com.ecommerce.identity.user.api;

import com.fasterxml.jackson.annotation.JsonProperty;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record UpdateMeRequest(
        @JsonProperty("full_name") @NotBlank @Size(max = 100) String fullName) {
    public UpdateMeRequest {
        fullName = fullName == null ? null : fullName.trim();
    }
}

package com.ecommerce.identity.user.api;

import com.ecommerce.identity.user.application.CurrentUser;
import com.ecommerce.identity.user.domain.Gender;
import java.time.LocalDate;
import java.util.Set;
import java.util.UUID;

public record MeResponse(
        UUID userId,
        String email,
        String fullName,
        Set<String> roles,
        String phone,
        LocalDate dateOfBirth,
        Gender gender,
        String provinceCode,
        String wardCode,
        String addressLine) {
    static MeResponse from(CurrentUser user) {
        return new MeResponse(user.userId(), user.email(), user.fullName(), user.roles(),
                user.phone(), user.dateOfBirth(), user.gender(),
                user.provinceCode(), user.wardCode(), user.addressLine());
    }
}

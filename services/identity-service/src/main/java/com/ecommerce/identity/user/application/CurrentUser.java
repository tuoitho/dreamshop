package com.ecommerce.identity.user.application;

import com.ecommerce.identity.user.domain.Gender;
import java.time.LocalDate;
import java.util.Set;
import java.util.UUID;

public record CurrentUser(UUID userId, String email, String fullName, Set<String> roles,
        String phone, LocalDate dateOfBirth, Gender gender,
        String provinceCode, String wardCode, String addressLine) {
}

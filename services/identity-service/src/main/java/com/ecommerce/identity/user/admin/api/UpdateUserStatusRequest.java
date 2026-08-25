package com.ecommerce.identity.user.admin.api;

import com.ecommerce.identity.auth.domain.AccountStatus;
import jakarta.validation.constraints.NotNull;

public record UpdateUserStatusRequest(@NotNull AccountStatus status) {
}

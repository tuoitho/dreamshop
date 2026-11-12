package com.ecommerce.order.application;

import com.ecommerce.order.common.DomainException;
import com.ecommerce.order.domain.OrderStatus;
import io.grpc.Status;
import java.util.Locale;
import java.util.UUID;

/** Transport-level parsing shared by the administrator order use cases. */
final class AdminRequests {

    private AdminRequests() {
    }

    static UUID requiredUuid(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            throw invalid(field + " is required");
        }
        try {
            return UUID.fromString(raw.trim());
        } catch (IllegalArgumentException ex) {
            throw invalid(field + " must be a valid UUID");
        }
    }

    static OrderStatus requiredStatus(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            throw invalid(field + " is required");
        }
        return status(raw, field);
    }

    static OrderStatus optionalStatus(String raw, String field) {
        return raw == null || raw.isBlank() ? null : status(raw, field);
    }

    private static OrderStatus status(String raw, String field) {
        try {
            return OrderStatus.valueOf(raw.trim().toUpperCase(Locale.ROOT));
        } catch (IllegalArgumentException ex) {
            throw invalid(field + " is not a valid order status");
        }
    }

    private static DomainException invalid(String message) {
        return new DomainException(Status.Code.INVALID_ARGUMENT, "INVALID_PARAMETER", message);
    }
}

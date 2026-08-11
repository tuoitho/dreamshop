package com.ecommerce.identity.common.error;

import com.fasterxml.jackson.annotation.JsonInclude;
import java.util.Map;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record ApiError(
        String code,
        String message,
        Map<String, String> errors,
        String traceId) {
}

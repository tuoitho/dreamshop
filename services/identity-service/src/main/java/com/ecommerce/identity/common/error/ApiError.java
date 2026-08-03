package com.ecommerce.identity.common.error;

import com.fasterxml.jackson.annotation.JsonProperty;

public record ApiError(String code, String message, @JsonProperty("trace_id") String traceId) {
}

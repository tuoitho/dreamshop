package com.ecommerce.identity.common.error;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Map;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record ApiError(
        String code,
        String message,
        Map<String, String> errors,
        @JsonProperty("trace_id") String traceId) {
}

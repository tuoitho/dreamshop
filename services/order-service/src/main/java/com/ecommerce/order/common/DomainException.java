package com.ecommerce.order.common;

import io.grpc.Status;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

public class DomainException extends RuntimeException {
    private final Status.Code grpcCode;
    private final String reason;
    private final Map<String, String> metadata;

    public DomainException(Status.Code grpcCode, String reason, String message) {
        this(grpcCode, reason, message, Map.of());
    }

    public DomainException(Status.Code grpcCode, String reason, String message, Map<String, String> metadata) {
        super(message);
        this.grpcCode = grpcCode;
        this.reason = reason;
        this.metadata = metadata == null || metadata.isEmpty()
                ? Map.of()
                : Collections.unmodifiableMap(new LinkedHashMap<>(metadata));
    }

    public Status.Code grpcCode() {
        return grpcCode;
    }

    public String reason() {
        return reason;
    }

    public Map<String, String> metadata() {
        return metadata;
    }
}

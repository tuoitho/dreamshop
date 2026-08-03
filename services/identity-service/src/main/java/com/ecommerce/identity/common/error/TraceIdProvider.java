package com.ecommerce.identity.common.error;

import java.util.UUID;
import org.slf4j.MDC;

public final class TraceIdProvider {
    static final String MDC_KEY = "trace_id";

    private TraceIdProvider() {
    }

    public static String current() {
        String traceId = MDC.get(MDC_KEY);
        return traceId == null ? newTraceId() : traceId;
    }

    static String newTraceId() {
        return UUID.randomUUID().toString().replace("-", "");
    }
}

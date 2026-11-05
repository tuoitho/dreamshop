package com.ecommerce.order.common;

import java.util.UUID;
import org.slf4j.MDC;

public final class TraceIdProvider {
    public static final String MDC_KEY = "trace_id";
    private static final int MAX_TRACE_ID_LENGTH = 64;

    private TraceIdProvider() {
    }

    public static String current() {
        String traceId = MDC.get(MDC_KEY);
        return traceId == null || traceId.isBlank() ? newTraceId() : traceId;
    }

    public static String resolve(String candidate) {
        return isValid(candidate) ? candidate : newTraceId();
    }

    private static String newTraceId() {
        return UUID.randomUUID().toString().replace("-", "");
    }

    private static boolean isValid(String candidate) {
        if (candidate == null || candidate.isEmpty() || candidate.length() > MAX_TRACE_ID_LENGTH) {
            return false;
        }
        for (int i = 0; i < candidate.length(); i++) {
            char c = candidate.charAt(i);
            if (Character.isLetterOrDigit(c) || c == '-' || c == '_' || c == '.') {
                continue;
            }
            return false;
        }
        return true;
    }
}

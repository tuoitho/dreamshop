package com.ecommerce.identity.common.error;

import java.util.UUID;
import org.slf4j.MDC;

public final class TraceIdProvider {
    static final String MDC_KEY = "trace_id";
    private static final int MAX_TRACE_ID_LENGTH = 64;

    private TraceIdProvider() {
    }

    public static String current() {
        String traceId = MDC.get(MDC_KEY);
        return traceId == null ? newTraceId() : traceId;
    }

    static String newTraceId() {
        return UUID.randomUUID().toString().replace("-", "");
    }

    static String resolve(String candidate) {
        return isValid(candidate) ? candidate : newTraceId();
    }

    private static boolean isValid(String candidate) {
        if (candidate == null || candidate.isEmpty() || candidate.length() > MAX_TRACE_ID_LENGTH) {
            return false;
        }
        for (int index = 0; index < candidate.length(); index++) {
            char value = candidate.charAt(index);
            if (Character.isLetterOrDigit(value) || value == '-' || value == '_' || value == '.') {
                continue;
            }
            return false;
        }
        return true;
    }
}

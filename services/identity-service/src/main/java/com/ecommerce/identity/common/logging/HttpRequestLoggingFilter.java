package com.ecommerce.identity.common.logging;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.concurrent.TimeUnit;
import java.util.function.LongSupplier;
import lombok.extern.slf4j.Slf4j;
import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
@Order(Ordered.HIGHEST_PRECEDENCE + 1)
@Slf4j
public class HttpRequestLoggingFilter extends OncePerRequestFilter {
    private static final String COMPLETION_MESSAGE =
            "http_request_completed method={} path={} status={} duration_ms={}";
    private final LongSupplier nanoTime;

    public HttpRequestLoggingFilter() {
        this(System::nanoTime);
    }

    HttpRequestLoggingFilter(LongSupplier nanoTime) {
        this.nanoTime = nanoTime;
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response,
            FilterChain filterChain) throws ServletException, IOException {
        long startedAt = nanoTime.getAsLong();
        boolean failed = false;
        try {
            filterChain.doFilter(request, response);
        } catch (Throwable exception) {
            failed = true;
            throw exception;
        } finally {
            int status;
            if (failed && !response.isCommitted()
                    && response.getStatus() == HttpServletResponse.SC_OK) {
                status = HttpServletResponse.SC_INTERNAL_SERVER_ERROR;
            } else {
                status = response.getStatus();
            }
            long durationMs = Math.max(0L,
                    TimeUnit.NANOSECONDS.toMillis(nanoTime.getAsLong() - startedAt));
            logCompleted(request.getMethod(), request.getRequestURI(), status, durationMs);
        }
    }

    private void logCompleted(String method, String path, int status, long durationMs) {
        if (status >= HttpServletResponse.SC_INTERNAL_SERVER_ERROR) {
            log.error(COMPLETION_MESSAGE, method, path, status, durationMs);
        } else if (status == HttpServletResponse.SC_FORBIDDEN || status == 429) {
            log.warn(COMPLETION_MESSAGE, method, path, status, durationMs);
        } else {
            log.info(COMPLETION_MESSAGE, method, path, status, durationMs);
        }
    }
}

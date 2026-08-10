# Identity HTTP Request Logging Design

## Goal

Provide useful console visibility for every HTTP request handled by Identity service while preserving the existing trace ID behavior and preventing credentials or tokens from entering logs.

## Scope

Identity service will emit one completion event for every HTTP request, including successful requests, client errors, server errors, and actuator health checks. The change covers requests handled by Spring MVC and failures produced earlier by Spring Security.

The implementation will not log request or response bodies, query strings, cookies, authorization headers, refresh tokens, passwords, or arbitrary request headers. It will not change API responses, authentication behavior, or Kong logging.

## Architecture

Add a dedicated `HttpRequestLoggingFilter` immediately after `TraceIdFilter`. `TraceIdFilter` keeps `Ordered.HIGHEST_PRECEDENCE`; the logging filter uses `Ordered.HIGHEST_PRECEDENCE + 1`. Keeping trace creation and request logging separate gives each filter one responsibility while ensuring the logging filter runs while `trace_id` is present in SLF4J MDC. The logging filter uses Lombok's `@Slf4j`, consistent with the service's existing Lombok dependency.

The request logging filter records the start time, invokes the remaining filter chain, and emits one completion event when processing ends. Normal and handled-error responses use the final HTTP status from `HttpServletResponse`. If an unhandled exception escapes before a response status is set, the filter records an effective status of 500 and rethrows it so Spring retains ownership of the response lifecycle. The request summary never includes a stack trace.

The console logging pattern will include `trace_id` from MDC. Request fields will use stable key-value text suitable for local reading and later ingestion:

```text
http_request_completed method=POST path=/login status=401 duration_ms=23
```

Only `request.getRequestURI()` is logged. The query string is deliberately excluded.

## Log Levels

- Responses below 500 other than `403` and `429` are logged at `INFO`, including expected `400`, `401`, and `404` outcomes.
- `403 Forbidden` and `429 Too Many Requests` are logged at `WARN` because they are more actionable for access-control and rate-limit monitoring.
- Responses of 500 or greater are logged at `ERROR`.
- An exception escaping the filter chain is summarized at `ERROR` with an effective status of 500, without a stack trace in the request summary.

This policy keeps validation failures and occasional incorrect passwords visible without turning normal client behavior into warning noise. Repeated login failures remain the responsibility of a separate security audit or rate-limit mechanism.

## Request Flow

1. `TraceIdFilter` creates a 32-character trace ID, stores it in MDC, and adds `X-Trace-Id` to the response.
2. `HttpRequestLoggingFilter` captures the start time.
3. Spring Security, controllers, validation, and exception handlers process the request.
4. The logging filter reads the final status and elapsed time, then emits one completion event at the appropriate level.
5. `TraceIdFilter` removes the trace ID from MDC after downstream processing completes.

Because logging occurs around the full downstream chain, responses produced by `SecurityConfiguration` and `GlobalExceptionHandler` are included without adding duplicate request-summary statements to each handler.

## Error Handling and Security

The logger records only method, path, status, duration, and trace ID. It must never serialize request objects or include body, query, header, cookie, principal, email, or credential values. Expected 4xx responses do not include stack traces.

`GlobalExceptionHandler` is the single application-owned location that logs an unexpected exception with its stack trace before returning `INTERNAL_ERROR`. The request logging filter then emits the separate request summary without the exception. Business services and controllers do not catch-log-rethrow the same failure. Exceptions that bypass MVC are rethrown by the filter and left to the servlet container's exception handling; the filter still emits only its safe summary.

Logging must not swallow exceptions or alter response status codes. Failure to write a log event must not become part of the HTTP response contract.

## Testing

Focused filter tests will capture console output and verify:

- successful responses log at `INFO`;
- expected `400`, `401`, and `404` responses log at `INFO`;
- `403` and `429` responses log at `WARN`;
- 5xx responses and escaped exceptions log at `ERROR`;
- each event contains method, URI path, status, duration, and the current trace ID;
- query parameters, authorization values, cookies, and request bodies are absent;
- exceptions are rethrown after summary logging;
- unexpected MVC exceptions produce one stack trace from `GlobalExceptionHandler`, not from the request filter.

Existing Identity API tests will confirm response status, body, and trace ID behavior remain unchanged.

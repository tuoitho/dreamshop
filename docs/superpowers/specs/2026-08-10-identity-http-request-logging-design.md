# Identity HTTP Request Logging Design

## Goal

Provide useful console visibility for every HTTP request handled by Identity service while preserving the existing trace ID behavior and preventing credentials or tokens from entering logs.

## Scope

Identity service will emit one completion event for every HTTP request, including successful requests, client errors, server errors, and actuator health checks. The change covers requests handled by Spring MVC and failures produced earlier by Spring Security.

The implementation will not log request or response bodies, query strings, cookies, authorization headers, refresh tokens, passwords, or arbitrary request headers. It will not change API responses, authentication behavior, or Kong logging.

## Architecture

Add a dedicated `HttpRequestLoggingFilter` immediately after `TraceIdFilter`. Keeping trace creation and request logging separate gives each filter one responsibility while ensuring the logging filter runs while `trace_id` is present in SLF4J MDC.

The request logging filter records the start time, invokes the remaining filter chain, and emits one completion event when processing ends. Normal and handled-error responses use the final HTTP status from `HttpServletResponse`. If an unhandled exception escapes the chain, the filter records a server-error event with the exception and rethrows it so Spring retains ownership of the response lifecycle.

The console logging pattern will include `trace_id` from MDC. Request fields will use stable key-value text suitable for local reading and later ingestion:

```text
http_request_completed method=POST path=/login status=401 duration_ms=23
```

Only `request.getRequestURI()` is logged. The query string is deliberately excluded.

## Log Levels

- Responses below 400 are logged at `INFO`.
- Responses from 400 through 499 are logged at `WARN`.
- Responses of 500 or greater are logged at `ERROR`.
- An exception escaping the filter chain is logged at `ERROR` with its stack trace and an effective status of 500.

This policy makes normal request traffic visible while ensuring validation, authentication, and authorization failures stand out in the terminal.

## Request Flow

1. `TraceIdFilter` creates a 32-character trace ID, stores it in MDC, and adds `X-Trace-Id` to the response.
2. `HttpRequestLoggingFilter` captures the start time.
3. Spring Security, controllers, validation, and exception handlers process the request.
4. The logging filter reads the final status and elapsed time, then emits one completion event at the appropriate level.
5. `TraceIdFilter` removes the trace ID from MDC after downstream processing completes.

Because logging occurs around the full downstream chain, responses produced by `SecurityConfiguration` and `GlobalExceptionHandler` are included without adding duplicate logging statements to each handler.

## Error Handling and Security

The logger records only method, path, status, duration, and trace ID. It must never serialize request objects or include body, query, header, cookie, principal, email, or credential values. Expected 4xx responses do not include stack traces. Unexpected exceptions include a stack trace at `ERROR` but retain the same safe request metadata.

Logging must not swallow exceptions or alter response status codes. Failure to write a log event must not become part of the HTTP response contract.

## Testing

Focused filter tests will capture console output and verify:

- successful responses log at `INFO`;
- 4xx responses log at `WARN`;
- 5xx responses and escaped exceptions log at `ERROR`;
- each event contains method, URI path, status, duration, and the current trace ID;
- query parameters, authorization values, cookies, and request bodies are absent;
- exceptions are rethrown after logging.

Existing Identity API tests will confirm response status, body, and trace ID behavior remain unchanged.

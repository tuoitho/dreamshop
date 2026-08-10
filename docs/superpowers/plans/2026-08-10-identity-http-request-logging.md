# Identity HTTP Request Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emit one safe, trace-correlated console summary for every Identity HTTP request and one stack trace for unexpected MVC failures.

**Architecture:** A new `HttpRequestLoggingFilter` runs immediately after `TraceIdFilter`, measures elapsed time, and logs method, URI path, status, and duration without inspecting request content. `GlobalExceptionHandler` remains responsible for API responses and becomes the single application-owned logger of unexpected stack traces. The console pattern renders the existing MDC `trace_id`.

**Tech Stack:** Java 21, Spring Boot 4.1, `OncePerRequestFilter`, SLF4J/Logback, Lombok, JUnit 5, Spring mock web objects, AssertJ

---

## File Map

- Create `services/identity-service/src/main/java/com/ecommerce/identity/common/logging/HttpRequestLoggingFilter.java` for request summaries.
- Create `services/identity-service/src/test/java/com/ecommerce/identity/common/logging/HttpRequestLoggingFilterTest.java` for level, safety, timing, and exception tests.
- Modify `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java` to log unexpected failures once.
- Create `services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java` for exception ownership.
- Modify `services/identity-service/src/main/resources/application.yml` to render MDC trace IDs.
- Create `services/identity-service/src/test/java/com/ecommerce/identity/common/logging/LoggingConfigurationTest.java` for the console pattern.

### Task 1: Add request summary logging

**Files:**
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/common/logging/HttpRequestLoggingFilterTest.java`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/common/logging/HttpRequestLoggingFilter.java`

- [ ] **Step 1: Write the failing filter tests**

```java
package com.ecommerce.identity.common.logging;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.Cookie;
import java.util.List;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.LongSupplier;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;

class HttpRequestLoggingFilterTest {
    private static final String TRACE_ID = "ec1886d07ffb4438a4ecf4a56cdc1b39";
    private final Logger logger = (Logger) LoggerFactory.getLogger(HttpRequestLoggingFilter.class);
    private final ListAppender<ILoggingEvent> appender = new ListAppender<>();

    @BeforeEach
    void captureLogs() {
        appender.start();
        logger.addAppender(appender);
        MDC.put("trace_id", TRACE_ID);
    }

    @AfterEach
    void releaseLogs() {
        MDC.clear();
        logger.detachAppender(appender);
        appender.stop();
    }

    @Test
    void logsSuccessAtInfoWithTraceAndDuration() throws Exception {
        MockHttpServletRequest request = new MockHttpServletRequest("GET", "/actuator/health");
        MockHttpServletResponse response = new MockHttpServletResponse();

        filter(1_000_000L, 24_000_000L).doFilter(request, response,
                (currentRequest, currentResponse) -> ((MockHttpServletResponse) currentResponse).setStatus(200));

        ILoggingEvent event = singleEvent();
        assertThat(event.getLevel()).isEqualTo(Level.INFO);
        assertThat(event.getFormattedMessage()).isEqualTo(
                "http_request_completed method=GET path=/actuator/health status=200 duration_ms=23");
        assertThat(event.getMDCPropertyMap()).containsEntry("trace_id", TRACE_ID);
        assertThat(event.getThrowableProxy()).isNull();
    }

    @Test
    void logsExpectedClientErrorsAtInfoWithoutSensitiveData() throws Exception {
        for (int status : List.of(400, 401, 404)) {
            appender.list.clear();
            MockHttpServletRequest request = new MockHttpServletRequest("POST", "/login");
            request.setQueryString("password=secret-password");
            request.addHeader("Authorization", "Bearer secret-access-token");
            request.setCookies(new Cookie("refresh_token", "secret-refresh-token"));
            request.setContent("{\"password\":\"secret-password\"}".getBytes());
            MockHttpServletResponse response = new MockHttpServletResponse();

            filter(0L, 3_000_000L).doFilter(request, response,
                    (currentRequest, currentResponse) ->
                            ((MockHttpServletResponse) currentResponse).setStatus(status));

            ILoggingEvent event = singleEvent();
            assertThat(event.getLevel()).isEqualTo(Level.INFO);
            assertThat(event.getFormattedMessage())
                    .isEqualTo("http_request_completed method=POST path=/login status=" + status
                            + " duration_ms=3")
                    .doesNotContain("secret-password", "secret-access-token", "secret-refresh-token",
                            "Authorization");
        }
    }

    @Test
    void logsForbiddenAndRateLimitedAtWarn() throws Exception {
        for (int status : List.of(403, 429)) {
            appender.list.clear();
            MockHttpServletRequest request = new MockHttpServletRequest("GET", "/users/me");
            MockHttpServletResponse response = new MockHttpServletResponse();
            filter(0L, 1_000_000L).doFilter(request, response,
                    (currentRequest, currentResponse) -> ((MockHttpServletResponse) currentResponse).setStatus(status));
            assertThat(singleEvent().getLevel()).isEqualTo(Level.WARN);
        }
    }

    @Test
    void logsServerResponseAtErrorWithoutThrowable() throws Exception {
        MockHttpServletRequest request = new MockHttpServletRequest("POST", "/refresh");
        MockHttpServletResponse response = new MockHttpServletResponse();
        filter(0L, 8_000_000L).doFilter(request, response,
                (currentRequest, currentResponse) -> ((MockHttpServletResponse) currentResponse).setStatus(503));

        ILoggingEvent event = singleEvent();
        assertThat(event.getLevel()).isEqualTo(Level.ERROR);
        assertThat(event.getFormattedMessage()).contains("status=503", "duration_ms=8");
        assertThat(event.getThrowableProxy()).isNull();
    }

    @Test
    void summarizesEscapedExceptionAs500AndRethrowsIt() {
        MockHttpServletRequest request = new MockHttpServletRequest("GET", "/users/me");
        MockHttpServletResponse response = new MockHttpServletResponse();
        ServletException failure = new ServletException("database unavailable");

        assertThatThrownBy(() -> filter(0L, 5_000_000L).doFilter(request, response,
                (currentRequest, currentResponse) -> {
                    throw failure;
                })).isSameAs(failure);

        ILoggingEvent event = singleEvent();
        assertThat(event.getLevel()).isEqualTo(Level.ERROR);
        assertThat(event.getFormattedMessage()).contains("status=500", "duration_ms=5");
        assertThat(event.getThrowableProxy()).isNull();
    }

    private HttpRequestLoggingFilter filter(long... times) {
        AtomicInteger index = new AtomicInteger();
        LongSupplier nanoTime = () -> times[index.getAndIncrement()];
        return new HttpRequestLoggingFilter(nanoTime);
    }

    private ILoggingEvent singleEvent() {
        assertThat(appender.list).hasSize(1);
        return appender.list.getFirst();
    }
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `mvn -f services/identity-service/pom.xml -Dtest=HttpRequestLoggingFilterTest test`

Expected: compilation fails because `HttpRequestLoggingFilter` does not exist.

- [ ] **Step 3: Implement the filter**

```java
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
        } catch (IOException | ServletException | RuntimeException exception) {
            failed = true;
            throw exception;
        } finally {
            int status = failed ? HttpServletResponse.SC_INTERNAL_SERVER_ERROR : response.getStatus();
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
```

- [ ] **Step 4: Run the tests and verify GREEN**

Run: `mvn -f services/identity-service/pom.xml -Dtest=HttpRequestLoggingFilterTest test`

Expected: `Tests run: 5, Failures: 0, Errors: 0` and `BUILD SUCCESS`.

- [ ] **Step 5: Commit**

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/common/logging/HttpRequestLoggingFilter.java services/identity-service/src/test/java/com/ecommerce/identity/common/logging/HttpRequestLoggingFilterTest.java
git commit -m "feat(identity): log HTTP request summaries"
```

### Task 2: Log unexpected MVC exceptions exactly once

**Files:**
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java`
- Modify: `services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java`

- [ ] **Step 1: Write the failing handler tests**

```java
package com.ecommerce.identity.common.error;

import static org.assertj.core.api.Assertions.assertThat;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import com.ecommerce.identity.auth.domain.InvalidCredentialsException;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpStatus;

class GlobalExceptionHandlerTest {
    private final Logger logger = (Logger) LoggerFactory.getLogger(GlobalExceptionHandler.class);
    private final ListAppender<ILoggingEvent> appender = new ListAppender<>();

    @BeforeEach
    void captureLogs() {
        appender.start();
        logger.addAppender(appender);
    }

    @AfterEach
    void releaseLogs() {
        logger.detachAppender(appender);
        appender.stop();
    }

    @Test
    void logsUnexpectedFailureWithStackTrace() {
        GlobalExceptionHandler handler = new GlobalExceptionHandler();
        IllegalStateException failure = new IllegalStateException("database unavailable");

        var response = handler.handleUnexpectedFailure(failure);

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.INTERNAL_SERVER_ERROR);
        assertThat(appender.list).hasSize(1);
        ILoggingEvent event = appender.list.getFirst();
        assertThat(event.getLevel()).isEqualTo(Level.ERROR);
        assertThat(event.getFormattedMessage()).isEqualTo("unexpected_request_failure");
        assertThat(event.getThrowableProxy()).isNotNull();
        assertThat(event.getThrowableProxy().getMessage()).isEqualTo("database unavailable");
    }

    @Test
    void doesNotLogExpectedCredentialFailure() {
        GlobalExceptionHandler handler = new GlobalExceptionHandler();

        var response = handler.handleInvalidCredentials(new InvalidCredentialsException());

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.UNAUTHORIZED);
        assertThat(appender.list).isEmpty();
    }
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `mvn -f services/identity-service/pom.xml -Dtest=GlobalExceptionHandlerTest test`

Expected: `logsUnexpectedFailureWithStackTrace` fails because the handler emits no log event.

- [ ] **Step 3: Add single-owner exception logging**

Add the import and annotation, then change only the catch-all handler:

```java
import lombok.extern.slf4j.Slf4j;

@RestControllerAdvice
@Slf4j
public class GlobalExceptionHandler {
    // Keep every existing expected-exception handler unchanged.

    @ExceptionHandler(Exception.class)
    ResponseEntity<ApiError> handleUnexpectedFailure(Exception exception) {
        log.error("unexpected_request_failure", exception);
        return error(HttpStatus.INTERNAL_SERVER_ERROR, "INTERNAL_ERROR", "An unexpected error occurred");
    }
}
```

- [ ] **Step 4: Run the tests and verify GREEN**

Run: `mvn -f services/identity-service/pom.xml -Dtest=GlobalExceptionHandlerTest test`

Expected: `Tests run: 2, Failures: 0, Errors: 0` and `BUILD SUCCESS`.

- [ ] **Step 5: Commit**

```powershell
git add services/identity-service/src/main/java/com/ecommerce/identity/common/error/GlobalExceptionHandler.java services/identity-service/src/test/java/com/ecommerce/identity/common/error/GlobalExceptionHandlerTest.java
git commit -m "feat(identity): log unexpected request failures"
```

### Task 3: Render trace IDs in console logs

**Files:**
- Create: `services/identity-service/src/test/java/com/ecommerce/identity/common/logging/LoggingConfigurationTest.java`
- Modify: `services/identity-service/src/main/resources/application.yml`

- [ ] **Step 1: Write the failing configuration test**

```java
package com.ecommerce.identity.common.logging;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import org.junit.jupiter.api.Test;
import org.springframework.boot.env.YamlPropertySourceLoader;
import org.springframework.core.io.ClassPathResource;

class LoggingConfigurationTest {
    @Test
    void consolePatternIncludesTraceIdFromMdc() throws IOException {
        var sources = new YamlPropertySourceLoader()
                .load("application", new ClassPathResource("application.yml"));

        assertThat(sources)
                .anySatisfy(source -> assertThat(source.getProperty("logging.pattern.console"))
                        .asString()
                        .contains("trace_id=%X{trace_id:-none}"));
    }
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `mvn -f services/identity-service/pom.xml -Dtest=LoggingConfigurationTest test`

Expected: the assertion fails because `logging.pattern.console` is absent.

- [ ] **Step 3: Configure the console pattern**

Append this top-level YAML section:

```yaml
logging:
  pattern:
    console: "%d{yyyy-MM-dd'T'HH:mm:ss.SSSXXX} %-5level trace_id=%X{trace_id:-none} [%thread] %logger{36} - %msg%n"
```

- [ ] **Step 4: Run the focused logging suite**

Run:

```powershell
mvn -f services/identity-service/pom.xml -Dtest=LoggingConfigurationTest,HttpRequestLoggingFilterTest,GlobalExceptionHandlerTest test
```

Expected: `Tests run: 8, Failures: 0, Errors: 0` and `BUILD SUCCESS`.

- [ ] **Step 5: Commit**

```powershell
git add services/identity-service/src/main/resources/application.yml services/identity-service/src/test/java/com/ecommerce/identity/common/logging/LoggingConfigurationTest.java
git commit -m "feat(identity): include trace IDs in console logs"
```

### Task 4: Verify the complete Identity service

**Files:**
- Verify only; no additional files should change.

- [ ] **Step 1: Run all Identity tests**

Run: `mvn -f services/identity-service/pom.xml test`

Expected: all Identity unit and integration tests pass with `BUILD SUCCESS`.

- [ ] **Step 2: Check the implementation diff**

Run:

```powershell
git diff --check HEAD~3..HEAD
git status --short
```

Expected: `git diff --check` has no output. Status shows only changes that predate this implementation, such as the separate `AGENTS.md` edit.

- [ ] **Step 3: Verify terminal output manually**

Start infrastructure:

```powershell
docker compose -f deployments/docker/compose.foundation.yml up -d
```

Start Identity in another terminal:

```powershell
.\services\identity-service\start-local.ps1
```

Send an unauthenticated request:

```powershell
curl.exe -i http://localhost:8080/users/me
```

Expected: the response is `401` with `X-Trace-Id`, and the Identity terminal contains one line matching:

```text
INFO trace_id=<32-hex-character-id> ... http_request_completed method=GET path=/users/me status=401 duration_ms=<non-negative-number>
```

Do not commit files produced by local runtime tooling.

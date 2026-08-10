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

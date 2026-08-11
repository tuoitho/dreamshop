package com.ecommerce.identity.security;

import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;

import static org.assertj.core.api.Assertions.assertThat;

class CallerContextFilterTest {
    @Test
    void logsAcceptedCallerUserId() throws Exception {
        UUID userId = UUID.randomUUID();
        Logger logger = (Logger) LoggerFactory.getLogger(CallerContextFilter.class);
        ListAppender<ILoggingEvent> appender = new ListAppender<>();
        appender.start();
        logger.addAppender(appender);
        try {
            MockHttpServletRequest request = new MockHttpServletRequest("GET", "/users/me");
            request.addHeader("X-User-Id", userId.toString());

            new CallerContextFilter().doFilter(request, new MockHttpServletResponse(), (req, res) -> { });

            assertThat(appender.list).anySatisfy(event ->
                    assertThat(event.getFormattedMessage()).isEqualTo("caller_context_received user_id=" + userId));
        } finally {
            logger.detachAppender(appender);
        }
    }
}

package com.ecommerce.identity.common.error;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.Logger;
import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.core.read.ListAppender;
import com.ecommerce.identity.auth.domain.InvalidCredentialsException;
import jakarta.validation.ConstraintViolation;
import jakarta.validation.ConstraintViolationException;
import jakarta.validation.Validation;
import jakarta.validation.constraints.Min;
import java.util.Set;
import java.util.UUID;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

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

    @Test
    void mapsConstraintViolationsToBadRequestEnvelopeWithoutErrorLogs() throws Exception {
        MockMvc mockMvc = standaloneMockMvc();

        mockMvc.perform(get("/test/constraint"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.message").value("Request validation failed"))
                .andExpect(jsonPath("$.errors.page").value("must be greater than or equal to 1"));

        mockMvc.perform(get("/test/method-constraint"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.message").value("Request validation failed"))
                .andExpect(jsonPath("$.errors.page").value("must be greater than or equal to 1"));

        assertNoErrorLogs();
    }

    @Test
    void mapsHandlerMethodValidationAndTypeMismatchAndMissingParamToBadRequest() throws Exception {
        MockMvc mockMvc = standaloneMockMvc();

        mockMvc.perform(get("/test/items").param("page", "0").param("filter", "active"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.page").exists());

        mockMvc.perform(get("/test/items/not-a-uuid"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.id").value("Invalid value"));

        mockMvc.perform(get("/test/items"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.filter").exists());

        mockMvc.perform(get("/test/header"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors['X-Test-Header']").value("Header is required"));

        assertNoErrorLogs();
    }

    @Test
    void mapsStandardHttpProtocolClientErrorsWithoutInternalServerError() throws Exception {
        MockMvc mockMvc = standaloneMockMvc();

        mockMvc.perform(post("/test/items/00000000-0000-0000-0000-000000000001"))
                .andExpect(status().isMethodNotAllowed())
                .andExpect(jsonPath("$.code").value("METHOD_NOT_ALLOWED"))
                .andExpect(jsonPath("$.errors").doesNotExist());

        mockMvc.perform(post("/test/echo")
                        .contentType(MediaType.TEXT_PLAIN)
                        .content("plain text"))
                .andExpect(status().isUnsupportedMediaType())
                .andExpect(jsonPath("$.code").value("UNSUPPORTED_MEDIA_TYPE"))
                .andExpect(jsonPath("$.errors").doesNotExist());

        mockMvc.perform(get("/test/items/00000000-0000-0000-0000-000000000001")
                        .accept(MediaType.APPLICATION_XML))
                .andExpect(status().isNotAcceptable())
                .andExpect(jsonPath("$.code").value("NOT_ACCEPTABLE"))
                .andExpect(jsonPath("$.errors").doesNotExist());

        assertNoErrorLogs();
    }

    private void assertNoErrorLogs() {
        assertThat(appender.list)
                .filteredOn(event -> event.getLevel().isGreaterOrEqual(Level.ERROR))
                .isEmpty();
    }

    private static MockMvc standaloneMockMvc() {
        return MockMvcBuilders.standaloneSetup(new TestEndpoints())
                .setControllerAdvice(new GlobalExceptionHandler())
                .build();
    }

    record PageSample(@Min(1) int page) {
    }

    record EchoBody(String value) {
    }

    @RestController
    static class TestEndpoints {
        @GetMapping("/test/constraint")
        String constraint() {
            try (var factory = Validation.buildDefaultValidatorFactory()) {
                Set<ConstraintViolation<PageSample>> violations =
                        factory.getValidator().validate(new PageSample(0));
                throw new ConstraintViolationException(violations);
            }
        }

        @GetMapping("/test/method-constraint")
        String methodConstraint() throws NoSuchMethodException {
            try (var factory = Validation.buildDefaultValidatorFactory()) {
                var method = TestEndpoints.class.getDeclaredMethod("list", String.class, int.class);
                Set<ConstraintViolation<TestEndpoints>> violations = factory.getValidator()
                        .forExecutables()
                        .validateParameters(this, method, new Object[]{"active", 0});
                throw new ConstraintViolationException(violations);
            }
        }

        @GetMapping("/test/items")
        String list(
                @RequestParam("filter") String filter,
                @RequestParam(name = "page", defaultValue = "1") @Min(1) int page) {
            return filter + ":" + page;
        }

        @GetMapping(value = "/test/items/{id}", produces = MediaType.APPLICATION_JSON_VALUE)
        EchoBody getById(@PathVariable("id") UUID id) {
            return new EchoBody(id.toString());
        }

        @PostMapping(value = "/test/echo", consumes = MediaType.APPLICATION_JSON_VALUE)
        EchoBody echo(@RequestBody EchoBody body) {
            return body;
        }

        @GetMapping("/test/header")
        String header(@RequestHeader("X-Test-Header") String value) {
            return value;
        }
    }
}

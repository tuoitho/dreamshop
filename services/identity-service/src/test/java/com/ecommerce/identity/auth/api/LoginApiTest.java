package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.AccessTokenResult;
import com.ecommerce.identity.auth.domain.AuthenticationUnavailableException;
import com.ecommerce.identity.security.JwtTokenService;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

import static org.hamcrest.Matchers.containsString;
import static org.hamcrest.Matchers.matchesPattern;
import static org.hamcrest.Matchers.not;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@AutoConfigureMockMvc
class LoginApiTest extends PostgresIntegrationTestSupport {
    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @MockitoBean
    JwtTokenService jwtTokenService;

    @BeforeEach
    void resetDatabase() {
        jdbcTemplate.update("delete from user_roles");
        jdbcTemplate.update("delete from identity_profiles");
        jdbcTemplate.update("delete from identity_users");
        jdbcTemplate.update("delete from roles");
        jdbcTemplate.update("insert into roles (id, code) values (1, 'CUSTOMER')");
        reset(jwtTokenService);
    }

    @Test
    void authenticatesRegisteredCustomer() throws Exception {
        register("user@example.com", "correct horse battery staple");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("access-token", 900));

        mockMvc.perform(post("/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"user@example.com\",\"password\":\"correct horse battery staple\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.accessToken").isNotEmpty())
                .andExpect(jsonPath("$.data.tokenType").value("Bearer"))
                .andExpect(jsonPath("$.data.expiresIn").value(900))
                .andExpect(jsonPath("$.message").value("Login successful"));
    }

    @Test
    void authenticatesWhitespacePaddedEmail() throws Exception {
        register("user@example.com", "correct horse battery staple");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("access-token", 900));

        login("  user@example.com  ", "correct horse battery staple")
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.accessToken").value("access-token"));
    }

    @Test
    void rejectsUnknownEmailWithGenericCredentialsError() throws Exception {
        login("unknown@example.com", "correct horse battery staple")
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_CREDENTIALS"))
                .andExpect(jsonPath("$.message").value("Email or password is invalid."))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void rejectsWrongPasswordWithGenericCredentialsError() throws Exception {
        register("user@example.com", "correct horse battery staple");

        login("user@example.com", "wrong password")
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_CREDENTIALS"))
                .andExpect(jsonPath("$.message").value("Email or password is invalid."))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void rejectsDisabledAccountWithGenericCredentialsError() throws Exception {
        register("user@example.com", "correct horse battery staple");
        jdbcTemplate.update("update identity_users set status = 'DISABLED' where normalized_email = ?", "user@example.com");

        login("user@example.com", "correct horse battery staple")
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_CREDENTIALS"))
                .andExpect(jsonPath("$.message").value("Email or password is invalid."))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void returnsSafeServiceUnavailableErrorWhenTokenIssuanceFails() throws Exception {
        register("user@example.com", "correct horse battery staple");
        when(jwtTokenService.issue(any(), any())).thenThrow(new AuthenticationUnavailableException());

        login("user@example.com", "correct horse battery staple")
                .andExpect(status().isServiceUnavailable())
                .andExpect(jsonPath("$.code").value("AUTHENTICATION_UNAVAILABLE"))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")))
                .andExpect(content().string(not(containsString("private-key"))))
                .andExpect(content().string(not(containsString("PasswordEncoder"))));
    }

    private void register(String email, String password) throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"%s\",\"password\":\"%s\",\"fullName\":\"User\"}".formatted(email, password)))
                .andExpect(status().isCreated());
    }

    private org.springframework.test.web.servlet.ResultActions login(String email, String password) throws Exception {
        return mockMvc.perform(post("/login")
                .contentType(MediaType.APPLICATION_JSON)
                .content("{\"email\":\"%s\",\"password\":\"%s\"}".formatted(email, password)));
    }
}

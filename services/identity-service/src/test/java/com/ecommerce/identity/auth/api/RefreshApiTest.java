package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.AccessTokenResult;
import com.ecommerce.identity.security.JwtTokenService;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.rbac.support.RbacTestData;
import jakarta.servlet.http.Cookie;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.cookie;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@AutoConfigureMockMvc
class RefreshApiTest extends PostgresIntegrationTestSupport {
    private static final String COOKIE_NAME = "refresh_token";

    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @MockitoBean
    JwtTokenService jwtTokenService;

    @BeforeEach
    void resetDatabase() {
        RbacTestData.resetDatabase(jdbcTemplate);
        reset(jwtTokenService);
    }

    @Test
    void loginSetsHttpOnlyRefreshCookie() throws Exception {
        register("user@example.com");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("access-token", 900));

        mockMvc.perform(post("/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"user@example.com\",\"password\":\"correct horse battery staple\"}"))
                .andExpect(status().isOk())
                .andExpect(cookie().exists(COOKIE_NAME))
                .andExpect(cookie().httpOnly(COOKIE_NAME, true))
                .andExpect(cookie().secure(COOKIE_NAME, false))
                .andExpect(cookie().sameSite(COOKIE_NAME, "Lax"))
                .andExpect(cookie().path(COOKIE_NAME, "/api/v1/auth"));
    }

    @Test
    void refreshRotatesSessionAndIssuesNewJwt() throws Exception {
        String cookieValue = loginAndExtractCookie();
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("new-jwt", 900));

        mockMvc.perform(post("/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.accessToken").value("new-jwt"))
                .andExpect(jsonPath("$.data.tokenType").value("Bearer"))
                .andExpect(jsonPath("$.message").value("Token refreshed successfully"))
                .andExpect(cookie().exists(COOKIE_NAME))
                .andExpect(cookie().httpOnly(COOKIE_NAME, true))
                .andExpect(cookie().path(COOKIE_NAME, "/api/v1/auth"));
    }

    @Test
    void refreshRejectsMissingCookie() throws Exception {
        mockMvc.perform(post("/refresh"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_REFRESH_TOKEN"))
                .andExpect(jsonPath("$.message").value("Refresh token is invalid or expired"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0));
    }

    @Test
    void refreshRejectsExpiredCookie() throws Exception {
        String cookieValue = loginAndExtractCookie();
        jdbcTemplate.update("""
                update refresh_token_sessions
                set expires_at = now() - interval '1 hour'
                """);

        mockMvc.perform(post("/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_REFRESH_TOKEN"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0));
    }

    @Test
    void refreshRejectsReusedCookieAndClearsIt() throws Exception {
        String cookieValue = loginAndExtractCookie();
        mockMvc.perform(post("/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isOk());
        reset(jwtTokenService);

        mockMvc.perform(post("/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_REFRESH_TOKEN"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0));
    }

    @Test
    void logoutReturnsSuccessMessageAndClearsCookie() throws Exception {
        String cookieValue = loginAndExtractCookie();

        mockMvc.perform(post("/logout").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.message").value("Logout successful"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0))
                .andExpect(cookie().value(COOKIE_NAME, ""));

        mockMvc.perform(post("/logout"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.message").value("Logout successful"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0))
                .andExpect(cookie().value(COOKIE_NAME, ""));
    }

    private void register(String email) throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"%s\",\"password\":\"correct horse battery staple\",\"fullName\":\"User\"}"
                                .formatted(email)))
                .andExpect(status().isCreated());
    }

    private String loginAndExtractCookie() throws Exception {
        register("user@example.com");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("access-token", 900));
        MvcResult result = mockMvc.perform(post("/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"user@example.com\",\"password\":\"correct horse battery staple\"}"))
                .andExpect(status().isOk())
                .andReturn();
        String cookieValue = result.getResponse().getCookie(COOKIE_NAME).getValue();
        if (cookieValue == null || cookieValue.isBlank()) {
            throw new AssertionError("login did not set the refresh cookie");
        }
        return cookieValue;
    }
}

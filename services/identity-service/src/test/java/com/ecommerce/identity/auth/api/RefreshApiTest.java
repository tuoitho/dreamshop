package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.auth.application.AccessTokenResult;
import com.ecommerce.identity.security.jwt.JwtTokenService;
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
    private static final String ADMIN_COOKIE_NAME = "admin_refresh_token";
    private static final String ADMIN_COOKIE_PATH = "/api/v1/auth/admin";

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

        mockMvc.perform(post("/api/v1/auth/login")
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

        mockMvc.perform(post("/api/v1/auth/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
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
        mockMvc.perform(post("/api/v1/auth/refresh"))
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

        mockMvc.perform(post("/api/v1/auth/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_REFRESH_TOKEN"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0));
    }

    @Test
    void refreshRejectsReusedCookieAndClearsIt() throws Exception {
        String cookieValue = loginAndExtractCookie();
        mockMvc.perform(post("/api/v1/auth/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isOk());
        reset(jwtTokenService);

        mockMvc.perform(post("/api/v1/auth/refresh").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_REFRESH_TOKEN"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0));
    }

    @Test
    void logoutReturnsSuccessMessageAndClearsCookie() throws Exception {
        String cookieValue = loginAndExtractCookie();

        mockMvc.perform(post("/api/v1/auth/logout").cookie(new Cookie(COOKIE_NAME, cookieValue)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.message").value("Logout successful"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0))
                .andExpect(cookie().value(COOKIE_NAME, ""));

        mockMvc.perform(post("/api/v1/auth/logout"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.message").value("Logout successful"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0))
                .andExpect(cookie().value(COOKIE_NAME, ""));
    }

    @Test
    void adminLoginSetsScopedHttpOnlyRefreshCookie() throws Exception {
        register("admin-cookie@example.com");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("admin-access-token", 900));

        mockMvc.perform(post("/api/v1/auth/admin/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"admin-cookie@example.com\",\"password\":\"correct horse battery staple\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.accessToken").value("admin-access-token"))
                .andExpect(cookie().exists(ADMIN_COOKIE_NAME))
                .andExpect(cookie().httpOnly(ADMIN_COOKIE_NAME, true))
                .andExpect(cookie().secure(ADMIN_COOKIE_NAME, false))
                .andExpect(cookie().sameSite(ADMIN_COOKIE_NAME, "Lax"))
                .andExpect(cookie().path(ADMIN_COOKIE_NAME, ADMIN_COOKIE_PATH))
                .andExpect(cookie().doesNotExist(COOKIE_NAME));
    }

    @Test
    void adminAndStorefrontRefreshCookiesStayIsolated() throws Exception {
        register("customer-iso@example.com");
        register("admin-iso@example.com");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("token", 900));

        String storefrontCookie = mockMvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"customer-iso@example.com\",\"password\":\"correct horse battery staple\"}"))
                .andExpect(status().isOk())
                .andReturn()
                .getResponse()
                .getCookie(COOKIE_NAME)
                .getValue();

        String adminCookie = mockMvc.perform(post("/api/v1/auth/admin/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"admin-iso@example.com\",\"password\":\"correct horse battery staple\"}"))
                .andExpect(status().isOk())
                .andReturn()
                .getResponse()
                .getCookie(ADMIN_COOKIE_NAME)
                .getValue();

        mockMvc.perform(post("/api/v1/auth/admin/refresh").cookie(new Cookie(COOKIE_NAME, storefrontCookie)))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_REFRESH_TOKEN"))
                .andExpect(cookie().maxAge(ADMIN_COOKIE_NAME, 0))
                .andExpect(cookie().path(ADMIN_COOKIE_NAME, ADMIN_COOKIE_PATH))
                .andExpect(cookie().doesNotExist(COOKIE_NAME));

        mockMvc.perform(post("/api/v1/auth/refresh").cookie(new Cookie(ADMIN_COOKIE_NAME, adminCookie)))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("INVALID_REFRESH_TOKEN"))
                .andExpect(cookie().maxAge(COOKIE_NAME, 0))
                .andExpect(cookie().doesNotExist(ADMIN_COOKIE_NAME));

        String rotatedAdminCookie = mockMvc.perform(post("/api/v1/auth/admin/refresh")
                        .cookie(new Cookie(COOKIE_NAME, storefrontCookie), new Cookie(ADMIN_COOKIE_NAME, adminCookie)))
                .andExpect(status().isOk())
                .andExpect(cookie().exists(ADMIN_COOKIE_NAME))
                .andExpect(cookie().path(ADMIN_COOKIE_NAME, ADMIN_COOKIE_PATH))
                .andExpect(cookie().doesNotExist(COOKIE_NAME))
                .andReturn()
                .getResponse()
                .getCookie(ADMIN_COOKIE_NAME)
                .getValue();

        mockMvc.perform(post("/api/v1/auth/admin/logout")
                        .cookie(new Cookie(COOKIE_NAME, storefrontCookie), new Cookie(ADMIN_COOKIE_NAME, rotatedAdminCookie)))
                .andExpect(status().isOk())
                .andExpect(cookie().maxAge(ADMIN_COOKIE_NAME, 0))
                .andExpect(cookie().value(ADMIN_COOKIE_NAME, ""))
                .andExpect(cookie().path(ADMIN_COOKIE_NAME, ADMIN_COOKIE_PATH))
                .andExpect(cookie().doesNotExist(COOKIE_NAME));

        mockMvc.perform(post("/api/v1/auth/refresh").cookie(new Cookie(COOKIE_NAME, storefrontCookie)))
                .andExpect(status().isOk())
                .andExpect(cookie().exists(COOKIE_NAME))
                .andExpect(cookie().doesNotExist(ADMIN_COOKIE_NAME));
    }

    private void register(String email) throws Exception {
        mockMvc.perform(post("/api/v1/auth/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"%s\",\"password\":\"correct horse battery staple\",\"fullName\":\"User\"}"
                                .formatted(email)))
                .andExpect(status().isCreated());
    }

    private String loginAndExtractCookie() throws Exception {
        register("user@example.com");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("access-token", 900));
        MvcResult result = mockMvc.perform(post("/api/v1/auth/login")
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

package com.ecommerce.identity.api;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.cookie;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.auth.application.AccessTokenResult;
import com.ecommerce.identity.rbac.support.RbacTestData;
import com.ecommerce.identity.security.jwt.JwtTokenService;
import com.ecommerce.identity.support.IdentityContractSnapshotSupport;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import jakarta.servlet.http.Cookie;
import java.nio.file.Path;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;

@AutoConfigureMockMvc
class IdentityContractSnapshotApiTest extends PostgresIntegrationTestSupport {
    private static final String REFRESH_COOKIE = "refresh_token";
    private static final UUID SUPER_ADMIN_ID = UUID.fromString("00000000-0000-0000-0000-000000000091");
    private static final UUID CUSTOMER_ID = UUID.fromString("00000000-0000-0000-0000-000000000092");

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
    void locksIdentityAuthDomainSnapshots() throws Exception {
        Path authSnapshot = IdentityContractSnapshotSupport.resolveSnapshotPath("identity-auth.json");
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("e2e-mock-access-token", 900));

        String registerPayload = """
                {"email":"customer1@gmail.com","password":"123456123456","fullName":"Nguyễn Khách Hàng"}
                """.trim();
        MvcResult registerResult = mockMvc.perform(post("/api/v1/auth/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(registerPayload))
                .andExpect(status().isCreated())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "auth_register_201",
                "POST /api/v1/auth/register", registerPayload, registerResult);

        String customerUserId = jdbcTemplate.queryForObject(
                "select id::text from identity_users where email = 'customer1@gmail.com'",
                String.class);

        String loginPayload = """
                {"email":"customer1@gmail.com","password":"123456123456"}
                """.trim();
        MvcResult loginResult = mockMvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(loginPayload))
                .andExpect(status().isOk())
                .andExpect(cookie().exists(REFRESH_COOKIE))
                .andExpect(cookie().httpOnly(REFRESH_COOKIE, true))
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "auth_login_200",
                "POST /api/v1/auth/login", loginPayload, loginResult);

        String invalidLoginPayload = """
                {"email":"customer1@gmail.com","password":"wrong-password"}
                """.trim();
        MvcResult invalidLoginResult = mockMvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(invalidLoginPayload))
                .andExpect(status().isUnauthorized())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "auth_login_invalid_401",
                "POST /api/v1/auth/login", invalidLoginPayload, invalidLoginResult);

        Cookie refreshCookie = loginResult.getResponse().getCookie(REFRESH_COOKIE);
        when(jwtTokenService.issue(any(), any())).thenReturn(new AccessTokenResult("e2e-mock-refreshed-token", 900));
        MvcResult refreshResult = mockMvc.perform(post("/api/v1/auth/refresh").cookie(refreshCookie))
                .andExpect(status().isOk())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "auth_refresh_200",
                "POST /api/v1/auth/refresh", null, refreshResult);

        MvcResult refreshMissingResult = mockMvc.perform(post("/api/v1/auth/refresh"))
                .andExpect(status().isUnauthorized())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "auth_refresh_missing_401",
                "POST /api/v1/auth/refresh", null, refreshMissingResult);

        // Flat MeResponse object (unwrapped exception)
        MvcResult meResult = mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", customerUserId))
                .andExpect(status().isOk())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "users_me_200",
                "GET /api/v1/users/me", null, meResult);

        MvcResult unauthResult = mockMvc.perform(get("/api/v1/users/me"))
                .andExpect(status().isUnauthorized())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "unauthenticated_401",
                "GET /api/v1/users/me", null, unauthResult);

        MvcResult permissionsResult = mockMvc.perform(
                        get("/api/v1/users/me/permissions").header("X-User-Id", customerUserId))
                .andExpect(status().isOk())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "users_me_permissions_200",
                "GET /api/v1/users/me/permissions", null, permissionsResult);

        // Raw array exception (/locations/provinces)
        MvcResult provincesResult = mockMvc.perform(
                        get("/api/v1/locations/provinces").header("X-User-Id", customerUserId))
                .andExpect(status().isOk())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                authSnapshot, "identity-auth", "locations_provinces_200",
                "GET /api/v1/locations/provinces", null, provincesResult);
    }

    @Test
    void locksIdentityAdminDomainSnapshots() throws Exception {
        Path adminSnapshot = IdentityContractSnapshotSupport.resolveSnapshotPath("identity-admin.json");

        insertUser(SUPER_ADMIN_ID, "admin@example.com", "Super Admin");
        insertUser(CUSTOMER_ID, "customer1@gmail.com", "Customer One");
        assign(SUPER_ADMIN_ID, "SUPER_ADMIN");
        assign(CUSTOMER_ID, "CUSTOMER");

        MvcResult adminUsersResult = mockMvc.perform(get("/api/v1/admin/users")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .param("page", "1")
                        .param("pageSize", "20"))
                .andExpect(status().isOk())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                adminSnapshot, "identity-admin", "admin_users_list_200",
                "GET /api/v1/admin/users", null, adminUsersResult);

        MvcResult adminUsersForbidden = mockMvc.perform(get("/api/v1/admin/users")
                        .header("X-User-Id", CUSTOMER_ID.toString()))
                .andExpect(status().isForbidden())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                adminSnapshot, "identity-admin", "admin_users_forbidden_403",
                "GET /api/v1/admin/users", null, adminUsersForbidden);

        MvcResult rbacItemsResult = mockMvc.perform(get("/api/v1/admin/rbac/items")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk())
                .andReturn();
        IdentityContractSnapshotSupport.assertOrRecord(
                adminSnapshot, "identity-admin", "admin_rbac_items_list_200",
                "GET /api/v1/admin/rbac/items", null, rbacItemsResult);
    }

    private void insertUser(UUID id, String email, String fullName) {
        jdbcTemplate.update(
                "insert into identity_users (id, email, normalized_email, password_hash, status, created_at, updated_at) "
                        + "values (?, ?, ?, 'hash', 'ACTIVE', now(), now())",
                id, email, email);
        jdbcTemplate.update("insert into identity_profiles (user_id, full_name) values (?, ?)", id, fullName);
    }

    private void assign(UUID userId, String roleName) {
        jdbcTemplate.update(
                "insert into auth_assignments (user_id, item_name, created_at) values (?, ?, now())",
                userId, roleName);
    }
}

package com.ecommerce.identity.user.admin.api;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.patch;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.rbac.support.RbacTestData;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.web.servlet.MockMvc;

@AutoConfigureMockMvc
class AdminUserApiTest extends PostgresIntegrationTestSupport {
    private static final UUID ADMIN_ID = UUID.fromString("00000000-0000-0000-0000-000000000031");
    private static final UUID SUPER_ADMIN_ID = UUID.fromString("00000000-0000-0000-0000-000000000032");
    private static final UUID CUSTOMER_ID = UUID.fromString("00000000-0000-0000-0000-000000000033");

    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        RbacTestData.resetDatabase(jdbcTemplate);
        insertUser(ADMIN_ID, "admin-users@example.com", "Admin User");
        insertUser(SUPER_ADMIN_ID, "super-users@example.com", "Super User");
        insertUser(CUSTOMER_ID, "customer-users@example.com", "Customer User");
        assign(ADMIN_ID, "ADMIN");
        assign(SUPER_ADMIN_ID, "SUPER_ADMIN");
        assign(CUSTOMER_ID, "CUSTOMER");
    }

    @Test
    void listsUsersForCallerWithUserViewPermission() throws Exception {
        mockMvc.perform(get("/api/v1/admin/users")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .param("page", "1")
                        .param("pageSize", "20"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.items").isArray())
                .andExpect(jsonPath("$.data.page").value(1))
                .andExpect(jsonPath("$.data.pageSize").value(20))
                .andExpect(jsonPath("$.data.totalItems").value(3))
                .andExpect(jsonPath("$.data.totalPages").value(1))
                .andExpect(jsonPath("$.data.items[0].passwordHash").doesNotExist());
    }

    @Test
    void customerCannotListUsers() throws Exception {
        mockMvc.perform(get("/api/v1/admin/users")
                        .header("X-User-Id", CUSTOMER_ID.toString()))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.code").value("FORBIDDEN"));
    }

    @Test
    void adminCanDisableAndAssignNormalRole() throws Exception {
        mockMvc.perform(patch("/api/v1/admin/users/%s/status".formatted(CUSTOMER_ID))
                        .header("X-User-Id", ADMIN_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"status":"DISABLED"}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.status").value("DISABLED"));

        mockMvc.perform(put("/api/v1/admin/users/%s/roles/ADMIN".formatted(CUSTOMER_ID))
                        .header("X-User-Id", ADMIN_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.message").value("User role assigned."));
    }

    @Test
    void protectsSelfDisableAndSuperAdminEscalation() throws Exception {
        mockMvc.perform(patch("/api/v1/admin/users/%s/status".formatted(ADMIN_ID))
                        .header("X-User-Id", ADMIN_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"status":"DISABLED"}
                                """))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("SELF_DISABLE_NOT_ALLOWED"));

        mockMvc.perform(put("/api/v1/admin/users/%s/roles/SUPER_ADMIN".formatted(CUSTOMER_ID))
                        .header("X-User-Id", ADMIN_ID.toString()))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.code").value("FORBIDDEN"));
    }

    @Test
    void protectsTheLastSuperAdminAssignment() throws Exception {
        mockMvc.perform(delete("/api/v1/admin/users/%s/roles/SUPER_ADMIN".formatted(SUPER_ADMIN_ID))
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("LAST_SUPER_ADMIN_REQUIRED"));
    }

    @Test
    void mapsClientQueryPathMethodMediaTypeAndUnknownSubpathErrorsTo4xxEnvelope() throws Exception {
        mockMvc.perform(get("/api/v1/admin/users")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .param("page", "0"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.page").exists())
                .andExpect(jsonPath("$.traceId").isNotEmpty());

        mockMvc.perform(get("/api/v1/admin/users/not-a-uuid")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.userId").value("Invalid value"))
                .andExpect(jsonPath("$.traceId").isNotEmpty());

        mockMvc.perform(post("/api/v1/admin/users")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isMethodNotAllowed())
                .andExpect(jsonPath("$.code").value("METHOD_NOT_ALLOWED"))
                .andExpect(jsonPath("$.traceId").isNotEmpty());

        mockMvc.perform(patch("/api/v1/admin/users/%s/status".formatted(CUSTOMER_ID))
                        .header("X-User-Id", ADMIN_ID.toString())
                        .contentType(MediaType.TEXT_PLAIN)
                        .content("status=DISABLED"))
                .andExpect(status().isUnsupportedMediaType())
                .andExpect(jsonPath("$.code").value("UNSUPPORTED_MEDIA_TYPE"))
                .andExpect(jsonPath("$.traceId").isNotEmpty());

        mockMvc.perform(get("/api/v1/admin/users/%s/unknown".formatted(CUSTOMER_ID))
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("NOT_FOUND"))
                .andExpect(jsonPath("$.traceId").isNotEmpty());
    }

    private void insertUser(UUID id, String email, String fullName) {
        jdbcTemplate.update("""
                insert into identity_users
                    (id, email, normalized_email, password_hash, status, created_at, updated_at)
                values (?, ?, ?, 'hash', 'ACTIVE', now(), now())
                """, id, email, email);
        jdbcTemplate.update("insert into identity_profiles (user_id, full_name) values (?, ?)", id, fullName);
    }

    private void assign(UUID userId, String role) {
        jdbcTemplate.update("""
                insert into auth_assignments (user_id, item_name, created_at)
                values (?, ?, now())
                """, userId, role);
    }
}

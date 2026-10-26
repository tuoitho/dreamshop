package com.ecommerce.identity.rbac.api;

import static org.hamcrest.Matchers.containsString;
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
class RbacAdminApiTest extends PostgresIntegrationTestSupport {
    private static final UUID CUSTOMER_ID = UUID.fromString("00000000-0000-0000-0000-000000000021");
    private static final UUID SUPER_ADMIN_ID = UUID.fromString("00000000-0000-0000-0000-000000000022");

    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        RbacTestData.resetDatabase(jdbcTemplate);
        insertUser(CUSTOMER_ID, "customer-rbac@example.com");
        insertUser(SUPER_ADMIN_ID, "super-rbac@example.com");
        jdbcTemplate.update("insert into auth_assignments (user_id, item_name, created_at) values (?, 'CUSTOMER', now())",
                CUSTOMER_ID);
        jdbcTemplate.update("insert into auth_assignments (user_id, item_name, created_at) values (?, 'SUPER_ADMIN', now())",
                SUPER_ADMIN_ID);
    }

    @Test
    void customerCannotCreateRbacItems() throws Exception {
        mockMvc.perform(post("/api/v1/admin/rbac/items")
                        .header("X-User-Id", CUSTOMER_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name":"SUPPORT","type":"ROLE","description":"Support"}
                                """))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.code").value("FORBIDDEN"));
    }

    @Test
    void superAdminCreatesItemsAndHierarchy() throws Exception {
        mockMvc.perform(post("/api/v1/admin/rbac/items")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name":"SUPPORT","type":"ROLE","description":"Support"}
                                """))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.data.name").value("SUPPORT"));

        mockMvc.perform(post("/api/v1/admin/rbac/items")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name":"ticket.view","type":"PERMISSION","description":"View tickets"}
                                """))
                .andExpect(status().isCreated());

        mockMvc.perform(put("/api/v1/admin/rbac/items/SUPPORT/children/ticket.view")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data").value((Object) null));

        mockMvc.perform(get("/api/v1/admin/rbac/items/SUPPORT")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.directChildren[0]").value("ticket.view"));
    }

    @Test
    void rejectsHierarchyCycleAndProtectsBuiltInItems() throws Exception {
        createItem("SUPPORT", "ROLE");
        createItem("ticket.view", "PERMISSION");
        addChild("SUPPORT", "ticket.view");

        mockMvc.perform(put("/api/v1/admin/rbac/items/ticket.view/children/SUPPORT")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_HIERARCHY_CONFLICT"));

        mockMvc.perform(put("/api/v1/admin/rbac/items/SUPPORT/children/ticket.view")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk());

        mockMvc.perform(put("/api/v1/admin/rbac/items/CUSTOMER/children/address.create")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk());

        mockMvc.perform(patch("/api/v1/admin/rbac/items/CUSTOMER")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name":"RENAMED","description":"renamed"}
                                """))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_SYSTEM_ITEM_PROTECTED"))
                .andExpect(jsonPath("$.message").value(containsString("protected")));
    }

    private void createItem(String name, String type) throws Exception {
        mockMvc.perform(post("/api/v1/admin/rbac/items")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name":"%s","type":"%s","description":"test"}
                                """
                                .formatted(name, type)))
                .andExpect(status().isCreated());
    }

    private void addChild(String parent, String child) throws Exception {
        mockMvc.perform(put("/api/v1/admin/rbac/items/%s/children/%s".formatted(parent, child))
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk());
    }

    private void insertUser(UUID id, String email) {
        jdbcTemplate.update("""
                insert into identity_users
                    (id, email, normalized_email, password_hash, status, created_at, updated_at)
                values (?, ?, ?, 'hash', 'ACTIVE', now(), now())
                """, id, email, email);
    }
}

package com.ecommerce.identity.rbac.api;

import static org.hamcrest.Matchers.containsString;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.patch;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.rbac.support.RbacTestData;
import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.time.Duration;
import java.time.Instant;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import javax.sql.DataSource;
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

    @Autowired
    DataSource dataSource;

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
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.type").exists());

        mockMvc.perform(put("/api/v1/admin/rbac/items/SUPPORT/children/address.view")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data").value((Object) null));

        mockMvc.perform(get("/api/v1/admin/rbac/items/SUPPORT")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.directChildren[0]").value("address.view"));
    }

    @Test
    void rejectsHierarchyCycleAndProtectsBuiltInItems() throws Exception {
        createItem("SUPPORT", "ROLE");
        createItem("ESCALATION", "ROLE");
        addChild("SUPPORT", "ESCALATION");
        addChild("ESCALATION", "address.view");

        mockMvc.perform(put("/api/v1/admin/rbac/items/ESCALATION/children/SUPPORT")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_HIERARCHY_CONFLICT"));

        mockMvc.perform(put("/api/v1/admin/rbac/items/address.view/children/SUPPORT")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_HIERARCHY_CONFLICT"));

        mockMvc.perform(put("/api/v1/admin/rbac/items/address.view/children/address.create")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_HIERARCHY_CONFLICT"));

        mockMvc.perform(delete("/api/v1/admin/rbac/items/address.view/children/address.create")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_HIERARCHY_CONFLICT"));

        mockMvc.perform(patch("/api/v1/admin/rbac/items/address.view")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"name":"address.view","description":"mutated"}
                                """))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_SYSTEM_ITEM_PROTECTED"));

        mockMvc.perform(delete("/api/v1/admin/rbac/items/address.view")
                        .header("X-User-Id", SUPER_ADMIN_ID.toString()))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("RBAC_SYSTEM_ITEM_PROTECTED"));

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

    @Test
    void concurrentRenameCollisionReturns409RbacItemExistsInsteadOf500() throws Exception {
        createItem("SOURCE_ROLE", "ROLE");

        try (Connection competitor = dataSource.getConnection()) {
            competitor.setAutoCommit(false);
            try (PreparedStatement insertTarget = competitor.prepareStatement("""
                    insert into auth_items (name, type, description, system_item, created_at, updated_at)
                    values ('TARGET_ROLE', 'ROLE', 'Concurrent role', false, now(), now())
                    """)) {
                insertTarget.executeUpdate();
            }

            CompletableFuture<Void> renameRequest = CompletableFuture.runAsync(() -> {
                try {
                    mockMvc.perform(patch("/api/v1/admin/rbac/items/SOURCE_ROLE")
                                    .header("X-User-Id", SUPER_ADMIN_ID.toString())
                                    .contentType(MediaType.APPLICATION_JSON)
                                    .content("""
                                            {"name":"TARGET_ROLE","description":"renamed"}
                                            """))
                            .andExpect(status().isConflict())
                            .andExpect(jsonPath("$.code").value("RBAC_ITEM_EXISTS"));
                } catch (Exception exception) {
                    throw new RuntimeException(exception);
                }
            });

            waitForBlockedUpdate();
            competitor.commit();

            renameRequest.get(10, TimeUnit.SECONDS);
        }
    }

    private void waitForBlockedUpdate() throws InterruptedException {
        Instant deadline = Instant.now().plus(Duration.ofSeconds(10));
        while (Instant.now().isBefore(deadline)) {
            Integer waiting = jdbcTemplate.queryForObject("""
                    select count(*)
                    from pg_stat_activity
                    where wait_event_type = 'Lock'
                      and state = 'active'
                    """, Integer.class);
            if (waiting != null && waiting > 0) {
                return;
            }
            Thread.sleep(20);
        }
        throw new AssertionError("Concurrent RBAC rename did not block on unique constraint lock before timeout");
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

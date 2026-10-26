package com.ecommerce.identity.address.api;

import static org.hamcrest.Matchers.matchesPattern;
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
import org.springframework.test.web.servlet.MvcResult;

@AutoConfigureMockMvc
class ShippingAddressApiTest extends PostgresIntegrationTestSupport {
    private static final UUID USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000051");
    private static final UUID OTHER_USER_ID = UUID.fromString("00000000-0000-0000-0000-000000000052");

    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        RbacTestData.resetDatabase(jdbcTemplate);
        insertUser(USER_ID, "shipping@example.com", "Shipping User");
        insertUser(OTHER_USER_ID, "other-shipping@example.com", "Other User");
        assign(USER_ID, "CUSTOMER");
        assign(OTHER_USER_ID, "CUSTOMER");
    }

    @Test
    void createsListsUpdatesSetsDefaultAndDeletesOwnAddress() throws Exception {
        String addressId = createAddress(USER_ID.toString(), "Home");

        mockMvc.perform(get("/api/v1/users/me/addresses").header("X-User-Id", USER_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data[0].id").value(addressId))
                .andExpect(jsonPath("$.data[0].isDefault").value(true));

        mockMvc.perform(patch("/api/v1/users/me/addresses/%s".formatted(addressId))
                        .header("X-User-Id", USER_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"label":"Updated home","addressLine":"99 Le Loi"}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.label").value("Updated home"))
                .andExpect(jsonPath("$.data.addressLine").value("99 Le Loi"));

        mockMvc.perform(put("/api/v1/users/me/addresses/%s/default".formatted(addressId))
                        .header("X-User-Id", USER_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.isDefault").value(true));

        mockMvc.perform(delete("/api/v1/users/me/addresses/%s".formatted(addressId))
                        .header("X-User-Id", USER_ID.toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data").value((Object) null))
                .andExpect(jsonPath("$.message").value("Shipping address deleted."));
    }

    @Test
    void missingCreatePermissionReturnsForbidden() throws Exception {
        jdbcTemplate.update("""
                delete from auth_item_children
                where parent_name = 'CUSTOMER' and child_name = 'address.create'
                """);

        mockMvc.perform(post("/api/v1/users/me/addresses")
                        .header("X-User-Id", USER_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"recipientName":"Recipient","phone":"0901234567",
                                 "provinceCode":"79","wardCode":"26740",
                                 "addressLine":"12 Le Loi"}
                                """))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.code").value("FORBIDDEN"));
    }

    @Test
    void rejectsInvalidLocationAndMissingCaller() throws Exception {
        mockMvc.perform(post("/api/v1/users/me/addresses")
                        .header("X-User-Id", USER_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"recipientName":"Recipient","phone":"0901234567",
                                 "provinceCode":"79","wardCode":"00043",
                                 "addressLine":"12 Le Loi"}
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.wardCode").value("ward code does not belong to province code"));

        mockMvc.perform(get("/api/v1/users/me/addresses"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void addressIdFromAnotherUserIsNotEnumerable() throws Exception {
        String addressId = createAddress(USER_ID.toString(), "Private");

        mockMvc.perform(patch("/api/v1/users/me/addresses/%s".formatted(addressId))
                        .header("X-User-Id", OTHER_USER_ID.toString())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"label":"Should not update"}
                                """))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("ADDRESS_NOT_FOUND"));
    }

    private String createAddress(String userId, String label) throws Exception {
        MvcResult result = mockMvc.perform(post("/api/v1/users/me/addresses")
                        .header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "recipientName": "Recipient",
                                  "phone": "0901234567",
                                  "provinceCode": "79",
                                  "wardCode": "26740",
                                  "addressLine": "12 Le Loi",
                                  "label": "%s"
                                }
                                """.formatted(label)))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.message").value("Shipping address created."))
                .andReturn();
        return com.jayway.jsonpath.JsonPath.read(result.getResponse().getContentAsString(), "$.data.id");
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

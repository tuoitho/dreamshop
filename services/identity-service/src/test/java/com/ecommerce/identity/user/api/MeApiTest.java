package com.ecommerce.identity.user.api;

import static org.hamcrest.Matchers.matchesPattern;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.patch;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.jayway.jsonpath.JsonPath;
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
class MeApiTest extends PostgresIntegrationTestSupport {
    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        jdbcTemplate.update("delete from refresh_token_sessions");
        jdbcTemplate.update("delete from user_roles");
        jdbcTemplate.update("delete from identity_profiles");
        jdbcTemplate.update("delete from identity_users");
        jdbcTemplate.update("delete from roles");
        jdbcTemplate.update("insert into roles (id, code) values (1, 'CUSTOMER')");
    }

    @Test
    void returnsCallerProfileForVerifiedUserIdHeader() throws Exception {
        String userId = registerUser("me@example.com");

        mockMvc.perform(get("/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.user_id").value(userId))
                .andExpect(jsonPath("$.email").value("me@example.com"))
                .andExpect(jsonPath("$.full_name").value("Me User"))
                .andExpect(jsonPath("$.roles[0]").value("CUSTOMER"));
    }

    @Test
    void loadsRolesFromDatabaseNotFromForgedRolesHeader() throws Exception {
        String userId = registerUser("roles@example.com");

        mockMvc.perform(get("/users/me")
                        .header("X-User-Id", userId)
                        .header("X-User-Roles", "ADMIN"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.roles[0]").value("CUSTOMER"));
    }

    @Test
    void rejectsRequestWithoutCallerHeader() throws Exception {
        mockMvc.perform(get("/users/me"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"))
                .andExpect(jsonPath("$.trace_id", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void rejectsMalformedCallerHeader() throws Exception {
        mockMvc.perform(get("/users/me").header("X-User-Id", "not-a-uuid"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"));
    }

    @Test
    void returnsNotFoundForUnknownUser() throws Exception {
        mockMvc.perform(get("/users/me").header("X-User-Id", UUID.randomUUID().toString()))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("USER_NOT_FOUND"));
    }

    @Test
    void returnsNotFoundForDisabledAccount() throws Exception {
        String userId = registerUser("disabled@example.com");
        jdbcTemplate.update("update identity_users set status = 'DISABLED' where normalized_email = ?",
                "disabled@example.com");

        mockMvc.perform(get("/users/me").header("X-User-Id", userId))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("USER_NOT_FOUND"));
    }

    @Test
    void updatesCurrentUserFullName() throws Exception {
        String userId = registerUser("update@example.com");

        mockMvc.perform(patch("/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"full_name\":\"Updated Name\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.user_id").value(userId))
                .andExpect(jsonPath("$.full_name").value("Updated Name"));
    }

    @Test
    void rejectsBlankCurrentUserFullName() throws Exception {
        String userId = registerUser("blank@example.com");

        mockMvc.perform(patch("/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"full_name\":\"   \"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"));
    }

    private String registerUser(String email) throws Exception {
        MvcResult result = mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"%s\",\"password\":\"correct horse battery staple\",\"full_name\":\"Me User\"}"
                                .formatted(email)))
                .andExpect(status().isCreated())
                .andReturn();
        return JsonPath.read(result.getResponse().getContentAsString(), "$.user_id");
    }
}

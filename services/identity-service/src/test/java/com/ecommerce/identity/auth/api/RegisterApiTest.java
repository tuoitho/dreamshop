package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.web.servlet.MockMvc;

import static org.hamcrest.Matchers.matchesPattern;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@AutoConfigureMockMvc
class RegisterApiTest extends PostgresIntegrationTestSupport {
    @Autowired
    MockMvc mockMvc;

    @Autowired
    JdbcTemplate jdbcTemplate;

    @BeforeEach
    void resetDatabase() {
        jdbcTemplate.update("delete from user_roles");
        jdbcTemplate.update("delete from identity_profiles");
        jdbcTemplate.update("delete from identity_users");
        jdbcTemplate.update("delete from roles");
        jdbcTemplate.update("insert into roles (id, code) values (1, 'CUSTOMER')");
    }

    @Test
    void registersCustomerAndDoesNotReturnPassword() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"email":"User@Example.com","password":"correct horse battery staple","full_name":"Nguyen Van A"}
                                """))
                .andExpect(status().isCreated())
                .andExpect(header().string("X-Trace-Id", matchesPattern("[0-9a-f]{32}")))
                .andExpect(jsonPath("$.user_id").exists())
                .andExpect(jsonPath("$.email").value("user@example.com"))
                .andExpect(jsonPath("$.full_name").value("Nguyen Van A"))
                .andExpect(jsonPath("$.roles[0]").value("CUSTOMER"))
                .andExpect(jsonPath("$.password").doesNotExist());
    }

    @Test
    void trimsEmailAndFullNameBeforeValidationAndPersistence() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"email":"  User@Example.com  ","password":"correct horse battery staple","full_name":"  Nguyen Van A  "}
                                """))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.email").value("user@example.com"))
                .andExpect(jsonPath("$.full_name").value("Nguyen Van A"));

        org.assertj.core.api.Assertions.assertThat(jdbcTemplate.queryForObject(
                "select full_name from identity_profiles", String.class)).isEqualTo("Nguyen Van A");
    }

    @Test
    void rejectsInvalidRegistrationWithSafeValidationError() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"email":"not-an-email","password":"short","full_name":""}
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.message").value("Request validation failed"))
                .andExpect(jsonPath("$.errors.email").value("must be a well-formed email address"))
                .andExpect(jsonPath("$.errors.password").value("size must be between 12 and 128"))
                .andExpect(jsonPath("$.errors.fullName").value("must not be blank"))
                .andExpect(jsonPath("$.trace_id", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void rejectsMalformedBodyWithoutFieldErrors() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\""))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors").doesNotExist());
    }

    @Test
    void rejectsDuplicateRegistrationWithoutLeakingPersistenceDetails() throws Exception {
        String request = """
                {"email":"duplicate@example.com","password":"correct horse battery staple","full_name":"Nguyen Van A"}
                """;
        mockMvc.perform(post("/register").contentType(MediaType.APPLICATION_JSON).content(request))
                .andExpect(status().isCreated());

        mockMvc.perform(post("/register").contentType(MediaType.APPLICATION_JSON).content(request))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("EMAIL_ALREADY_REGISTERED"))
                .andExpect(jsonPath("$.message").value("Email is already registered"))
                .andExpect(jsonPath("$.trace_id", matchesPattern("[0-9a-f]{32}")))
                .andExpect(content().string(org.hamcrest.Matchers.not(org.hamcrest.Matchers.containsString("SQLException"))))
                .andExpect(content().string(org.hamcrest.Matchers.not(org.hamcrest.Matchers.containsString("EmailAlreadyRegisteredException"))));
    }

    @Test
    void permitsPublicEndpointsButDeniesUnlistedEndpoints() throws Exception {
        mockMvc.perform(get("/actuator/health"))
                .andExpect(status().isOk());

        mockMvc.perform(get("/private"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"))
                .andExpect(jsonPath("$.message").value("Missing or invalid access token"))
                .andExpect(jsonPath("$.trace_id", matchesPattern("[0-9a-f]{32}")));

        mockMvc.perform(post("/private"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"))
                .andExpect(jsonPath("$.message").value("Missing or invalid access token"))
                .andExpect(jsonPath("$.trace_id", matchesPattern("[0-9a-f]{32}")));

        mockMvc.perform(get("/.well-known/jwks.json"))
                .andExpect(status().isOk());
    }
}

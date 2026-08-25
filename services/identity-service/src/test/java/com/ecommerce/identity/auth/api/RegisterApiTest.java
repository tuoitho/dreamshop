package com.ecommerce.identity.auth.api;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.rbac.support.RbacTestData;
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
        RbacTestData.resetDatabase(jdbcTemplate);
    }

    @Test
    void registersCustomerAndDoesNotReturnPassword() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"email":"User@Example.com","password":"correct horse battery staple","fullName":"Nguyen Van A"}
                                """))
                .andExpect(status().isCreated())
                .andExpect(header().string("X-Trace-Id", matchesPattern("[0-9a-f]{32}")))
                .andExpect(jsonPath("$.data.userId").exists())
                .andExpect(jsonPath("$.data.email").value("user@example.com"))
                .andExpect(jsonPath("$.data.fullName").value("Nguyen Van A"))
                .andExpect(jsonPath("$.data.roles[0]").value("CUSTOMER"))
                .andExpect(jsonPath("$.message").value("Registration successful"))
                .andExpect(jsonPath("$.password").doesNotExist());
    }

    @Test
    void trimsEmailAndFullNameBeforeValidationAndPersistence() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"email":"  User@Example.com  ","password":"correct horse battery staple","fullName":"  Nguyen Van A  "}
                                """))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.data.email").value("user@example.com"))
                .andExpect(jsonPath("$.data.fullName").value("Nguyen Van A"));

        org.assertj.core.api.Assertions.assertThat(jdbcTemplate.queryForObject(
                "select full_name from identity_profiles", String.class)).isEqualTo("Nguyen Van A");
    }

    @Test
    void rejectsInvalidRegistrationWithSafeValidationError() throws Exception {
        mockMvc.perform(post("/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"email":"not-an-email","password":"short","fullName":""}
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.message").value("Request validation failed"))
                .andExpect(jsonPath("$.errors.email").value("must be a well-formed email address"))
                .andExpect(jsonPath("$.errors.password").value("size must be between 12 and 128"))
                .andExpect(jsonPath("$.errors.fullName").value("must not be blank"))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));
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
                {"email":"duplicate@example.com","password":"correct horse battery staple","fullName":"Nguyen Van A"}
                """;
        mockMvc.perform(post("/register").contentType(MediaType.APPLICATION_JSON).content(request))
                .andExpect(status().isCreated());

        mockMvc.perform(post("/register").contentType(MediaType.APPLICATION_JSON).content(request))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("EMAIL_ALREADY_REGISTERED"))
                .andExpect(jsonPath("$.message").value("Email is already registered"))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")))
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
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));

        mockMvc.perform(post("/private"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"))
                .andExpect(jsonPath("$.message").value("Missing or invalid access token"))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));

        mockMvc.perform(get("/.well-known/jwks.json"))
                .andExpect(status().isOk());
    }
}

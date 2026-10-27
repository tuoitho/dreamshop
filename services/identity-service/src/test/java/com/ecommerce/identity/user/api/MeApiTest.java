package com.ecommerce.identity.user.api;

import static org.hamcrest.Matchers.matchesPattern;
import static org.hamcrest.Matchers.nullValue;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.patch;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import com.ecommerce.identity.rbac.support.RbacTestData;
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
        RbacTestData.resetDatabase(jdbcTemplate);
    }

    @Test
    void returnsCallerProfileForVerifiedUserIdHeader() throws Exception {
        String userId = registerUser("me@example.com");

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.userId").value(userId))
                .andExpect(jsonPath("$.email").value("me@example.com"))
                .andExpect(jsonPath("$.fullName").value("Me User"))
                .andExpect(jsonPath("$.roles[0]").value("CUSTOMER"));
    }

    @Test
    void loadsRolesFromDatabaseNotFromForgedRolesHeader() throws Exception {
        String userId = registerUser("roles@example.com");

        mockMvc.perform(get("/api/v1/users/me")
                        .header("X-User-Id", userId)
                        .header("X-User-Roles", "ADMIN"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.roles[0]").value("CUSTOMER"));
    }

    @Test
    void rejectsRequestWithoutCallerHeader() throws Exception {
        mockMvc.perform(get("/api/v1/users/me"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"))
                .andExpect(jsonPath("$.traceId", matchesPattern("[0-9a-f]{32}")));
    }

    @Test
    void rejectsMalformedCallerHeader() throws Exception {
        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", "not-a-uuid"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"));
    }

    @Test
    void returnsNotFoundForUnknownUser() throws Exception {
        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", UUID.randomUUID().toString()))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("USER_NOT_FOUND"));
    }

    @Test
    void returnsNotFoundForDisabledAccount() throws Exception {
        String userId = registerUser("disabled@example.com");
        jdbcTemplate.update("update identity_users set status = 'DISABLED' where normalized_email = ?",
                "disabled@example.com");

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("USER_NOT_FOUND"));
    }

    @Test
    void returnsEffectivePermissionsForActiveCallerAndRejectsUnauthenticatedOrDisabledCaller() throws Exception {
        String userId = registerUser("perms@example.com");
        jdbcTemplate.update("insert into auth_assignments (user_id, item_name, created_at) values (?, 'ADMIN', now())",
                UUID.fromString(userId));

        mockMvc.perform(get("/api/v1/users/me/permissions").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.message").value("Permissions retrieved."))
                .andExpect(jsonPath("$.data[?(@ == 'product.view')]").isNotEmpty())
                .andExpect(jsonPath("$.data[?(@ == 'inventory.update')]").isNotEmpty())
                .andExpect(jsonPath("$.data[?(@ == 'favorite.manage')]").isNotEmpty());

        mockMvc.perform(get("/api/v1/users/me/permissions"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"));

        jdbcTemplate.update("update identity_users set status = 'DISABLED' where id = ?", UUID.fromString(userId));
        mockMvc.perform(get("/api/v1/users/me/permissions").header("X-User-Id", userId))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("USER_NOT_FOUND"));
    }

    @Test
    void updatesCurrentUserFullName() throws Exception {
        String userId = registerUser("update@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Updated Name\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.userId").value(userId))
                .andExpect(jsonPath("$.data.fullName").value("Updated Name"))
                .andExpect(jsonPath("$.message").value("Profile updated successfully"));
    }

    @Test
    void rejectsBlankCurrentUserFullName() throws Exception {
        String userId = registerUser("blank@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"   \"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"));
    }

    @Test
    void acceptsPatchWithOnlyPhoneAndPreservesStoredFullName() throws Exception {
        String userId = registerUser("phone-only@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"phone\":\"0912345678\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.fullName").value("Me User"))
                .andExpect(jsonPath("$.data.phone").value("0912345678"));

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.fullName").value("Me User"))
                .andExpect(jsonPath("$.phone").value("0912345678"));
    }

    @Test
    void acceptsPatchWithOnlyLocationPairAndPreservesStoredFullName() throws Exception {
        String userId = registerUser("location-only@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"provinceCode\":\"79\",\"wardCode\":\"26740\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.fullName").value("Me User"))
                .andExpect(jsonPath("$.data.provinceCode").value("79"))
                .andExpect(jsonPath("$.data.wardCode").value("26740"));

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.fullName").value("Me User"))
                .andExpect(jsonPath("$.provinceCode").value("79"))
                .andExpect(jsonPath("$.wardCode").value("26740"));
    }

    @Test
    void rejectsExplicitNullFullName() throws Exception {
        String userId = registerUser("null-name@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":null}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"));
    }

    @Test
    void returnsNullableProfileFieldsInGetResponse() throws Exception {
        String userId = registerUser("fields@example.com");

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.phone").value(nullValue()))
                .andExpect(jsonPath("$.dateOfBirth").value(nullValue()))
                .andExpect(jsonPath("$.gender").value(nullValue()))
                .andExpect(jsonPath("$.provinceCode").value(nullValue()))
                .andExpect(jsonPath("$.wardCode").value(nullValue()))
                .andExpect(jsonPath("$.addressLine").value(nullValue()));
    }

    @Test
    void updatesAllProfileFieldsAndPersistsThem() throws Exception {
        String userId = registerUser("all-fields@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "fullName": "Updated Name",
                                  "phone": "0901234567",
                                  "dateOfBirth": "1995-04-20",
                                  "gender": "MALE",
                                  "provinceCode": "79",
                                  "wardCode": "26740",
                                  "addressLine": "12 Le Loi"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.userId").value(userId))
                .andExpect(jsonPath("$.data.fullName").value("Updated Name"))
                .andExpect(jsonPath("$.data.phone").value("0901234567"))
                .andExpect(jsonPath("$.data.dateOfBirth").value("1995-04-20"))
                .andExpect(jsonPath("$.data.gender").value("MALE"))
                .andExpect(jsonPath("$.data.provinceCode").value("79"))
                .andExpect(jsonPath("$.data.wardCode").value("26740"))
                .andExpect(jsonPath("$.data.addressLine").value("12 Le Loi"));

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.phone").value("0901234567"))
                .andExpect(jsonPath("$.dateOfBirth").value("1995-04-20"))
                .andExpect(jsonPath("$.gender").value("MALE"))
                .andExpect(jsonPath("$.provinceCode").value("79"))
                .andExpect(jsonPath("$.wardCode").value("26740"))
                .andExpect(jsonPath("$.addressLine").value("12 Le Loi"));
    }

    @Test
    void preservesOmittedLocationCodes() throws Exception {
        String userId = registerUser("omit@example.com");
        patchProfile(userId, """
                {"fullName":"Original Name","phone":"0901234567","provinceCode":"79","wardCode":"26740"}
                """);

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Renamed\",\"phone\":\"0912345678\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.fullName").value("Renamed"))
                .andExpect(jsonPath("$.data.phone").value("0912345678"))
                .andExpect(jsonPath("$.data.provinceCode").value("79"))
                .andExpect(jsonPath("$.data.wardCode").value("26740"));
    }

    @Test
    void clearsOptionalStringFieldWithEmptyString() throws Exception {
        String userId = registerUser("clear@example.com");
        patchProfile(userId, """
                {"fullName":"Original Name","phone":"0901234567","provinceCode":"79","wardCode":"26740"}
                """);

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Kept Name\",\"phone\":\"\",\"provinceCode\":\"\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.phone").value(nullValue()))
                .andExpect(jsonPath("$.data.provinceCode").value(nullValue()))
                .andExpect(jsonPath("$.data.wardCode").value(nullValue()));

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.phone").value(nullValue()))
                .andExpect(jsonPath("$.provinceCode").value(nullValue()))
                .andExpect(jsonPath("$.wardCode").value(nullValue()));
    }

    @Test
    void clearsDateOfBirthWithEmptyString() throws Exception {
        String userId = registerUser("clear-dob@example.com");
        patchProfile(userId, "{\"fullName\":\"Original Name\",\"dateOfBirth\":\"1995-04-20\"}");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Kept Name\",\"dateOfBirth\":\"\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.dateOfBirth").value(nullValue()));

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.dateOfBirth").value(nullValue()));
    }

    @Test
    void clearsGenderWithEmptyString() throws Exception {
        String userId = registerUser("clear-gender@example.com");
        patchProfile(userId, "{\"fullName\":\"Original Name\",\"gender\":\"MALE\"}");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Kept Name\",\"gender\":\"\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.gender").value(nullValue()));

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", userId))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.gender").value(nullValue()));
    }

    @Test
    void rejectsInvalidVietnameseMobileNumber() throws Exception {
        String userId = registerUser("bad-phone@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"phone\":\"0123456789\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.phone").value("must be a valid Vietnamese mobile number"));
    }

    @Test
    void rejectsFutureDateOfBirth() throws Exception {
        String userId = registerUser("future-dob@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"dateOfBirth\":\"2999-01-01\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.dateOfBirth").value("must be a past date"));
    }

    @Test
    void rejectsUnknownGender() throws Exception {
        String userId = registerUser("unknown-gender@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"gender\":\"ALIEN\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.gender").value("must be one of MALE, FEMALE, OTHER"));
    }

    @Test
    void clearsBothLocationCodesWhenOnlyProvinceIsEmpty() throws Exception {
        String userId = registerUser("clear-province@example.com");
        patchProfile(userId, "{\"fullName\":\"Original Name\",\"provinceCode\":\"79\",\"wardCode\":\"26740\"}");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Kept Name\",\"provinceCode\":\"\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.provinceCode").value(nullValue()))
                .andExpect(jsonPath("$.data.wardCode").value(nullValue()));
    }

    @Test
    void clearsBothLocationCodesWhenOnlyWardIsEmpty() throws Exception {
        String userId = registerUser("clear-ward@example.com");
        patchProfile(userId, "{\"fullName\":\"Original Name\",\"provinceCode\":\"79\",\"wardCode\":\"26740\"}");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Kept Name\",\"wardCode\":\"\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.data.provinceCode").value(nullValue()))
                .andExpect(jsonPath("$.data.wardCode").value(nullValue()));
    }

    @Test
    void rejectsProvinceCodeWithoutWardCode() throws Exception {
        String userId = registerUser("province-only@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"provinceCode\":\"79\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.wardCode").value("must be supplied together with provinceCode"));
    }

    @Test
    void rejectsWardCodeWithoutProvinceCode() throws Exception {
        String userId = registerUser("ward-only@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"wardCode\":\"26740\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.provinceCode").value("must be supplied together with wardCode"));
    }

    @Test
    void rejectsWardCodeNotBelongingToProvince() throws Exception {
        String userId = registerUser("mismatched@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"provinceCode\":\"79\",\"wardCode\":\"00043\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.wardCode").value("ward code does not belong to province code"));
    }

    @Test
    void rejectsUnknownProvinceCode() throws Exception {
        String userId = registerUser("unknown-province@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"provinceCode\":\"999\",\"wardCode\":\"26740\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.provinceCode").value("unknown province code"));
    }

    @Test
    void rejectsUnknownWardCode() throws Exception {
        String userId = registerUser("unknown-ward@example.com");

        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"fullName\":\"Me User\",\"provinceCode\":\"79\",\"wardCode\":\"99999\"}"))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value("VALIDATION_FAILED"))
                .andExpect(jsonPath("$.errors.wardCode").value("ward code does not belong to province code"));
    }

    private void patchProfile(String userId, String json) throws Exception {
        mockMvc.perform(patch("/api/v1/users/me").header("X-User-Id", userId)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(json))
                .andExpect(status().isOk());
    }

    private String registerUser(String email) throws Exception {
        MvcResult result = mockMvc.perform(post("/api/v1/auth/register")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"email\":\"%s\",\"password\":\"correct horse battery staple\",\"fullName\":\"Me User\"}"
                                .formatted(email)))
                .andExpect(status().isCreated())
                .andReturn();
        return JsonPath.read(result.getResponse().getContentAsString(), "$.data.userId");
    }
}

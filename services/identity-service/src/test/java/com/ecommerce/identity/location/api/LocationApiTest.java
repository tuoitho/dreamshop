package com.ecommerce.identity.location.api;

import static org.hamcrest.Matchers.greaterThan;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.ecommerce.identity.support.PostgresIntegrationTestSupport;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.test.web.servlet.MockMvc;

@AutoConfigureMockMvc
class LocationApiTest extends PostgresIntegrationTestSupport {
    @Autowired
    MockMvc mockMvc;

    @Test
    void returnsSnapshotProvinces() throws Exception {
        mockMvc.perform(get("/api/v1/locations/provinces").header("X-User-Id", UUID.randomUUID().toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(34))
                .andExpect(jsonPath("$[0].code").value("79"))
                .andExpect(jsonPath("$[0].name").value("Thành phố Hồ Chí Minh"));
    }

    @Test
    void returnsWardsForKnownProvince() throws Exception {
        mockMvc.perform(get("/api/v1/locations/provinces/79/wards").header("X-User-Id", UUID.randomUUID().toString()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(greaterThan(0)))
                .andExpect(jsonPath("$[0].code").value("26734"))
                .andExpect(jsonPath("$[0].name").value("Phường Tân Định"));
    }

    @Test
    void returnsNotFoundForUnknownProvince() throws Exception {
        mockMvc.perform(get("/api/v1/locations/provinces/999/wards").header("X-User-Id", UUID.randomUUID().toString()))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("NOT_FOUND"));
    }

    @Test
    void requiresCallerContext() throws Exception {
        mockMvc.perform(get("/api/v1/locations/provinces"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value("UNAUTHORIZED"));
    }
}
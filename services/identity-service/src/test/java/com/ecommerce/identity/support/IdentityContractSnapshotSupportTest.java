package com.ecommerce.identity.support;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.test.web.servlet.MvcResult;

class IdentityContractSnapshotSupportTest {

    @Test
    void failsOnMissingFileMissingKeyAndShapeDriftWithoutUpdateFlag(@TempDir Path tempDir) throws Exception {
        Path missingPath = tempDir.resolve("missing.json");
        MvcResult okResult = mvcResult(200, "{\"data\":{\"token\":\"abc\"},\"message\":\"ok\"}");

        AssertionError missingFileErr = assertThrows(AssertionError.class, () ->
                IdentityContractSnapshotSupport.assertOrRecord(
                        missingPath, "identity-auth", "auth_login_200", "POST /api/v1/auth/login",
                        "{\"email\":\"a@b.com\",\"password\":\"secret\"}", okResult));
        assertTrue(missingFileErr.getMessage().contains("UPDATE_CONTRACT_SNAPSHOTS=1"));
        assertFalse(Files.exists(missingPath), "Must not create snapshot file without UPDATE_CONTRACT_SNAPSHOTS=1");

        Path validPath = tempDir.resolve("identity-auth.json");
        Files.writeString(validPath, """
                {
                  "domain": "identity-auth",
                  "snapshots": {
                    "auth_login_200": {
                      "service": "identity-service",
                      "endpoint": "POST /api/v1/auth/login",
                      "status": 200,
                      "requestBody": {"email": "a@b.com", "password": "secret"},
                      "responseBody": {"data": {"token": "abc"}, "message": "ok"}
                    }
                  }
                }
                """);

        assertThrows(AssertionError.class, () ->
                IdentityContractSnapshotSupport.assertOrRecord(
                        validPath, "identity-auth", "renamed_key", "POST /api/v1/auth/login",
                        "{\"email\":\"a@b.com\",\"password\":\"secret\"}", okResult));

        MvcResult driftedResult = mvcResult(200, "{\"data\":{\"token\":123,\"extra\":true},\"message\":\"ok\"}");
        assertThrows(AssertionError.class, () ->
                IdentityContractSnapshotSupport.assertOrRecord(
                        validPath, "identity-auth", "auth_login_200", "POST /api/v1/auth/login",
                        "{\"email\":\"a@b.com\",\"password\":\"secret\"}", driftedResult));

        assertDoesNotThrow(() ->
                IdentityContractSnapshotSupport.assertOrRecord(
                        validPath, "identity-auth", "auth_login_200", "POST /api/v1/auth/login",
                        "{\"email\":\"a@b.com\",\"password\":\"secret\"}", okResult));
    }

    private static MvcResult mvcResult(int status, String body) throws Exception {
        MockHttpServletResponse response = new MockHttpServletResponse();
        response.setStatus(status);
        response.getWriter().write(body);
        return (MvcResult) java.lang.reflect.Proxy.newProxyInstance(
                MvcResult.class.getClassLoader(),
                new Class<?>[]{MvcResult.class},
                (proxy, method, args) -> "getResponse".equals(method.getName()) ? response : null
        );
    }
}

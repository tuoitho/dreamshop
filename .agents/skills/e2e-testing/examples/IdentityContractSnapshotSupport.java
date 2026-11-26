package com.ecommerce.identity.support;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import org.springframework.test.web.servlet.MvcResult;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;
import tools.jackson.databind.SerializationFeature;
import tools.jackson.databind.json.JsonMapper;
import tools.jackson.databind.node.ArrayNode;
import tools.jackson.databind.node.ObjectNode;

/**
 * Backend contract snapshot recorder and shape verifier for Spring MockMvc tests in identity-service.
 *
 * Uses Jackson 3 (tools.jackson.databind.*) matching Spring Boot 4 and shares the exact JSON format
 * with Go bff-service snapshot tests so Playwright Mode 2 can merge domain snapshot files under
 * contracts/http-snapshots/.
 */
public final class IdentityContractSnapshotSupport {
    private static final ObjectMapper MAPPER = JsonMapper.builder()
            .enable(SerializationFeature.INDENT_OUTPUT)
            .build();

    private IdentityContractSnapshotSupport() {}

    public static Path resolveSnapshotPath(String fileName) {
        String override = System.getenv("CONTRACT_SNAPSHOT_PATH");
        if (override != null && !override.isBlank()) {
            return Path.of(override).toAbsolutePath().normalize();
        }
        Path candidateFromModule = Path.of("..", "..", "contracts", "http-snapshots", fileName)
                .toAbsolutePath()
                .normalize();
        if (Files.isDirectory(Path.of("..", "..", "contracts"))) {
            return candidateFromModule;
        }
        return Path.of("contracts", "http-snapshots", fileName).toAbsolutePath().normalize();
    }

    public static void assertOrRecord(
            Path snapshotFilePath,
            String domain,
            String snapshotKey,
            String endpoint,
            String requestBodyJson,
            MvcResult mvcResult
    ) throws IOException {
        int status = mvcResult.getResponse().getStatus();
        String rawResponse = mvcResult.getResponse().getContentAsString();
        JsonNode actualResponse = rawResponse.isBlank() ? MAPPER.nullNode() : MAPPER.readTree(rawResponse);
        JsonNode actualRequest = (requestBodyJson == null || requestBodyJson.isBlank())
                ? null
                : MAPPER.readTree(requestBodyJson);

        boolean updateMode = "1".equals(System.getenv("UPDATE_CONTRACT_SNAPSHOTS"));
        boolean fileExists = Files.exists(snapshotFilePath);

        if (!updateMode && !fileExists) {
            throw new AssertionError("Missing contract snapshot file " + snapshotFilePath
                    + " for key " + snapshotKey + " (run with UPDATE_CONTRACT_SNAPSHOTS=1 to record)");
        }

        ObjectNode doc;
        if (fileExists) {
            doc = (ObjectNode) MAPPER.readTree(Files.readString(snapshotFilePath));
        } else {
            doc = MAPPER.createObjectNode();
            doc.put("domain", domain);
            doc.set("snapshots", MAPPER.createObjectNode());
        }

        if (!doc.has("domain")) {
            doc.put("domain", domain);
        }
        ObjectNode snapshots = doc.withObjectProperty("snapshots");

        if (updateMode) {
            ObjectNode entry = MAPPER.createObjectNode();
            entry.put("service", "identity-service");
            entry.put("endpoint", endpoint);
            entry.put("status", status);
            if (actualRequest != null) {
                entry.set("requestBody", actualRequest);
            }
            entry.set("responseBody", actualResponse);
            snapshots.set(snapshotKey, entry);

            if (snapshotFilePath.getParent() != null) {
                Files.createDirectories(snapshotFilePath.getParent());
            }
            Files.writeString(snapshotFilePath, MAPPER.writeValueAsString(doc) + "\n");
            return;
        }

        if (!snapshots.has(snapshotKey)) {
            throw new AssertionError("Missing contract snapshot key '" + snapshotKey + "' in " + snapshotFilePath
                    + " (run with UPDATE_CONTRACT_SNAPSHOTS=1 to record)");
        }

        JsonNode expected = snapshots.get(snapshotKey);
        assertEquals("service", "identity-service", expected.path("service").asString(), snapshotKey);
        assertEquals("endpoint", endpoint, expected.path("endpoint").asString(), snapshotKey);
        if (expected.path("status").asInt() != status) {
            throw new AssertionError("HTTP status mismatch for " + snapshotKey
                    + ": expected " + expected.path("status").asInt() + ", got " + status);
        }

        Object expectedResponseShape = extractShape(expected.get("responseBody"));
        Object actualResponseShape = extractShape(actualResponse);
        if (!expectedResponseShape.equals(actualResponseShape)) {
            throw new AssertionError("Contract shape drift at " + snapshotKey + ".responseBody"
                    + "\nexpected: " + expectedResponseShape
                    + "\nactual:   " + actualResponseShape);
        }

        if (expected.has("requestBody") || actualRequest != null) {
            Object expectedRequestShape = extractShape(expected.get("requestBody"));
            Object actualRequestShape = extractShape(actualRequest);
            if (!expectedRequestShape.equals(actualRequestShape)) {
                throw new AssertionError("Contract shape drift at " + snapshotKey + ".requestBody"
                        + "\nexpected: " + expectedRequestShape
                        + "\nactual:   " + actualRequestShape);
            }
        }
    }

    private static void assertEquals(String field, String expected, String actual, String key) {
        if (!expected.equals(actual)) {
            throw new AssertionError(field + " mismatch for " + key + ": expected " + expected + ", got " + actual);
        }
    }

    /**
     * Extracts structural key/type shape. Non-empty arrays are normalized to a sorted set of
     * unique element shapes so array length changes do not trigger false contract drift.
     */
    static Object extractShape(JsonNode node) {
        if (node == null || node.isNull()) {
            return "null";
        }
        if (node.isObject()) {
            Map<String, Object> shape = new TreeMap<>();
            for (Map.Entry<String, JsonNode> field : node.properties()) {
                shape.put(field.getKey(), extractShape(field.getValue()));
            }
            return shape;
        }
        if (node.isArray()) {
            ArrayNode arr = (ArrayNode) node;
            if (arr.isEmpty()) {
                return List.of();
            }
            Map<String, Object> uniqueShapes = new TreeMap<>();
            for (JsonNode item : arr) {
                Object childShape = extractShape(item);
                String canonical = MAPPER.writeValueAsString(childShape);
                uniqueShapes.putIfAbsent(canonical, childShape);
            }
            return new ArrayList<>(uniqueShapes.values());
        }
        if (node.isString()) {
            return "string";
        }
        if (node.isNumber()) {
            return "number";
        }
        if (node.isBoolean()) {
            return "boolean";
        }
        return node.getNodeType().name();
    }
}

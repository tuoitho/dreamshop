package com.ecommerce.identity.support;

import org.springframework.boot.testcontainers.service.connection.ServiceConnection;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.annotation.DirtiesContext;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.postgresql.PostgreSQLContainer;

@Testcontainers
@SpringBootTest
@DirtiesContext(classMode = DirtiesContext.ClassMode.AFTER_CLASS)
public abstract class PostgresIntegrationTestSupport {
    private static final TestRsaKeyFiles KEYS = TestRsaKeyFiles.create(2048);

    @Container
    @ServiceConnection
    protected static final PostgreSQLContainer POSTGRES =
            new PostgreSQLContainer("postgres:16-alpine")
                    .withDatabaseName("identity_test")
                    .withUsername("identity")
                    .withPassword("identity");

    @DynamicPropertySource
    static void jwtProperties(DynamicPropertyRegistry registry) {
        registry.add("MANAGEMENT_SERVER_PORT", () -> "0");
        registry.add("identity.grpc.host", () -> "127.0.0.1");
        registry.add("identity.grpc.port", () -> "0");
        registry.add("identity.grpc.shutdown-grace", () -> "2s");
        registry.add("identity.jwt.private-key-path", () -> KEYS.privateKeyPath().toString());
        registry.add("identity.jwt.public-key-path", () -> KEYS.publicKeyPath().toString());
        registry.add("identity.jwt.key-id", () -> "identity-test-key");
        registry.add("identity.jwt.issuer", () -> "identity-test");
        registry.add("identity.jwt.audience", () -> "ecommerce-api");
        registry.add("identity.jwt.access-ttl", () -> "15m");
        registry.add("identity.session.refresh-ttl", () -> "30d");
        registry.add("identity.session.cookie-name", () -> "refresh_token");
        registry.add("identity.session.cookie-path", () -> "/api/v1/auth");
        registry.add("identity.session.admin-cookie-name", () -> "admin_refresh_token");
        registry.add("identity.session.admin-cookie-path", () -> "/api/v1/auth/admin");
        registry.add("identity.session.cookie-secure", () -> "false");
    }
}

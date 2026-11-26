package com.ecommerce.order.support;

import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.testcontainers.service.connection.ServiceConnection;
import org.springframework.test.annotation.DirtiesContext;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.postgresql.PostgreSQLContainer;
import org.testcontainers.rabbitmq.RabbitMQContainer;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
@DirtiesContext(classMode = DirtiesContext.ClassMode.AFTER_CLASS)
public abstract class PostgresRabbitIntegrationTestSupport {

    @ServiceConnection
    protected static final PostgreSQLContainer POSTGRES =
            new PostgreSQLContainer("postgres:16-alpine")
                    .withDatabaseName("order_test")
                    .withUsername("order_user")
                    .withPassword("order_pass");

    @ServiceConnection
    protected static final RabbitMQContainer RABBIT =
            new RabbitMQContainer("rabbitmq:3.13-management-alpine");

    static {
        POSTGRES.start();
        RABBIT.start();
    }

    @DynamicPropertySource
    static void orderProperties(DynamicPropertyRegistry registry) {
        registry.add("MANAGEMENT_SERVER_PORT", () -> "0");
        registry.add("order.grpc.host", () -> "127.0.0.1");
        registry.add("order.grpc.port", () -> "0");
        registry.add("order.grpc.shutdown-grace", () -> "2s");
        registry.add("order.outbox.poll-interval-ms", () -> "100");
        registry.add("order.reconciliation.interval-ms", () -> "600000");
        registry.add("order.cod.auto-confirm-interval-ms", () -> "600000");
    }
}

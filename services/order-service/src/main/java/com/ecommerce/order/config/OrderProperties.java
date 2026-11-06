package com.ecommerce.order.config;

import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "order")
public record OrderProperties(
        GrpcServer grpc,
        Clients clients,
        Duration paymentTtl,
        Duration idempotencyTtl,
        Outbox outbox,
        Reconciliation reconciliation) {

    public record GrpcServer(String host, int port, Duration shutdownGrace) {
    }

    public record Clients(
            String identityAddr,
            Duration identityTimeout,
            String catalogAddr,
            Duration catalogTimeout,
            String inventoryAddr,
            Duration inventoryTimeout,
            String paymentAddr,
            Duration paymentTimeout) {
    }

    public record Outbox(long pollIntervalMs) {
    }

    public record Reconciliation(long intervalMs, Duration pendingStockTimeout) {
    }
}

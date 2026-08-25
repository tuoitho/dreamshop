package com.ecommerce.identity.common.config;

import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties("identity.grpc")
public record IdentityGrpcProperties(String host, int port, Duration shutdownGrace) {
    public IdentityGrpcProperties {
        if (host == null || host.isBlank()) {
            throw new IllegalArgumentException("Identity gRPC host is required");
        }
        if (port < 0 || port > 65535) {
            throw new IllegalArgumentException("Identity gRPC port must be 0 through 65535");
        }
        if (shutdownGrace == null || shutdownGrace.isZero() || shutdownGrace.isNegative()) {
            throw new IllegalArgumentException("Identity gRPC shutdown grace must be positive");
        }
    }
}

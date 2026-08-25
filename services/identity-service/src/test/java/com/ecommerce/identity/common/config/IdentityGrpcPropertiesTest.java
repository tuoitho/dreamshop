package com.ecommerce.identity.common.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Duration;
import org.junit.jupiter.api.Test;

class IdentityGrpcPropertiesTest {
    @Test
    void acceptsPrivateListenerConfiguration() {
        IdentityGrpcProperties properties =
                new IdentityGrpcProperties("127.0.0.1", 50054, Duration.ofSeconds(5));

        assertThat(properties.port()).isEqualTo(50054);
    }

    @Test
    void rejectsBlankHostInvalidPortAndNonPositiveGrace() {
        assertThatThrownBy(() -> new IdentityGrpcProperties(" ", 50054, Duration.ofSeconds(5)))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> new IdentityGrpcProperties("127.0.0.1", -1, Duration.ofSeconds(5)))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> new IdentityGrpcProperties("127.0.0.1", 50054, Duration.ZERO))
                .isInstanceOf(IllegalArgumentException.class);
    }
}

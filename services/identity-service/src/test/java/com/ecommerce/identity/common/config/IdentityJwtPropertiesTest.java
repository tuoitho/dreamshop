package com.ecommerce.identity.common.config;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.file.Path;
import java.time.Duration;
import org.junit.jupiter.api.Test;

class IdentityJwtPropertiesTest {
    private static final Path PRIVATE_KEY = Path.of("private.pem");
    private static final Path PUBLIC_KEY = Path.of("public.pem");

    @Test
    void rejectsAccessTtlOutsideOneToThirtyMinutes() {
        assertThatThrownBy(() -> properties(Duration.ofSeconds(59)))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("1 through 30 minutes");
        assertThatThrownBy(() -> properties(Duration.ofMinutes(31)))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("1 through 30 minutes");
    }

    @Test
    void rejectsBlankIssuerAudienceAndKeyId() {
        assertThatThrownBy(() -> new IdentityJwtProperties(
                " ", "ecommerce-api", PRIVATE_KEY, PUBLIC_KEY, "key-1", Duration.ofMinutes(15)))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> new IdentityJwtProperties(
                "identity", " ", PRIVATE_KEY, PUBLIC_KEY, "key-1", Duration.ofMinutes(15)))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> new IdentityJwtProperties(
                "identity", "ecommerce-api", PRIVATE_KEY, PUBLIC_KEY, " ", Duration.ofMinutes(15)))
                .isInstanceOf(IllegalArgumentException.class);
    }

    private IdentityJwtProperties properties(Duration ttl) {
        return new IdentityJwtProperties(
                "http://localhost:8090", "ecommerce-api", PRIVATE_KEY, PUBLIC_KEY, "key-1", ttl);
    }
}

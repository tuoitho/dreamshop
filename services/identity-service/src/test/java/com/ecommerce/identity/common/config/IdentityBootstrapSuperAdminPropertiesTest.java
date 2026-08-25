package com.ecommerce.identity.common.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;

class IdentityBootstrapSuperAdminPropertiesTest {
    @Test
    void isDisabledWhenAllBootstrapValuesAreBlank() {
        IdentityBootstrapSuperAdminProperties properties =
                new IdentityBootstrapSuperAdminProperties(" ", null, "");

        assertThat(properties.configured()).isFalse();
    }

    @Test
    void acceptsCompleteBootstrapValuesAndTrimsTextFields() {
        IdentityBootstrapSuperAdminProperties properties =
                new IdentityBootstrapSuperAdminProperties(
                        "  root@example.test ", "SecurePassword123!", "  Root Admin  ");

        assertThat(properties.configured()).isTrue();
        assertThat(properties.email()).isEqualTo("root@example.test");
        assertThat(properties.fullName()).isEqualTo("Root Admin");
        assertThat(properties.password()).isEqualTo("SecurePassword123!");
    }

    @Test
    void rejectsPartialOrInvalidBootstrapValues() {
        assertThatThrownBy(() -> new IdentityBootstrapSuperAdminProperties(
                "root@example.test", null, "Root Admin"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("must be supplied together");
        assertThatThrownBy(() -> new IdentityBootstrapSuperAdminProperties(
                "root@example.test", "short", "Root Admin"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("12 through 128");
        assertThatThrownBy(() -> new IdentityBootstrapSuperAdminProperties(
                "not-an-email", "SecurePassword123!", "Root Admin"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("email");
    }
}

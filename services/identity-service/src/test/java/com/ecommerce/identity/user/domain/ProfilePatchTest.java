package com.ecommerce.identity.user.domain;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class ProfilePatchTest {
    @Test
    void keepPreservesCurrentValue() {
        ProfilePatch<String> patch = ProfilePatch.keep();

        assertThat(patch.applyTo("current")).isEqualTo("current");
    }

    @Test
    void clearNullsCurrentValue() {
        ProfilePatch<String> patch = ProfilePatch.clear();

        assertThat(patch.applyTo("current")).isNull();
    }

    @Test
    void setReplacesCurrentValue() {
        ProfilePatch<String> patch = ProfilePatch.set("new");

        assertThat(patch.applyTo("current")).isEqualTo("new");
    }
}

package com.ecommerce.identity.user.api;

import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class PastDateValidatorTest {
    private final PastDateValidator validator = new PastDateValidator(
            Clock.fixed(Instant.parse("2000-01-15T00:00:00Z"), ZoneOffset.UTC));

    @Test
    void acceptsNullAndBlankValues() {
        assertThat(validator.isValid(null, null)).isTrue();
        assertThat(validator.isValid("", null)).isTrue();
        assertThat(validator.isValid("   ", null)).isTrue();
    }

    @Test
    void acceptsDateBeforeInjectedClock() {
        assertThat(validator.isValid("1999-12-31", null)).isTrue();
    }

    @Test
    void rejectsDateAfterInjectedClock() {
        assertThat(validator.isValid("2001-01-01", null)).isFalse();
    }

    @Test
    void rejectsMalformedDate() {
        assertThat(validator.isValid("not-a-date", null)).isFalse();
    }
}

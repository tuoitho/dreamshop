package com.ecommerce.identity.user.api;

import jakarta.validation.ConstraintValidator;
import jakarta.validation.ConstraintValidatorContext;
import java.time.Clock;
import java.time.LocalDate;
import java.time.format.DateTimeParseException;
import org.springframework.stereotype.Component;

@Component
public class PastDateValidator implements ConstraintValidator<PastDate, String> {
    private final Clock clock;

    public PastDateValidator(Clock clock) {
        this.clock = clock;
    }

    @Override
    public boolean isValid(String value, ConstraintValidatorContext context) {
        if (value == null || value.isBlank()) {
            return true;
        }
        try {
            return LocalDate.parse(value.trim()).isBefore(LocalDate.now(clock));
        } catch (DateTimeParseException exception) {
            return false;
        }
    }
}
